package ux

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

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
	GraphModePhEffect
	GraphModePh
)

type Graph struct {
	simulation      *s.Simulation
	mode            GraphMode
	graphImage      *ebiten.Image
	currentBarCount int
}

func NewGraph(sim *s.Simulation) *Graph {
	return &Graph{
		simulation: sim,
	}
}

func (g *Graph) SetMode(mode GraphMode) {
	if mode != g.mode {
		g.mode = mode
		g.graphImage = nil
		g.currentBarCount = 0
	}
}

func (g *Graph) Render() *ebiten.Image {
	if g.simulation.IsPaused() {
		return g.graphImage
	}

	if !g.shouldUpdate() {
		return g.graphImage
	}

	switch g.mode {
	case GraphModePhEffect:
		g.graphImage = g.renderPhEffect()
	case GraphModePh:
		g.graphImage = g.renderPh()
	default:
		g.graphImage = g.renderPopulation()
	}
	return g.graphImage
}

func (g *Graph) shouldUpdate() bool {
	if g.graphImage == nil {
		return true
	}
	if g.simulation.Cycle()%c.PopulationUpdateInterval() != 0 {
		return false
	}
	return 1+(g.simulation.Cycle()/c.PopulationUpdateInterval()) > g.currentBarCount
}

// renderPopulation draws the population stacked area chart colored by ancestor
func (g *Graph) renderPopulation() *ebiten.Image {
	fmt.Printf("\nrenderPopulation")
	g.currentBarCount = 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())

	maxPop := g.getMaxPopulation()
	if maxPop < 1 {
		maxPop = 1
	}
	heightPerPop := realGraphHeight / float64(maxPop)
	barWidth := realGraphWidth / float64(g.currentBarCount)

	img := ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	src := whiteSrc()

	populationMap := g.simulation.GetHistory()
	ancestorColorMap := g.simulation.GetAncestorColors()
	sortedAncestorIDs := g.simulation.GetAncestorsSorted()

	var vertices []ebiten.Vertex
	var indices []uint16

	for barIdx := 0; barIdx < g.currentBarCount; barIdx++ {
		cycle := barIdx * c.PopulationUpdateInterval()
		prevPops := populationMap[cycle-c.PopulationUpdateInterval()]
		newPops := populationMap[cycle]

		xLeft := float32(barIdx) * float32(barWidth)
		xRight := float32(barIdx+1) * float32(barWidth)

		prevBottom := float32(realGraphHeight)
		newBottom := float32(realGraphHeight)

		for _, id := range sortedAncestorIDs {
			prevY1, prevY2 := prevBottom, prevBottom
			newY1, newY2 := newBottom, newBottom

			prevPop, foundPrev := prevPops[id]
			if foundPrev {
				prevBottom -= float32(prevPop) * float32(heightPerPop)
				prevY2 = prevBottom
			}
			newPop, foundNew := newPops[id]
			if foundNew {
				newBottom -= float32(newPop) * float32(heightPerPop)
				newY2 = newBottom
			}
			if !foundPrev && !foundNew {
				continue
			}

			cr, cg, cb, ca := colorToFloat(ancestorColorMap[id])
			flushAndAppendQuad(&vertices, &indices, img, src,
				xLeft, prevY1, prevY2, xRight, newY1, newY2,
				cr, cg, cb, ca)
		}
	}

	if len(vertices) > 0 {
		img.DrawTriangles(vertices, indices, src, nil)
	}
	return img
}

// renderPhEffect draws a stacked area chart of organisms bucketed by phEffect
func (g *Graph) renderPhEffect() *ebiten.Image {
	fmt.Printf("\nrenderPhEffect")
	g.currentBarCount = 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())

	maxPop := g.getMaxOrganismCount()
	if maxPop < 1 {
		maxPop = 1
	}
	heightPerOrg := realGraphHeight / float64(maxPop)
	barWidth := realGraphWidth / float64(g.currentBarCount)
	numBuckets := 10

	img := ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	src := whiteSrc()

	phEffectMap := g.simulation.GetPhEffectHistory()

	var vertices []ebiten.Vertex
	var indices []uint16

	for barIdx := 0; barIdx < g.currentBarCount; barIdx++ {
		cycle := barIdx * c.PopulationUpdateInterval()
		prevDist := phEffectMap[cycle-c.PopulationUpdateInterval()]
		newDist := phEffectMap[cycle]

		xLeft := float32(barIdx) * float32(barWidth)
		xRight := float32(barIdx+1) * float32(barWidth)

		prevBottom := float32(realGraphHeight)
		newBottom := float32(realGraphHeight)

		for bucket := 0; bucket < numBuckets; bucket++ {
			prevY1, prevY2 := prevBottom, prevBottom
			newY1, newY2 := newBottom, newBottom

			prevCount := prevDist[bucket]
			if prevCount > 0 {
				prevBottom -= float32(prevCount) * float32(heightPerOrg)
				prevY2 = prevBottom
			}
			newCount := newDist[bucket]
			if newCount > 0 {
				newBottom -= float32(newCount) * float32(heightPerOrg)
				newY2 = newBottom
			}
			if prevCount == 0 && newCount == 0 {
				continue
			}

			// Color by the midpoint phEffect of this bucket
			// bucket 0 = most negative effect, bucket 9 = most positive
			spectrumValue := (float64(bucket) + 0.5) / float64(numBuckets)
			cr, cg, cb, ca := colorToFloat(PhEffectColor(spectrumValue))
			flushAndAppendQuad(&vertices, &indices, img, src,
				xLeft, prevY1, prevY2, xRight, newY1, newY2,
				cr, cg, cb, ca)
		}
	}

	if len(vertices) > 0 {
		img.DrawTriangles(vertices, indices, src, nil)
	}
	return img
}

// renderPh draws a stacked area chart of pH distribution across grid cells
func (g *Graph) renderPh() *ebiten.Image {
	fmt.Printf("\nrenderPh")
	g.currentBarCount = 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())

	totalCells := c.GridUnitsWide() * c.GridUnitsHigh()
	if totalCells < 1 {
		totalCells = 1
	}
	heightPerCell := realGraphHeight / float64(totalCells)
	barWidth := realGraphWidth / float64(g.currentBarCount)
	phBucketWidth := 0.5
	numBuckets := int(c.MaxPh() / phBucketWidth)

	img := ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	src := whiteSrc()

	phDistMap := g.simulation.GetPhDistributionHistory()

	var vertices []ebiten.Vertex
	var indices []uint16

	for barIdx := 0; barIdx < g.currentBarCount; barIdx++ {
		cycle := barIdx * c.PopulationUpdateInterval()
		prevCycle := cycle - c.PopulationUpdateInterval()
		prevDist := phDistMap[prevCycle]
		newDist := phDistMap[cycle]

		xLeft := float32(barIdx) * float32(barWidth)
		xRight := float32(barIdx+1) * float32(barWidth)

		prevBottom := float32(realGraphHeight)
		newBottom := float32(realGraphHeight)

		for bucket := 0; bucket < numBuckets; bucket++ {
			prevY1, prevY2 := prevBottom, prevBottom
			newY1, newY2 := newBottom, newBottom

			prevCount := prevDist[bucket]
			if prevCount > 0 {
				prevBottom -= float32(prevCount) * float32(heightPerCell)
				prevY2 = prevBottom
			}
			newCount := newDist[bucket]
			if newCount > 0 {
				newBottom -= float32(newCount) * float32(heightPerCell)
				newY2 = newBottom
			}
			if prevCount == 0 && newCount == 0 {
				continue
			}

			// Color by the midpoint pH of this bucket
			phMid := (float64(bucket) + 0.5) * phBucketWidth
			cr, cg, cb, ca := phValueColor(phMid)
			flushAndAppendQuad(&vertices, &indices, img, src,
				xLeft, prevY1, prevY2, xRight, newY1, newY2,
				cr, cg, cb, ca)
		}
	}

	if len(vertices) > 0 {
		img.DrawTriangles(vertices, indices, src, nil)
	}

	// Overlay a thin line showing average pH across the grid, derived from
	// the bucket distribution. Top of graph = MinPh, bottom = MaxPh.
	// Each bar segment connects prev cycle avg (left) to current cycle avg (right),
	// matching how the stacked area bands are drawn.
	var lineVerts []ebiten.Vertex
	var lineIdx []uint16
	lineHalf := float32(1.5) // half-thickness in pixels

	avgPhY := func(dist map[int]int32) (float32, bool) {
		totalCount := float64(0)
		weightedSum := float64(0)
		for bucket, count := range dist {
			mid := (float64(bucket) + 0.5) * phBucketWidth
			weightedSum += mid * float64(count)
			totalCount += float64(count)
		}
		if totalCount == 0 {
			return 0, false
		}
		avgPh := weightedSum / totalCount
		y := float32((avgPh - c.MinPh()) / (c.MaxPh() - c.MinPh()) * realGraphHeight)
		return y, true
	}

	for barIdx := 0; barIdx < g.currentBarCount; barIdx++ {
		cycle := barIdx * c.PopulationUpdateInterval()
		prevCycle := cycle - c.PopulationUpdateInterval()

		leftY, hasLeft := avgPhY(phDistMap[prevCycle])
		rightY, hasRight := avgPhY(phDistMap[cycle])
		if !hasLeft && !hasRight {
			continue
		}
		if !hasLeft {
			leftY = rightY
		}
		if !hasRight {
			rightY = leftY
		}

		xLeft := float32(barIdx) * float32(barWidth)
		xRight := float32(barIdx+1) * float32(barWidth)

		if len(lineVerts) >= 65532 {
			img.DrawTriangles(lineVerts, lineIdx, src, nil)
			lineVerts = lineVerts[:0]
			lineIdx = lineIdx[:0]
		}

		base := uint16(len(lineVerts))
		lineVerts = append(lineVerts,
			colorVertex(xLeft, leftY-lineHalf, 1, 1, 1, 1),
			colorVertex(xLeft, leftY+lineHalf, 1, 1, 1, 1),
			colorVertex(xRight, rightY-lineHalf, 1, 1, 1, 1),
			colorVertex(xRight, rightY+lineHalf, 1, 1, 1, 1),
		)
		lineIdx = append(lineIdx,
			base, base+1, base+2,
			base+2, base+1, base+3,
		)
	}

	if len(lineVerts) > 0 {
		img.DrawTriangles(lineVerts, lineIdx, src, nil)
	}

	return img
}

// --- helpers ---

func whiteSrc() *ebiten.Image {
	whiteImg := ebiten.NewImage(1, 1)
	whiteImg.Fill(color.White)
	return whiteImg.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image)
}

func colorToFloat(clr color.Color) (float32, float32, float32, float32) {
	r, g, b, a := clr.RGBA()
	return float32(r) / 65535, float32(g) / 65535, float32(b) / 65535, float32(a) / 65535
}

// phValueColor maps a pH value to a color using the same spectrum as the grid
// pH rendering (green for acid, pink for base, dark for neutral)
func phValueColor(ph float64) (float32, float32, float32, float32) {
	hue := phMaxHue - (phMaxHue * ph / c.MaxPh())
	sat := math.Abs(ph-((c.MaxPh()+c.MinPh())/2.0)) / (c.MaxPh() - c.MinPh())
	light := 0.5 + (0.5 * math.Sin(math.Pi*(sat-0.5)))
	col := colorful.HSLuv(hue, sat, light)
	return colorToFloat(col)
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

// getMaxOrganismCount returns the max total organism count across all cycles
// (sum of all buckets in the phEffect distribution)
func (g *Graph) getMaxOrganismCount() int {
	phEffectMap := g.simulation.GetPhEffectHistory()
	maxTotal := int32(0)
	for cycle := 0; cycle <= g.simulation.Cycle(); cycle += c.PopulationUpdateInterval() {
		dist := phEffectMap[cycle]
		total := int32(0)
		for _, count := range dist {
			total += count
		}
		if total > maxTotal {
			maxTotal = total
		}
	}
	return int(maxTotal)
}

func (g *Graph) getMaxPopulation() int {
	maxTotal := int32(0)
	for cycle := 0; cycle <= g.simulation.Cycle(); cycle += c.PopulationUpdateInterval() {
		total := g.getPopulationByCycle(cycle)
		if total > maxTotal {
			maxTotal = total
		}
	}
	return int(maxTotal)
}

func (g *Graph) getPopulationByCycle(cycle int) int32 {
	populationMap := g.simulation.GetHistory()
	populationAtCycle, ok := populationMap[cycle]
	if !ok {
		return 0
	}

	total := int32(0)
	for _, familyPopulation := range populationAtCycle {
		total += familyPopulation
	}
	return total
}
