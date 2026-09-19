package ux

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// buildSelectionBoxImg creates a white outlined-square image at the
// current unit size, used as the source for every per-organism
// selection box. Stamping a tinted copy of this image is one DrawImage
// per highlight; the previous version drew 4 lines × N visible tiles
// per box, which dominated frames with many highlights at low zoom.
//
// Uses 1px-tall / 1px-wide filled rects (not DrawLine) so each side
// lands on a whole-pixel row/column and the corners overlap cleanly.
// DrawLine's sub-pixel boundary places y=0 on the row above pixel 0
// and gets clipped, leaving visible gaps on the top and left.
func (g *Grid) buildSelectionBoxImg() {
	us := g.unitSize()
	img := ebiten.NewImage(us, us)
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	usf := float64(us)
	ebitenutil.DrawRect(img, 0, 0, usf, 1, white)     // top
	ebitenutil.DrawRect(img, 0, 0, 1, usf, white)     // left
	ebitenutil.DrawRect(img, 0, usf-1, usf, 1, white) // bottom
	ebitenutil.DrawRect(img, usf-1, 0, 1, usf, white) // right
	g.selectionBoxImg = img
}

// populateSelectionLayer redraws the selection-box layer in world
// coordinates. Each highlight is a single DrawImage of selectionBoxImg
// with a per-color tint; the surrounding compose loop handles wallpaper
// tiling so we don't re-stamp the same box at every visible tile.
//
// Tiered styling, brightest last so it overdraws the rest:
//   - In selectMostSuccessful mode, every currently-living organism on
//     the most-successful set gets a faded box.
//   - The selected organism itself gets the full themed foreground.
//
// Living *descendants* used to get a faded box each, off a cached walk
// of the selection's subtree. The FAMILY colour mode replaced that: a box
// answered only "descendant or not", identically for a child and a
// great-great-grandchild, and said nothing about ancestors or cousins —
// where a tint carries the whole relationship and costs nothing extra to
// draw, since the sprite is being tinted anyway. See family.go.
//
// The hover-cell box is drawn separately in screen space (see Render),
// since it shouldn't tile across the wallpaper.
func (g *Grid) populateSelectionLayer(aliveInfos map[int]*organism.Info) {
	layer := g.layers[layerSelection]
	layer.Clear()

	selID := g.simulation.GetSelected()

	if g.selectMode == selectMostSuccessful {
		successfulColor := fadedForeground(0x40)
		for _, id := range g.simulation.GetMostSuccessfulIds() {
			if id == selID {
				continue // drawn last in bright colour
			}
			if info, ok := aliveInfos[id]; ok {
				g.stampSelectionBox(layer, info.Location, successfulColor)
			}
		}
	}

	if selID >= 0 {
		if info, ok := aliveInfos[selID]; ok {
			g.stampSelectionBox(layer, info.Location, themedForeground())
		}
	}
}

// stampSelectionBox draws a single tinted copy of selectionBoxImg onto
// the selection layer at the given world cell. ColorScale handles the
// alpha tint, so the same source bitmap covers every selection
// variant.
func (g *Grid) stampSelectionBox(layer *ebiten.Image, point utils.Point, col color.Color) {
	us := g.unitSize()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(point.X*us), float64(point.Y*us))
	r, gv, b, a := col.RGBA()
	op.ColorScale.Scale(float32(r)/0xffff, float32(gv)/0xffff, float32(b)/0xffff, float32(a)/0xffff)
	layer.DrawImage(g.selectionBoxImg, op)
}

// renderHoverCellBox draws a single hover-cell outline at the cursor's
// actual viewport position. Unlike per-organism selection highlights,
// which live on a tiled world layer, the hover cursor is a UI element
// that should appear once where the cursor is. The cell origin in
// viewport coords accounts for the camera's sub-cell offset — cells
// in a tiled view don't generally align to viewport pixel 0.
func (g *Grid) renderHoverCellBox(img *ebiten.Image, col color.Color) {
	mx, my := ebiten.CursorPosition()
	// Convert from screen pixels to viewport pixels (the same image
	// space the rest of the grid renders into).
	vx := float64(mx-panelWidth) / float64(GridDisplayScale)
	vy := float64(my) / float64(GridDisplayScale)
	us := float64(g.unitSize())
	camPxX := g.Camera.NormalizedX() * us
	camPxY := g.Camera.NormalizedY() * us
	// World pixel coordinates align to the camera. The cell containing
	// the cursor has its origin at world-pixel floor((vx+camPx)/us)*us;
	// translating back into viewport coords subtracts camPx.
	x0 := math.Floor((vx+camPxX)/us)*us - camPxX
	y0 := math.Floor((vy+camPxY)/us)*us - camPxY
	x1 := x0 + us
	y1 := y0 + us
	ebitenutil.DrawLine(img, x0, y0, x1, y0, col)
	ebitenutil.DrawLine(img, x0, y0, x0, y1, col)
	ebitenutil.DrawLine(img, x0, y1, x1, y1, col)
	ebitenutil.DrawLine(img, x1, y0, x1, y1, col)
}
