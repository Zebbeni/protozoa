package simulation

import (
	"image/color"
	"time"

	c "github.com/Zebbeni/protozoa/constants"
	"github.com/Zebbeni/protozoa/decisions"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

const (
	sizeSmall size = iota
	sizeMedium
	sizeLarge
)

func (s *Simulation) renderGrid(screen *ebiten.Image) {
	renderFrame, _ := ebiten.NewImage(c.GridWidth, c.GridHeight, ebiten.FilterDefault)
	organisms, mostReproductiveID := s.world.GetOrganisms()
	start := time.Now()
	// Come up with a better way to trigger a refresh than this
	if s.shouldRefreshGrid() {
		ebitenutil.DrawRect(renderFrame, 0, 0, c.GridWidth, c.GridHeight, backgroundColor)
		for _, foodItem := range s.world.GetFoodItems() {
			renderFood(foodItem, renderFrame)
		}
		for _, o := range organisms {
			renderOrganism(*o, renderFrame, mostReproductiveID)
		}
	} else {
		renderFrame.DrawImage(s.previousRenderFrame, nil)
		for _, point := range s.world.PointsToUpdate {
			// paint background over grid square to update first
			ebitenutil.DrawRect(renderFrame, float64(point.X)*c.GridUnitSize, float64(point.Y)*c.GridUnitSize, c.GridUnitSize, c.GridUnitSize, backgroundColor)
			if foodItem := s.world.FoodManager.GetFoodAtPoint(point); foodItem != nil {
				renderFood(foodItem, renderFrame)
				continue
			}
			if o := s.world.OrganismManager.GetOrganismAtPoint(point); o != nil {
				renderOrganism(*o, renderFrame, mostReproductiveID)
				continue
			}
		}
	}
	s.totalRenderDuration = time.Since(start)

	s.previousRenderFrame, _ = ebiten.NewImage(c.GridWidth, c.GridHeight, ebiten.FilterDefault)
	s.previousRenderFrame.DrawImage(renderFrame, nil)

	if selectedOrganism, ok := organisms[mostReproductiveID]; ok {
		selectionBox, _ := ebiten.NewImage(c.GridWidth, c.GridHeight, ebiten.FilterDefault)
		renderSelection(selectedOrganism.Location, selectionBox, selectedOrganism.Color())
		renderFrame.DrawImage(selectionBox, nil)
	}

	offsetOptions := &ebiten.DrawImageOptions{}
	offsetOptions.GeoM.Translate(float64(c.PanelWidth), 0)
	screen.DrawImage(renderFrame, offsetOptions)

	s.world.ResetGridPointsToUpdate()
}

func (s *Simulation) shouldRefreshGrid() bool {
	return len(s.world.PointsToUpdate) == 0
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
func renderOrganism(o organism.Organism, img *ebiten.Image, mostReproductiveID int) {
	point := o.Location.Times(c.GridUnitSize)
	x, y := float64(point.X), float64(point.Y)

	organismSize := sizeSmall
	if o.Size < c.MaximumMaxSize*0.4375 {
		organismSize = sizeSmall
	} else if o.Size < c.MaximumMaxSize*0.8125 {
		organismSize = sizeMedium
	} else {
		organismSize = sizeLarge
	}

	organismColor := o.Color()
	if o.Action() == decisions.ActAttack {
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
