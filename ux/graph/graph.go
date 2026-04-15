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

	if g.simulation.IsPaused() {
		if selectionChanged && !g.rendering {
			g.rendering = true
			barCount := g.currentBarCount
			go g.renderSelectedInBackground(barCount)
		}
		return g.currentImage()
	}

	if g.shouldUpdate() && !g.rendering {
		g.rendering = true
		newBarCount := 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())
		oldBarCount := g.currentBarCount
		selOldBarCount := g.selBarCount
		go g.renderAllInBackground(oldBarCount, newBarCount, selOldBarCount)
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

	for _, r := range g.selRenderers {
		r.Reset()
	}
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
	if g.selectedSubTreeRoot != nil {
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

func (g *Graph) renderAllInBackground(oldBarCount, newBarCount, selOldBarCount int) {
	images := make(map[Mode]*ebiten.Image)
	for mode, renderer := range g.renderers {
		images[mode] = renderer.Render(g.simulation, oldBarCount, newBarCount)
	}

	result := renderResult{
		images:   images,
		barCount: newBarCount,
	}

	if g.selectedSubTreeRoot != nil {
		selImages := make(map[Mode]*ebiten.Image)
		for mode, renderer := range g.selRenderers {
			selImages[mode] = renderer.Render(g.simulation, selOldBarCount, newBarCount)
		}
		result.selImages = selImages
	}

	g.pendingResult <- result
}

func (g *Graph) renderSelectedInBackground(barCount int) {
	result := renderResult{
		images:   g.images,
		barCount: barCount,
	}

	if g.selectedSubTreeRoot != nil {
		selImages := make(map[Mode]*ebiten.Image)
		for mode, renderer := range g.selRenderers {
			renderer.Reset()
			selImages[mode] = renderer.Render(g.simulation, 0, barCount)
		}
		result.selImages = selImages
	}

	g.pendingResult <- result
}
