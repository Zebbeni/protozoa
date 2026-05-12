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
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
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
// Both light and dark sets are watched so saving an aseprite export
// to either tree triggers a reload regardless of the active theme.
var hotReloadDirs = []string{
	"resources/images/grid_light/4x4",
	"resources/images/grid_light/8x8",
	"resources/images/grid_light/16x16",
	"resources/images/grid_light/32x32",
	"resources/images/grid_dark/4x4",
	"resources/images/grid_dark/8x8",
	"resources/images/grid_dark/16x16",
	"resources/images/grid_dark/32x32",
}

// AnimationTest is a standalone ebiten.Game for previewing every organism
// animation in isolation. No simulation runs; a fixed matrix of demo cells
// (3 sizes × N animations) loops the same animation forever so the user can
// inspect each sprite sheet visually. Zoom and a small color palette let
// them change the sprite set and tint.
type AnimationTest struct {
	spriteSet int // 0 = 4x4 sprites, 1 = 8x8 sprites, 2 = 16x16 sprites, 3 = 32x32 sprites
	unitSize  int // pixels per cell before GridDisplayScale
	color     colorful.Color
	palette   []colorful.Color
	swatches  []swatchRect
	// features is the physiology bitmask used to drive the layered
	// render at 16x16 / 32x32. Updated by clicks on featureButtons.
	// 4x4 / 8x8 fall back to the bare single-layer sprite — their art
	// has no overlays to composite.
	features       physiology.Set
	featureButtons []featureButton
	startTime      time.Time
	windowW        int
	windowH        int

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

// featureButton is one option in the feature-toggle bar. Clicking it
// rebuilds AnimationTest.features by clearing every bit in `tree` and
// then setting the bits along `path` (the root → leaf walk of the
// chosen branch). An empty `path` selects "none" for the tree.
type featureButton struct {
	x, y, w, h int
	label      string
	tree       physiology.Tree
	path       []physiology.Feature
}

// featureTreeRows defines the feature-toggle UI: one row per modality
// tree, in the same top-to-bottom order the renderer stacks them. Each
// row's options are the "none" option plus every node along the tree's
// branches, in physiology.All declaration order. The first non-empty
// path becomes the default selection for organisms-as-drawn-here so
// the matrix renders something interesting out of the box.
var featureTreeRows = []struct {
	label   string
	tree    physiology.Tree
	options []featureRowOption
}{
	{
		label: "BODY",
		tree:  physiology.TreeDefense,
		options: []featureRowOption{
			{"BASIC", nil},
			{"SHELL", []physiology.Feature{physiology.FeatShell}},
			{"SPIKES", []physiology.Feature{physiology.FeatShell, physiology.FeatSpikes}},
			{"CAMO", []physiology.Feature{physiology.FeatShell, physiology.FeatCamouflage}},
		},
	},
	{
		label: "FLAG",
		tree:  physiology.TreeFlagellae,
		options: []featureRowOption{
			{"NONE", nil},
			{"FLAGELLAE", []physiology.Feature{physiology.FeatFlagellae}},
			{"CILIA", []physiology.Feature{physiology.FeatFlagellae, physiology.FeatCilia}},
			{"STINGER", []physiology.Feature{physiology.FeatFlagellae, physiology.FeatStinger}},
		},
	},
	{
		label: "SENS",
		tree:  physiology.TreeSensors,
		options: []featureRowOption{
			{"NONE", nil},
			{"ANTENNAE", []physiology.Feature{physiology.FeatAntennae}},
			{"FEELERS", []physiology.Feature{physiology.FeatAntennae, physiology.FeatFeelers}},
			{"TASTERS", []physiology.Feature{physiology.FeatAntennae, physiology.FeatTasters}},
		},
	},
	{
		label: "TEETH",
		tree:  physiology.TreeTeeth,
		options: []featureRowOption{
			{"NONE", nil},
			{"TEETH", []physiology.Feature{physiology.FeatTeeth}},
			{"FANGS", []physiology.Feature{physiology.FeatTeeth, physiology.FeatFangs}},
			{"TUSKS", []physiology.Feature{physiology.FeatTeeth, physiology.FeatTusks}},
		},
	},
}

type featureRowOption struct {
	label string
	path  []physiology.Feature
}

// setFeatureBranch replaces every bit of tree in s with the bits along
// path. Used by feature-button clicks so each click is a complete
// per-tree pick rather than an additive toggle.
func setFeatureBranch(s physiology.Set, tree physiology.Tree, path []physiology.Feature) physiology.Set {
	for _, f := range physiology.All {
		if physiology.Specs[f].Tree == tree {
			s = s.Without(f)
		}
	}
	for _, f := range path {
		s = s.With(f)
	}
	return s
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
	{"TURN L", animation.AnimTurnLeft},
	{"TURN R", animation.AnimTurnRight},
	{"ATTACK", animation.AnimAttack},
	{"EAT", animation.AnimEat},
	{"EAT FAIL", animation.AnimEatFail},
	{"CHEMO", animation.AnimChemo},
	{"CHEMO FAIL", animation.AnimChemoFail},
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
		spriteSet: 2,
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
	// T toggles light / dark theme — reloads sheets so <theme>_*.png
	// swaps in on the fly for iterating on both palettes side by side.
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		if config.Theme() == "dark" {
			setTheme("light")
		} else {
			setTheme("dark")
		}
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
			for _, b := range a.featureButtons {
				if mx >= b.x && mx < b.x+b.w && my >= b.y && my < b.y+b.h {
					a.features = setFeatureBranch(a.features, b.tree, b.path)
					picked = true
					break
				}
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
	// Match the replay viewer's background — dark mode clears to
	// transparent black, light mode fills with the configured light bg.
	fillThemeBackground(screen)

	elapsed := time.Since(a.startTime)
	_, frameIdx := animation.LoopProgress(elapsed, zoomSpriteFrameCounts[a.spriteSet])

	// Switch the active resource set to match the demo's spriteSet — the
	// main renderer isn't running so we own this global in demo mode.
	resources.SelectZoom(a.spriteSet)

	a.drawColorPicker(screen)
	featureBarBottom := a.drawFeatureBar(screen)
	a.drawMatrix(screen, frameIdx, featureBarBottom)
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
		{0, 4}, {1, 8}, {2, 16}, {3, 32},
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
		{0, 4}, {1, 8}, {2, 16}, {3, 32},
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
				float64(swatchW+2*selBorder), float64(selBorder), themedForeground())
			ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y+swatchH),
				float64(swatchW+2*selBorder), float64(selBorder), themedForeground())
			ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y),
				float64(selBorder), float64(swatchH), themedForeground())
			ebitenutil.DrawRect(screen, float64(x+swatchW), float64(y),
				float64(selBorder), float64(swatchH), themedForeground())
		}
		a.swatches = append(a.swatches, rect)
		x += swatchW + gap
	}
}

// drawFeatureBar paints four labeled rows of selectable feature options
// below the colour picker, one per modality tree. Each option is a small
// box with its label; the currently-selected option in each row gets a
// thin foreground-colour border (mirroring the swatch-selected marker).
// Hitboxes are recorded into a.featureButtons for click handling.
// Returns the y-pixel below the bar so the matrix can anchor under it.
func (a *AnimationTest) drawFeatureBar(screen *ebiten.Image) int {
	const (
		barLeft       = 12
		rowTopPx      = 44 // just below the colour swatches (12 + 24 + 8 pad)
		rowH          = 22
		labelW        = 60
		btnH          = 18
		btnPad        = 6 // x-padding inside button around label
		btnGap        = 4
		selBorder     = 2
		btnRadiusPad  = 2 // tiny vertical offset of label inside btn
	)
	a.featureButtons = a.featureButtons[:0]
	fg := themedForeground()
	for ri, row := range featureTreeRows {
		y := rowTopPx + ri*rowH
		text.Draw(screen, row.label+":", resources.FontSourceCodePro10, barLeft, y+rowH-7, fg)
		x := barLeft + labelW
		for _, opt := range row.options {
			lblW := boundString(resources.FontSourceCodePro10, opt.label).Dx()
			btnW := lblW + 2*btnPad
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(btnW), float64(btnH), themedButtonBackground())
			text.Draw(screen, opt.label, resources.FontSourceCodePro10, x+btnPad, y+btnH-5+btnRadiusPad, fg)
			if a.matchesRowSelection(row.tree, opt.path) {
				ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y-selBorder),
					float64(btnW+2*selBorder), float64(selBorder), fg)
				ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y+btnH),
					float64(btnW+2*selBorder), float64(selBorder), fg)
				ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y),
					float64(selBorder), float64(btnH), fg)
				ebitenutil.DrawRect(screen, float64(x+btnW), float64(y),
					float64(selBorder), float64(btnH), fg)
			}
			a.featureButtons = append(a.featureButtons, featureButton{
				x: x, y: y, w: btnW, h: btnH,
				label: opt.label, tree: row.tree, path: opt.path,
			})
			x += btnW + btnGap
		}
	}
	return rowTopPx + len(featureTreeRows)*rowH
}

// themedButtonBackground returns the colour used to fill feature
// buttons. A subtle tint so the click target is visible against either
// theme background without competing with the row label.
func themedButtonBackground() color.Color {
	if config.IsLightTheme() {
		return color.RGBA{R: 220, G: 220, B: 220, A: 255}
	}
	return color.RGBA{R: 40, G: 40, B: 40, A: 255}
}

// matchesRowSelection reports whether a.features currently matches the
// given path inside tree — i.e. the deepest-held feature in tree is the
// last entry in path (or both are empty, the "none" case). Used to
// highlight the selected button in each feature-bar row.
func (a *AnimationTest) matchesRowSelection(tree physiology.Tree, path []physiology.Feature) bool {
	deepest := a.features.Deepest(tree)
	if len(path) == 0 {
		return deepest == physiology.FeatNone
	}
	return deepest == path[len(path)-1]
}

// drawMatrix lays out the (4 directions × 3 sizes) × N-animation grid,
// plus row and column labels. Each column reserves 3 grid-cells of width
// and each row reserves 2 grid-cells of height to accommodate 2-cell
// sprites (move, attack) and their rotated extensions. matrixTopPx
// leaves a full cell of clearance above the first row so north-extending
// sprites in that row don't paint over the column headers.
//
// The pan offset (panX, panY) shifts everything matrix-related — sprites,
// row labels, column headers — so the user can scroll around when zoomed
// in. UI chrome (color picker, hint line) stays anchored.
func (a *AnimationTest) drawMatrix(screen *ebiten.Image, frameIdx int, chromeBottom int) {
	cell := a.cellPixelSize()
	colW := cell * 3
	rowH := cell * 2

	const matrixLeftPx = 120
	matrixTopPx := chromeBottom + 12 + int(cell) // reserve one cell of space for N extensions in the first row

	leftPx := float64(matrixLeftPx) + a.panX
	topPx := float64(matrixTopPx) + a.panY

	// Column headers drawn above the extension buffer of row 0.
	for c, demo := range demoAnimations {
		x := int(leftPx + float64(c)*colW)
		text.Draw(screen, demo.label, resources.FontSourceCodePro10, x, int(topPx)-int(cell)-8, themedForeground())
	}

	// Rows: outer = direction, inner = size.
	rowIdx := 0
	for _, dir := range demoDirections {
		for _, roleRow := range demoRoles {
			rowY := topPx + float64(rowIdx)*rowH
			label := dir.label + " " + roleRow.label
			text.Draw(screen, label, resources.FontSourceCodePro10, int(8+a.panX), int(rowY+cell*0.75), themedForeground())

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
// orientation. Below minOrganismAnimationUnitSize we pin to frame 0 so
// the demo matches what the live grid renders at the same zoom.
//
// Iterates the same OrganismLayersFor + SpriteLayer path the grid
// renderer uses, so the feature-toggle selections preview correctly at
// 16x16 / 32x32. At 4x4 / 8x8 every layered lookup misses and we fall
// back to resources.Sprite (single LayerBody).
func (a *AnimationTest) drawDemoSprite(screen *ebiten.Image, x, y float64,
	role resources.ImageRole, anim animation.Animation, direction utils.Point, frameIdx int) {

	scale := a.scale()
	cellSize := float64(zoomSpriteSizes[a.spriteSet])

	if a.unitSize < minOrganismAnimationUnitSize {
		frameIdx = 0
	}

	layers := resources.OrganismLayersFor(a.features)
	stampedAny := false
	for _, layer := range layers {
		sprite := resources.SpriteLayer(role, layer, anim, frameIdx)
		if sprite == nil {
			continue
		}
		drawAnimatedSprite(screen, x, y, sprite, direction, a.color, cellSize, scale)
		stampedAny = true
	}
	if !stampedAny {
		sprite := resources.Sprite(role, anim, frameIdx)
		drawAnimatedSprite(screen, x, y, sprite, direction, a.color, cellSize, scale)
	}
}

func (a *AnimationTest) drawHint(screen *ebiten.Image) {
	hint := "wheel / + / - : zoom   |   drag or arrows : pan   |   home : recentre   |   t : toggle theme   |   sheet edits hot-reload"
	hintCol := color.RGBA{R: 230, G: 230, B: 230, A: 255}
	if config.IsLightTheme() {
		hintCol = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	}
	text.Draw(screen, hint, resources.FontSourceCodePro10, 12, a.windowH-12, hintCol)
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
