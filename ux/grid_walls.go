package ux

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

var wallColor = colorful.HSLuv(0, 0, 0.5)

// wallPhTintStrength caps how far wallColor blends toward the pH-target colour at the most extreme pH.
const wallPhTintStrength = 0.7

// wallPhBrightenAdd is the HSLuv-lightness boost applied at the most extreme pH (weight 1).
const wallPhBrightenAdd = 0.2

// wallConnectors pairs each cardinal-neighbour offset with the layer drawn over the base when that neighbour holds a wall.
var wallConnectors = []struct {
	offset utils.Point
	layer  resources.Layer
}{
	{utils.Point{X: 0, Y: -1}, resources.LayerWallUp},
	{utils.Point{X: 0, Y: 1}, resources.LayerWallDown},
	{utils.Point{X: -1, Y: 0}, resources.LayerWallLeft},
	{utils.Point{X: 1, Y: 0}, resources.LayerWallRight},
}

func (g *Grid) renderWalls(wallsImage *ebiten.Image, refresh bool) {
	if refresh {
		// Walls are dynamic — placed and removed by ActDig (which now damages the wall in front and reinforces walls on either side in a single action).
		for point, strength := range g.simulation.GetWalls() {
			g.renderWallAt(wallsImage, point, strength)
		}
		return
	}
	us := g.unitSize()
	repaint := make(map[utils.Point]bool)
	for point := range g.simulation.GetUpdatedWallPoints() {
		repaint[point] = true
		for _, c := range wallConnectors {
			n := point.Add(c.offset)
			if g.simulation.GetWallStrengthAtPoint(n) > 0 {
				repaint[n] = true
			}
		}
	}
	// pH drift only re-tints actual walls — cells without a wall have nothing on this layer.
	for point := range g.simulation.GetUpdatedPhPoints() {
		if g.simulation.GetWallStrengthAtPoint(point) > 0 {
			repaint[point] = true
		}
	}
	for point := range repaint {
		x, y := float64(point.X*us), float64(point.Y*us)
		// Clear first: a dig-removed wall needs its old sprite gone.
		g.clearSquare(wallsImage, x, y)
		if strength := g.simulation.GetWallStrengthAtPoint(point); strength > 0 {
			g.renderWallAt(wallsImage, point, strength)
		}
	}
}

func (g *Grid) renderWallAt(wallsImage *ebiten.Image, point utils.Point, strength int) {
	us := g.unitSize()
	x := float64(point.X) * float64(us)
	y := float64(point.Y) * float64(us)
	tint := g.wallTintAt(point)
	role := wallRoleForStrength(strength)

	base := resources.SpriteLayer(role, resources.LayerWallBase, animation.AnimIdle, 0)
	g.drawStaticSprite(wallsImage, x, y, base, tint)

	for _, c := range wallConnectors {
		if g.simulation.GetWallStrengthAtPoint(point.Add(c.offset)) <= 0 {
			continue
		}
		sprite := resources.SpriteLayer(role, c.layer, animation.AnimIdle, 0)
		if sprite == nil {
			continue
		}
		g.drawStaticSprite(wallsImage, x, y, sprite, tint)
	}
}

func (g *Grid) wallTintAt(point utils.Point) colorful.Color {
	return wallTintForPh(g.simulation.GetPhAtPoint(point))
}

// wallTintForPh returns wallColor pushed toward the pH-target colour by the same weighted blend the env layer uses, capped at wallPhTintStrength so walls retain enough of their neutral gray to stay visible against an extreme-pH env layer painting the same cell.
func wallTintForPh(ph float64) colorful.Color {
	neutral := (config.MaxPh() + config.MinPh()) / 2.0
	halfRange := (config.MaxPh() - config.MinPh()) / 2.0
	if halfRange <= 0 {
		return wallColor
	}
	weight := math.Abs(ph-neutral) / halfRange
	if weight > 1 {
		weight = 1
	}
	tgtR, tgtG, tgtB := config.PhTargetColorRGB(ph)
	target := colorful.Color{R: tgtR, G: tgtG, B: tgtB}
	blended := wallColor.BlendRgb(target, weight*wallPhTintStrength).Clamped()

	h, s, l := blended.HSLuv()
	l = math.Min(1.0, l+weight*wallPhBrightenAdd)
	return colorful.HSLuv(h, s, l)
}

// wallRoleForStrength maps a wall's current strength to one of the three sprite-tier roles.
func wallRoleForStrength(strength int) resources.ImageRole {
	// Quarters of the strength range rather than fixed numbers.
	switch {
	case strength <= manager.MaxWallStrength/4:
		return resources.RoleWallWeak
	case strength <= manager.MaxWallStrength/2:
		return resources.RoleWallMedium
	case strength <= 3*manager.MaxWallStrength/4:
		return resources.RoleWallStrong
	default:
		return resources.RoleWallGiant
	}
}
