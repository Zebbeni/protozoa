package graph

import (
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
}

func NewGraph(sim *s.Simulation) *Graph {
	g := &Graph{
		simulation:    sim,
		selectedID:    -1,
		pendingResult: make(chan renderResult, 1),
		renderers:     make(map[Mode]Renderer),
		images:        make(map[Mode]*ebiten.Image),
		selRenderers:  make(map[Mode]Renderer),
		selImages:     make(map[Mode]*ebiten.Image),
		now:           time.Now,
	}

	g.renderers[ModePopulation] = population.NewRenderer(g.popColor.colorFn(), nil, 0)
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
	g.popColor = pc
	g.discardCachedRenders()
}

// discardCachedRenders throws away every cached population render and
// schedules a full repaint, without touching anything a background
// render may be using.
//
// Bumping renderGen makes an in-flight render's result stale, so it is
// dropped on arrival rather than restoring images of the old state.
func (g *Graph) discardCachedRenders() {
	g.renderGen++
	g.replacePopulationRenderers()
	g.forceRender = true
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
	renderers[ModePopulation] = population.NewRenderer(g.popColor.colorFn(), nil, 0)
	g.renderers = renderers

	selRenderers := make(map[Mode]Renderer, len(g.selRenderers))
	for mode, r := range g.selRenderers {
		selRenderers[mode] = r
	}
	if g.selectedSubTreeRoot != nil {
		startBar := g.selStartCycle / c.PopulationUpdateInterval()
		selRenderers[ModePopulation] = population.NewRenderer(g.popColor.colorFn(), g.selectedSubTreeRoot, startBar)
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
	g.discardCachedRenders()
}

// Mode returns the currently-displayed graph mode.
func (g *Graph) Mode() Mode {
	return g.mode
}

func (g *Graph) Render() *ebiten.Image {
	selectionChanged := g.updateSelection()

	select {
	case result := <-g.pendingResult:
		g.rendering = false
		g.progress = nil
		// A render that started before the last colour change painted
		// the old colours; keep showing the previous images until the
		// forced repaint lands rather than flashing stale ones.
		if result.renderGen == g.renderGen {
			g.images = result.images
			g.currentBarCount = result.barCount
			if result.selImages != nil {
				g.selImages = result.selImages
				g.selBarCount = result.barCount
			}
		}
	default:
	}

	if g.forceRender && !g.rendering {
		g.forceRender = false
		progress := g.beginRender()
		barCount := 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())
		hasSelection := g.selectedSubTreeRoot != nil
		go g.renderInBackground(g.renderers, g.selRenderers, hasSelection, 0, barCount, 0, g.renderGen, progress)
		return g.currentImage()
	}

	// Backward seek: the sim is now at a cycle earlier than what our cached
	// images and per-renderer caches depict. Neither the paused branch nor
	// shouldUpdate() will catch this (both assume monotonic forward motion),
	// so handle it explicitly here. At this point g.rendering is false, so
	// calling Reset() on the renderers is safe.
	targetBarCount := 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())
	seekedBack := g.images[ModePopulation] != nil && targetBarCount < g.currentBarCount
	if seekedBack && !g.rendering {
		for _, r := range g.renderers {
			r.Reset()
		}
		for _, r := range g.selRenderers {
			r.Reset()
		}
		progress := g.beginRender()
		hasSelection := g.selectedSubTreeRoot != nil
		go g.renderInBackground(g.renderers, g.selRenderers, hasSelection, 0, targetBarCount, 0, g.renderGen, progress)
		return g.currentImage()
	}

	if g.simulation.IsPaused() {
		if selectionChanged && !g.rendering {
			progress := g.beginRender()
			barCount := g.currentBarCount
			selRenderers := g.selRenderers
			hasSelection := g.selectedSubTreeRoot != nil
			go g.renderSelectedOnly(selRenderers, hasSelection, barCount, g.renderGen, progress)
		}
		return g.currentImage()
	}

	if g.shouldUpdate() && !g.rendering {
		progress := g.beginRender()
		newBarCount := 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())
		oldBarCount := g.currentBarCount
		selOldBarCount := g.selBarCount
		// Capture renderer references to avoid racing with selection changes
		renderers := g.renderers
		selRenderers := g.selRenderers
		hasSelection := g.selectedSubTreeRoot != nil
		go g.renderInBackground(renderers, selRenderers, hasSelection, oldBarCount, newBarCount, selOldBarCount, g.renderGen, progress)
	}

	return g.currentImage()
}

// beginRender marks a background render as in flight and returns the
// progress tracker to hand it.
func (g *Graph) beginRender() *gh.Progress {
	g.rendering = true
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

func (g *Graph) updateSelection() bool {
	selID := g.simulation.GetSelected()
	if selID == g.selectedID {
		return false
	}
	g.selectedID = selID
	g.selectedSubTreeRoot = nil
	g.selStartCycle = 0
	g.selBarCount = 0
	g.selImages = make(map[Mode]*ebiten.Image)
	// Don't Reset() old renderers — a goroutine may still be using them.
	// Just drop the references and create new ones.
	g.selRenderers = make(map[Mode]Renderer)

	if selID >= 0 {
		if node := g.simulation.GetOrganismTreeNode(selID); node != nil {
			root := node.AncestorAtGeneration(selectedAncestorGenerations)
			g.selectedSubTreeRoot = root
			g.selStartCycle = root.StartCycle
			startBar := g.selStartCycle / c.PopulationUpdateInterval()

			g.selRenderers[ModePopulation] = population.NewRenderer(g.popColor.colorFn(), root, startBar)
		}
	}
	return true
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
	if g.simulation.Cycle()%c.PopulationUpdateInterval() != 0 {
		return false
	}
	return 1+(g.simulation.Cycle()/c.PopulationUpdateInterval()) > g.currentBarCount
}

func (g *Graph) renderInBackground(
	renderers map[Mode]Renderer, selRenderers map[Mode]Renderer, hasSelection bool,
	oldBarCount, newBarCount, selOldBarCount, renderGen int, progress *gh.Progress,
) {
	images := make(map[Mode]*ebiten.Image)
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

func (g *Graph) renderSelectedOnly(selRenderers map[Mode]Renderer, hasSelection bool, barCount, renderGen int, progress *gh.Progress) {
	result := renderResult{
		images:    g.images,
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
