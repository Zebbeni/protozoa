package ux

import (
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/resources"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
)

type size int

const (
	sizeSmall size = iota
	sizeMedium
	sizeLarge
)

var (
	gridBackground                                                      = colorful.HSLuv(0, 0, 0)
	foodColor                                                           = colorful.HSLuv(120, 0.2, 0.25)
	squareImgSmall, squareImgMedium, squareImgLarge, poisonImg, wallImg *ebiten.Image
)

type Grid struct {
	simulation        *simulation.Simulation
	previousGridImage *ebiten.Image
	selectedID        int
}

func NewGrid(simulation *simulation.Simulation) *Grid {
	g := &Grid{
		simulation:        simulation,
		previousGridImage: nil,
		selectedID:        -1,
	}
	loadOrganismImages()
	return g
}

func loadOrganismImages() {
	squareImgSmall = resources.SquareSmall
	squareImgMedium = resources.SquareMedium
	squareImgLarge = resources.SquareLarge
	poisonImg = resources.Poison
	wallImg = resources.Wall
}

// Render draws all organisms and food on the simulation grid
func (g *Grid) Render() *ebiten.Image {
	gridImage := ebiten.NewImage(config.GridWidth(), config.GridHeight())
	ebitenutil.DrawRect(gridImage, 0, 0, float64(config.GridWidth()), float64(config.GridHeight()), gridBackground)

	mostReproductiveID := g.simulation.GetMostReproductiveID()
	// Come up with a better way to trigger a refresh than this
	if g.shouldRefresh() {
		foodItems := g.simulation.GetFoodItems()
		organismInfo := g.simulation.GetAllOrganismInfo()

		ebitenutil.DrawRect(gridImage, 0, 0, float64(config.GridWidth()), float64(config.GridHeight()), gridBackground)
		for _, foodItem := range foodItems {
			g.renderFood(foodItem, gridImage)
		}
		for _, info := range organismInfo {
			g.renderOrganism(info, gridImage, mostReproductiveID)
		}
	} else {
		gridImage.DrawImage(g.previousGridImage, nil)

		for _, point := range g.simulation.UpdatedPoints {
			// paint background over grid square to update first
			x, y := point.X*config.GridUnitSize(), point.Y*config.GridUnitSize()
			ebitenutil.DrawRect(gridImage, float64(x), float64(y), float64(config.GridUnitSize()), float64(config.GridUnitSize()), gridBackground)
			if item := g.simulation.GetFoodAtPoint(point); item != nil {
				g.renderFood(item, gridImage)
				continue
			}
			if info := g.simulation.GetOrganismInfoAtPoint(point); info != nil {
				g.renderOrganism(info, gridImage, mostReproductiveID)
				continue
			}
		}
	}

	g.previousGridImage = ebiten.NewImage(config.GridWidth(), config.GridHeight())
	g.previousGridImage.DrawImage(gridImage, nil)

	if info := g.simulation.GetOrganismInfoByID(g.selectedID); info != nil {
		selectionBox := ebiten.NewImage(config.GridWidth(), config.GridHeight())
		g.renderSelection(info.Location, selectionBox, info.Color)

		gridImage.DrawImage(selectionBox, nil)
	}

	g.simulation.ClearUpdatedGridPoints()

	return gridImage
}

func (g *Grid) shouldRefresh() bool {
	return len(g.simulation.UpdatedPoints) == 0 || g.simulation.Cycle() == 0
}

// renderSelection draws a square around a single item on the grid
func (g *Grid) renderSelection(point utils.Point, img *ebiten.Image, col colorful.Color) {
	x, y := float64(point.X*config.GridUnitSize()), float64(point.Y*config.GridUnitSize())
	ebitenutil.DrawLine(img, x-2, y-2, x+float64(config.GridUnitSize())+3, y-2, col)                                                               // top
	ebitenutil.DrawLine(img, x-2, y-2, x-2, y+float64(config.GridUnitSize())+3, col)                                                               // left
	ebitenutil.DrawLine(img, x-2, y+float64(config.GridUnitSize())+3, x+float64(config.GridUnitSize())+3, y+float64(config.GridUnitSize())+3, col) // bottom
	ebitenutil.DrawLine(img, x+float64(config.GridUnitSize())+3, y-2, x+float64(config.GridUnitSize())+3, y+float64(config.GridUnitSize())+3, col) // right
}

// renderFood draws a food source to the given image
func (g *Grid) renderFood(item *food.Item, img *ebiten.Image) {
	x := float64(item.Point.X) * float64(config.GridUnitSize())
	y := float64(item.Point.Y) * float64(config.GridUnitSize())

	value := float64(item.Value)
	foodSize := sizeSmall
	if value < float64(config.MaxFoodValue())*0.4375 {
		foodSize = sizeSmall
	} else if value < float64(config.MaxFoodValue())*0.8125 {
		foodSize = sizeMedium
	} else {
		foodSize = sizeLarge
	}

	g.drawSquare(img, x, y, foodSize, foodColor)
}

// renderOrganism draws a food source to the given image
func (g *Grid) renderOrganism(info *organism.Info, img *ebiten.Image, mostReproductiveID int) {
	point := info.Location.Times(config.GridUnitSize())
	x, y := float64(point.X), float64(point.Y)

	organismSize := sizeSmall
	if info.Size < config.MaximumMaxSize()*0.4375 {
		organismSize = sizeSmall
	} else if info.Size < config.MaximumMaxSize()*0.8125 {
		organismSize = sizeMedium
	} else {
		organismSize = sizeLarge
	}

	organismColor := info.Color
	if info.Action == decision.ActAttack {
		organismColor = colorful.HSLuv(0.0, 255.0, 1.0)
	}

	g.drawSquare(img, x, y, organismSize, organismColor)
}

func (g *Grid) drawSquare(screen *ebiten.Image, x, y float64, sz size, col colorful.Color) {
	var squareImg *ebiten.Image
	switch sz {
	case sizeSmall:
		squareImg = squareImgSmall
		break
	case sizeMedium:
		squareImg = squareImgMedium
		break
	case sizeLarge:
		squareImg = squareImgLarge
		break
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.ColorM.Translate(col.R, col.G, col.B, 0)

	screen.DrawImage(squareImg, op)
}
