package ux

import (
	c "github.com/Zebbeni/protozoa/constants"
	s "github.com/Zebbeni/protozoa/simulation"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	"math"
)

const (
	barWidthPx  = 1.0 //
	orgHeightPx = 0.5 // height of bar to draw per-organism
)

type Graph struct {
	MouseHandler
	simulation *s.Simulation
	graphImage *ebiten.Image
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
	var image *ebiten.Image
	if g.shouldRefresh() {
		image = g.renderAll()
	} else if g.shouldAddBar() {
		image = g.renderNewBar()
	} else {
		return g.graphImage
	}

	g.graphImage, _ = ebiten.NewImage(image.Bounds().Dx(), image.Bounds().Dy(), ebiten.FilterDefault)
	g.graphImage.DrawImage(image, nil)

	return image
}

func (g *Graph) renderAll() *ebiten.Image {
	width := g.simulation.Cycle() / c.PopulationUpdateInterval
	height := g.getMaxPopulation()
	image, _ := ebiten.NewImage(width, height, ebiten.FilterDefault)

	for cycle := 0; cycle <= g.simulation.Cycle(); cycle += c.PopulationUpdateInterval {
		barImage := g.renderGraphBar(cycle)
		options := &ebiten.DrawImageOptions{}
		xOffset := float64(cycle/c.PopulationUpdateInterval) * barWidthPx
		yOffset := float64(height - barImage.Bounds().Dy())
		options.GeoM.Translate(xOffset, yOffset)
		image.DrawImage(barImage, options)
	}

	return image
}

func (g *Graph) renderNewBar() *ebiten.Image {
	cycle := g.simulation.Cycle()
	barImage := g.renderGraphBar(cycle)

	newWidth := g.graphImage.Bounds().Dx() + barWidthPx
	newHeight := int(math.Max(float64(g.graphImage.Bounds().Dy()), float64(barImage.Bounds().Dy())))
	image, _ := ebiten.NewImage(newWidth, newHeight, ebiten.FilterDefault)

	originalOptions := &ebiten.DrawImageOptions{}
	originalOptions.GeoM.Translate(0, float64(newHeight-g.graphImage.Bounds().Dy()))
	image.DrawImage(g.graphImage, originalOptions)

	newBarOptions := &ebiten.DrawImageOptions{}
	xOffset := float64(newWidth - barWidthPx)
	yOffset := float64(newHeight - barImage.Bounds().Dy())
	newBarOptions.GeoM.Translate(xOffset, yOffset)
	image.DrawImage(barImage, newBarOptions)

	return image
}

// draw and return an image exactly the size of the requested bar
func (g *Graph) renderGraphBar(cycle int) *ebiten.Image {
	populationMap := g.simulation.GetHistory()
	ancestorColorMap := g.simulation.GetAncestorColors()
	sortedAncestorIDs := g.simulation.GetAncestorsSorted()

	familyPopulations, found := populationMap[cycle]
	if !found {
		barImage, _ := ebiten.NewImage(barWidthPx, 1, ebiten.FilterDefault)
		return barImage
	}

	total := int16(0)
	for _, familyPopulation := range familyPopulations {
		total += familyPopulation
	}
	barHeightPx := int(math.Ceil(float64(total) * orgHeightPx))
	barImage, _ := ebiten.NewImage(barWidthPx, barHeightPx, ebiten.FilterDefault)

	bottom := float64(barHeightPx)
	for _, id := range sortedAncestorIDs {
		if population, found := familyPopulations[id]; found {
			popHeight := float64(population) * orgHeightPx
			bottom -= popHeight
			ebitenutil.DrawRect(barImage, 0, bottom, barWidthPx, popHeight, ancestorColorMap[id])
		}
	}

	return barImage
}

func (g *Graph) getMaxPopulation() int {
	maxTotal := int16(0)
	for cycle := 0; cycle <= g.simulation.Cycle(); cycle += c.PopulationUpdateInterval {
		total := g.getPopulationByCycle(cycle)
		if total > maxTotal {
			maxTotal = total
		}
	}
	return int(maxTotal)
}

func (g *Graph) getPopulationByCycle(cycle int) int16 {
	populationMap := g.simulation.GetHistory()
	populationAtCycle, ok := populationMap[cycle]
	if !ok {
		return 0
	}

	total := int16(0)
	for _, familyPopulation := range populationAtCycle {
		total += familyPopulation
	}
	return total
}

func (g *Graph) shouldRefresh() bool {
	return g.graphImage == nil
}

func (g *Graph) shouldAddBar() bool {
	return g.simulation.Cycle()%c.PopulationUpdateInterval == 0
}
