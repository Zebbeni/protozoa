package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/resources"
)

var foodColor = colorful.HSLuv(35, 0.45, 0.3)

func (g *Grid) renderFood(foodImage *ebiten.Image, refresh bool) {
	if refresh {
		items := g.simulation.GetFoodItems()
		for _, item := range items {
			g.renderFoodItem(item, foodImage)
		}
	} else {
		updatedPoints := g.simulation.GetUpdatedFoodPoints()
		for point := range updatedPoints {
			us := g.unitSize()
			x, y := point.X*us, point.Y*us
			g.clearSquare(foodImage, float64(x), float64(y))
			if item, exists := g.simulation.GetFoodAtPoint(point); exists {
				g.renderFoodItem(item, foodImage)
			}
		}
	}
}

func (g *Grid) renderFoodItem(item *food.Item, img *ebiten.Image) {
	us := g.unitSize()
	x := float64(item.Point.X) * float64(us)
	y := float64(item.Point.Y) * float64(us)
	sprite := resources.Sprite(foodRoleForValue(item.Value), animation.AnimIdle, 0)
	g.drawStaticSprite(img, x, y, sprite, foodColor)
}

// foodRoleForValue maps a food item's value to one of the four food
// size-tier sprites. Quarters of MaxFoodValue, matching the organism
// and wall splits:
//
//	value < MaxFoodValue/4     → tiny
//	value < MaxFoodValue/2     → small
//	value < 3*MaxFoodValue/4   → medium
//	else                       → large
func foodRoleForValue(value int) resources.ImageRole {
	maxVal := config.MaxFoodValue()
	if maxVal <= 0 {
		return resources.RoleFoodMedium
	}
	// Quarters of the value range: under 25% is tiny, over 75% large.
	quarter := float64(maxVal) / 4.0
	switch v := float64(value); {
	case v < quarter:
		return resources.RoleFoodTiny
	case v < 2*quarter:
		return resources.RoleFoodSmall
	case v < 3*quarter:
		return resources.RoleFoodMedium
	default:
		return resources.RoleFoodLarge
	}
}
