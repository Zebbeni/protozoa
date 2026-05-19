package ux

import (
	"math"
	"time"

	c "github.com/Zebbeni/protozoa/config"
)

// GridDisplayScale is the universal scale applied to the composed grid image
// when drawing it to the screen. Increase this to make every pixel of the grid
// view (sprites, environment, walls) appear larger without changing zoom levels
// or per-layer memory. All grid-area coordinate math (camera viewport, mouse
// input) is divided by this factor so inputs and world state stay consistent.
const GridDisplayScale = 2

// ZoomLevel represents the discrete zoom levels
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
// Zoom4..Zoom32 render at their own native sprite resolution so the
// renderer never has to upscale past 1x. Zoom64 reuses the 32x32
// sprite set scaled 2x — the highest-detail art the project authors
// without doubling memory for a dedicated 64x sheet.
var zoomUnitSizes = [5]int{4, 8, 16, 32, 64}

// zoomSpriteSet maps each zoom level to the sprite set index
// (0=4x4, 1=8x8, 2=16x16, 3=32x32). Zoom64 reuses sprite set 3, so
// sprites are sampled from the 32x32 sheets and scaled up by
// SpriteScale to fill the 64px cells.
var zoomSpriteSet = [5]int{0, 1, 2, 3, 3}

// zoomSpriteSizes is the native pixel size per sprite set.
var zoomSpriteSizes = [4]int{4, 8, 16, 32}

// zoomSpriteFrameCounts is the per-cycle frame count per sprite set.
// Caps at BaseFramesPerCycle (4): 4x4 → 1, 8x8 → 2, 16x16 and 32x32 → 4.
// 32x32 doesn't add more frames than 16x16 — the extra resolution buys
// detail per frame, not more animation steps. Authored sprite sheets
// contain this many frames per animation tag; the renderer divides
// animation.State.Progress() proportionally across them (see
// animation.SpriteFrameIndex).
var zoomSpriteFrameCounts = [4]int{1, 2, 4, 4}

// SpriteSet returns the sprite set index (0-3) for the current zoom level.
func (cam *Camera) SpriteSet() int {
	return zoomSpriteSet[cam.Zoom]
}

// SpriteFrameCount returns the per-cycle sprite frame count for the
// active sprite set (1 at 4x4, 2 at 8x8, 4 at 16x16 and 32x32).
func (cam *Camera) SpriteFrameCount() int {
	return zoomSpriteFrameCounts[cam.SpriteSet()]
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

	// Smooth-pan animation state. When panActive, UpdatePan
	// interpolates X/Y toward panTo over panDuration starting at
	// panStart. Triggered by PanTo; manual Pan / SetZoom cancel it
	// so the user always wins over an in-flight transition.
	panActive    bool
	panStart     time.Time
	panDuration  time.Duration
	panFromX     float64
	panFromY     float64
	panToX       float64
	panToY       float64
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

// ScreenToGrid converts a screen pixel position (relative to the grid
// viewport area) to world grid coordinates. The world is rendered as
// a tiled wallpaper, so any screen pixel always lands on some grid
// cell — onGrid is always true.
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
// Free panning on both axes — when the world is smaller than the
// viewport the renderer tiles copies of the world to fill the
// viewport, and panning shifts which copy sits where. Cancels any
// in-flight smooth-pan animation so manual input wins.
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

	// Cancel any in-flight smooth pan — the target was computed in the
	// old zoom's units and would be wrong after the change.
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

// PanTo starts a smooth animated pan to centre the camera on
// (gridX, gridY) over the given duration. Wraps the shorter way around
// the toroidal world. Cancels any in-flight animation and replaces it
// with the new target.
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

// UpdatePan advances any active smooth-pan animation. Safe to call
// every frame; no-op when nothing is in flight.
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

// shortestWrappedTarget returns the (possibly out-of-range) target
// coordinate that produces the shortest signed delta from `from`,
// given a toroidal world of size `world`. Lets the smooth-pan
// animation cross the world wrap edge in a straight line instead of
// looping all the way around.
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

// easeInOutCubic is a slow→fast→slow easing curve. Cheap enough for
// per-frame use and visually preferable to linear interpolation.
func easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	p := 2*t - 2
	return 1 + p*p*p/2
}
