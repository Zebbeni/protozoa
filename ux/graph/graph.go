package graph

import (
	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	s "github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/ux/graph/ph"
	"github.com/Zebbeni/protozoa/ux/graph/pheffect"
	"github.com/Zebbeni/protozoa/ux/graph/population"
)

const selectedAncestorGenerations = 3

// Mode determines which data set the graph displays
type Mode int

const (
	ModePopulation Mode = iota
	ModePopulationPhEffect
	ModePhEffect
	ModePh
)

// Renderer renders a single graph mode. Each implementation owns its own
// cache state and render logic.
type Renderer interface {
	Render(sim *s.Simulation, oldBarCount, newBarCount int) *ebiten.Image
	Reset()
}

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
}

type renderResult struct {
	images    map[Mode]*ebiten.Image
	selImages map[Mode]*ebiten.Image
	barCount  int
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
	}

	g.renderers[ModePopulation] = population.NewRenderer(population.TraitColor, nil, 0)
	g.renderers[ModePopulationPhEffect] = population.NewRenderer(population.PhEffectNodeColor, nil, 0)
	g.renderers[ModePhEffect] = pheffect.NewRenderer()
	g.renderers[ModePh] = ph.NewRenderer()

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

// Mode returns the currently-displayed graph mode.
// Invalidate clears every cached renderer image so the next Render
// call rebuilds from scratch. Used when an external setting (e.g. the
// pH colour scheme) shifts the colour mapping for already-painted
// bars.
func (g *Graph) Invalidate() {
	for _, r := range g.renderers {
		r.Reset()
	}
	for _, r := range g.selRenderers {
		r.Reset()
	}
	for k := range g.images {
		delete(g.images, k)
	}
	for k := range g.selImages {
		delete(g.selImages, k)
	}
	g.currentBarCount = 0
	g.selBarCount = 0
}

func (g *Graph) Mode() Mode {
	return g.mode
}

func (g *Graph) Render() *ebiten.Image {
	selectionChanged := g.updateSelection()

	select {
	case result := <-g.pendingResult:
		g.images = result.images
		g.currentBarCount = result.barCount
		if result.selImages != nil {
			g.selImages = result.selImages
			g.selBarCount = result.barCount
		}
		g.rendering = false
	default:
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
		g.rendering = true
		hasSelection := g.selectedSubTreeRoot != nil
		go g.renderInBackground(g.renderers, g.selRenderers, hasSelection, 0, targetBarCount, 0)
		return g.currentImage()
	}

	if g.simulation.IsPaused() {
		if selectionChanged && !g.rendering {
			g.rendering = true
			barCount := g.currentBarCount
			selRenderers := g.selRenderers
			hasSelection := g.selectedSubTreeRoot != nil
			go g.renderSelectedOnly(selRenderers, hasSelection, barCount)
		}
		return g.currentImage()
	}

	if g.shouldUpdate() && !g.rendering {
		g.rendering = true
		newBarCount := 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())
		oldBarCount := g.currentBarCount
		selOldBarCount := g.selBarCount
		// Capture renderer references to avoid racing with selection changes
		renderers := g.renderers
		selRenderers := g.selRenderers
		hasSelection := g.selectedSubTreeRoot != nil
		go g.renderInBackground(renderers, selRenderers, hasSelection, oldBarCount, newBarCount, selOldBarCount)
	}

	return g.currentImage()
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

			g.selRenderers[ModePopulation] = population.NewRenderer(population.TraitColor, root, startBar)
			g.selRenderers[ModePopulationPhEffect] = population.NewRenderer(population.PhEffectNodeColor, root, startBar)
			g.selRenderers[ModePhEffect] = population.NewRenderer(population.PhEffectNodeColor, root, startBar)
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
	oldBarCount, newBarCount, selOldBarCount int,
) {
	images := make(map[Mode]*ebiten.Image)
	for mode, renderer := range renderers {
		images[mode] = renderer.Render(g.simulation, oldBarCount, newBarCount)
	}

	result := renderResult{
		images:   images,
		barCount: newBarCount,
	}

	if hasSelection {
		selImages := make(map[Mode]*ebiten.Image)
		for mode, renderer := range selRenderers {
			selImages[mode] = renderer.Render(g.simulation, selOldBarCount, newBarCount)
		}
		result.selImages = selImages
	}

	g.pendingResult <- result
}

func (g *Graph) renderSelectedOnly(selRenderers map[Mode]Renderer, hasSelection bool, barCount int) {
	result := renderResult{
		images:   g.images,
		barCount: barCount,
	}

	if hasSelection {
		selImages := make(map[Mode]*ebiten.Image)
		for mode, renderer := range selRenderers {
			renderer.Reset()
			selImages[mode] = renderer.Render(g.simulation, 0, barCount)
		}
		result.selImages = selImages
	}

	g.pendingResult <- result
}
