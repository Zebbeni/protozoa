package graph

import (
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	s "github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/ux/graph/count"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
	"github.com/Zebbeni/protozoa/ux/graph/ph"
	"github.com/Zebbeni/protozoa/ux/graph/population"
)

const selectedAncestorGenerations = 3

// Mode determines which data set the graph displays
type Mode int

const (
	ModePopulation Mode = iota
	ModePh
	ModeFood
	ModeWalls
)

// Renderer renders a single graph mode. Each implementation owns its own
// cache state and render logic.
//
// Renderers are handed to a background goroutine by value-of-map, so the
// main goroutine must never mutate one a render may be holding: Reset()
// is only safe when no render is in flight (g.rendering is false). To
// discard a renderer's cache while a render might be running, replace it
// with a fresh instance in a NEW map instead — see replacePopulationRenderers.
//
// Render may report its work through progress so the panel can show a
// progress bar during slow renders; renderers that are always fast can
// ignore it. progress is never shared between two renders.
type Renderer interface {
	Render(sim *s.Simulation, oldBarCount, newBarCount int, progress *gh.Progress) *ebiten.Image
	Reset()
}

// progressDelay is how long a render must have been running before the
// panel replaces the graph with a progress bar. Routine incremental
// renders finish well inside it, so the bar only appears for genuinely
// large rebuilds instead of flickering on every update.
const progressDelay = 250 * time.Millisecond

// Graph coordinates async rendering of multiple Renderer implementations,
// mode selection, and organism selection state.
type Graph struct {
	simulation *s.Simulation
	mode       Mode

	// showSelected controls which image currentImage returns when an
	// organism is selected. False (default) → always show the
	// overall image; True → show the selection's sub-tree image when
	// available, falling back to overall otherwise. Toggled by the
	// panel's graph-mode buttons so "Population (all)" vs
	// "Population (selected)" are explicit user choices.
	showSelected bool

	renderers map[Mode]Renderer
	images    map[Mode]*ebiten.Image

	selRenderers map[Mode]Renderer
	selImages    map[Mode]*ebiten.Image

	currentBarCount int
	selBarCount     int

	selectedID          int
	selectedSubTreeRoot *organism.DescendantNode
	selStartCycle       int

	// selectionDirty means the selection changed and no render has
	// covered it yet. A flag rather than acting on updateSelection's
	// return value directly, because the change can land while a render
	// is already in flight — dropping it then left the selected graph
	// blank with nothing to schedule the render it was waiting for.
	selectionDirty bool

	pendingResult chan renderResult
	rendering     bool

	// progress tracks the in-flight render's work, and renderStarted when
	// it began; see RenderProgress. shownProgress is the highest fraction
	// reported for this render, so the bar never moves backwards when a
	// later renderer declares more work. now is time.Now, swappable in
	// tests.
	progress      *gh.Progress
	renderStarted time.Time
	shownProgress float64
	now           func() time.Time

	// popColor is how the population graphs colour each organism:
	// lineage colour, or gray→green by one ability. Applies to both the
	// overall and the selection sub-tree graph.
	popColor PopulationColor
	// renderGen increments whenever cached renders become invalid — a
	// population colour change or an Invalidate. Each background render
	// is stamped with the generation it started under, so a render still
	// painting the old state when the cache is invalidated is discarded
	// on arrival instead of overwriting the new view.
	renderGen int
	// forceRender requests a full repaint on the next frame, whether or
	// not the sim is paused. Every already painted column is stale after
	// an invalidation, and a paused sim would otherwise never repaint.
	forceRender bool
	// popOnlyRender limits the forced repaint to the population graphs,
	// carrying the pH, food and wall images over untouched. A population
	// colour change doesn't alter a single pixel of those, and
	// re-rendering them made switching colour cost more than it had to.
	popOnlyRender bool

	// aliveCache is shared by the population renderers of every
	// colouring, so switching colour redraws from cached alive sets
	// instead of walking the descendant trees again. See
	// population.AliveCache.
	aliveCache *population.AliveCache

	// popCache keeps the whole rendered state of each colouring the user
	// has visited, so switching back to one is instant.
	//
	// The alive-set cache already made the tree *walk* free, but the
	// colour switch still threw the renderer away and built a fresh one,
	// and drawing thousands of bars is seconds of work on a long run. The
	// renderer owns its base image and its incremental state, so keeping
	// it is what makes coming back free — caching only the finished image
	// would leave the next incremental update with nothing to draw onto.
	popCache map[PopulationColor]popRender

	// renderingPopColor is the colouring whose renderers were handed to
	// the in-flight render, meaningful only while rendering is true. A
	// renderer a goroutine is writing to must not be handed back out of
	// the cache, and this is how that case is recognised.
	renderingPopColor PopulationColor

	// prewarming is set while the in-flight render is speculative work.
	// It shares the rendering flag so only one render ever runs at a
	// time, but keeps the progress bar off the user's graph — nobody
	// asked for this one.
	prewarming bool

	// prewarmOrder is the colourings to draw ahead of being asked for,
	// in the order the buttons offer them. Populated once the first real
	// render has landed, so speculative work never delays the graph the
	// user is actually looking at.
	prewarmOrder []PopulationColor

	// pendingPrewarm carries the in-flight speculative render's renderers
	// from the goroutine back to the main one, which is the only place
	// popCache is touched. Atomic because the two ends are different
	// goroutines; only ever one prewarm at a time, so one slot is enough.
	pendingPrewarm atomic.Pointer[prewarmRenderers]
}

// popRender is one colouring's cached population state: the renderers
// that own the drawing, the images they last produced, and the bar count
// they reached, so a stale entry can catch up incrementally rather than
// rebuilding.
type popRender struct {
	renderer    Renderer
	selRenderer Renderer
	image       *ebiten.Image
	selImage    *ebiten.Image
	barCount    int
}

// PopulationColor selects how the population graphs colour organisms.
// The zero value is lineage colouring.
type PopulationColor struct {
	ByAbility bool
	Ability   physiology.Ability
}

func (pc PopulationColor) colorFn() population.NodeColorFunc {
	if pc.ByAbility {
		return population.AbilityColor(pc.Ability)
	}
	return population.TraitColor
}

type renderResult struct {
	images    map[Mode]*ebiten.Image
	selImages map[Mode]*ebiten.Image
	barCount  int
	renderGen int
	// prewarm names the colouring this result was drawn for when it is
	// speculative work rather than something the user asked to see. The
	// result is filed into popCache instead of being displayed.
	prewarm   PopulationColor
	isPrewarm bool
}

func NewGraph(sim *s.Simulation) *Graph {
	g := &Graph{
		simulation:    sim,
		selectedID:    -1,
		pendingResult: make(chan renderResult, 1),
		popCache:      make(map[PopulationColor]popRender),
		renderers:     make(map[Mode]Renderer),
		images:        make(map[Mode]*ebiten.Image),
		selRenderers:  make(map[Mode]Renderer),
		selImages:     make(map[Mode]*ebiten.Image),
		now:           time.Now,
	}

	g.aliveCache = population.NewAliveCache()
	g.renderers[ModePopulation] = population.NewRenderer(g.popColor.colorFn(), nil, 0, g.aliveCache)
	g.renderers[ModePh] = ph.NewRenderer()
	g.renderers[ModeFood] = count.NewRenderer(manager.HistoryFood, count.FoodColor)
	g.renderers[ModeWalls] = count.NewRenderer(manager.HistoryWalls, count.WallColor)

	return g
}

func (g *Graph) SelectedStartCycle() int {
	if g.selectedSubTreeRoot == nil {
		return -1
	}
	return g.selStartCycle
}

func (g *Graph) HasSelection() bool {
	return g.selectedSubTreeRoot != nil
}

func (g *Graph) LastAvgPh() float64 {
	if r, ok := g.renderers[ModePh].(*ph.Renderer); ok {
		return r.LastAvgPh
	}
	return -1
}

func (g *Graph) SetMode(mode Mode) {
	g.mode = mode
}

// SetPopulationColor changes how the population graphs colour organisms.
// A no-op when the setting is unchanged, so the panel can call it every
// frame.
//
// On a change, fresh renderers replace the population ones in NEW maps
// rather than being written into the existing ones: a background render
// may be ranging over the old maps right now, and writing to a map being
// iterated is a runtime panic. The old renderers are left to finish and
// be discarded, the same way a selection change retires them.
func (g *Graph) SetPopulationColor(pc PopulationColor) {
	if pc == g.popColor {
		return
	}
	g.stashPopulationRender()
	g.popColor = pc

	if entry, ok := g.popCache[pc]; ok && g.canReusePopRender(pc) {
		g.installPopulationRender(entry)
		return
	}
	// Nothing to come back to, so draw it. Only the population images are
	// stale; the alive sets behind them are the same organisms in a
	// different colour.
	g.discardCachedRenders(true)
}

// stashPopulationRender files the colouring being left under its own key,
// so returning to it costs nothing.
//
// Skipped while that colouring's renderers are in a background render:
// the goroutine is still writing to them, and filing one away would hand
// it back out later mid-write.
func (g *Graph) stashPopulationRender() {
	if g.rendering && g.renderingPopColor == g.popColor {
		return
	}
	r, ok := g.renderers[ModePopulation]
	if !ok || r == nil {
		return
	}
	if g.popCache == nil {
		// A Graph built as a literal rather than through NewGraph — the
		// tests do, and a nil map here would panic on the first colour
		// switch rather than simply not caching.
		g.popCache = make(map[PopulationColor]popRender)
	}
	g.popCache[g.popColor] = popRender{
		renderer:    r,
		selRenderer: g.selRenderers[ModePopulation],
		image:       g.images[ModePopulation],
		selImage:    g.selImages[ModePopulation],
		barCount:    g.currentBarCount,
	}
}

// canReusePopRender reports whether the cached entry for pc is safe to
// install — that is, whether a background render might be writing to it.
func (g *Graph) canReusePopRender(pc PopulationColor) bool {
	return !g.rendering || g.renderingPopColor != pc
}

// installPopulationRender puts a cached colouring back on screen.
//
// The renderers go into NEW maps rather than being written into the
// existing ones, for the same reason replacePopulationRenderers does it:
// a background render may be ranging over the map it was handed, and
// writing into that is a runtime panic.
//
// The entry's own bar count is restored along with it, so a colouring
// filed away earlier in a live run draws only the bars it missed instead
// of the whole graph. In a replay the bar count is the whole recording
// from the first frame, so an entry is never behind and nothing is
// redrawn at all — which is the case this exists for.
func (g *Graph) installPopulationRender(entry popRender) {
	renderers := make(map[Mode]Renderer, len(g.renderers))
	for mode, r := range g.renderers {
		renderers[mode] = r
	}
	renderers[ModePopulation] = entry.renderer
	g.renderers = renderers

	selRenderers := make(map[Mode]Renderer, len(g.selRenderers))
	for mode, r := range g.selRenderers {
		selRenderers[mode] = r
	}
	if entry.selRenderer != nil {
		selRenderers[ModePopulation] = entry.selRenderer
	}
	g.selRenderers = selRenderers

	images := make(map[Mode]*ebiten.Image, len(g.images))
	for mode, img := range g.images {
		images[mode] = img
	}
	if entry.image != nil {
		images[ModePopulation] = entry.image
	}
	g.images = images

	if entry.selImage != nil {
		selImages := make(map[Mode]*ebiten.Image, len(g.selImages))
		for mode, img := range g.selImages {
			selImages[mode] = img
		}
		selImages[ModePopulation] = entry.selImage
		g.selImages = selImages
	}

	g.currentBarCount = entry.barCount
	// An in-flight render still carries the old colouring's result, which
	// would land on top of what was just installed.
	g.renderGen++
}

// discardCachedRenders throws away every cached population render and
// schedules a full repaint, without touching anything a background
// render may be using.
//
// Bumping renderGen makes an in-flight render's result stale, so it is
// dropped on arrival rather than restoring images of the old state.
func (g *Graph) discardCachedRenders(popOnly bool) {
	g.renderGen++
	g.replacePopulationRenderers()
	g.forceRender = true
	g.popOnlyRender = popOnly
}

// replacePopulationRenderers installs fresh population renderers —
// overall and, when there's a selection, sub-tree — in NEW renderer maps.
//
// Both halves matter. A background render ranges over the maps it was
// given, and writing into a map another goroutine is iterating is a
// runtime panic; and it calls into the renderers it found there, so
// resetting one in place nils its image mid-draw. That second one is
// exactly the crash Invalidate used to cause: a pH colour-scheme click
// Reset() the population renderer while a background render was drawing
// its columns, and DrawImage dereferenced the now-nil base image.
//
// Only the population renderers carry cached state; the pH and food
// renderers repaint fully every time and have no-op Resets, so they are
// carried over as-is.
func (g *Graph) replacePopulationRenderers() {
	renderers := make(map[Mode]Renderer, len(g.renderers))
	for mode, r := range g.renderers {
		renderers[mode] = r
	}
	renderers[ModePopulation] = population.NewRenderer(g.popColor.colorFn(), nil, 0, g.aliveCache)
	g.renderers = renderers

	selRenderers := make(map[Mode]Renderer, len(g.selRenderers))
	for mode, r := range g.selRenderers {
		selRenderers[mode] = r
	}
	if g.selectedSubTreeRoot != nil {
		startBar := g.selStartCycle / c.PopulationUpdateInterval()
		selRenderers[ModePopulation] = population.NewRenderer(g.popColor.colorFn(), g.selectedSubTreeRoot, startBar, nil)
	}
	g.selRenderers = selRenderers
}

// SetShowSelected toggles whether the currently-rendered graph shows
// the selected organism's sub-tree (true) or the overall sim (false).
// No effect when nothing is selected — there's no sub-tree image to
// fall back to in that case, so currentImage returns the overall
// image regardless.
func (g *Graph) SetShowSelected(show bool) {
	g.showSelected = show
}

// ShowSelected reports the current setting of the all/selected toggle.
func (g *Graph) ShowSelected() bool {
	return g.showSelected
}

// Invalidate discards every cached graph render so the next frame
// rebuilds from scratch. Used when an external setting (e.g. the pH
// colour scheme) shifts the colour mapping for already-painted bars.
//
// Safe to call at any time, including while a background render is in
// flight: it replaces renderers rather than resetting them, and the
// in-flight result is discarded as stale. The previous images stay on
// screen until the repaint lands, rather than the graph going blank.
func (g *Graph) Invalidate() {
	g.dropPopCache()
	g.discardCachedRenders(false)
}

// dropPopCache forgets every colouring's cached render. Called wherever
// the data behind the graphs changes: a cached entry is the *answer* for
// a set of trees and a colour mapping, and both of those move.
//
// The entries are dropped rather than Reset() in place, because a
// background render may still hold one — the same rule replacePopulation-
// Renderers follows.
func (g *Graph) dropPopCache() {
	g.popCache = make(map[PopulationColor]popRender)
}

// InvalidateTrees discards every cached render AND the cached alive
// sets. Used when the descendant trees themselves are rebuilt — a replay
// seek restores them from a snapshot — after which a cached alive set
// holds pointers into trees that no longer exist.
func (g *Graph) InvalidateTrees() {
	g.aliveCache.Invalidate()
	g.dropPopCache()
	g.discardCachedRenders(false)
}

// prewarmColours is every colouring the buttons can select, in the order
// they appear: the lineage colours first, then one per ability.
func prewarmColours() []PopulationColor {
	out := []PopulationColor{{}}
	for _, a := range physiology.AllAbilities {
		out = append(out, PopulationColor{ByAbility: true, Ability: a})
	}
	return out
}

// nextPrewarm is the first colouring with nothing cached, or ok=false
// when every one has been drawn.
func (g *Graph) nextPrewarm() (PopulationColor, bool) {
	for _, pc := range g.prewarmOrder {
		if _, done := g.popCache[pc]; !done && pc != g.popColor {
			return pc, true
		}
	}
	return PopulationColor{}, false
}

// startPrewarm draws one un-cached colouring in the background so that
// switching to it later costs nothing.
//
// Only ever runs when the graph is otherwise idle: nothing rendering,
// nothing forced, nothing pending, and the displayed graph already up to
// date. Speculative work must never be the reason the graph the user is
// looking at is late, and it holds the same one-at-a-time flag as every
// other render so it can't overlap one.
func (g *Graph) startPrewarm() bool {
	if g.rendering || g.forceRender || g.currentBarCount <= 0 {
		return false
	}
	if g.targetBarCount() != g.currentBarCount {
		// The live graph still has bars to catch up on; that comes first.
		return false
	}
	pc, ok := g.nextPrewarm()
	if !ok {
		return false
	}

	renderer := population.NewRenderer(pc.colorFn(), nil, 0, g.aliveCache)
	var selRenderer Renderer
	if g.selectedSubTreeRoot != nil {
		startBar := g.selStartCycle / c.PopulationUpdateInterval()
		selRenderer = population.NewRenderer(pc.colorFn(), g.selectedSubTreeRoot, startBar, nil)
	}

	progress := g.beginRender()
	g.prewarming = true
	barCount := g.currentBarCount
	gen := g.renderGen
	go func() {
		result := renderResult{
			barCount:  barCount,
			renderGen: gen,
			prewarm:   pc,
			isPrewarm: true,
			images:    map[Mode]*ebiten.Image{ModePopulation: renderer.Render(g.simulation, 0, barCount, progress)},
		}
		if selRenderer != nil {
			result.selImages = map[Mode]*ebiten.Image{
				ModePopulation: selRenderer.Render(g.simulation, 0, barCount, progress),
			}
		}
		g.prewarmRenderers(pc, renderer, selRenderer)
		g.pendingResult <- result
	}()
	return true
}

// prewarmRenderers hands the goroutine's renderers back for filing. Kept
// separate from the result so the cache entry is assembled on the main
// goroutine, where popCache is only ever touched.
func (g *Graph) prewarmRenderers(pc PopulationColor, renderer, selRenderer Renderer) {
	g.pendingPrewarm.Store(&prewarmRenderers{pc: pc, renderer: renderer, selRenderer: selRenderer})
}

// filePrewarm puts a finished speculative render into the cache. Dropped
// if the data moved underneath it while it was drawing — the entry would
// describe trees that no longer exist.
func (g *Graph) filePrewarm(result renderResult) {
	held := g.pendingPrewarm.Load()
	g.pendingPrewarm.Store(nil)
	if held == nil || held.pc != result.prewarm || result.renderGen != g.renderGen {
		return
	}
	if g.popCache == nil {
		g.popCache = make(map[PopulationColor]popRender)
	}
	entry := popRender{
		renderer:    held.renderer,
		selRenderer: held.selRenderer,
		barCount:    result.barCount,
	}
	if result.images != nil {
		entry.image = result.images[ModePopulation]
	}
	if result.selImages != nil {
		entry.selImage = result.selImages[ModePopulation]
	}
	g.popCache[result.prewarm] = entry
}

// prewarmRenderers carries a speculative render's renderers back to the
// main goroutine alongside its images.
type prewarmRenderers struct {
	pc          PopulationColor
	renderer    Renderer
	selRenderer Renderer
}

// Mode returns the currently-displayed graph mode.
func (g *Graph) Mode() Mode {
	return g.mode
}

func (g *Graph) Render() *ebiten.Image {
	g.updateSelection()

	select {
	case result := <-g.pendingResult:
		g.rendering = false
		g.prewarming = false
		g.progress = nil
		if result.isPrewarm {
			g.filePrewarm(result)
			break
		}
		// A render that started before the last colour change painted
		// the old colours; keep showing the previous images until the
		// forced repaint lands rather than flashing stale ones.
		if result.renderGen == g.renderGen {
			g.images = result.images
			g.currentBarCount = result.barCount
			if g.prewarmOrder == nil {
				// Only now, so speculative work can never be the reason the
				// first real graph is late.
				g.prewarmOrder = prewarmColours()
			}
			// selectionDirty still set means the selection changed after
			// this render was scheduled, so its sub-tree images are of
			// the organism that *was* selected. Dropping them leaves the
			// graph blank for the frame or two until the render this
			// change schedules lands, which beats showing someone else's
			// family tree under the new selection's title.
			if result.selImages != nil && !g.selectionDirty {
				g.selImages = result.selImages
				g.selBarCount = result.barCount
			}
		}
	default:
	}

	if g.forceRender && !g.rendering {
		g.forceRender = false
		popOnly := g.popOnlyRender
		g.popOnlyRender = false
		progress := g.beginRender()
		barCount := g.targetBarCount()
		hasSelection := g.selectedSubTreeRoot != nil
		g.selectionDirty = false
		renderers := g.renderers
		carried := map[Mode]*ebiten.Image(nil)
		if popOnly {
			renderers = map[Mode]Renderer{ModePopulation: g.renderers[ModePopulation]}
			carried = g.images
		}
		go g.renderInBackground(renderers, g.selRenderers, hasSelection, 0, barCount, 0, g.renderGen, progress, carried)
		return g.currentImage()
	}

	// Backward seek: the sim is now at a cycle earlier than what our cached
	// images and per-renderer caches depict. Neither the paused branch nor
	// shouldUpdate() will catch this (both assume monotonic forward motion),
	// so handle it explicitly here. At this point g.rendering is false, so
	// calling Reset() on the renderers is safe.
	targetBarCount := g.targetBarCount()
	seekedBack := g.images[ModePopulation] != nil && targetBarCount < g.currentBarCount
	if seekedBack && !g.rendering {
		// A backward seek restores the descendant trees from a snapshot,
		// so cached alive sets point into trees that no longer exist.
		g.aliveCache.Invalidate()
		for _, r := range g.renderers {
			r.Reset()
		}
		for _, r := range g.selRenderers {
			r.Reset()
		}
		progress := g.beginRender()
		hasSelection := g.selectedSubTreeRoot != nil
		g.selectionDirty = false
		go g.renderInBackground(g.renderers, g.selRenderers, hasSelection, 0, targetBarCount, 0, g.renderGen, progress, nil)
		return g.currentImage()
	}

	// A sub-tree render, for a selection nothing else is going to cover.
	// This used to sit inside the paused branch, on the assumption that a
	// running sim repaints everything at the next bar boundary anyway —
	// true live, false in a replay, where the bar count is the whole
	// recording from the first frame and shouldUpdate therefore never
	// fires again. Selecting an organism mid-playback and switching to
	// the selected view showed nothing at all until some unrelated
	// setting forced a repaint.
	if g.needsSelectionRender() && !g.rendering {
		g.selectionDirty = false
		progress := g.beginRender()
		go g.renderSelectedOnly(g.images, g.selRenderers, g.selectedSubTreeRoot != nil,
			g.currentBarCount, g.renderGen, progress)
		return g.currentImage()
	}

	if g.simulation.IsPaused() {
		// A paused sim is exactly when there is time for speculative work.
		g.startPrewarm()
		return g.currentImage()
	}

	if g.shouldUpdate() && !g.rendering {
		progress := g.beginRender()
		newBarCount := g.targetBarCount()
		oldBarCount := g.currentBarCount
		selOldBarCount := g.selBarCount
		// Capture renderer references to avoid racing with selection changes
		renderers := g.renderers
		selRenderers := g.selRenderers
		hasSelection := g.selectedSubTreeRoot != nil
		g.selectionDirty = false
		go g.renderInBackground(renderers, selRenderers, hasSelection, oldBarCount, newBarCount, selOldBarCount, g.renderGen, progress, nil)
	}

	// Last, and only when everything above declined: draw a colouring
	// nobody has asked for yet, so that switching to it is instant.
	g.startPrewarm()

	return g.currentImage()
}

// needsSelectionRender reports whether the selected organism's sub-tree
// graph has to be drawn now.
//
// Two ways in. The selection changed and no render has covered it since
// (selectionDirty), or the user has switched to the selected view for a
// mode whose image isn't there — a render dropped as stale, or one that
// ran while nothing was selected. The second is deliberately limited to
// modes that *have* a sub-tree renderer: only population does, so asking
// it of pH would spin, re-rendering every frame for an image that is
// never going to appear.
//
// Nothing to draw before the first full render has set a bar count, so
// it waits — that render covers the selection itself.
func (g *Graph) needsSelectionRender() bool {
	if g.selectedSubTreeRoot == nil || g.currentBarCount <= 0 {
		return false
	}
	if g.selectionDirty {
		return true
	}
	if _, ok := g.selRenderers[g.mode]; !ok {
		return false
	}
	return g.showSelected && g.selImages[g.mode] == nil
}

// beginRender marks a background render as in flight and returns the
// progress tracker to hand it.
func (g *Graph) beginRender() *gh.Progress {
	g.rendering = true
	// Whose renderers this render is about to be handed, so the cache
	// knows not to give that one back out while it is being written to.
	g.renderingPopColor = g.popColor
	g.progress = &gh.Progress{}
	g.renderStarted = g.now()
	g.shownProgress = 0
	return g.progress
}

// RenderProgress reports whether the panel should replace the graph with
// a progress bar, and how full to draw it. It shows only once a render
// has run past progressDelay and has reported some work; the fraction is
// held monotonic so the bar never slides back.
func (g *Graph) RenderProgress() (fraction float64, show bool) {
	// A prewarm is speculative: blanking the user's graph for a bar
	// showing work they didn't ask for would be worse than the wait it
	// is saving them.
	if g.prewarming {
		return 0, false
	}
	if !g.rendering || g.now().Sub(g.renderStarted) < progressDelay {
		return 0, false
	}
	f, ok := g.progress.Fraction()
	if !ok {
		return 0, false
	}
	g.shownProgress = max(g.shownProgress, f)
	return g.shownProgress, true
}

func (g *Graph) updateSelection() {
	selID := g.simulation.GetSelected()
	if selID == g.selectedID {
		return
	}
	g.selectedID = selID
	g.selectedSubTreeRoot = nil
	g.selStartCycle = 0
	g.selBarCount = 0
	g.selImages = make(map[Mode]*ebiten.Image)
	g.selectionDirty = true
	// Don't Reset() old renderers — a goroutine may still be using them.
	// Just drop the references and create new ones.
	g.selRenderers = make(map[Mode]Renderer)

	if selID >= 0 {
		if node := g.simulation.GetOrganismTreeNode(selID); node != nil {
			root := node.AncestorAtGeneration(selectedAncestorGenerations)
			g.selectedSubTreeRoot = root
			g.selStartCycle = root.StartCycle
			startBar := g.selStartCycle / c.PopulationUpdateInterval()

			g.selRenderers[ModePopulation] = population.NewRenderer(g.popColor.colorFn(), root, startBar, nil)
		}
	}
}

// RenderedEndCycle is the cycle the graph image currently on show runs
// up to, or -1 before the first render. Renders land every few cycles, so
// this lags the simulation; anything mapping positions on the image to
// cycles should use it rather than the live cycle.
func (g *Graph) RenderedEndCycle() int {
	bars := g.currentBarCount
	if g.showSelected && g.selectedSubTreeRoot != nil && g.selImages[g.mode] != nil {
		bars = g.selBarCount
	}
	if bars <= 0 {
		return -1
	}
	return bars * c.PopulationUpdateInterval()
}

func (g *Graph) currentImage() *ebiten.Image {
	if g.showSelected && g.selectedSubTreeRoot != nil {
		if img, ok := g.selImages[g.mode]; ok && img != nil {
			return img
		}
	}
	return g.images[g.mode]
}

func (g *Graph) shouldUpdate() bool {
	if g.images[ModePopulation] == nil {
		return true
	}
	if g.endCycle() == g.simulation.Cycle() && g.simulation.Cycle()%c.PopulationUpdateInterval() != 0 {
		return false
	}
	return g.targetBarCount() > g.currentBarCount
}

// endCycle is the last cycle the graphs cover. Replaying a recording,
// that's the end of the whole recording — its family history and
// per-cycle counts are all loaded up front, so the graphs are drawn once
// for the entire run and stay on show in full, with the viewer marking
// the playhead rather than cropping to it. A live run only knows up to
// the current cycle.
func (g *Graph) endCycle() int {
	if recorded := g.simulation.RecordedEndCycle(); recorded > 0 {
		return recorded
	}
	return g.simulation.Cycle()
}

// targetBarCount is how many bars the graphs should cover.
func (g *Graph) targetBarCount() int {
	return 1 + (g.endCycle() / c.PopulationUpdateInterval())
}

// heightScaler is implemented by renderers whose y-axis is scaled to the
// whole run, so the viewer can stretch the part of the image the data has
// actually reached. See population.Renderer.HeightFraction.
type heightScaler interface {
	HeightFraction(throughBar int) float64
}

// HeightFraction is how much of the current graph image's height holds
// data up to throughBar: 1 for renderers with a fixed y-axis.
func (g *Graph) HeightFraction(throughBar int) float64 {
	renderers := g.renderers
	if g.showSelected && g.selectedSubTreeRoot != nil {
		if _, ok := g.selImages[g.mode]; ok {
			renderers = g.selRenderers
		}
	}
	if scaler, ok := renderers[g.mode].(heightScaler); ok {
		return scaler.HeightFraction(throughBar)
	}
	return 1
}

// carried, when non-nil, supplies the images for modes that aren't being
// re-rendered, so a population-only repaint keeps the pH, food and wall
// graphs it already has.
func (g *Graph) renderInBackground(
	renderers map[Mode]Renderer, selRenderers map[Mode]Renderer, hasSelection bool,
	oldBarCount, newBarCount, selOldBarCount, renderGen int, progress *gh.Progress,
	carried map[Mode]*ebiten.Image,
) {
	images := make(map[Mode]*ebiten.Image)
	for mode, img := range carried {
		images[mode] = img
	}
	for mode, renderer := range renderers {
		images[mode] = renderer.Render(g.simulation, oldBarCount, newBarCount, progress)
	}

	result := renderResult{
		images:    images,
		barCount:  newBarCount,
		renderGen: renderGen,
	}

	if hasSelection {
		selImages := make(map[Mode]*ebiten.Image)
		for mode, renderer := range selRenderers {
			selImages[mode] = renderer.Render(g.simulation, selOldBarCount, newBarCount, progress)
		}
		result.selImages = selImages
	}

	g.pendingResult <- result
}

// images is passed in rather than read off g inside the goroutine: the
// main goroutine replaces that map on every completed render, and reading
// it from here is a data race.
func (g *Graph) renderSelectedOnly(images map[Mode]*ebiten.Image, selRenderers map[Mode]Renderer, hasSelection bool, barCount, renderGen int, progress *gh.Progress) {
	result := renderResult{
		images:    images,
		barCount:  barCount,
		renderGen: renderGen,
	}

	if hasSelection {
		selImages := make(map[Mode]*ebiten.Image)
		for mode, renderer := range selRenderers {
			renderer.Reset()
			selImages[mode] = renderer.Render(g.simulation, 0, barCount, progress)
		}
		result.selImages = selImages
	}

	g.pendingResult <- result
}
