package ux

import (
	"github.com/Zebbeni/protozoa/constants"
	s "github.com/Zebbeni/protozoa/simulation"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	"image/color"
	"sort"
)

const (
	barWidth = 5
)

type Graph struct {
	MouseHandler
	simulation         *s.Simulation
	previousGraphImage *ebiten.Image

	graphWidth, graphHeight float64
	lastCycleRendered       int
}

func NewGraph(sim *s.Simulation) *Graph {
	return &Graph{
		simulation:         sim,
		previousGraphImage: nil,
		lastCycleRendered:  0,
	}
}

// TODO: Maintain a large, previously-drawn graph image and draw it to a new one that
// is slightly wider when we need to re-render, so we can just draw the newest
// population bar instead of re-rendering the full history. We can scale it down
// to whatever dimensions we need when we return it.
func (g *Graph) Render() *ebiten.Image {
	cyclesPassed := (g.simulation.Cycle() / constants.PopulationUpdateInterval) + 1
	g.graphWidth = float64(cyclesPassed * barWidth)

	var graphImage *ebiten.Image

	// redraw everything if selection has changed or we're drawing for the first time.
	if g.shouldRefresh() {
		graphImage, _ = ebiten.NewImage(graphWidth, graphHeight, ebiten.FilterDefault)
		g.renderPopulation(graphImage)
		g.renderBorders(graphImage)
	} else if g.shouldAddBar() {
		graphImage, _ = ebiten.NewImage(graphWidth, graphHeight, ebiten.FilterDefault)
		graphImage.DrawImage(g.previousGraphImage, nil)
	} else {
		return g.previousGraphImage
	}

	g.previousGraphImage, _ = ebiten.NewImage(graphWidth, graphHeight, ebiten.FilterDefault)
	g.previousGraphImage.DrawImage(graphImage, nil)

	return graphImage
}

func (g *Graph) renderBorders(graphImage *ebiten.Image) {
	left, top, right, bottom := float64(1), float64(0), float64(graphWidth), float64(graphHeight-1)
	ebitenutil.DrawLine(graphImage, left, top, right, top, color.White)
	ebitenutil.DrawLine(graphImage, right, top, right, bottom, color.White)
	ebitenutil.DrawLine(graphImage, left, bottom, right, bottom, color.White)
	ebitenutil.DrawLine(graphImage, left, top, left, bottom, color.White)
}

func (g *Graph) renderPopulation(graphImage *ebiten.Image) {
	populationMap := g.simulation.GetHistory()
	ancestorColorMap := g.simulation.GetOriginalAncestors()
	sortedAncestorIDs := sortAncestorIDs(ancestorColorMap)

	maxTotal := int16(0)
	for _, populationCycle := range populationMap {
		total := int16(0)
		for _, familyPopulation := range populationCycle {
			total += familyPopulation
		}
		if total > maxTotal {
			maxTotal = total
		}
	}
	yPixelsPerPop := float64(graphHeight) / float64(maxTotal)
	barWidth := float64(graphWidth) / float64(len(populationMap))

	left := float64(0)
	for c := 0; c <= g.simulation.Cycle(); c += constants.PopulationUpdateInterval {
		populationMap, found := populationMap[c]
		if !found {
			continue
		}

		bottom := float64(graphHeight)
		for _, id := range sortedAncestorIDs {
			if population, found := populationMap[id]; found {
				barHeight := float64(population) * yPixelsPerPop
				bottom -= barHeight
				ebitenutil.DrawRect(graphImage, left, bottom, barWidth, barHeight, ancestorColorMap[id])
			}
		}

		left += barWidth
	}

	g.lastCycleRendered = g.simulation.Cycle()
}

func sortAncestorIDs(ancestorMap map[int]color.Color) []int {
	ancestorIDs := make([]int, 0, len(ancestorMap))
	for k := range ancestorMap {
		ancestorIDs = append(ancestorIDs, k)
	}
	sort.Ints(ancestorIDs)
	return ancestorIDs
}

func (g *Graph) shouldRefresh() bool {
	return g.previousGraphImage == nil
}

func (g *Graph) shouldAddBar() bool {
	return g.simulation.Cycle()%0 == 0 && g.simulation.Cycle() > g.lastCycleRendered
}
