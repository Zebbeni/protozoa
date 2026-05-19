package ux

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// descHighlightCache is the precomputed answer to "which IDs are
// descendants of selID and alive somewhere in [fromCycle, toCycle]?"
// Rebuilt on selection change or when the playhead crosses the window.
type descHighlightCache struct {
	selID              int
	fromCycle, toCycle int
	ids                map[int]struct{}
}

// descHighlightBufferCycles is how far ahead of the playhead each
// rebuild looks. Bigger means fewer rebuilds but a larger walk each
// time; ~1000 cycles at default speed = a rebuild every several seconds
// of wall-clock playback, which is barely perceptible.
const descHighlightBufferCycles = 1000

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
//   - Otherwise, every living descendant of the selected organism
//     (resolved through the descendant-highlight cache) gets a faded
//     box.
//   - The selected organism itself gets the full themed foreground.
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
	} else if selID >= 0 {
		descColor := fadedForeground(0x40)
		currentCycle := g.simulation.Cycle()

		// Rebuild the descendant set when selection changed or the
		// playhead left the cached window.
		c := g.descHighlight
		if c == nil || c.selID != selID || currentCycle < c.fromCycle || currentCycle > c.toCycle {
			from := currentCycle
			to := currentCycle + descHighlightBufferCycles
			g.descHighlight = &descHighlightCache{
				selID:     selID,
				fromCycle: from,
				toCycle:   to,
				ids:       g.buildDescendantHighlightSet(selID, from, to),
			}
			c = g.descHighlight
		}

		// Iterate the smaller of the two sets so the per-render cost
		// is O(min(live, descendants)).
		if len(c.ids) > 0 {
			if len(c.ids) <= len(aliveInfos) {
				for id := range c.ids {
					if info, ok := aliveInfos[id]; ok {
						g.stampSelectionBox(layer, info.Location, descColor)
					}
				}
			} else {
				for id, info := range aliveInfos {
					if _, ok := c.ids[id]; ok {
						g.stampSelectionBox(layer, info.Location, descColor)
					}
				}
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

// buildDescendantHighlightSet walks selID's descendant subtree once
// and returns the set of node IDs that are alive at any cycle in
// [fromCycle, toCycle]. Used to populate descHighlight; callers don't
// hit this code on every render — only when the cache is invalid
// (selection change or playhead crossed the window boundary).
//
// Two prunes keep the walk bounded for very old selections:
//   - StartCycle > toCycle: subtree not yet born by window's end.
//     Children always have StartCycle >= parent.StartCycle, so the
//     entire subtree is irrelevant.
//   - AllBranchesDeadCycle != 0 && < fromCycle: every node in this
//     subtree died strictly before the window opens; no descendant
//     could be alive in the window.
//
// A node N is alive somewhere in [fromCycle, toCycle] iff
// N.StartCycle <= toCycle AND (N.EndCycle == 0 OR N.EndCycle >= fromCycle).
func (g *Grid) buildDescendantHighlightSet(selID, fromCycle, toCycle int) map[int]struct{} {
	set := make(map[int]struct{})
	root := g.simulation.GetTreeNodeByID(selID)
	if root == nil {
		return set
	}
	var walk func(n *organism.DescendantNode)
	walk = func(n *organism.DescendantNode) {
		n.ForEachChild(func(child *organism.DescendantNode) {
			if child.StartCycle > toCycle {
				return
			}
			if child.AllBranchesDeadCycle != 0 && child.AllBranchesDeadCycle < fromCycle {
				return
			}
			if child.EndCycle == 0 || child.EndCycle >= fromCycle {
				set[child.ID] = struct{}{}
			}
			walk(child)
		})
	}
	walk(root)
	return set
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
