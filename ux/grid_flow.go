package ux

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// minFlowVectorUnitSize is the smallest cell size at which a flow
// vector is drawn as a directed line. Below 8px the line collapses
// into a 2-3px smear that reads as noise rather than direction.
//
// Smaller cells don't go blank, though: they fall back to a single
// dot per stirred cell (see renderFlow). Stirred cells are typically
// well under 1% of the grid and scattered, so "where is there any
// current at all?" is a question best answered zoomed out — going
// blank there made the overlay look broken exactly when you were
// most likely to be hunting for it.
const minFlowVectorUnitSize = 8

// flowReachFraction is how far a full-magnitude vector extends from its
// cell centre, as a fraction of the cell. 0.45 stops just short of the
// cell edge so neighbouring arrows stay visually separate instead of
// merging into continuous streaks — the field should read as a grid of
// samples, which is what it is.
const flowReachFraction = 0.45

// flowMinAlpha keeps a barely-moving cell visible. Scaling alpha
// straight from magnitude would fade the slowest currents to nothing,
// which hides exactly the thing you want to see when a current is
// first forming or has nearly decayed away.
const flowMinAlpha = 70

// renderFlow paints the environment's current field as one line per
// stirred cell: tail at the cell centre, head in the direction of
// flow, length and opacity both scaled by magnitude. A head marker
// disambiguates direction, since a bare line segment reads the same
// both ways.
//
// Redrawn in full every frame rather than incrementally. The field
// decays every cycle, so in practice every non-still cell changes
// every cycle and there is no meaningful dirty subset to track. The
// scan is O(grid) with a zero-vector fast path, and a still map — the
// common case — costs one cheap comparison per cell.
func (g *Grid) renderFlow(layer *ebiten.Image) {
	layer.Clear()

	us := g.unitSize()
	flowMap := g.simulation.GetFlowMap()
	usf := float64(us)
	reach := usf * flowReachFraction
	// Head marker scales with zoom but never drops below a pixel.
	head := max(1.0, usf/8)
	drawVectors := us >= minFlowVectorUnitSize

	for x, column := range flowMap {
		for y, v := range column {
			if v.IsZero() {
				continue
			}
			// Magnitude is kept in [0, 1] by ClampUnit on the write
			// path, so it maps directly onto both length and alpha.
			mag := min(1.0, v.Length())
			stroke := flowColor(mag)

			cx := (float64(x) + 0.5) * usf
			cy := (float64(y) + 0.5) * usf

			if !drawVectors {
				// Zoomed out: mark presence, not direction. Fills the
				// cell so a lone stirred cell is still findable among
				// 8000 at 4px each.
				ebitenutil.DrawRect(layer, float64(x)*usf, float64(y)*usf, usf, usf, stroke)
				continue
			}

			hx := cx + v.X*reach
			hy := cy + v.Y*reach
			ebitenutil.DrawLine(layer, cx, cy, hx, hy, stroke)
			// Square centred on the head, so the arrow reads as
			// pointing somewhere rather than just lying there.
			ebitenutil.DrawRect(layer, hx-head/2, hy-head/2, head, head, stroke)
		}
	}
}

// flowColor returns the stroke colour for a flow vector of the given
// magnitude: the themed foreground, with opacity ramping from
// flowMinAlpha at a standstill to fully opaque at full current.
//
// Foreground rather than a hue on purpose — the flow overlay sits
// directly on top of the pH wash, and both pH colour schemes
// (green-pink, blue-orange) already occupy most of the wheel. A
// neutral stroke stays legible over any pH value instead of
// disappearing wherever it happens to match the background.
func flowColor(mag float64) color.Color {
	alpha := uint8(flowMinAlpha + (255-flowMinAlpha)*mag)
	return fadedForeground(alpha)
}
