package ux

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	s "github.com/Zebbeni/protozoa/simulation"
)

const (
	realGraphWidth  = 1000.0
	realGraphHeight = 1000.0
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
	currentBarCount int

	// Population graph base image caches (1px per bar, fixed height)
	popBaseImage *ebiten.Image
	popBaseWidth int
	popMaxAlive  int

	popPhEffectBaseImage *ebiten.Image
	popPhEffectBaseWidth int
	popPhEffectMaxAlive  int

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
}

func NewGraph(sim *s.Simulation) *Graph {
	return &Graph{
		simulation:    sim,
		pendingResult: make(chan renderResult, 1),
	}
}

// LastAvgPh returns the most recent average pH value, or -1 if unavailable.
func (g *Graph) LastAvgPh() float64 {
	return g.lastAvgPh
}

func (g *Graph) SetMode(mode GraphMode) {
	g.mode = mode
}

func (g *Graph) Render() *ebiten.Image {
	if g.simulation.IsPaused() {
		return g.currentImage()
	}

	// Check for completed async result
	select {
	case result := <-g.pendingResult:
		g.popImage = result.popImage
		g.popPhEffectImage = result.popPhEffectImage
		g.phEffectBucketImage = result.phEffectBucketImage
		g.phImage = result.phImage
		g.currentBarCount = result.barCount
		g.rendering = false
	default:
	}

	// Start async render if needed and not already in progress
	if g.shouldUpdate() && !g.rendering {
		g.rendering = true
		newBarCount := 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())
		oldBarCount := g.currentBarCount
		go g.renderAllInBackground(oldBarCount, newBarCount)
	}

	return g.currentImage()
}

func (g *Graph) currentImage() *ebiten.Image {
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

func (g *Graph) renderAllInBackground(oldBarCount, newBarCount int) {
	popImg := g.renderPopulation(oldBarCount, newBarCount)
	popPhEffectImg := g.renderPopulationPhEffect(oldBarCount, newBarCount)
	phEffectBucketImg := g.renderPhEffect(newBarCount)
	phImg := g.renderPh(newBarCount)

	g.pendingResult <- renderResult{
		popImage:            popImg,
		popPhEffectImage:    popPhEffectImg,
		phEffectBucketImage: phEffectBucketImg,
		phImage:             phImg,
		barCount:            newBarCount,
	}
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
