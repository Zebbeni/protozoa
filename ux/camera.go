package ux

import (
	"math"
	"time"

	c "github.com/Zebbeni/protozoa/config"
)

// GridDisplayScale is the universal scale applied to the composed grid image when drawing it to the screen.
const GridDisplayScale = 2

type ZoomLevel int

const (
	Zoom4  ZoomLevel = 0
	Zoom8  ZoomLevel = 1
	Zoom16 ZoomLevel = 2
	Zoom32 ZoomLevel = 3
	Zoom64 ZoomLevel = 4

	ZoomMin = Zoom4
	ZoomMax = Zoom64
)

// zoomUnitSizes maps each zoom level to the display pixel size per cell.
var zoomUnitSizes = [5]int{4, 8, 16, 32, 64}

// zoomSpriteSet maps each zoom level to the sprite set index (0=4x4, 1=8x8, 2=16x16).
//
// Zoom8 draws the 16x16 art at half scale rather than the 8x8 set: only the
// high-res set carries the layered overlays, so the 8x8 art shows a bare
// body with no motor, mouth or sensors. EXPERIMENT — the 8x8 set is still
// loaded and this is a one-value revert.
var zoomSpriteSet = [5]int{0, 2, 2, 2, 2}

var zoomSpriteSizes = [3]int{4, 8, 16}

var zoomSpriteFrameCounts = [3]int{2, 2, 4}

func (cam *Camera) SpriteSet() int {
	return zoomSpriteSet[cam.Zoom]
}

// SpriteFrameCount returns the per-cycle sprite frame count for the active sprite set (2 at 4x4, 2 at 8x8, 4 at 16x16).
func (cam *Camera) SpriteFrameCount() int {
	return zoomSpriteFrameCounts[cam.SpriteSet()]
}

// SpriteScale returns the factor to scale sprites up to the display unit size.
func (cam *Camera) SpriteSize() int {
	return zoomSpriteSizes[cam.SpriteSet()]
}

func (cam *Camera) SpriteScale() float64 {
	return float64(cam.GridUnitSize()) / float64(zoomSpriteSizes[cam.SpriteSet()])
}

// Camera tracks viewport position and zoom level for the world view.
type Camera struct {
	X, Y      float64   // top-left corner of viewport in grid units (can be any value; wraps)
	Zoom      ZoomLevel // current zoom level
	ViewportW int       // viewport pixel width (screen area for grid)
	ViewportH int       // viewport pixel height

	panActive   bool
	panStart    time.Time
	panDuration time.Duration
	panFromX    float64
	panFromY    float64
	panToX      float64
	panToY      float64
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

func (cam *Camera) NormalizedX() float64 {
	w := cam.WorldUnitsWide()
	return math.Mod(math.Mod(cam.X, w)+w, w)
}

func (cam *Camera) NormalizedY() float64 {
	h := cam.WorldUnitsHigh()
	return math.Mod(math.Mod(cam.Y, h)+h, h)
}

// CenterOffset returns the pixel offset to center the world in the viewport when the world is smaller than the viewport on an axis.
func (cam *Camera) CenterOffset() (offsetX, offsetY int) {
	if !cam.WrapsX() {
		offsetX = (cam.ViewportW - cam.WorldPixelWidth()) / 2
	}
	if !cam.WrapsY() {
		offsetY = (cam.ViewportH - cam.WorldPixelHeight()) / 2
	}
	return
}

// ScreenToGrid converts a screen pixel position (relative to the grid viewport area) to world grid coordinates.
func (cam *Camera) ScreenToGrid(screenX, screenY int) (gridX, gridY int, onGrid bool) {
	us := float64(cam.GridUnitSize())
	w := c.GridUnitsWide()
	h := c.GridUnitsHigh()

	nx := cam.NormalizedX()
	ny := cam.NormalizedY()
	gridX = int(math.Floor(nx+float64(screenX)/us)) % w
	if gridX < 0 {
		gridX += w
	}
	gridY = int(math.Floor(ny+float64(screenY)/us)) % h
	if gridY < 0 {
		gridY += h
	}

	onGrid = true
	return
}

// Pan adjusts the camera position by the given grid-unit deltas.
func (cam *Camera) Pan(dx, dy float64) {
	cam.panActive = false
	cam.X += dx
	cam.Y += dy
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

	cam.panActive = false

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

// PanTo starts a smooth animated pan to centre the camera on (gridX, gridY) over the given duration.
func (cam *Camera) PanTo(gridX, gridY int, duration time.Duration) {
	if duration <= 0 {
		cam.CenterOn(gridX, gridY)
		return
	}
	unitSize := cam.GridUnitSize()
	targetX := float64(gridX) - float64(cam.ViewportW)/float64(unitSize)/2.0
	targetY := float64(gridY) - float64(cam.ViewportH)/float64(unitSize)/2.0

	// Wrap-aware: pan whichever direction is closer around the world.
	targetX = shortestWrappedTarget(cam.X, targetX, cam.WorldUnitsWide())
	targetY = shortestWrappedTarget(cam.Y, targetY, cam.WorldUnitsHigh())

	cam.panActive = true
	cam.panStart = time.Now()
	cam.panDuration = duration
	cam.panFromX, cam.panFromY = cam.X, cam.Y
	cam.panToX, cam.panToY = targetX, targetY
}

func (cam *Camera) UpdatePan() {
	if !cam.panActive {
		return
	}
	elapsed := time.Since(cam.panStart)
	if elapsed >= cam.panDuration {
		cam.X = cam.panToX
		cam.Y = cam.panToY
		cam.panActive = false
		return
	}
	t := float64(elapsed) / float64(cam.panDuration)
	eased := easeInOutCubic(t)
	cam.X = cam.panFromX + (cam.panToX-cam.panFromX)*eased
	cam.Y = cam.panFromY + (cam.panToY-cam.panFromY)*eased
}

// shortestWrappedTarget returns the (possibly out-of-range) target coordinate that produces the shortest signed delta from `from`, given a toroidal world of size `world`.
func shortestWrappedTarget(from, to, world float64) float64 {
	if world <= 0 {
		return to
	}
	delta := math.Mod(to-from, world)
	if delta > world/2 {
		delta -= world
	} else if delta < -world/2 {
		delta += world
	}
	return from + delta
}

// easeInOutCubic is a slow→fast→slow easing curve.
func easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	p := 2*t - 2
	return 1 + p*p*p/2
}
