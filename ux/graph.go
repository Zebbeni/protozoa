package ux

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	s "github.com/Zebbeni/protozoa/simulation"
)

const (
	realGraphWidth  = 1000.0
	realGraphHeight = 1000.0

	selectedAncestorGenerations = 3
)

// GraphMode determines which data set the graph displays
type GraphMode int

const (
	GraphModePopulation GraphMode = iota
	GraphModePopulationPhEffect
	GraphModePhEffect
	GraphModePh
)

type Graph struct {
	simulation *s.Simulation
	mode       GraphMode

	// Cached images for each graph mode (always kept up to date)
	popImage            *ebiten.Image
	popPhEffectImage    *ebiten.Image
	phEffectBucketImage *ebiten.Image
	phImage             *ebiten.Image
	currentBarCount     int

	// Full population graph base image caches
	popBaseImage *ebiten.Image
	popBaseWidth int
	popMaxAlive  int

	popPhEffectBaseImage *ebiten.Image
	popPhEffectBaseWidth int
	popPhEffectMaxAlive  int

	// Selected sub-tree population graph state
	selectedID          int
	selectedSubTreeRoot *organism.DescendantNode
	selPopImage         *ebiten.Image
	selPopPhEffImage    *ebiten.Image
	selBarCount         int

	selStartCycle int // cycle when the sub-tree root was born

	selPopBaseImage      *ebiten.Image
	selPopBaseWidth      int
	selPopMaxAlive       int
	selPopPhEffBaseImage *ebiten.Image
	selPopPhEffBaseWidth int
	selPopPhEffMaxAlive  int

	lastAvgPh float64 // last recorded average pH, for display

	// Async rendering
	pendingResult chan renderResult
	rendering     bool
}

type renderResult struct {
	popImage            *ebiten.Image
	popPhEffectImage    *ebiten.Image
	phEffectBucketImage *ebiten.Image
	phImage             *ebiten.Image
	barCount            int

	selPopImage      *ebiten.Image
	selPopPhEffImage *ebiten.Image
	selBarCount      int
}

func NewGraph(sim *s.Simulation) *Graph {
	return &Graph{
		simulation:    sim,
		selectedID:    -1,
		pendingResult: make(chan renderResult, 1),
	}
}

// SelectedStartCycle returns the start cycle of the selected sub-tree, or -1.
func (g *Graph) SelectedStartCycle() int {
	if g.selectedSubTreeRoot == nil {
		return -1
	}
	return g.selStartCycle
}

// HasSelection returns true if a sub-tree is being displayed.
func (g *Graph) HasSelection() bool {
	return g.selectedSubTreeRoot != nil
}

// LastAvgPh returns the most recent average pH value, or -1 if unavailable.
func (g *Graph) LastAvgPh() float64 {
	return g.lastAvgPh
}

func (g *Graph) SetMode(mode GraphMode) {
	g.mode = mode
}

func (g *Graph) Render() *ebiten.Image {
	// Always detect selection changes (even when paused)
	selectionChanged := g.updateSelection()

	// Check for completed async result
	select {
	case result := <-g.pendingResult:
		g.popImage = result.popImage
		g.popPhEffectImage = result.popPhEffectImage
		g.phEffectBucketImage = result.phEffectBucketImage
		g.phImage = result.phImage
		g.currentBarCount = result.barCount
		if result.selPopImage != nil {
			g.selPopImage = result.selPopImage
			g.selPopPhEffImage = result.selPopPhEffImage
			g.selBarCount = result.selBarCount
		}
		g.rendering = false
	default:
	}

	if g.simulation.IsPaused() {
		// When paused, only re-render if the selection just changed
		if selectionChanged && !g.rendering {
			g.rendering = true
			barCount := g.currentBarCount
			go g.renderSelectedInBackground(barCount)
		}
		return g.currentImage()
	}

	// Start async render if needed and not already in progress
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
	// Reset selected graph state
	g.selectedSubTreeRoot = nil
	g.selStartCycle = 0
	g.selPopImage = nil
	g.selPopPhEffImage = nil
	g.selBarCount = 0
	g.selPopBaseImage = nil
	g.selPopBaseWidth = 0
	g.selPopMaxAlive = 0
	g.selPopPhEffBaseImage = nil
	g.selPopPhEffBaseWidth = 0
	g.selPopPhEffMaxAlive = 0

	if selID >= 0 {
		if node := g.simulation.GetOrganismTreeNode(selID); node != nil {
			root := node.AncestorAtGeneration(selectedAncestorGenerations)
			g.selectedSubTreeRoot = root
			g.selStartCycle = root.StartCycle
		}
	}
	return true
}

func (g *Graph) currentImage() *ebiten.Image {
	// Use selected sub-tree graphs for population modes when a selection is active
	if g.selectedSubTreeRoot != nil {
		switch g.mode {
		case GraphModePopulation:
			if g.selPopImage != nil {
				return g.selPopImage
			}
		case GraphModePopulationPhEffect:
			if g.selPopPhEffImage != nil {
				return g.selPopPhEffImage
			}
		}
	}

	switch g.mode {
	case GraphModePopulationPhEffect:
		return g.popPhEffectImage
	case GraphModePhEffect:
		return g.phEffectBucketImage
	case GraphModePh:
		return g.phImage
	default:
		return g.popImage
	}
}

func (g *Graph) shouldUpdate() bool {
	if g.popImage == nil {
		return true
	}
	if g.simulation.Cycle()%c.PopulationUpdateInterval() != 0 {
		return false
	}
	return 1+(g.simulation.Cycle()/c.PopulationUpdateInterval()) > g.currentBarCount
}

func (g *Graph) renderAllInBackground(oldBarCount, newBarCount, selOldBarCount int) {
	popImg := g.renderPopulation(oldBarCount, newBarCount)
	popPhEffectImg := g.renderPopulationPhEffect(oldBarCount, newBarCount)
	phEffectBucketImg := g.renderPhEffect(newBarCount)
	phImg := g.renderPh(newBarCount)

	result := renderResult{
		popImage:            popImg,
		popPhEffectImage:    popPhEffectImg,
		phEffectBucketImage: phEffectBucketImg,
		phImage:             phImg,
		barCount:            newBarCount,
	}

	// Render selected sub-tree graphs if a selection is active
	if g.selectedSubTreeRoot != nil {
		result.selPopImage = g.renderSelectedPopulation(selOldBarCount, newBarCount)
		result.selPopPhEffImage = g.renderSelectedPopulationPhEffect(selOldBarCount, newBarCount)
		result.selBarCount = newBarCount
	}

	g.pendingResult <- result
}

// renderSelectedInBackground renders only the sub-tree graphs (used when paused
// and the selection changes, since the full graphs don't need updating).
func (g *Graph) renderSelectedInBackground(barCount int) {
	result := renderResult{
		popImage:            g.popImage,
		popPhEffectImage:    g.popPhEffectImage,
		phEffectBucketImage: g.phEffectBucketImage,
		phImage:             g.phImage,
		barCount:            barCount,
	}

	if g.selectedSubTreeRoot != nil {
		result.selPopImage = g.renderSelectedPopulation(0, barCount)
		result.selPopPhEffImage = g.renderSelectedPopulationPhEffect(0, barCount)
		result.selBarCount = barCount
	}

	g.pendingResult <- result
}

// --- shared helpers ---

func whiteSrc() *ebiten.Image {
	whiteImg := ebiten.NewImage(1, 1)
	whiteImg.Fill(color.White)
	return whiteImg.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image)
}

func colorToFloat(clr color.Color) (float32, float32, float32, float32) {
	r, g, b, a := clr.RGBA()
	return float32(r) / 65535, float32(g) / 65535, float32(b) / 65535, float32(a) / 65535
}

func flushAndAppendQuad(vertices *[]ebiten.Vertex, indices *[]uint16,
	img *ebiten.Image, src *ebiten.Image,
	xLeft, prevY1, prevY2, xRight, newY1, newY2 float32,
	cr, cg, cb, ca float32) {

	if len(*vertices) >= 65532 {
		img.DrawTriangles(*vertices, *indices, src, nil)
		*vertices = (*vertices)[:0]
		*indices = (*indices)[:0]
	}

	base := uint16(len(*vertices))
	*vertices = append(*vertices,
		colorVertex(xLeft, prevY1, cr, cg, cb, ca),
		colorVertex(xLeft, prevY2, cr, cg, cb, ca),
		colorVertex(xRight, newY1, cr, cg, cb, ca),
		colorVertex(xRight, newY2, cr, cg, cb, ca),
	)
	*indices = append(*indices,
		base, base+1, base+2,
		base+2, base+1, base+3,
	)
}

func colorVertex(x, y, r, g, b, a float32) ebiten.Vertex {
	return ebiten.Vertex{
		DstX:   x,
		DstY:   y,
		SrcX:   0,
		SrcY:   0,
		ColorR: r,
		ColorG: g,
		ColorB: b,
		ColorA: a,
	}
}
