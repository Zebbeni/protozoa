package ux

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

// Hot-reload tuning. Polling is cheap (a couple of directory listings per
// second) and good enough for a dev tool; we wait a short settle delay
// after the newest mtime so we don't try to decode a file that's still
// being written by a generator or image editor.
const (
	hotReloadPollInterval = 1 * time.Second
	hotReloadSettleDelay  = 200 * time.Millisecond
)

// hotReloadDirs lists the sprite directories watched for mtime changes.
var hotReloadDirs = []string{
	"resources/images/grid/4x4",
	"resources/images/grid/16x16",
}

// AnimationTest is a standalone ebiten.Game for previewing every organism
// animation in isolation. No simulation runs; a fixed matrix of demo cells
// (3 sizes × N animations) loops the same animation forever so the user can
// inspect each sprite sheet visually. Zoom and a small color palette let
// them change the sprite set and tint.
type AnimationTest struct {
	spriteSet int // 0 = 4x4 sprites, 1 = 16x16 sprites
	unitSize  int // pixels per cell before GridDisplayScale
	color     colorful.Color
	palette   []colorful.Color
	swatches  []swatchRect
	startTime time.Time
	windowW   int
	windowH   int

	// Pan offset applied to the matrix (labels + sprites). Color picker and
	// hint line stay anchored to the window. Held as float64 so arrow-key
	// panning can accumulate fractional deltas cleanly.
	panX, panY   float64
	dragging     bool
	lastDragPos  image.Point

	// Hot-reload state.
	lastMaxMTime time.Time // newest mtime observed on the last successful scan
	nextPollAt   time.Time // wall-clock moment for the next scan
}

type swatchRect struct {
	x, y, w, h int
	color      colorful.Color
}

// demoCell pairs a role with an animation to preview.
type demoCell struct {
	label string
	anim  animation.Animation
}

// demoRoles is the set of organism sizes shown in the matrix. Each is
// replicated once per direction.
var demoRoles = []struct {
	role  resources.ImageRole
	label string
}{
	{resources.RoleOrganismTiny, "TINY"},
	{resources.RoleOrganismSmall, "SMALL"},
	{resources.RoleOrganismMedium, "MEDIUM"},
	{resources.RoleOrganismLarge, "LARGE"},
}

// demoDirections is the direction ordering of the matrix rows. The order
// is chosen so vertically-extending 2-cell sprites (NORTH extends up,
// SOUTH extends down) don't collide across row boundaries — in particular
// we avoid a "SOUTH row immediately above a NORTH row" transition because
// those two would extend into each other. EAST→SOUTH→WEST→NORTH keeps
// every transition either horizontal-to-anything or vertical-to-
// horizontal, and only stacks NORTH rows at the bottom (each N row
// extends into its predecessor's empty buffer, not into a sprite).
var demoDirections = []struct {
	label string
	point utils.Point
}{
	{"E", utils.Point{X: 1, Y: 0}},
	{"S", utils.Point{X: 0, Y: 1}},
	{"W", utils.Point{X: -1, Y: 0}},
	{"N", utils.Point{X: 0, Y: -1}},
}

// demoAnimations is the column ordering (left to right).
var demoAnimations = []demoCell{
	{"IDLE", animation.AnimIdle},
	{"MOVE", animation.AnimMove},
	{"BLOCKED", animation.AnimBlocked},
	{"TURN", animation.AnimTurn},
	{"ATTACK", animation.AnimAttack},
	{"EAT", animation.AnimEat},
	{"CHEMO", animation.AnimChemo},
	{"DIE", animation.AnimDie},
}

// NewAnimationTest builds a demo game starting at 16x16 sprites with the
// first palette color selected.
func NewAnimationTest() *AnimationTest {
	palette := []colorful.Color{
		colorful.HSLuv(0, 0.7, 0.55),    // red
		colorful.HSLuv(30, 0.85, 0.55),  // orange
		colorful.HSLuv(60, 0.9, 0.6),    // yellow
		colorful.HSLuv(120, 0.6, 0.5),   // green
		colorful.HSLuv(180, 0.7, 0.55),  // cyan
		colorful.HSLuv(240, 0.7, 0.55),  // blue
		colorful.HSLuv(290, 0.7, 0.55),  // purple
		colorful.HSLuv(0, 0, 0.85),      // near-white
	}
	a := &AnimationTest{
		spriteSet: 1,
		unitSize:  16,
		color:     palette[0],
		palette:   palette,
		startTime: time.Now(),
	}
	// Seed the hot-reload watermark so we don't reload on the very first
	// tick just because we hadn't scanned yet.
	a.lastMaxMTime, _ = latestSpriteMTime()
	return a
}

// Layout implements ebiten.Game; returns the raw window size so input and
// draw coordinates match the OS window pixel grid.
func (a *AnimationTest) Layout(outsideWidth, outsideHeight int) (int, int) {
	a.windowW = outsideWidth
	a.windowH = outsideHeight
	return outsideWidth, outsideHeight
}

// Update handles zoom (wheel + / -), color-swatch clicks, and polls the
// sprite directories so sheet edits hot-reload into the running preview.
func (a *AnimationTest) Update() error {
	a.pollHotReload()

	// Wheel zoom
	if _, wy := ebiten.Wheel(); wy != 0 {
		if wy > 0 {
			a.zoomIn()
		} else {
			a.zoomOut()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		a.zoomIn()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		a.zoomOut()
	}

	// Arrow-key panning. Held keys accumulate — speed is in screen pixels
	// per frame. Positive panX/panY shifts the matrix right / down.
	const panSpeed = 8.0
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		a.panX += panSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		a.panX -= panSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		a.panY += panSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		a.panY -= panSpeed
	}
	// Home recentres (useful after getting lost while zoomed in).
	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		a.panX, a.panY = 0, 0
	}

	// Mouse input: click on a swatch picks a color; click-and-drag anywhere
	// else pans the matrix. Swatch check happens first so clicks on
	// swatches never start a drag.
	mx, my := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		picked := false
		for _, s := range a.swatches {
			if mx >= s.x && mx < s.x+s.w && my >= s.y && my < s.y+s.h {
				a.color = s.color
				picked = true
				break
			}
		}
		if !picked {
			a.dragging = true
			a.lastDragPos = image.Pt(mx, my)
		}
	}
	if a.dragging && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		a.panX += float64(mx - a.lastDragPos.X)
		a.panY += float64(my - a.lastDragPos.Y)
		a.lastDragPos = image.Pt(mx, my)
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		a.dragging = false
	}
	return nil
}

// Draw paints the color picker, the labelled sprite matrix, and a hint line.
func (a *AnimationTest) Draw(screen *ebiten.Image) {
	// Match the replay viewer's grid area (ux.Interface.Render clears the
	// screen to transparent-black and overlays its layers on top), so
	// colour preview in the demo reads the same way it will in-sim.
	screen.Clear()

	elapsed := time.Since(a.startTime)
	_, frameIdx := animation.LoopProgress(elapsed)

	// Switch the active resource set to match the demo's spriteSet — the
	// main renderer isn't running so we own this global in demo mode.
	resources.SelectZoom(a.spriteSet)

	a.drawColorPicker(screen)
	a.drawMatrix(screen, frameIdx)
	a.drawHint(screen)
}

// scale is the final pixel scale from sprite-native pixels to screen pixels.
// Mirrors the main grid's spriteScale * GridDisplayScale composition so the
// preview matches what the live renderer produces.
func (a *AnimationTest) scale() float64 {
	spriteSize := zoomSpriteSizes[a.spriteSet]
	return float64(a.unitSize) / float64(spriteSize) * float64(GridDisplayScale)
}

// cellPixelSize returns the on-screen size of one grid cell in this preview.
func (a *AnimationTest) cellPixelSize() float64 {
	return float64(a.unitSize) * float64(GridDisplayScale)
}

func (a *AnimationTest) zoomIn() {
	order := []struct {
		spriteSet, unitSize int
	}{
		{0, 4}, {0, 6}, {0, 8}, {0, 12}, {1, 16}, {1, 24}, {1, 32}, {1, 48}, {1, 64},
	}
	for i, step := range order {
		if step.spriteSet == a.spriteSet && step.unitSize == a.unitSize {
			if i+1 < len(order) {
				a.spriteSet = order[i+1].spriteSet
				a.unitSize = order[i+1].unitSize
			}
			return
		}
	}
}

func (a *AnimationTest) zoomOut() {
	order := []struct {
		spriteSet, unitSize int
	}{
		{0, 4}, {0, 6}, {0, 8}, {0, 12}, {1, 16}, {1, 24}, {1, 32}, {1, 48}, {1, 64},
	}
	for i, step := range order {
		if step.spriteSet == a.spriteSet && step.unitSize == a.unitSize {
			if i-1 >= 0 {
				a.spriteSet = order[i-1].spriteSet
				a.unitSize = order[i-1].unitSize
			}
			return
		}
	}
}

// drawColorPicker paints palette swatches along the top of the window and
// records their hitboxes for click handling.
func (a *AnimationTest) drawColorPicker(screen *ebiten.Image) {
	const (
		padding   = 12
		swatchW   = 36
		swatchH   = 24
		gap       = 6
		selBorder = 2
	)
	a.swatches = a.swatches[:0]
	x := padding
	y := padding
	for _, c := range a.palette {
		r, g, b, _ := c.RGBA()
		rect := swatchRect{x: x, y: y, w: swatchW, h: swatchH, color: c}
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(swatchW), float64(swatchH),
			color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255})
		if c == a.color {
			// thin white border to mark the selected swatch
			ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y-selBorder),
				float64(swatchW+2*selBorder), float64(selBorder), color.White)
			ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y+swatchH),
				float64(swatchW+2*selBorder), float64(selBorder), color.White)
			ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y),
				float64(selBorder), float64(swatchH), color.White)
			ebitenutil.DrawRect(screen, float64(x+swatchW), float64(y),
				float64(selBorder), float64(swatchH), color.White)
		}
		a.swatches = append(a.swatches, rect)
		x += swatchW + gap
	}
}

// drawMatrix lays out the (4 directions × 3 sizes) × N-animation grid,
// plus row and column labels. Each column reserves 3 grid-cells of width
// and each row reserves 2 grid-cells of height to accommodate 2-cell
// sprites (move, attack) and their rotated extensions. The matrixTopPx
// leaves a full cell of clearance above the first row so north-extending
// sprites in that row don't paint over the column headers.
//
// The pan offset (panX, panY) shifts everything matrix-related — sprites,
// row labels, column headers — so the user can scroll around when zoomed
// in. UI chrome (color picker, hint line) stays anchored.
func (a *AnimationTest) drawMatrix(screen *ebiten.Image, frameIdx int) {
	cell := a.cellPixelSize()
	colW := cell * 3
	rowH := cell * 2

	const matrixLeftPx = 120
	matrixTopPx := 60 + int(cell) // reserve one cell of space for N extensions in the first row

	leftPx := float64(matrixLeftPx) + a.panX
	topPx := float64(matrixTopPx) + a.panY

	// Column headers drawn above the extension buffer of row 0.
	for c, demo := range demoAnimations {
		x := int(leftPx + float64(c)*colW)
		text.Draw(screen, demo.label, resources.FontSourceCodePro10, x, int(topPx)-int(cell)-8, color.White)
	}

	// Rows: outer = direction, inner = size.
	rowIdx := 0
	for _, dir := range demoDirections {
		for _, roleRow := range demoRoles {
			rowY := topPx + float64(rowIdx)*rowH
			label := dir.label + " " + roleRow.label
			text.Draw(screen, label, resources.FontSourceCodePro10, int(8+a.panX), int(rowY+cell*0.75), color.White)

			for c, demo := range demoAnimations {
				drawX := leftPx + float64(c)*colW
				a.drawDemoSprite(screen, drawX, rowY, roleRow.role, demo.anim, dir.point, frameIdx)
			}
			rowIdx++
		}
	}
}

// drawDemoSprite draws one cell of the matrix. All cells paint at their
// static matrix position — motion (move, attack) lives entirely inside the
// 2-cell spritesheet; rotation applied via drawAnimatedSprite handles
// orientation.
func (a *AnimationTest) drawDemoSprite(screen *ebiten.Image, x, y float64,
	role resources.ImageRole, anim animation.Animation, direction utils.Point, frameIdx int) {

	scale := a.scale()
	cellSize := float64(zoomSpriteSizes[a.spriteSet])

	sprite := resources.Sprite(role, anim, frameIdx)
	drawAnimatedSprite(screen, x, y, sprite, direction, a.color, cellSize, scale)
}

func (a *AnimationTest) drawHint(screen *ebiten.Image) {
	hint := "wheel / + / - : zoom   |   drag or arrows : pan   |   home : recentre   |   sheet edits hot-reload"
	text.Draw(screen, hint, resources.FontSourceCodePro10, 12, a.windowH-12, color.White)
}

// pollHotReload scans hotReloadDirs for the newest .png mtime and, if it's
// newer than what we loaded with AND the write looks settled (older than
// hotReloadSettleDelay), calls resources.ReloadImages. The settle delay
// protects against loading a file that's still being written — important
// because resources.loadImage panics on decode errors.
func (a *AnimationTest) pollHotReload() {
	now := time.Now()
	if now.Before(a.nextPollAt) {
		return
	}
	a.nextPollAt = now.Add(hotReloadPollInterval)

	maxMTime, ok := latestSpriteMTime()
	if !ok {
		return
	}
	if maxMTime.After(a.lastMaxMTime) && time.Since(maxMTime) > hotReloadSettleDelay {
		resources.ReloadImages()
		a.lastMaxMTime = maxMTime
	}
}

// latestSpriteMTime walks the watched sprite directories and returns the
// newest mtime among their .png files. Returns (zero, false) if no files
// were found (e.g. the dirs don't exist yet).
func latestSpriteMTime() (time.Time, bool) {
	var newest time.Time
	found := false
	for _, dir := range hotReloadDirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if !found || info.ModTime().After(newest) {
				newest = info.ModTime()
				found = true
			}
		}
	}
	return newest, found
}
