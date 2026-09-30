package ux

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

const buriedFoodAlpha = 0.25

// renderBuriedFood paints the buried store, using the same size-tier food sprites as the surface layer at a quarter opacity.
func (g *Grid) renderBuriedFood(img *ebiten.Image, refresh bool) {
	if refresh {
		for point, value := range g.simulation.GetBuriedFood() {
			g.renderBuriedAt(point, value, img)
		}
		return
	}
	us := g.unitSize()
	for point := range g.simulation.GetUpdatedFoodPoints() {
		g.clearSquare(img, float64(point.X*us), float64(point.Y*us))
		if value := g.simulation.GetBuriedFoodAtPoint(point); value > 0 {
			g.renderBuriedAt(point, value, img)
		}
	}
}

func (g *Grid) renderBuriedAt(point utils.Point, value int, img *ebiten.Image) {
	if value <= 0 {
		return
	}
	us := float64(g.unitSize())
	sprite := resources.Sprite(foodRoleForValue(value), animation.AnimIdle, 0)
	g.drawStaticSpriteAlpha(img, float64(point.X)*us, float64(point.Y)*us,
		sprite, foodColor, buriedFoodAlpha)
}
