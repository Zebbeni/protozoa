package ux

import (
	"fmt"
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
// (4 zoom levels × 3 sizes × N animations) loops the same animation forever
// so the user can inspect every sprite sheet visually without zooming. A
// small color palette lets them change the body/feature tints.
type AnimationTest struct {
	color          colorful.Color
	secondaryColor colorful.Color
	palette        []colorful.Color
	swatches       []swatchRect
	// features is the physiology bitmask used to drive the layered
	// render at 16x16 / 32x32. Updated by clicks on featureButtons.
	// 4x4 / 8x8 fall back to the bare single-layer sprite — their art
	// has no overlays to composite.
	features       physiology.Set
	featureButtons []featureButton
	// bgMode selects the background fill: 0 = theme, 1 = low-pH tint,
	// 2 = high-pH tint. The pH options match what the grid env layer
	// paints at MinPh / MaxPh, so the artist can preview how sprites
	// read against the most-saturated environment backgrounds.
	bgMode    int
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

	// cellHits records the screen rect and (zoom, role, anim) tuple for
	// every matrix cell drawn this frame. The Update click handler walks
	// this list to dispatch GIF exports — populated fresh each Draw so
	// pan offsets are accounted for naturally.
	cellHits []cellHit

	// lastExport* are the brief overlay message shown after an export
	// click. lastExportTime gates the message so it fades on its own.
	lastExportMsg  string
	lastExportTime time.Time
}

// cellHit is the click-target metadata for one matrix cell. Stored per
// Draw and consulted in Update so click-to-export resolves directly to
// the tuple drawMatrix already iterates.
type cellHit struct {
	x, y, w, h int
	role       resources.ImageRole
	roleLabel  string
	anim       animation.Animation
	animLabel  string
	spriteSet  int
	nativeCell int
}

type swatchRect struct {
	x, y, w, h int
	color      colorful.Color
	// secondary distinguishes which colour slot the swatch sets. The
	// two rows draw from the same palette but click to different
	// AnimationTest fields (body vs feature overlay tint).
	secondary bool
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

// demoDirection is the orientation every preview sprite is drawn in.
// East lets 2-cell sprites extend rightwards within their column without
// reaching into the row above or below. Direction cycling isn't supported
// because per-row vertical extension would collide with adjacent rows.
var demoDirection = utils.Point{X: 1, Y: 0}

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
		color:          palette[0],
		secondaryColor: palette[2], // yellow — visibly different from primary so the split is obvious by default
		palette:        palette,
		startTime:      time.Now(),
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

// Update handles wheel/arrow-key panning, color-swatch and feature-button
// clicks, and polls the sprite directories so sheet edits hot-reload into
// the running preview.
func (a *AnimationTest) Update() error {
	a.pollHotReload()

	// Mouse wheel pans vertically — the matrix now stacks every zoom level
	// at once, so it's typically taller than the window. Wheel-up scrolls
	// the matrix DOWN (panY increases) so the user sees content above,
	// matching the conventional scroll direction.
	if _, wy := ebiten.Wheel(); wy != 0 {
		a.panY += wy * 24
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
	// R force-reloads every sprite sheet from disk. The mtime poller
	// catches saves automatically, but R is the explicit "I know I
	// changed something, refresh now" path — useful when the polling
	// settle delay slows down a tight art-iterate loop, or when the
	// mtime check missed a change (touched but identical-content save).
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		resources.ReloadImages()
		if t, ok := latestSpriteMTime(); ok {
			a.lastMaxMTime = t
		}
	}
	// B cycles the background through theme → low-pH → high-pH and back
	// so the artist can preview sprites against the most-saturated
	// env-layer fills without firing up the full sim.
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		a.bgMode = (a.bgMode + 1) % 3
	}

	// Mouse input: click on a swatch picks a color; click-and-drag anywhere
	// else pans the matrix. Swatch check happens first so clicks on
	// swatches never start a drag.
	mx, my := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		picked := false
		for _, s := range a.swatches {
			if mx >= s.x && mx < s.x+s.w && my >= s.y && my < s.y+s.h {
				if s.secondary {
					a.secondaryColor = s.color
				} else {
					a.color = s.color
				}
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
			for _, h := range a.cellHits {
				if mx >= h.x && mx < h.x+h.w && my >= h.y && my < h.y+h.h {
					if fpath, err := a.exportCellGif(h); err != nil {
						a.lastExportMsg = "EXPORT FAILED: " + err.Error()
					} else {
						a.lastExportMsg = "SAVED: " + fpath
					}
					a.lastExportTime = time.Now()
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
	// The B hotkey overrides this with the env layer's MinPh / MaxPh
	// fill so the artist can read sprites against the extreme cell
	// tints (weight=1 at the bounds, so the env blend collapses to the
	// pH target colour with no theme background mixed in).
	switch a.bgMode {
	case 1:
		fillPhExtremeBackground(screen, config.MinPh())
	case 2:
		fillPhExtremeBackground(screen, config.MaxPh())
	default:
		fillThemeBackground(screen)
	}

	elapsed := time.Since(a.startTime)
	pickerBottom := a.drawColorPicker(screen)
	featureBarBottom := a.drawFeatureBar(screen, pickerBottom)
	matrixBottom := a.drawMatrix(screen, elapsed, featureBarBottom)
	staticBottom := a.drawStaticSection(screen, matrixBottom)
	a.drawWallDemoSection(screen, staticBottom)
	a.drawHint(screen)
	a.drawExportOverlay(screen)
}

// drawExportOverlay paints a short-lived banner above the hint line when
// the user just clicked a matrix cell to export. The 4-second window is
// long enough to read the saved path but short enough that it doesn't
// linger across a flurry of exports.
func (a *AnimationTest) drawExportOverlay(screen *ebiten.Image) {
	if a.lastExportMsg == "" || time.Since(a.lastExportTime) > 4*time.Second {
		return
	}
	text.Draw(screen, a.lastExportMsg, resources.FontSourceCodePro10, 12, a.windowH-30, themedForeground())
}

// drawColorPicker paints two rows of palette swatches: the first sets
// the body (primary) tint, the second sets the feature-overlay
// (secondary) tint. Returns the y-pixel below the picker so the feature
// bar can anchor under it. Hitboxes are recorded with a `secondary`
// flag so the click handler knows which slot to update.
func (a *AnimationTest) drawColorPicker(screen *ebiten.Image) int {
	const (
		padding   = 12
		labelW    = 60
		swatchW   = 36
		swatchH   = 24
		gap       = 6
		rowGap    = 8
		selBorder = 2
	)
	a.swatches = a.swatches[:0]
	rows := []struct {
		label     string
		selected  colorful.Color
		secondary bool
	}{
		{"BODY", a.color, false},
		{"FEATS", a.secondaryColor, true},
	}
	fg := themedForeground()
	y := padding
	for _, row := range rows {
		text.Draw(screen, row.label+":", resources.FontSourceCodePro10, padding, y+swatchH-7, fg)
		x := padding + labelW
		for _, c := range a.palette {
			r, g, b, _ := c.RGBA()
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(swatchW), float64(swatchH),
				color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255})
			if c == row.selected {
				ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y-selBorder),
					float64(swatchW+2*selBorder), float64(selBorder), fg)
				ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y+swatchH),
					float64(swatchW+2*selBorder), float64(selBorder), fg)
				ebitenutil.DrawRect(screen, float64(x-selBorder), float64(y),
					float64(selBorder), float64(swatchH), fg)
				ebitenutil.DrawRect(screen, float64(x+swatchW), float64(y),
					float64(selBorder), float64(swatchH), fg)
			}
			a.swatches = append(a.swatches, swatchRect{
				x: x, y: y, w: swatchW, h: swatchH,
				color: c, secondary: row.secondary,
			})
			x += swatchW + gap
		}
		y += swatchH + rowGap
	}
	return y
}

// drawFeatureBar paints four labeled rows of selectable feature options
// below the colour picker, one per modality tree. Each option is a small
// box with its label; the currently-selected option in each row gets a
// thin foreground-colour border (mirroring the swatch-selected marker).
// Hitboxes are recorded into a.featureButtons for click handling.
// Returns the y-pixel below the bar so the matrix can anchor under it.
func (a *AnimationTest) drawFeatureBar(screen *ebiten.Image, topPx int) int {
	const (
		barLeft       = 12
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
		y := topPx + ri*rowH
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
	return topPx + len(featureTreeRows)*rowH
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

// drawMatrix lays out the (4 zoom levels × 3 sizes) × N-animation grid,
// plus row and column labels. Each zoom row renders sprites at that
// zoom's native cell size, so all four sprite resolutions are visible
// without switching the active set. Columns align to the largest zoom's
// 2-cell sprite width so animation columns line up across rows —
// smaller-zoom rows have horizontal slack inside each column, which
// trades pack density for legibility.
//
// Each row is sized to fit a 1-cell-tall East-facing sprite plus a
// small inter-row gap; the matrix locks direction to East so there's
// no need to reserve N/S extension space. matrixTopPx leaves room for
// the column headers above row 0 — no extra padding for vertical
// sprite extension.
//
// The pan offset (panX, panY) shifts everything matrix-related — sprites,
// row labels, column headers. UI chrome (color picker, feature bar, hint
// line) stays anchored.
func (a *AnimationTest) drawMatrix(screen *ebiten.Image, elapsed time.Duration, chromeBottom int) int {
	const (
		colGap    = 8  // horizontal gap between animation columns
		rowGap    = 4  // vertical gap between rows
		headerGap = 8  // gap between column headers and first row
	)

	maxNativeCell := zoomSpriteSizes[len(zoomSpriteSizes)-1]
	maxCellPx := float64(maxNativeCell * GridDisplayScale)
	colW := maxCellPx*2 + colGap // 2 cells fits the widest East-facing sprite; gap separates columns

	const matrixLeftPx = 140
	matrixTopPx := chromeBottom + 12 + 16 // chrome padding + ~one header text line above row 0

	leftPx := float64(matrixLeftPx) + a.panX
	rowY := float64(matrixTopPx) + a.panY

	fg := themedForeground()
	a.cellHits = a.cellHits[:0]

	// Column headers anchored just above the first row.
	for c, demo := range demoAnimations {
		x := int(leftPx + float64(c)*colW)
		text.Draw(screen, demo.label, resources.FontSourceCodePro10, x, int(rowY)-headerGap, fg)
	}

	// Rows: outer = zoom level (sprite set), inner = size. Each zoom
	// activates its own resource set before drawing so SpriteLayer
	// lookups hit that set's images.
	for zi, nativeCell := range zoomSpriteSizes {
		cellSize := float64(nativeCell)    // sprite-native px
		scale := float64(GridDisplayScale) // unit/sprite ratio is 1 at the native pair
		cellPx := cellSize * scale         // on-screen px per cell at this zoom
		rowH := cellPx + rowGap            // 1-cell sprite height (East) plus inter-row gap
		_, frameIdx := animation.LoopProgress(elapsed, zoomSpriteFrameCounts[zi])
		resources.SelectZoom(zi)

		for _, roleRow := range demoRoles {
			label := fmt.Sprintf("%dx%d %s", nativeCell, nativeCell, roleRow.label)
			text.Draw(screen, label, resources.FontSourceCodePro10, int(8+a.panX), int(rowY+cellPx*0.75), fg)

			for c, demo := range demoAnimations {
				drawX := leftPx + float64(c)*colW
				a.drawDemoSprite(screen, drawX, rowY, roleRow.role, demoDirection, demo.anim, frameIdx, cellSize, scale, nativeCell)
				a.cellHits = append(a.cellHits, cellHit{
					x: int(drawX), y: int(rowY), w: int(colW), h: int(rowH),
					role: roleRow.role, roleLabel: roleRow.label,
					anim: demo.anim, animLabel: demo.label,
					spriteSet: zi, nativeCell: nativeCell,
				})
			}
			rowY += rowH
		}
	}
	return int(rowY)
}

// staticDemoRoles is the set of non-organism sprites previewed under
// the animation matrix: every food size tier and every wall strength
// tier, in the order the role enum declares them. They're static
// (single-frame) art, tinted with foodColor / wallColor to match the
// live grid.
var staticDemoRoles = []struct {
	role  resources.ImageRole
	label string
}{
	{resources.RoleFoodSmall, "FOOD S"},
	{resources.RoleFoodMedium, "FOOD M"},
	{resources.RoleFoodLarge, "FOOD L"},
	{resources.RoleWallWeak, "WALL W"},
	{resources.RoleWallMedium, "WALL M"},
	{resources.RoleWallStrong, "WALL S"},
}

// drawStaticSection paints food + wall sprites at every zoom level
// directly below the animation matrix. One row per zoom; columns are
// the staticDemoRoles entries side by side. Column widths only need to
// hold a 1-cell sprite (food / walls don't extend), so the layout is
// tighter than the matrix above.
//
// Sprites use the same per-role tint the live grid applies — the
// preview shows the exact colour the artist will see in-game, not the
// user-picked organism palette. Static sprites aren't recorded in
// cellHits — click-to-export is organism-only for now.
func (a *AnimationTest) drawStaticSection(screen *ebiten.Image, topPx int) int {
	const (
		sectionGap = 24
		colGap     = 8
		rowGap     = 4
		headerGap  = 8
	)

	fg := themedForeground()

	maxNativeCell := zoomSpriteSizes[len(zoomSpriteSizes)-1]
	maxCellPx := float64(maxNativeCell * GridDisplayScale)
	colW := maxCellPx + colGap // 1-cell sprite width + small inter-column gap

	const sectionLeftPx = 140 // align with matrixLeftPx
	leftPx := float64(sectionLeftPx) + a.panX
	rowY := float64(topPx+sectionGap) + a.panY

	// Section header above the per-zoom rows.
	text.Draw(screen, "FOOD & WALLS", resources.FontSourceCodePro10, int(8+a.panX), int(rowY)-headerGap-12, fg)

	// Column headers anchored just above the first row.
	for c, sr := range staticDemoRoles {
		x := int(leftPx + float64(c)*colW)
		text.Draw(screen, sr.label, resources.FontSourceCodePro10, x, int(rowY)-headerGap, fg)
	}

	for zi, nativeCell := range zoomSpriteSizes {
		cellSize := float64(nativeCell)
		scale := float64(GridDisplayScale)
		cellPx := cellSize * scale
		rowH := cellPx + rowGap

		label := fmt.Sprintf("%dx%d", nativeCell, nativeCell)
		text.Draw(screen, label, resources.FontSourceCodePro10, int(8+a.panX), int(rowY+cellPx*0.75), fg)

		for c, sr := range staticDemoRoles {
			sprite := resources.SpriteAtZoom(zi, sr.role, animation.AnimIdle, 0)
			if sprite == nil {
				continue
			}
			drawX := leftPx + float64(c)*colW
			drawAnimatedSprite(screen, drawX, rowY, sprite, utils.Point{}, a.tintForStaticRole(sr.role), cellSize, scale)
		}
		rowY += rowH
	}
	return int(rowY)
}

// tintForStaticRole returns the colour to tint food / wall sprites
// with — matches the live grid's foodColor / wallColor so the preview
// is faithful. Walls additionally pick up the pH-tint the live grid
// would apply when the user has cycled the bg to a non-neutral pH via
// the B hotkey, so the preview reflects how walls read against each
// pH extreme. Unknown roles fall back to white (identity tint).
func (a *AnimationTest) tintForStaticRole(role resources.ImageRole) colorful.Color {
	switch role {
	case resources.RoleFoodSmall, resources.RoleFoodMedium, resources.RoleFoodLarge:
		return foodColor
	case resources.RoleWallWeak, resources.RoleWallMedium, resources.RoleWallStrong:
		return wallTintForPh(a.bgPh())
	}
	return colorful.Color{R: 1, G: 1, B: 1}
}

// bgPh maps the B-hotkey bg mode to a pH value to drive sprite tints.
// bgMode 0 (theme) is treated as neutral pH so walls render at the
// untinted wallColor — matching how a fresh neutral cell looks in the
// live grid. bgMode 1 / 2 use the configured pH extremes so the
// preview matches what the live grid would paint at MinPh / MaxPh.
func (a *AnimationTest) bgPh() float64 {
	switch a.bgMode {
	case 1:
		return config.MinPh()
	case 2:
		return config.MaxPh()
	default:
		return (config.MaxPh() + config.MinPh()) / 2.0
	}
}

// wallDemoPattern is the wall-strength grid rendered under the FOOD &
// WALLS section to showcase how the directional connector overlays
// join adjacent walls. The layout is hand-picked to exercise every
// neighbour combination at least once:
//   - isolated pile (no neighbours)
//   - single-direction connectors (up / down / left / right)
//   - two-direction corners (UR, UL, DR, DL)
//   - straight runs (horizontal and vertical)
//   - T-junctions (3 neighbours)
//   - cross (all 4 neighbours)
//
// Strength is uniform across the grid so the only thing that varies
// per cell is which connector overlays are stamped on top of the base.
var wallDemoPattern = [][]int{
	{4, 0, 4, 4, 4, 0, 4, 0},
	{4, 4, 4, 4, 4, 4, 4, 0},
	{0, 0, 4, 0, 4, 0, 0, 4},
}

// drawWallDemoSection paints wallDemoPattern at every zoom level,
// directly below the food + wall row. Each cell's connector
// composition is computed against the pattern itself (not against
// any real sim) so the demo is self-contained. Tint is driven by the
// B-hotkey bg mode via wallTintForPh, mirroring the live grid, so
// the section doubles as a preview of how connector composites read
// against each pH extreme.
func (a *AnimationTest) drawWallDemoSection(screen *ebiten.Image, topPx int) int {
	const (
		sectionGap    = 24
		rowGap        = 6
		headerGap     = 8
		sectionLeftPx = 140 // align with the other static rows
	)
	fg := themedForeground()

	leftPx := float64(sectionLeftPx) + a.panX
	rowY := float64(topPx+sectionGap) + a.panY

	text.Draw(screen, "WALL CONNECTIONS", resources.FontSourceCodePro10, int(8+a.panX), int(rowY)-headerGap-12, fg)

	rows := len(wallDemoPattern)
	if rows == 0 {
		return int(rowY)
	}
	cols := len(wallDemoPattern[0])
	tint := wallTintForPh(a.bgPh())

	for zi, nativeCell := range zoomSpriteSizes {
		cellSize := float64(nativeCell)
		scale := float64(GridDisplayScale)
		cellPx := cellSize * scale

		label := fmt.Sprintf("%dx%d", nativeCell, nativeCell)
		text.Draw(screen, label, resources.FontSourceCodePro10, int(8+a.panX), int(rowY+cellPx*0.75), fg)

		for gy := 0; gy < rows; gy++ {
			for gx := 0; gx < cols; gx++ {
				strength := wallDemoPattern[gy][gx]
				if strength == 0 {
					continue
				}
				cx := leftPx + float64(gx)*cellPx
				cy := rowY + float64(gy)*cellPx

				hasUp := gy > 0 && wallDemoPattern[gy-1][gx] > 0
				hasDown := gy+1 < rows && wallDemoPattern[gy+1][gx] > 0
				hasLeft := gx > 0 && wallDemoPattern[gy][gx-1] > 0
				hasRight := gx+1 < cols && wallDemoPattern[gy][gx+1] > 0

				a.stampWallComposite(screen, zi, cx, cy, strength, cellSize, scale, tint, hasUp, hasDown, hasLeft, hasRight)
			}
		}
		rowY += float64(rows)*cellPx + rowGap
	}
	return int(rowY)
}

// stampWallComposite lays one wall cell (base + directional connector
// overlays) onto target at the given zoom. Mirrors what the live
// grid renderer does in renderWallAt, but takes neighbour booleans
// directly rather than querying a simulation — the animation test
// has no sim and walks a fixed pattern instead.
func (a *AnimationTest) stampWallComposite(target *ebiten.Image, zoom int, x, y float64, strength int, cellSize, scale float64, tint colorful.Color, hasUp, hasDown, hasLeft, hasRight bool) {
	role := wallRoleForStrength(strength)
	if base := resources.SpriteLayerAtZoom(zoom, role, resources.LayerWallBase, animation.AnimIdle, 0); base != nil {
		drawAnimatedSprite(target, x, y, base, utils.Point{}, tint, cellSize, scale)
	}
	for _, c := range []struct {
		active bool
		layer  resources.Layer
	}{
		{hasUp, resources.LayerWallUp},
		{hasDown, resources.LayerWallDown},
		{hasLeft, resources.LayerWallLeft},
		{hasRight, resources.LayerWallRight},
	} {
		if !c.active {
			continue
		}
		sprite := resources.SpriteLayerAtZoom(zoom, role, c.layer, animation.AnimIdle, 0)
		if sprite == nil {
			continue
		}
		drawAnimatedSprite(target, x, y, sprite, utils.Point{}, tint, cellSize, scale)
	}
}

// drawDemoSprite draws one cell of the matrix at the given zoom's native
// sprite size and scale. Below minOrganismAnimationUnitSize we pin to
// frame 0 so the demo matches what the live grid renders at the same
// zoom.
//
// Iterates the same OrganismLayersFor + SpriteLayer path the grid
// renderer uses, so the feature-toggle selections preview correctly at
// 16x16 / 32x32. At 4x4 / 8x8 every layered lookup misses and we fall
// back to resources.Sprite (single LayerBody).
func (a *AnimationTest) drawDemoSprite(screen *ebiten.Image, x, y float64,
	role resources.ImageRole, direction utils.Point, anim animation.Animation, frameIdx int,
	cellSize, scale float64, unitSize int) {

	if unitSize < minOrganismAnimationUnitSize {
		frameIdx = 0
	}

	layers := resources.OrganismLayersFor(a.features)
	stampedAny := false
	for _, layer := range layers {
		sprite := resources.SpriteLayer(role, layer, anim, frameIdx)
		if sprite == nil {
			continue
		}
		col := a.secondaryColor
		if resources.UsesPrimaryColor(layer) {
			col = a.color
		}
		drawAnimatedSprite(screen, x, y, sprite, direction, col, cellSize, scale)
		stampedAny = true
	}
	if !stampedAny {
		sprite := resources.Sprite(role, anim, frameIdx)
		drawAnimatedSprite(screen, x, y, sprite, direction, a.color, cellSize, scale)
	}
}

func (a *AnimationTest) drawHint(screen *ebiten.Image) {
	hint := "click cell : export gif   |   wheel / drag / arrows : pan   |   home : recentre   |   t : toggle theme   |   b : cycle bg (theme / low pH / high pH)   |   r : reload sprites   |   sheet edits hot-reload"
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
