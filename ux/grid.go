package ux

import (
	c "github.com/Zebbeni/protozoa/constants"
	"github.com/Zebbeni/protozoa/decisions"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	s "github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	"image/color"
)

type size int

const (
	sizeSmall size = iota
	sizeMedium
	sizeLarge
)

var (
	gridBackground = color.RGBA{15, 5, 15, 255}
)

type Grid struct {
	simulation        *s.Simulation
	previousGridImage *ebiten.Image
	selectedID        int
}

func NewGrid(simulation *s.Simulation) *Grid {
	return &Grid{
		simulation:        simulation,
		previousGridImage: nil,
		selectedID:        -1,
	}
}

// Render draws all organisms and food on the simulation grid
func (g *Grid) Render() *ebiten.Image {
	gridImage, _ := ebiten.NewImage(c.GridWidth, c.GridHeight, ebiten.FilterDefault)
	ebitenutil.DrawRect(gridImage, 0, 0, float64(c.GridWidth), float64(c.GridHeight), gridBackground)

	mostReproductiveID := g.simulation.GetMostReproductiveID()
	// Come up with a better way to trigger a refresh than this
	if g.shouldRefresh() {
		foodItems := g.simulation.GetFoodItems()
		organismInfo := g.simulation.GetAllOrganismInfo()

		ebitenutil.DrawRect(gridImage, 0, 0, c.GridWidth, c.GridHeight, gridBackground)
		for _, foodItem := range foodItems {
			renderFood(foodItem, gridImage)
		}
		for _, info := range organismInfo {
			renderOrganism(info, gridImage, mostReproductiveID)
		}
	} else {
		gridImage.DrawImage(g.previousGridImage, nil)
		for _, point := range g.simulation.UpdatedPoints {
			// paint background over grid square to update first
			x, y := float64(point.X)*c.GridUnitSize, float64(point.Y)*c.GridUnitSize
			ebitenutil.DrawRect(gridImage, x, y, c.GridUnitSize, c.GridUnitSize, gridBackground)
			if item := g.simulation.GetFoodAtPoint(point); item != nil {
				renderFood(item, gridImage)
				continue
			}
			if info := g.simulation.GetOrganismInfoAtPoint(point); info != nil {
				renderOrganism(info, gridImage, mostReproductiveID)
				continue
			}
		}
	}

	g.previousGridImage, _ = ebiten.NewImage(c.GridWidth, c.GridHeight, ebiten.FilterDefault)
	g.previousGridImage.DrawImage(gridImage, nil)

	if info := g.simulation.GetOrganismInfoByID(g.selectedID); info != nil {
		selectionBox, _ := ebiten.NewImage(c.GridWidth, c.GridHeight, ebiten.FilterDefault)
		renderSelection(info.Location, selectionBox, info.Color)
		gridImage.DrawImage(selectionBox, nil)
	}

	g.simulation.ClearUpdatedGridPoints()

	return gridImage
}

func (g *Grid) shouldRefresh() bool {
	return len(g.simulation.UpdatedPoints) == 0
}

// renderSelection draws a square around a single item on the grid
func renderSelection(point utils.Point, img *ebiten.Image, col color.Color) {
	x, y := float64(point.X*c.GridUnitSize), float64(point.Y*c.GridUnitSize)
	ebitenutil.DrawLine(img, x-2, y-2, x+c.GridUnitSize+3, y-2, col)                               // top
	ebitenutil.DrawLine(img, x-2, y-2, x-2, y+c.GridUnitSize+3, col)                               // left
	ebitenutil.DrawLine(img, x-2, y+c.GridUnitSize+3, x+c.GridUnitSize+3, y+c.GridUnitSize+3, col) // bottom
	ebitenutil.DrawLine(img, x+c.GridUnitSize+3, y-2, x+c.GridUnitSize+3, y+c.GridUnitSize+3, col) // right
}

// renderFood draws a food source to the given image
func renderFood(item *food.Item, img *ebiten.Image) {
	x := float64(item.Point.X) * c.GridUnitSize
	y := float64(item.Point.Y) * c.GridUnitSize
	alpha := 60
	foodColor := color.RGBA{100, 255, 100, uint8(alpha)}

	value := float64(item.Value)
	foodSize := sizeSmall
	if value < c.MaxFoodValue*0.4375 {
		foodSize = sizeSmall
	} else if value < c.MaxFoodValue*0.8125 {
		foodSize = sizeMedium
	} else {
		foodSize = sizeLarge
	}

	drawSquare(img, x, y, foodSize, foodColor)
}

// renderOrganism draws a food source to the given image
func renderOrganism(info *organism.Info, img *ebiten.Image, mostReproductiveID int) {
	point := info.Location.Times(c.GridUnitSize)
	x, y := float64(point.X), float64(point.Y)

	organismSize := sizeSmall
	if info.Size < c.MaximumMaxSize*0.4375 {
		organismSize = sizeSmall
	} else if info.Size < c.MaximumMaxSize*0.8125 {
		organismSize = sizeMedium
	} else {
		organismSize = sizeLarge
	}

	organismColor := info.Color
	if info.Action == decisions.ActAttack {
		organismColor = color.White
	}

	drawSquare(img, x, y, organismSize, organismColor)
}

func drawSquare(screen *ebiten.Image, x, y float64, sz size, col color.Color) {
	padding := 1.5
	switch sz {
	case sizeSmall:
		padding = 1.5
		break
	case sizeMedium:
		padding = 1.0
		break
	case sizeLarge:
		padding = 0.5
		break
	}

	ebitenutil.DrawRect(screen, x+padding, y+padding, c.GridUnitSize-(2*padding), c.GridUnitSize-(2*padding), col)
}
