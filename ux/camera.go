package ux

import (
	"image"

	c "github.com/Zebbeni/protozoa/config"
)

// ZoomLevel represents the discrete zoom levels
type ZoomLevel int

const (
	ZoomFar    ZoomLevel = 0 // 4px per cell
	ZoomMedium ZoomLevel = 1 // 8px per cell
	ZoomClose  ZoomLevel = 2 // 16px per cell
)

var zoomUnitSizes = [3]int{4, 8, 16}

// Camera tracks viewport position and zoom level for the world view.
type Camera struct {
	X, Y      float64   // top-left corner of viewport in grid units
	Zoom      ZoomLevel // current zoom level
	ViewportW int       // viewport pixel width (screen area for grid)
	ViewportH int       // viewport pixel height
}

// NewCamera creates a camera at medium zoom, centered on the world.
func NewCamera(viewportW, viewportH int) *Camera {
	cam := &Camera{
		Zoom:      ZoomMedium,
		ViewportW: viewportW,
		ViewportH: viewportH,
	}
	// Center on the world
	cam.CenterOn(c.GridUnitsWide()/2, c.GridUnitsHigh()/2)
	return cam
}

// GridUnitSize returns the pixel size per grid cell at the current zoom level.
func (cam *Camera) GridUnitSize() int {
	return zoomUnitSizes[cam.Zoom]
}

// WorldPixelWidth returns the full world width in pixels at current zoom.
func (cam *Camera) WorldPixelWidth() int {
	return c.GridUnitsWide() * cam.GridUnitSize()
}

// WorldPixelHeight returns the full world height in pixels at current zoom.
func (cam *Camera) WorldPixelHeight() int {
	return c.GridUnitsHigh() * cam.GridUnitSize()
}

// VisibleRect returns the pixel rectangle to extract from the full-world image.
func (cam *Camera) VisibleRect() image.Rectangle {
	unitSize := cam.GridUnitSize()
	x0 := int(cam.X) * unitSize
	y0 := int(cam.Y) * unitSize
	x1 := x0 + cam.ViewportW
	y1 := y0 + cam.ViewportH

	// Clamp to world bounds
	ww := cam.WorldPixelWidth()
	wh := cam.WorldPixelHeight()
	if x1 > ww {
		x1 = ww
	}
	if y1 > wh {
		y1 = wh
	}
	return image.Rect(x0, y0, x1, y1)
}

// VisibleGridBounds returns the range of visible grid cells (inclusive min, exclusive max).
func (cam *Camera) VisibleGridBounds() (minX, minY, maxX, maxY int) {
	unitSize := cam.GridUnitSize()
	minX = int(cam.X)
	minY = int(cam.Y)
	maxX = minX + (cam.ViewportW / unitSize) + 1
	maxY = minY + (cam.ViewportH / unitSize) + 1
	if maxX > c.GridUnitsWide() {
		maxX = c.GridUnitsWide()
	}
	if maxY > c.GridUnitsHigh() {
		maxY = c.GridUnitsHigh()
	}
	return
}

// ScreenToGrid converts a screen pixel position (relative to the grid viewport area)
// to world grid coordinates.
func (cam *Camera) ScreenToGrid(screenX, screenY int) (gridX, gridY int, onGrid bool) {
	unitSize := cam.GridUnitSize()
	gridX = int(cam.X) + screenX/unitSize
	gridY = int(cam.Y) + screenY/unitSize
	onGrid = gridX >= 0 && gridY >= 0 && gridX < c.GridUnitsWide() && gridY < c.GridUnitsHigh()
	return
}

// Pan adjusts the camera position by the given grid-unit deltas, clamped to world bounds.
func (cam *Camera) Pan(dx, dy float64) {
	cam.X += dx
	cam.Y += dy
	cam.ClampPosition()
}

// SetZoom changes the zoom level, keeping the point under (pivotScreenX, pivotScreenY)
// stationary. pivotScreenX/Y are relative to the grid viewport area.
func (cam *Camera) SetZoom(level ZoomLevel, pivotScreenX, pivotScreenY int) {
	if level < ZoomFar {
		level = ZoomFar
	}
	if level > ZoomClose {
		level = ZoomClose
	}
	if level == cam.Zoom {
		return
	}

	// Grid coordinate under the pivot point before zoom
	oldUnitSize := cam.GridUnitSize()
	pivotGridX := cam.X + float64(pivotScreenX)/float64(oldUnitSize)
	pivotGridY := cam.Y + float64(pivotScreenY)/float64(oldUnitSize)

	cam.Zoom = level

	// Adjust position so the same grid coordinate stays under the pivot
	newUnitSize := cam.GridUnitSize()
	cam.X = pivotGridX - float64(pivotScreenX)/float64(newUnitSize)
	cam.Y = pivotGridY - float64(pivotScreenY)/float64(newUnitSize)
	cam.ClampPosition()
}

// ZoomIn zooms in one level, pivoting around the given screen point.
func (cam *Camera) ZoomIn(pivotScreenX, pivotScreenY int) {
	cam.SetZoom(cam.Zoom+1, pivotScreenX, pivotScreenY)
}

// ZoomOut zooms out one level, pivoting around the given screen point.
func (cam *Camera) ZoomOut(pivotScreenX, pivotScreenY int) {
	cam.SetZoom(cam.Zoom-1, pivotScreenX, pivotScreenY)
}

// CenterOn centers the viewport on the given grid coordinate.
func (cam *Camera) CenterOn(gridX, gridY int) {
	unitSize := cam.GridUnitSize()
	cam.X = float64(gridX) - float64(cam.ViewportW)/float64(unitSize)/2.0
	cam.Y = float64(gridY) - float64(cam.ViewportH)/float64(unitSize)/2.0
	cam.ClampPosition()
}

// ClampPosition ensures the viewport stays within world bounds.
func (cam *Camera) ClampPosition() {
	unitSize := cam.GridUnitSize()
	maxX := float64(c.GridUnitsWide()) - float64(cam.ViewportW)/float64(unitSize)
	maxY := float64(c.GridUnitsHigh()) - float64(cam.ViewportH)/float64(unitSize)

	cam.X = max(0, min(cam.X, maxX))
	cam.Y = max(0, min(cam.Y, maxY))
}
