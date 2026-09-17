package ux

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/config"
)

// renderPh maintains the W × H pH buffer (one pixel per cell, no
// border) and stamps it into the (W+2) × (H+2) bordered scratch with
// sub-image draws — interior, four edges, and four corners. The
// border source is always the buffer image, never phBordered itself,
// so there's no self-draw.
//
// The bordered scratch is smoothed once, with FilterLinear, to the
// active sprite set's resolution — 4, 8 or 16 px per cell — so the
// gradient has exactly as many pixels per cell as the sprites drawn on
// top of it and its pixels line up with theirs. At the zooms where cells
// are drawn at native sprite size that is the only upscale. At the
// larger zooms, where sprites are themselves enlarged with nearest
// neighbour by Camera.SpriteScale, the gradient gets the same
// enlargement so it stays aligned with them.
//
// (This replaced an earlier scheme that always smoothed to a fixed 4 px
// per cell and then enlarged with nearest neighbour, which at the 16x16
// sprite set quantised the gradient into visible 4x4 blocks per cell.)
func (g *Grid) renderPh(phImage *ebiten.Image, refresh bool) {
	W := config.GridUnitsWide()
	H := config.GridUnitsHigh()
	if g.phBordered == nil {
		g.phBordered = ebiten.NewImage(W+2, H+2)
		g.phBuffer = ebiten.NewImage(W, H)
		refresh = true
	}
	cell := g.Camera.SpriteSize()
	if g.phLinear == nil || g.phLinearCell != cell {
		g.phLinear = ebiten.NewImage((W+2)*cell, (H+2)*cell)
		g.phLinearCell = cell
		refresh = true
	}

	dirty := refresh
	if refresh {
		g.rebuildPhBuffer()
	} else {
		updated := g.simulation.GetUpdatedPhPoints()
		if len(updated) == 0 {
			return
		}
		for point := range updated {
			col := g.phToColor(g.simulation.GetPhAtPoint(point))
			g.phBuffer.Set(point.X, point.Y, color.RGBA{
				R: uint8(col.R * 255),
				G: uint8(col.G * 255),
				B: uint8(col.B * 255),
				A: 255,
			})
		}
		dirty = true
	}

	if dirty {
		g.stampPhBorderedFromBuffer()
	}

	// The one smoothing pass: linear upscale to sprite resolution.
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.GeoM.Scale(float64(cell), float64(cell))
	g.phLinear.DrawImage(g.phBordered, op)

	// Place it in the layer at world-pixel size, cropping the 1-cell
	// wrap border. SpriteScale is 1 at native zooms (a straight copy)
	// and 2 or 4 at the larger ones, where nearest neighbour matches how
	// the sprites are enlarged.
	unit := float64(g.Camera.GridUnitSize())
	scale := g.Camera.SpriteScale()
	op = &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(-unit, -unit)
	phImage.DrawImage(g.phLinear, op)
}

// rebuildPhBuffer writes every cell's colour into phBuffer in one
// bulk WritePixels. Used on full refreshes (initial render, theme
// change, zoom change).
func (g *Grid) rebuildPhBuffer() {
	W := config.GridUnitsWide()
	H := config.GridUnitsHigh()
	phMap := g.simulation.GetPhMap()
	buf := make([]byte, 4*W*H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			col := g.phToColor(phMap[x][y])
			i := (y*W + x) * 4
			buf[i] = uint8(col.R * 255)
			buf[i+1] = uint8(col.G * 255)
			buf[i+2] = uint8(col.B * 255)
			buf[i+3] = 255
		}
	}
	g.phBuffer.WritePixels(buf)
}

// stampPhBorderedFromBuffer rebuilds phBordered from phBuffer with 9
// sub-image draws: one for the interior, four for the edges, four for
// the wrap corners. Source is always phBuffer, destination is always
// phBordered — no self-draw. Edges and corners come from the opposite
// side of the world so FilterLinear sampling at the destination's
// outer pixels produces a seamless wrap.
func (g *Grid) stampPhBorderedFromBuffer() {
	W := config.GridUnitsWide()
	H := config.GridUnitsHigh()
	bw, bh := W+2, H+2

	stamp := func(srcRect image.Rectangle, dx, dy int) {
		sub := g.phBuffer.SubImage(srcRect).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(dx), float64(dy))
		g.phBordered.DrawImage(sub, op)
	}

	// Interior: full buffer at (1, 1).
	stamp(image.Rect(0, 0, W, H), 1, 1)
	// Edges: opposite-side row / column copies (no corners).
	stamp(image.Rect(0, H-1, W, H), 1, 0)  // top   ← bottom row
	stamp(image.Rect(0, 0, W, 1), 1, bh-1) // bottom ← top row
	stamp(image.Rect(W-1, 0, W, H), 0, 1)  // left  ← right column
	stamp(image.Rect(0, 0, 1, H), bw-1, 1) // right ← left column
	// Corners: diagonal-opposite cell at each world corner.
	stamp(image.Rect(W-1, H-1, W, H), 0, 0)   // top-left     ← bottom-right
	stamp(image.Rect(0, H-1, 1, H), bw-1, 0)  // top-right    ← bottom-left
	stamp(image.Rect(W-1, 0, W, 1), 0, bh-1)  // bottom-left  ← top-right
	stamp(image.Rect(0, 0, 1, 1), bw-1, bh-1) // bottom-right ← top-left
}

// phToColor maps a pH value to its display colour.
//
// Hue runs acid→base across the range. Saturation grows with distance
// from neutral pH (mid-pH is grey, extremes are colourful). Lightness
// flips with the theme so extremes stay high-contrast against the
// window fill, and the low-sat end blends towards the active theme's
// background so neutral cells visually disappear into the grid fill
// (black under dark, white under light).
func (g *Grid) phToColor(phVal float64) colorful.Color {
	neutral := (config.MaxPh() + config.MinPh()) / 2.0
	halfRange := (config.MaxPh() - config.MinPh()) / 2.0
	weight := 0.0
	if halfRange > 0 {
		weight = math.Abs(phVal-neutral) / halfRange
		if weight > 1 {
			weight = 1
		}
	}
	bgR, bgG, bgB := config.ThemeBackgroundRGB()
	tgtR, tgtG, tgtB := config.PhTargetColorRGB(phVal)
	bg := colorful.Color{R: bgR, G: bgG, B: bgB}
	target := colorful.Color{R: tgtR, G: tgtG, B: tgtB}
	return bg.BlendRgb(target, weight).Clamped()
}
