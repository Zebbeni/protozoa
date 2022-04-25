package ux

import (
	"github.com/Zebbeni/protozoa/resources"
	"github.com/lucasb-eyer/go-colorful"
	"image/color"

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
	sizeFill
)

const (
	phMinHue     = 0.0
	phMaxHue     = 120.0
	phSaturation = 0.5
	phLightness  = 0.15
)

var (
	clearColor                                                     = color.Alpha{A: 0x00}
	foodColor                                                      = colorful.HSLuv(120, 0.2, 0.25)
	attackColor                                                    = colorful.HSLuv(0.0, 255.0, 1.0)
	selectColor                                                    = colorful.HSLuv(0.0, 255.0, 1.0)
	hoverColor                                                     = colorful.HSLuv(0.0, 0, 0.5)
	squareImgSmall, squareImgMedium, squareImgLarge, squareImgFill *ebiten.Image
)

type Grid struct {
	simulation *simulation.Simulation

	previousEnvImage  *ebiten.Image
	previousFoodImage *ebiten.Image
	previousOrgsImage *ebiten.Image

	mouseHoverLocation utils.Point
	mouseOnGrid        bool
}

func NewGrid(simulation *simulation.Simulation) *Grid {
	g := &Grid{
		simulation:        simulation,
		previousEnvImage:  newBlankLayer(),
		previousFoodImage: newBlankLayer(),
		previousOrgsImage: newBlankLayer(),
	}
	loadOrganismImages()
	return g
}

func loadOrganismImages() {
	squareImgSmall = resources.SquareSmall
	squareImgMedium = resources.SquareMedium
	squareImgLarge = resources.SquareLarge
	squareImgFill = resources.SquareFill
}

// Render draws all organisms and food on the simulation grid
func (g *Grid) Render() *ebiten.Image {
	envImage := newBlankLayer()
	foodImage := newBlankLayer()
	orgsImage := newBlankLayer()
	selImage := newBlankLayer()
	gridImage := newBlankLayer()

	doRefresh := g.shouldRefresh()

	g.renderEnvironment(envImage, doRefresh)
	g.renderFood(foodImage, doRefresh)
	g.renderOrganisms(orgsImage, doRefresh)
	g.renderSelections(selImage)

	g.previousEnvImage = envImage
	g.previousFoodImage = foodImage
	g.previousOrgsImage = orgsImage

	g.simulation.ClearUpdatedPoints()

	gridImage.DrawImage(envImage, nil)
	gridImage.DrawImage(foodImage, nil)
	gridImage.DrawImage(orgsImage, nil)
	gridImage.DrawImage(selImage, nil)

	return gridImage
}

func (g *Grid) renderEnvironment(envImage *ebiten.Image, refresh bool) {
	if refresh {
		phMap := g.simulation.GetPhMap()
		for x := range phMap {
			for y := range phMap[x] {
				g.renderPhValue(envImage, x, y, phMap[x][y])
			}
		}
	} else {
		envImage.DrawImage(g.previousEnvImage, nil)
		updatedPoints := g.simulation.GetUpdatedOrganismPoints()
		for _, point := range updatedPoints {
			// clear square to be updated
			x, y := point.X*config.GridUnitSize(), point.Y*config.GridUnitSize()
			g.clearSquare(envImage, float64(x), float64(y))

			phVal := g.simulation.GetPhAtPoint(point)
			g.renderPhValue(envImage, x, y, phVal)
		}
	}
}

func (g *Grid) renderPhValue(envImage *ebiten.Image, gridX, gridY int, phVal float64) {
	x := float64(gridX) * float64(config.GridUnitSize())
	y := float64(gridY) * float64(config.GridUnitSize())
	col := colorful.HSLuv((phVal/config.MaxPh())*phMaxHue, phSaturation, phLightness)
	g.drawSquare(envImage, x, y, sizeFill, col)
}

func (g *Grid) renderFood(foodImage *ebiten.Image, refresh bool) {
	if refresh {
		items := g.simulation.GetFoodItems()
		for _, item := range items {
			g.renderFoodItem(item, foodImage)
		}
	} else {
		foodImage.DrawImage(g.previousFoodImage, nil)
		updatedPoints := g.simulation.GetUpdatedFoodPoints()
		for _, point := range updatedPoints {
			// clear square to be updated
			x, y := point.X*config.GridUnitSize(), point.Y*config.GridUnitSize()
			g.clearSquare(foodImage, float64(x), float64(y))

			if item := g.simulation.GetFoodAtPoint(point); item != nil {
				g.renderFoodItem(item, foodImage)
			}
		}
	}
}

func (g *Grid) renderOrganisms(organismsImage *ebiten.Image, refresh bool) {
	if refresh {
		organismInfo := g.simulation.GetAllOrganismInfo()
		for _, info := range organismInfo {
			g.renderOrganism(info, organismsImage)
		}
	} else {
		organismsImage.DrawImage(g.previousOrgsImage, nil)
		updatedPoints := g.simulation.GetUpdatedOrganismPoints()
		for _, point := range updatedPoints {
			// clear square to be updated
			x, y := point.X*config.GridUnitSize(), point.Y*config.GridUnitSize()
			g.clearSquare(organismsImage, float64(x), float64(y))

			if info := g.simulation.GetOrganismInfoAtPoint(point); info != nil {
				g.renderOrganism(info, organismsImage)
			}
		}
	}
}

func (g *Grid) renderSelections(selectionsImage *ebiten.Image) {
	if g.mouseOnGrid {
		if info := g.simulation.GetOrganismInfoAtPoint(g.mouseHoverLocation); info != nil {
			g.renderSelection(g.mouseHoverLocation, selectionsImage, info.Color)
		} else {
			g.renderSelection(g.mouseHoverLocation, selectionsImage, hoverColor)
		}
	}

	if info := g.simulation.GetOrganismInfoByID(g.simulation.GetSelected()); info != nil {
		g.renderSelection(info.Location, selectionsImage, selectColor)
	}
}

func (g *Grid) shouldRefresh() bool {
	return g.simulation.Cycle() == 0
}

func newBlankLayer() *ebiten.Image {
	return ebiten.NewImage(config.GridWidth(), config.GridHeight())
}

func (g *Grid) MouseHover(point utils.Point, onGrid bool) {
	g.mouseHoverLocation = point
	g.mouseOnGrid = onGrid
}

// renderSelection draws a square around a single item on the grid
func (g *Grid) renderSelection(point utils.Point, img *ebiten.Image, col colorful.Color) {
	x, y := float64(point.X*config.GridUnitSize()), float64(point.Y*config.GridUnitSize())
	ebitenutil.DrawLine(img, x-2, y-2, x+float64(config.GridUnitSize())+3, y-2, col)                                                               // top
	ebitenutil.DrawLine(img, x-2, y-2, x-2, y+float64(config.GridUnitSize())+3, col)                                                               // left
	ebitenutil.DrawLine(img, x-2, y+float64(config.GridUnitSize())+3, x+float64(config.GridUnitSize())+3, y+float64(config.GridUnitSize())+3, col) // bottom
	ebitenutil.DrawLine(img, x+float64(config.GridUnitSize())+3, y-2, x+float64(config.GridUnitSize())+3, y+float64(config.GridUnitSize())+3, col) // right
}

// renderFoodItem draws a food item to the given image
func (g *Grid) renderFoodItem(item *food.Item, img *ebiten.Image) {
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

// renderOrganism draws an organism to the given image
func (g *Grid) renderOrganism(info *organism.Info, img *ebiten.Image) {
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
		organismColor = attackColor
	}

	g.drawSquare(img, x, y, organismSize, organismColor)
}

func (g *Grid) drawSquare(img *ebiten.Image, x, y float64, sz size, col colorful.Color) {
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
	case sizeFill:
		squareImg = squareImgFill
		break
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.ColorM.Translate(col.R, col.G, col.B, 0)

	img.DrawImage(squareImg, op)
}

func (g *Grid) clearSquare(img *ebiten.Image, x, y float64) {
	squareImg := squareImgLarge
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.CompositeMode = ebiten.CompositeModeDestinationOut

	img.DrawImage(squareImg, op)
}
