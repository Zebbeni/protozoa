package ux

import (
	"math"

	c "github.com/Zebbeni/protozoa/config"
)

// ZoomLevel represents the discrete zoom levels
type ZoomLevel int

const (
	Zoom4  ZoomLevel = 0
	Zoom6  ZoomLevel = 1
	Zoom8  ZoomLevel = 2
	Zoom12 ZoomLevel = 3
	Zoom16 ZoomLevel = 4
	Zoom24 ZoomLevel = 5
	Zoom32 ZoomLevel = 6
	Zoom48 ZoomLevel = 7
	Zoom64 ZoomLevel = 8

	ZoomMin = Zoom4
	ZoomMax = Zoom64
)

// zoomUnitSizes maps each zoom level to the display pixel size per cell.
var zoomUnitSizes = [9]int{4, 6, 8, 12, 16, 24, 32, 48, 64}

// zoomSpriteSet maps each zoom level to the sprite set index (0=4x4, 1=8x8, 2=16x16).
// Intermediate levels use the largest sprite set that doesn't exceed the unit size.
var zoomSpriteSet = [9]int{0, 0, 1, 1, 2, 2, 2, 2, 2}

// zoomSpriteSizes is the native pixel size per sprite set.
var zoomSpriteSizes = [3]int{4, 8, 16}

// SpriteSet returns the sprite set index (0-2) for the current zoom level.
func (cam *Camera) SpriteSet() int {
	return zoomSpriteSet[cam.Zoom]
}

// SpriteScale returns the factor to scale sprites up to the display unit size.
func (cam *Camera) SpriteScale() float64 {
	return float64(cam.GridUnitSize()) / float64(zoomSpriteSizes[cam.SpriteSet()])
}

// Camera tracks viewport position and zoom level for the world view.
type Camera struct {
	X, Y      float64   // top-left corner of viewport in grid units (can be any value; wraps)
	Zoom      ZoomLevel // current zoom level
	ViewportW int       // viewport pixel width (screen area for grid)
	ViewportH int       // viewport pixel height
}

// NewCamera creates a camera at medium zoom, centered on the world.
func NewCamera(viewportW, viewportH int) *Camera {
	cam := &Camera{
		Zoom:      Zoom8,
		ViewportW: viewportW,
		ViewportH: viewportH,
	}
	cam.CenterOn(c.GridUnitsWide()/2, c.GridUnitsHigh()/2)
	return cam
}

func (cam *Camera) GridUnitSize() int       { return zoomUnitSizes[cam.Zoom] }
func (cam *Camera) WorldPixelWidth() int    { return c.GridUnitsWide() * cam.GridUnitSize() }
func (cam *Camera) WorldPixelHeight() int   { return c.GridUnitsHigh() * cam.GridUnitSize() }
func (cam *Camera) WorldUnitsWide() float64 { return float64(c.GridUnitsWide()) }
func (cam *Camera) WorldUnitsHigh() float64 { return float64(c.GridUnitsHigh()) }

// WrapsX returns true if the world is wider than the viewport (panning wraps horizontally).
func (cam *Camera) WrapsX() bool {
	return cam.WorldPixelWidth() > cam.ViewportW
}

// WrapsY returns true if the world is taller than the viewport (panning wraps vertically).
func (cam *Camera) WrapsY() bool {
	return cam.WorldPixelHeight() > cam.ViewportH
}

// NormalizedX returns the camera X wrapped into [0, WorldUnitsWide).
func (cam *Camera) NormalizedX() float64 {
	w := cam.WorldUnitsWide()
	return math.Mod(math.Mod(cam.X, w)+w, w)
}

// NormalizedY returns the camera Y wrapped into [0, WorldUnitsHigh).
func (cam *Camera) NormalizedY() float64 {
	h := cam.WorldUnitsHigh()
	return math.Mod(math.Mod(cam.Y, h)+h, h)
}

// CenterOffset returns the pixel offset to center the world in the viewport
// when the world is smaller than the viewport on an axis.
func (cam *Camera) CenterOffset() (offsetX, offsetY int) {
	if !cam.WrapsX() {
		offsetX = (cam.ViewportW - cam.WorldPixelWidth()) / 2
	}
	if !cam.WrapsY() {
		offsetY = (cam.ViewportH - cam.WorldPixelHeight()) / 2
	}
	return
}

// ScreenToGrid converts a screen pixel position (relative to the grid viewport area)
// to world grid coordinates with wrapping.
func (cam *Camera) ScreenToGrid(screenX, screenY int) (gridX, gridY int, onGrid bool) {
	ox, oy := cam.CenterOffset()
	us := float64(cam.GridUnitSize())
	w := c.GridUnitsWide()
	h := c.GridUnitsHigh()

	if cam.WrapsX() {
		// Screen pixel offset from camera origin, converted to fractional grid units
		nx := cam.NormalizedX()
		gridX = int(math.Floor(nx+float64(screenX-ox)/us)) % w
		if gridX < 0 {
			gridX += w
		}
	} else {
		gridX = int(math.Floor(float64(screenX-ox) / us))
	}

	if cam.WrapsY() {
		ny := cam.NormalizedY()
		gridY = int(math.Floor(ny+float64(screenY-oy)/us)) % h
		if gridY < 0 {
			gridY += h
		}
	} else {
		gridY = int(math.Floor(float64(screenY-oy) / us))
	}

	onGrid = gridX >= 0 && gridY >= 0 && gridX < w && gridY < h
	return
}

// Pan adjusts the camera position by the given grid-unit deltas.
// Only wrapping axes allow free panning; non-wrapping axes are clamped.
func (cam *Camera) Pan(dx, dy float64) {
	cam.X += dx
	cam.Y += dy

	// Clamp non-wrapping axes so the world stays visible
	if !cam.WrapsX() {
		cam.X = 0
	}
	if !cam.WrapsY() {
		cam.Y = 0
	}
}

func (cam *Camera) SetZoom(level ZoomLevel, pivotScreenX, pivotScreenY int) {
	if level < ZoomMin {
		level = ZoomMin
	}
	if level > ZoomMax {
		level = ZoomMax
	}
	if level == cam.Zoom {
		return
	}

	oldUnitSize := cam.GridUnitSize()
	pivotGridX := cam.X + float64(pivotScreenX)/float64(oldUnitSize)
	pivotGridY := cam.Y + float64(pivotScreenY)/float64(oldUnitSize)

	cam.Zoom = level

	newUnitSize := cam.GridUnitSize()
	cam.X = pivotGridX - float64(pivotScreenX)/float64(newUnitSize)
	cam.Y = pivotGridY - float64(pivotScreenY)/float64(newUnitSize)
}

func (cam *Camera) ZoomIn(pivotScreenX, pivotScreenY int) {
	cam.SetZoom(cam.Zoom+1, pivotScreenX, pivotScreenY)
}

func (cam *Camera) ZoomOut(pivotScreenX, pivotScreenY int) {
	cam.SetZoom(cam.Zoom-1, pivotScreenX, pivotScreenY)
}

func (cam *Camera) CenterOn(gridX, gridY int) {
	unitSize := cam.GridUnitSize()
	cam.X = float64(gridX) - float64(cam.ViewportW)/float64(unitSize)/2.0
	cam.Y = float64(gridY) - float64(cam.ViewportH)/float64(unitSize)/2.0
}
