package ux

import (
	"github.com/hajimehoshi/ebiten/v2"

	s "github.com/Zebbeni/protozoa/simulation"
)

const (
	realGraphWidth  = 1000.0
	realGraphHeight = 1000.0
)

type Graph struct {
	simulation *s.Simulation
	graphImage *ebiten.Image

	maxTotalPopulation int
}

func NewGraph(sim *s.Simulation) *Graph {
	return &Graph{
		simulation: sim,
		graphImage: nil,
	}
}

// TODO: Maintain a large, previously-drawn graph image and draw it to a new one that
// is slightly wider when we need to re-render, so we can just draw the newest
// population bar instead of re-rendering the full history. We can scale it down
// to whatever dimensions we need when we return it.
func (g *Graph) Render() *ebiten.Image {
	//if g.simulation.IsPaused() {
	//	return g.graphImage
	//}
	return ebiten.NewImage(realGraphWidth, realGraphHeight)
	//
	//var img *ebiten.Image
	//if g.shouldRefresh() {
	//	img = g.renderAll()
	//} else if g.shouldAddBar() {
	//	img = g.renderNewBar()
	//} else {
	//	return g.graphImage
	//}
	//
	//g.graphImage = ebiten.NewImage(realGraphWidth, realGraphHeight)
	//g.graphImage.DrawImage(img, nil)
	//
	//return img
}

//func (g *Graph) renderAll() *ebiten.Image {
//	// add 1 to make sure cycle 0 gives us a bar count of 1
//	barCount := 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())
//	barWidth := realGraphWidth / float64(barCount)
//	g.maxTotalPopulation = g.getMaxPopulation()
//	img := ebiten.NewImage(realGraphWidth, realGraphHeight)
//
//	for cycle := 0; cycle <= g.simulation.Cycle(); cycle += c.PopulationUpdateInterval() {
//		barImage, graphBarPopulation := g.renderGraphBar(cycle)
//		options := &ebiten.DrawImageOptions{}
//		scaleX := barWidth / float64(barImage.Bounds().Dx())
//		scaleY := float64(graphBarPopulation) / float64(g.maxTotalPopulation)
//		xOffset := float64(cycle/c.PopulationUpdateInterval()) * barWidth
//		yOffset := realGraphHeight - (float64(barImage.Bounds().Dy()) * scaleY)
//		options.GeoM.Scale(scaleX, scaleY)
//		options.GeoM.Translate(xOffset, yOffset)
//		img.DrawImage(barImage, options)
//	}
//
//	return img
//}
//
//func (g *Graph) renderNewBar() *ebiten.Image {
//	barCount := 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())
//	barWidth := realGraphWidth / float64(barCount)
//	barImage, graphBarPopulation := g.renderGraphBar(g.simulation.Cycle())
//
//	img := ebiten.NewImage(realGraphWidth, realGraphHeight)
//
//	originalOptions := &ebiten.DrawImageOptions{}
//	xScaleOriginal := (float64(barCount) - 1) / float64(barCount)
//	yScaleOriginal := 1.0
//	if graphBarPopulation > g.maxTotalPopulation {
//		yScaleOriginal = float64(g.maxTotalPopulation) / float64(graphBarPopulation)
//		g.maxTotalPopulation = graphBarPopulation
//	}
//	originalOptions.GeoM.Scale(xScaleOriginal, yScaleOriginal)
//	xOffsetOriginal := 0.0
//	yOffsetOriginal := realGraphHeight - float64(g.graphImage.Bounds().Dy())*yScaleOriginal
//	originalOptions.GeoM.Translate(xOffsetOriginal, yOffsetOriginal)
//	img.DrawImage(g.graphImage, originalOptions)
//
//	newBarOptions := &ebiten.DrawImageOptions{}
//	xScaleNewBar := barWidth / float64(barImage.Bounds().Dx())
//	yScaleNewBar := 1.0
//	if graphBarPopulation < g.maxTotalPopulation {
//		yScaleNewBar = float64(graphBarPopulation) / float64(g.maxTotalPopulation)
//	}
//	xOffsetNewBar := realGraphWidth - barWidth
//	yOffsetNewBar := realGraphHeight - float64(barImage.Bounds().Dy())*yScaleNewBar
//	newBarOptions.GeoM.Scale(xScaleNewBar, yScaleNewBar)
//	newBarOptions.GeoM.Translate(xOffsetNewBar, yOffsetNewBar)
//
//	img.DrawImage(barImage, newBarOptions)
//
//	return img
//}
//
//// draw and return an image of the stacked graph bar for a single cycle
//// also return the number of
//func (g *Graph) renderGraphBar(cycle int) (*ebiten.Image, int) {
//	barCount := 1 + (g.simulation.Cycle() / c.PopulationUpdateInterval())
//	realBarWidth := realGraphWidth / barCount
//
//	ancestorColorMap := g.simulation.GetAncestorColors()
//	sortedAncestorIDs := g.simulation.GetAncestorsSorted()
//
//	previousFamilyPopulations := populationMap[cycle-c.PopulationUpdateInterval()]
//	prevTotal := getTotalPopulation(previousFamilyPopulations)
//	newFamilyPopulations := populationMap[cycle]
//	newTotal := getTotalPopulation(newFamilyPopulations)
//
//	maxTotal := math.Max(float64(newTotal), float64(prevTotal))
//	heightPerPop := realGraphHeight / maxTotal
//
//	barImage := ebiten.NewImage(realBarWidth, realGraphHeight)
//
//	newBottom, prevBottom := float32(realGraphHeight), float32(realGraphHeight)
//	for _, id := range sortedAncestorIDs {
//		prevX1, prevY1, prevX2, prevY2 := float32(0), prevBottom, float32(0), prevBottom
//		newX1, newY1, newX2, newY2 := float32(realBarWidth), newBottom, float32(realBarWidth), newBottom
//
//		prevPopulation, foundPrevious := previousFamilyPopulations[id]
//		if foundPrevious {
//			popHeight := float32(prevPopulation) * float32(heightPerPop)
//			prevBottom -= popHeight
//			prevY2 = prevBottom
//		}
//		newPopulation, foundNew := newFamilyPopulations[id]
//		if foundNew {
//			popHeight := float32(newPopulation) * float32(heightPerPop)
//			newBottom -= popHeight
//			newY2 = newBottom
//		}
//		if foundPrevious == false && foundNew == false {
//			continue
//		}
//
//		prevV1 := createVertex(prevX1, prevY1)
//		prevV2 := createVertex(prevX2, prevY2)
//		newV1 := createVertex(newX1, newY1)
//		newV2 := createVertex(newX2, newY2)
//		vertexes := make([]ebiten.Vertex, 0, 6)
//
//		emptyImage := ebiten.NewImage(1, 1)
//		emptyImage.Fill(ancestorColorMap[id])
//
//		src := emptyImage.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image)
//
//		vertexes = append(vertexes, prevV1, prevV2, newV1, newV2)
//		indices := []uint16{0, 1, 2, 2, 1, 3}
//
//		barImage.DrawTriangles(vertexes, indices, src, nil)
//	}
//
//	return barImage, int(maxTotal)
//}
//
//func createVertex(x, y float32) ebiten.Vertex {
//	return ebiten.Vertex{
//		DstX:   x,
//		DstY:   y,
//		SrcX:   0,
//		SrcY:   0,
//		ColorR: 1.0,
//		ColorG: 1.0,
//		ColorB: 1.0,
//		ColorA: 1.0,
//	}
//}
//
//func getTotalPopulation(populationMap map[int]int16) int {
//	total := int16(0)
//	for _, population := range populationMap {
//		total += population
//	}
//	return int(total)
//}
//
//func (g *Graph) shouldRefresh() bool {
//	return g.graphImage == nil
//}
//
//func (g *Graph) shouldAddBar() bool {
//	return g.simulation.Cycle()%c.PopulationUpdateInterval() == 0
//}
