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

type Mode int

const (
	ModePopulation Mode = iota
	ModePh
	ModeFood
	ModeWalls
	ModeBuriedFood
)

type Renderer interface {
	Render(sim *s.Simulation, oldBarCount, newBarCount int, progress *gh.Progress) *ebiten.Image
	Reset()
}

const progressDelay = 250 * time.Millisecond

// Graph coordinates async rendering of multiple Renderer implementations, mode selection.
type Graph struct {
	simulation *s.Simulation
	mode       Mode

	// showSelected controls which image currentImage returns when an organism is selected.
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

	// selectionDirty means the selection changed and no render has covered it yet.
	selectionDirty bool

	pendingResult chan renderResult
	rendering     bool

	// progress tracks the in-flight render's work, and renderStarted when it began; see RenderProgress.
	progress      *gh.Progress
	renderStarted time.Time
	shownProgress float64
	now           func() time.Time

	// popColor is how the population graphs colour each organism: lineage colour, or gray→green by one ability.
	popColor PopulationColor
	// renderGen increments whenever cached renders become invalid — a population colour change or an Invalidate.
	renderGen int
	// forceRender requests a full repaint on the next frame, whether or not the sim is paused.
	forceRender bool
	// popOnlyRender limits the forced repaint to the population graphs, carrying the pH, food and wall images over untouched.
	popOnlyRender bool

	// aliveCache is shared by the population renderers of every colouring.
	aliveCache *population.AliveCache

	// popCache keeps the whole rendered state of each colouring the user has visited.
	popCache map[PopulationColor]popRender

	// renderingPopColor is the colouring whose renderers were handed to the in-flight render, meaningful only.
	renderingPopColor PopulationColor

	// prewarming is set while the in-flight render is speculative work.
	prewarming bool
	// prewarmColor is the colouring that speculative render is drawing, so a
	// switch to it can take the work instead of discarding it.
	prewarmColor PopulationColor
	// adoptPrewarm marks that in-flight prewarm as the render the user is now
	// waiting on: its result is installed rather than filed away.
	adoptPrewarm bool

	// prewarmOrder is the colourings to draw ahead of being asked for, in the order the buttons offer them.
	prewarmOrder []PopulationColor

	// pendingPrewarm carries the in-flight speculative render's renderers from the goroutine back to the main one.
	pendingPrewarm atomic.Pointer[prewarmRenderers]
}

// popRender is one colouring's cached population state.
type popRender struct {
	renderer    Renderer
	selRenderer Renderer
	image       *ebiten.Image
	selImage    *ebiten.Image
	barCount    int
}

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
	// prewarm names the colouring this result was drawn for when it is speculative work.
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
	g.renderers[ModeBuriedFood] = count.NewRenderer(manager.HistoryBuriedFood, count.BuriedFoodColor)

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
	// A speculative render is already drawing exactly this, so take it rather
	// than bumping renderGen — which would drop that result when it lands and
	// queue an identical render behind it, making the user wait twice.
	if g.rendering && g.prewarming && g.prewarmColor == pc {
		g.adoptPrewarm = true
		return
	}
	// Nothing to come back to, so draw it.
	g.discardCachedRenders(true)
}

// stashPopulationRender files the colouring being left under its own key, so returning to it costs nothing.
func (g *Graph) stashPopulationRender() {
	if g.popRenderInFlight(g.popColor) {
		return
	}
	r, ok := g.renderers[ModePopulation]
	if !ok || r == nil {
		return
	}
	if g.popCache == nil {
		// A Graph built as a literal rather than through NewGraph.
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

// canReusePopRender reports whether the cached entry for pc is safe to install.
func (g *Graph) canReusePopRender(pc PopulationColor) bool {
	return !g.popRenderInFlight(pc)
}

// popRenderInFlight reports whether a goroutine is writing to the renderers
// belonging to pc, which is the only reason to withhold them.
//
// A PREWARM never is: it builds its own renderers and hands those to the
// goroutine, so nothing on display is being written to. beginRender records
// the displayed colouring either way, which made a prewarm look like a real
// render of whatever was on screen — so switching colouring while one ran
// refused to cache the colouring being left, and returning to it redrew from
// scratch a moment after it had been in hand.
func (g *Graph) popRenderInFlight(pc PopulationColor) bool {
	return g.rendering && !g.prewarming && g.renderingPopColor == pc
}

// installPopulationRender puts a cached colouring back on screen.
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
	// An in-flight render still carries the old colouring's result.
	g.renderGen++
}

// discardCachedRenders throws away every cached population render and schedules a full repaint, without touching anything a background render may be using.
func (g *Graph) discardCachedRenders(popOnly bool) {
	g.renderGen++
	// Whatever was in flight is stale now, adopted or not.
	g.adoptPrewarm = false
	g.replacePopulationRenderers()
	g.forceRender = true
	g.popOnlyRender = popOnly
}

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

// SetShowSelected toggles whether the currently-rendered graph shows the selected organism's sub-tree (true) or the overall sim (false).
func (g *Graph) SetShowSelected(show bool) {
	g.showSelected = show
}

func (g *Graph) ShowSelected() bool {
	return g.showSelected
}

// Invalidate discards every cached graph render so the next frame rebuilds from scratch.
func (g *Graph) Invalidate() {
	g.dropPopCache()
	g.discardCachedRenders(false)
}

// dropPopCache forgets every colouring's cached render.
func (g *Graph) dropPopCache() {
	g.popCache = make(map[PopulationColor]popRender)
}

// InvalidateTrees discards every cached render AND the cached alive sets.
func (g *Graph) InvalidateTrees() {
	g.aliveCache.Invalidate()
	g.dropPopCache()
	g.discardCachedRenders(false)
}

// prewarmColours is every colouring the buttons can select, in the order they appear.
func prewarmColours() []PopulationColor {
	out := []PopulationColor{{}}
	for _, a := range physiology.AllAbilities {
		out = append(out, PopulationColor{ByAbility: true, Ability: a})
	}
	return out
}

// nextPrewarm is the first colouring with nothing cached, or ok=false when every one has been drawn.
func (g *Graph) nextPrewarm() (PopulationColor, bool) {
	for _, pc := range g.prewarmOrder {
		if _, done := g.popCache[pc]; !done && pc != g.popColor {
			return pc, true
		}
	}
	return PopulationColor{}, false
}

// startPrewarm draws one un-cached colouring in the background so that switching to it later costs nothing.
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
	g.prewarmColor = pc
	g.adoptPrewarm = false
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

// prewarmRenderers hands the goroutine's renderers back for filing.
func (g *Graph) prewarmRenderers(pc PopulationColor, renderer, selRenderer Renderer) {
	g.pendingPrewarm.Store(&prewarmRenderers{pc: pc, renderer: renderer, selRenderer: selRenderer})
}

// filePrewarm puts a finished speculative render into the cache.
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

// installPrewarm puts an adopted speculative render on screen, using the
// renderers the goroutine built rather than the ones on display.
//
// Shares installPopulationRender so an adopted prewarm and a cache hit end in
// exactly the same state; the only difference between them is where the
// renderers came from.
func (g *Graph) installPrewarm(result renderResult) {
	held := g.pendingPrewarm.Load()
	g.pendingPrewarm.Store(nil)
	if held == nil || held.pc != result.prewarm {
		// The renderers did not come back, so there is nothing to install;
		// fall back to drawing it.
		g.discardCachedRenders(true)
		return
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
	g.installPopulationRender(entry)
	g.currentBarCount = result.barCount
}

// prewarmRenderers carries a speculative render's renderers back to the main goroutine alongside its images.
type prewarmRenderers struct {
	pc          PopulationColor
	renderer    Renderer
	selRenderer Renderer
}

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
			// Adopted: the user switched to this colouring while it was being
			// drawn speculatively, so it is the render they are waiting on
			// and goes on screen rather than into the cache.
			if g.adoptPrewarm && result.prewarm == g.popColor && result.renderGen == g.renderGen {
				g.adoptPrewarm = false
				g.installPrewarm(result)
				break
			}
			g.filePrewarm(result)
			break
		}
		// A render that started before the last colour change painted the old colours.
		if result.renderGen == g.renderGen {
			g.images = result.images
			g.currentBarCount = result.barCount
			if g.prewarmOrder == nil {
				// Only now, so speculative work can never be the reason the first real graph is late.
				g.prewarmOrder = prewarmColours()
			}
			// selectionDirty still set means the selection changed after this render was scheduled.
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

	// Backward seek: the sim is now at a cycle earlier than what our cached images and per-renderer caches depict.
	targetBarCount := g.targetBarCount()
	seekedBack := g.images[ModePopulation] != nil && targetBarCount < g.currentBarCount
	if seekedBack && !g.rendering {
		// A backward seek restores the descendant trees from a snapshot.
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

	// Last, and only when everything above declined.
	g.startPrewarm()

	return g.currentImage()
}

// needsSelectionRender reports whether the selected organism's sub-tree graph has to be drawn now.
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

// beginRender marks a background render as in flight and returns the progress tracker to hand it.
func (g *Graph) beginRender() *gh.Progress {
	g.rendering = true
	// Whose renderers this render is about to be handed.
	g.renderingPopColor = g.popColor
	g.progress = &gh.Progress{}
	g.renderStarted = g.now()
	g.shownProgress = 0
	return g.progress
}

// RenderProgress reports whether the panel should replace the graph with a progress bar.
func (g *Graph) RenderProgress() (fraction float64, show bool) {
	// A prewarm is speculative: blanking the user's graph for a bar showing work they didn't ask for would be worse than the wait it is saving them.
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

// RenderedEndCycle is the cycle the graph image currently on show runs up to, or -1 before the first render.
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

// heightScaler is implemented by renderers whose y-axis is scaled to the whole run.
type heightScaler interface {
	HeightFraction(throughBar int) float64
}

// HeightFraction is how much of the current graph image's height holds data up to throughBar.
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

// carried, when non-nil, supplies the images for modes that aren't being re-rendered.
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

// images is passed in rather than read off g inside the goroutine.
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
