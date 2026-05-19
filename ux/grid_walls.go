package ux

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

var wallColor = colorful.HSLuv(0, 0, 0.5)

// wallPhTintStrength caps how far wallColor blends toward the pH-target
// colour at the most extreme pH. 1.0 would make walls indistinguishable
// from the env layer at the same cell; 0.7 keeps a visible gray cast so
// the wall sprite still reads as wall, not env.
const wallPhTintStrength = 0.7

// wallPhBrightenAdd is the HSLuv-lightness boost applied at the most
// extreme pH (weight 1). Scales linearly with weight, so neutral cells
// get no boost and the brightest walls are at MinPh / MaxPh. Tuned so
// pH-tinted walls clearly out-bright the env layer that paints the
// same cell.
const wallPhBrightenAdd = 0.2

// wallConnectors pairs each cardinal-neighbour offset with the layer
// drawn over the base when that neighbour holds a wall. Order
// doesn't matter — every connector overlay is composited on the same
// cell, just additively layered.
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
		// Walls are dynamic — placed and removed by ActDig (which now
		// damages the wall in front and reinforces walls on either
		// side in a single action). On full refresh, iterate the entire
		// strength map and render each cell with brightness scaled
		// to its current strength (low strength → faint, max → full).
		for point, strength := range g.simulation.GetWalls() {
			g.renderWallAt(wallsImage, point, strength)
		}
		return
	}
	// Incremental repaint covers three triggers:
	//   1. dig flagged points — wall strength or existence changed
	//      at that cell
	//   2. pH-updated points that happen to hold a wall — the wall's
	//      pH-derived tint needs to follow env drift so the user
	//      sees the same colour gradient on walls as on the env layer
	//   3. cardinal neighbours of a (1) point that still hold a
	//      wall — their connector composition changed when their
	//      neighbour appeared / disappeared / changed strength,
	//      so they have to repaint even though nothing happened at
	//      the neighbour cell itself
	// Clear before redraw so an old higher strength (or stale tint)
	// doesn't bleed through under the new sprite.
	us := g.unitSize()
	repaint := make(map[utils.Point]bool)
	// Dig-flagged cells always repaint. Their wall-bearing cardinal
	// neighbours repaint too — their connector composite includes a
	// piece pointing at this newly-changed cell.
	for point := range g.simulation.GetUpdatedWallPoints() {
		repaint[point] = true
		for _, c := range wallConnectors {
			n := point.Add(c.offset)
			if g.simulation.GetWallStrengthAtPoint(n) > 0 {
				repaint[n] = true
			}
		}
	}
	// pH drift only re-tints actual walls — cells without a wall
	// have nothing on this layer.
	for point := range g.simulation.GetUpdatedPhPoints() {
		if g.simulation.GetWallStrengthAtPoint(point) > 0 {
			repaint[point] = true
		}
	}
	for point := range repaint {
		x, y := float64(point.X*us), float64(point.Y*us)
		// Clear first: a dig-removed wall needs its old sprite
		// gone, and any cell still holding a wall is about to get a
		// fresh composite that replaces stale connector overlays.
		g.clearSquare(wallsImage, x, y)
		if strength := g.simulation.GetWallStrengthAtPoint(point); strength > 0 {
			g.renderWallAt(wallsImage, point, strength)
		}
	}
}

// renderWallAt composites a wall cell from up to five sprites: the
// strength-tiered base "pile of sediment" (LayerWallBase) plus one
// directional connector layer for each cardinal neighbour that also
// holds a wall. All sprites share the same pH-derived tint so the
// cell reads as a single object. Connector layers return nil when
// the artist hasn't authored them yet — the renderer simply skips
// the overlay and the wall appears as the isolated base.
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

// wallTintAt is the per-point wrapper used by the live grid. The
// pH-blending formula itself lives in wallTintForPh so the animation
// test can share it without going through a Grid / Simulation.
func (g *Grid) wallTintAt(point utils.Point) colorful.Color {
	return wallTintForPh(g.simulation.GetPhAtPoint(point))
}

// wallTintForPh returns wallColor pushed toward the pH-target colour by
// the same weighted blend the env layer uses, capped at
// wallPhTintStrength so walls retain enough of their neutral gray to
// stay visible against an extreme-pH env layer painting the same cell.
// The blended colour is then brightened in HSLuv lightness by an amount
// scaled with the pH distance from neutral, so walls stand out more
// against the saturated env at the extremes. A pH at the neutral
// midpoint yields exactly wallColor — the appearance the user signed
// off on for neutral cells (weight 0 → no blend, no brighten).
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

// wallRoleForStrength maps a wall's current strength to one of the
// three sprite-tier roles. Buckets mirror what the user authored
// in the .aseprite source:
//
//	1-2 → weak,  3-5 → medium,  6-7 → strong.
//
// Strengths outside [1, MaxWallStrength] are clamped — the renderer
// only ever calls this with strength > 0 (a 0-strength wall isn't
// kept in the WallManager), but the bounds keep this honest.
func wallRoleForStrength(strength int) resources.ImageRole {
	switch {
	case strength <= 2:
		return resources.RoleWallWeak
	case strength <= 5:
		return resources.RoleWallMedium
	default:
		return resources.RoleWallStrong
	}
}
