package ux

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/replay"
	r "github.com/Zebbeni/protozoa/resources"
	s "github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/ux/graph"
)

const (
	padding     = 15
	panelWidth  = 400
	panelInnerH = 2000

	titleXOffset = padding
	titleYOffset = padding
	playXOffset  = padding
	playYOffset  = 0

	replayCtrlY      = 55  // Y offset for replay controls (below title)
	replayCtrlHeight = 35  // height of the replay control bar

	graphXOffset = padding
	graphYOffset = 75 // graph sits closely under the timeline / replay controls
	graphWidth   = 370
	graphHeight  = 120

	// statsYOffset positions the single living/dead row below the
	// graph + graph-mode buttons. graphYOffset + graph-title (~10) +
	// graphHeight + buttons-gap (14) + 2 button rows (40) ≈ 260.
	statsXOffset = padding
	statsYOffset = 265

	// MODE / DISPLAY / ORGANISM COLOR / FIND MOST are inline rows: a
	// fixed-width label on the left and a strip of equally-sized
	// buttons filling the rest of the row. Saves vertical space vs the
	// previous header-above + column layouts.
	sectionRowX       = padding
	sectionLabelWidth = 105 // reserved for the row label (FontSourceCodePro12)
	sectionRowHeight  = 16
	sectionRowGap     = 6
	sectionBtnGap     = 4
	sectionBtnsX      = sectionRowX + sectionLabelWidth
	sectionBtnsWidth  = panelWidth - padding - sectionBtnsX
	modeYOffset     = 295
	phColorYOffset  = modeYOffset + sectionRowHeight + sectionRowGap
	displayYOffset  = phColorYOffset + sectionRowHeight + sectionRowGap
	orgColorYOffset = displayYOffset + sectionRowHeight + sectionRowGap
	findMostYOffset = orgColorYOffset + sectionRowHeight + sectionRowGap

	// Selected Statistics section starts below the Find Most row, with
	// a comfortable gap so it reads as its own section rather than a
	// fifth row.
	selectedXOffset = padding
	selectedYOffset = findMostYOffset + sectionRowHeight + 28

	// Portrait window: an animated 96x96 spotlight of the selected
	// organism's sprite, drawn as the third column to the right of two
	// stat columns. The 16x16 sprite is scaled 4x (nearest-neighbour)
	// and centred on its base cell.
	portraitSize       = 96
	portraitSpriteCell = 16
	portraitScale      = 4
	// Asymmetric gaps: a tight 8px between the two stat columns, then
	// a larger 16px before the portrait so it has visible breathing
	// room from the column 2 values.
	selectedColInnerGap = 8
	selectedPortraitGap = 16
	selectedColWidth    = (panelWidth - 2*padding - portraitSize - selectedColInnerGap - selectedPortraitGap) / 2
	selectedCol1X       = padding
	selectedCol2X       = selectedCol1X + selectedColWidth + selectedColInnerGap
	selectedPortraitX   = panelWidth - padding - portraitSize

	// Scrubber dimensions
	scrubberX      = padding
	scrubberW      = panelWidth - padding*2
	scrubberH      = 8
	scrubberHandleW = 4
)

type Panel struct {
	simulation         *s.Simulation
	grid               *Grid
	replayCtrl         *replay.Controller
	previousPanelImage *ebiten.Image
	graph              *graph.Graph
	scrollY            float64
	contentHeight      int // actual height of rendered content

	// graphMode and graphShowSelected pick which graph the panel
	// renders. They're independent of the Grid's view mode (M key),
	// so the user can be inspecting "ORGANISMS ONLY" on the grid
	// while looking at the pH-distribution histogram in the panel.
	graphMode         graph.Mode
	graphShowSelected bool

	// graphButtonRects caches the on-screen hitboxes of the graph
	// mode buttons so handleClick can dispatch without recomputing
	// the layout.
	graphButtonRects []graphButtonHitbox

	// selectButtonRects caches the on-screen hitboxes of the auto-
	// select buttons in the SELECTIONS section above the graph.
	selectButtonRects []selectButtonHitbox

	// displayBtnRects caches hitboxes for the GRID DISPLAY toggle row
	// (ORGANISMS / PH / FOOD).
	displayBtnRects []displayBtnHitbox

	// orgColorBtnRects caches hitboxes for the ORGANISM COLOR radio row
	// (TRUE / PH EFFECT / HEALTH).
	orgColorBtnRects []orgColorBtnHitbox

	// themeButtonRects caches the on-screen hitboxes of the MODE row
	// buttons (DARK / LIGHT theme pickers).
	themeButtonRects []themeButtonHitbox

	// phColorButtonRects caches hitboxes for the PH COLOR row
	// (GREEN/PINK / BLUE/ORANGE palette pickers).
	phColorButtonRects []phColorButtonHitbox

	// selectedTab toggles the bottom-of-panel detail view between the
	// decision tree (0) and the descendant tree (1).
	selectedTab int

	// detailTabRects caches the on-screen hitboxes for the tab buttons.
	detailTabRects []detailTabHitbox

	// descCollapsed tracks which descendant tree subtrees are collapsed
	// in the descendant-tree tab. Keyed by tree node ID. Default
	// (missing) is expanded.
	descCollapsed map[int]bool

	// descRowRects caches the row hitboxes from the descendant-tree
	// tab so handleClick can route clicks to row-select / expand-toggle.
	descRowRects []descRowHitbox

	// showParentRect caches the "Show Parent" button hitbox (one per
	// render). Only populated when the displayed tree root has a parent
	// to walk up to.
	showParentRect *showParentHitbox

	// cachedSel* hold the most recent live snapshot of the selected
	// organism's stats so the panel can keep showing them after death,
	// dimmed. cachedSelID is the ID those values were captured for; the
	// cache is repopulated on every render where a live lookup succeeds.
	cachedSelID        int
	cachedInfo         *organism.Info
	cachedTraits       organism.Traits
	cachedTraitsValid  bool
	cachedDecisionTree *d.Tree

	// descRootID is the displayed root of the descendant tree. 0 means
	// "follow the live selection". Show Parent walks this up the tree.
	descRootID int

	// lastObservedSelectedID lets the panel detect when the simulation's
	// selection changed from outside the descendant tree (grid click,
	// auto-select switch) so descRootID can reset to follow. Tree-internal
	// row clicks set skipNextRootReset to suppress the reset, keeping the
	// user's family-tree view stable as they browse siblings.
	lastObservedSelectedID int
	skipNextRootReset      bool

	// portraitImg is a reusable 96x96 image for the Selected Org
	// portrait. Allocated lazily on first render, cleared and redrawn
	// each frame; reused so we don't churn GPU textures every frame.
	portraitImg *ebiten.Image


	// Reconstruction goroutine state. When the user selects a dead
	// organism in replay mode and we have no cached stats, a goroutine
	// reconstructs the simulation at EndCycle-1 from the nearest
	// snapshot. While in flight reconstructInFlight is true and
	// reconstructForID names the target. Stale results (the user moved
	// on while we were working) are discarded when drained.
	reconstructInFlight bool
	reconstructForID    int
	reconstructResult   chan reconstructPayload
}

// reconstructPayload carries a goroutine-reconstructed organism back to
// the main panel. orgID lets the receiver discard stale results.
type reconstructPayload struct {
	orgID  int
	result replay.ReconstructionResult
}

// detailTabHitbox associates a detail-view tab button's screen rect
// with the tab index it activates.
type detailTabHitbox struct {
	x, y, w, h int
	tab        int
}

// descRowHitbox associates a descendant-tree row with the click
// targets it exposes: the whole row selects the organism, and the
// expand area (when present) toggles the collapsed state.
type descRowHitbox struct {
	rowX, rowY, rowW, rowH int
	expandX, expandW       int // 0 width means no expand button on this row
	nodeID                 int
}

// showParentHitbox caches the "Show Parent" button rect at the top of
// the descendant tree tab, so handleDetailTabClick can dispatch the
// walk-up without recomputing layout.
type showParentHitbox struct {
	x, y, w, h int
	parentID   int
}

// selectButtonHitbox associates a select-mode button's screen rect
// with the auto-select mode it activates.
type selectButtonHitbox struct {
	x, y, w, h int
	sel        mode
}

// displayBtnHitbox associates a GRID DISPLAY toggle button's screen
// rect with the layer it toggles.
type displayBtnHitbox struct {
	x, y, w, h int
	kind       displayToggleKind
}

// orgColorBtnHitbox associates an ORGANISM COLOR radio button's
// screen rect with the colour mode it selects.
type orgColorBtnHitbox struct {
	x, y, w, h int
	color      mode
}

// themeButtonHitbox associates a MODE row button's screen rect with
// the theme name it activates.
type themeButtonHitbox struct {
	x, y, w, h int
	theme      string
}

// phColorButtonHitbox associates a PH COLOR row button's screen rect
// with the palette scheme name it activates.
type phColorButtonHitbox struct {
	x, y, w, h int
	scheme     string
}

// graphButtonHitbox associates a graph mode button's screen rect with
// the (mode, showSelected) pair it activates.
type graphButtonHitbox struct {
	x, y, w, h   int
	mode         graph.Mode
	showSelected bool
}

func NewPanel(sim *s.Simulation, grid *Grid) *Panel {
	return &Panel{
		simulation:             sim,
		grid:                   grid,
		graph:                  graph.NewGraph(sim),
		graphMode:              graph.ModePopulation,
		descCollapsed:          make(map[int]bool),
		cachedSelID:            -1,
		lastObservedSelectedID: -1,
		reconstructForID:       -1,
		// Buffered so the goroutine never blocks on send even if the
		// panel hasn't drained yet (e.g. stalled render).
		reconstructResult: make(chan reconstructPayload, 4),
	}
}

// graphModeButton describes one button in the strip beneath the graph.
type graphModeButton struct {
	label        string
	mode         graph.Mode
	showSelected bool
}

// graphModeButtons is the static layout of buttons under the graph,
// in the order they're rendered (left → right, top → bottom). Two
// rows of three so each label has room to breathe at panel scale.
var graphModeButtons = [...]graphModeButton{
	{label: "POP (all)", mode: graph.ModePopulation, showSelected: false},
	{label: "POP (sel)", mode: graph.ModePopulation, showSelected: true},
	{label: "PH HIST", mode: graph.ModePh, showSelected: false},
}

const (
	graphButtonRowHeight = 16
	graphButtonGap       = 4
	graphButtonsPerRow   = 3
)

// selectModeButton is one entry in the FIND MOST button strip. With
// "FIND MOST" as the row title the per-button labels drop the "MOST"
// prefix to fit comfortably across five columns.
type selectModeButton struct {
	label string
	sel   mode
}

var selectModeButtons = [...]selectModeButton{
	{label: "OLDEST", sel: selectOldest},
	{label: "CHILDREN", sel: selectMostChildren},
	{label: "TRAVELED", sel: selectMostTraveled},
	{label: "ATTACKS", sel: selectMostAggressive},
	{label: "SUCCESSFUL", sel: selectMostSuccessful},
}

// displayToggleKind tags each button in the GRID DISPLAY toggle row
// with the layer it controls.
type displayToggleKind int

const (
	displayOrganisms displayToggleKind = iota
	displayPh
	displayFood
	displayWalls
)

var displayToggles = [...]struct {
	label string
	kind  displayToggleKind
}{
	{label: "ORGANISMS", kind: displayOrganisms},
	{label: "PH", kind: displayPh},
	{label: "FOOD", kind: displayFood},
	{label: "WALLS", kind: displayWalls},
}

var orgColorButtons = [...]struct {
	label string
	color mode
}{
	{label: "TRUE", color: orgColorTrue},
	{label: "PH EFFECT", color: orgColorPhEffect},
	{label: "HEALTH", color: orgColorHealth},
}

// graphModeLabel returns the title rendered above the graph for the
// given (mode, showSelected) pair.
func graphModeLabel(mode graph.Mode, showSelected bool) string {
	switch mode {
	case graph.ModePopulation:
		if showSelected {
			return "POPULATION (SELECTED)"
		}
		return "POPULATION HISTORY"
	case graph.ModePh:
		return "PH DISTRIBUTION"
	}
	return ""
}

// renderGraphButtons paints the strip of mode-selection buttons below
// the graph and refreshes Panel.graphButtonRects so handleGraphButtonClick
// has up-to-date hitboxes.
func (p *Panel) renderGraphButtons(panelImage *ebiten.Image, topY int) {
	totalWidth := graphWidth
	btnW := (totalWidth - graphButtonGap*(graphButtonsPerRow-1)) / graphButtonsPerRow

	rects := make([]graphButtonHitbox, 0, len(graphModeButtons))
	for i, b := range graphModeButtons {
		row := i / graphButtonsPerRow
		col := i % graphButtonsPerRow
		x := graphXOffset + col*(btnW+graphButtonGap)
		y := topY + row*(graphButtonRowHeight+graphButtonGap)

		active := b.mode == p.graphMode && b.showSelected == p.graphShowSelected
		drawGraphButton(panelImage, x, y, btnW, graphButtonRowHeight, b.label, active)

		rects = append(rects, graphButtonHitbox{
			x: x, y: y, w: btnW, h: graphButtonRowHeight,
			mode:         b.mode,
			showSelected: b.showSelected,
		})
	}
	p.graphButtonRects = rects
}

// drawGraphButton renders one mode-selection button. Active button
// gets a brighter fill; the rest stay subdued so the active choice
// reads at a glance. Both palettes share the same blue accent so the
// active state is visible regardless of theme.
func drawGraphButton(img *ebiten.Image, x, y, w, h int, label string, active bool) {
	bg := chrome(
		color.RGBA{R: 35, G: 35, B: 45, A: 255},
		color.RGBA{R: 215, G: 215, B: 220, A: 255},
	)
	fg := chrome(
		color.RGBA{R: 170, G: 170, B: 180, A: 255},
		color.RGBA{R: 70, G: 70, B: 80, A: 255},
	)
	if active {
		bg = chrome(
			color.RGBA{R: 70, G: 90, B: 130, A: 255},
			color.RGBA{R: 110, G: 140, B: 200, A: 255},
		)
		fg = chrome(
			color.RGBA{R: 235, G: 235, B: 245, A: 255},
			color.RGBA{R: 250, G: 250, B: 255, A: 255},
		)
	}
	ebitenutil.DrawRect(img, float64(x), float64(y), float64(w), float64(h), bg)
	bounds := boundString(r.FontSourceCodePro8, label)
	tx := x + (w-bounds.Dx())/2
	ty := y + (h+bounds.Dy())/2
	text.Draw(img, label, r.FontSourceCodePro8, tx, ty, fg)
}

// handleGraphButtonClick consumes a click at (mx, my) if it landed on
// any graph-mode button. Returns true on hit. Caller must have
// scroll-adjusted my already.
func (p *Panel) handleGraphButtonClick(mx, my int) bool {
	for _, r := range p.graphButtonRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			p.graphMode = r.mode
			p.graphShowSelected = r.showSelected
			return true
		}
	}
	return false
}

// sectionRowButtonRect computes the (x, w) for the idx-th button in a
// row of count buttons, fitting the area to the right of the row label.
// Buttons share the same height (sectionRowHeight) so callers only need
// a y-coord.
func sectionRowButtonRect(idx, count int) (x, w int) {
	totalGap := sectionBtnGap * (count - 1)
	w = (sectionBtnsWidth - totalGap) / count
	x = sectionBtnsX + idx*(w+sectionBtnGap)
	return x, w
}

// drawRowLabel paints a row's left-aligned label vertically centred
// against the button strip on its right.
func drawRowLabel(panelImage *ebiten.Image, label string, y int) {
	bounds := boundString(r.FontSourceCodePro12, label)
	// FontSourceCodePro12 is drawn with baseline at y; offset to centre
	// against the button row whose top is also at y.
	ty := y + sectionRowHeight/2 + bounds.Dy()/2
	text.Draw(panelImage, label, r.FontSourceCodePro12, sectionRowX, ty, themedForeground())
}

// renderMode paints the MODE row: a label plus DARK / LIGHT theme
// buttons sharing the row to its right.
func (p *Panel) renderMode(panelImage *ebiten.Image, yOff int) {
	y := modeYOffset + yOff
	drawRowLabel(panelImage, "MODE", y)

	current := config.Theme()
	entries := [...]struct{ label, theme string }{
		{"DARK", "dark"},
		{"LIGHT", "light"},
	}
	rects := make([]themeButtonHitbox, 0, len(entries))
	for i, e := range entries {
		x, w := sectionRowButtonRect(i, len(entries))
		drawGraphButton(panelImage, x, y, w, sectionRowHeight, e.label, current == e.theme)
		rects = append(rects, themeButtonHitbox{
			x: x, y: y, w: w, h: sectionRowHeight, theme: e.theme,
		})
	}
	p.themeButtonRects = rects
}

// renderPhColor paints the PH COLOR row: a label plus two palette
// pickers (GREEN/PINK and BLUE/ORANGE). Blue/Orange is the
// colour-blind-safer alternative.
func (p *Panel) renderPhColor(panelImage *ebiten.Image, yOff int) {
	y := phColorYOffset + yOff
	drawRowLabel(panelImage, "PH COLOR", y)

	current := config.PhColorScheme()
	entries := [...]struct{ label, scheme string }{
		{"GREEN/PINK", config.PhColorSchemeGreenPink},
		{"BLUE/ORANGE", config.PhColorSchemeBlueOrange},
	}
	rects := make([]phColorButtonHitbox, 0, len(entries))
	for i, e := range entries {
		x, w := sectionRowButtonRect(i, len(entries))
		drawGraphButton(panelImage, x, y, w, sectionRowHeight, e.label, current == e.scheme)
		rects = append(rects, phColorButtonHitbox{
			x: x, y: y, w: w, h: sectionRowHeight, scheme: e.scheme,
		})
	}
	p.phColorButtonRects = rects
}

// handlePhColorButtonClick consumes a click on a pH-colour palette
// button. On hit it swaps the active scheme and invalidates everything
// that caches a pH-derived colour: descendant tree node tints, the
// graph caches, and the grid env layer.
func (p *Panel) handlePhColorButtonClick(mx, my int) bool {
	for _, r := range p.phColorButtonRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			if config.PhColorScheme() == r.scheme {
				return true
			}
			config.SetPhColorScheme(r.scheme)
			// Force cached graph images to rebuild so the pH
			// histogram picks up the new palette.
			if p.graph != nil {
				p.graph.Invalidate()
			}
			// Grid env layer caches per-cell pH colours; refreshing
			// the whole grid is the simplest invalidation path.
			p.grid.doRefresh = true
			return true
		}
	}
	return false
}

// handleThemeButtonClick consumes a click on a theme button. Returns
// true on hit. Caller must have scroll-adjusted my.
func (p *Panel) handleThemeButtonClick(mx, my int) bool {
	for _, r := range p.themeButtonRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			setTheme(r.theme)
			p.grid.doRefresh = true
			return true
		}
	}
	return false
}

// renderDisplay paints the GRID DISPLAY row: layer toggles for
// ORGANISMS / PH / FOOD, each independently on/off.
func (p *Panel) renderDisplay(panelImage *ebiten.Image, yOff int) {
	y := displayYOffset + yOff
	drawRowLabel(panelImage, "GRID DISPLAY", y)

	rects := make([]displayBtnHitbox, 0, len(displayToggles))
	for i, b := range displayToggles {
		x, w := sectionRowButtonRect(i, len(displayToggles))
		var active bool
		switch b.kind {
		case displayOrganisms:
			active = p.grid.showOrganisms
		case displayPh:
			active = p.grid.showPh
		case displayFood:
			active = p.grid.showFood
		case displayWalls:
			active = p.grid.showWalls
		}
		drawGraphButton(panelImage, x, y, w, sectionRowHeight, b.label, active)
		rects = append(rects, displayBtnHitbox{
			x: x, y: y, w: w, h: sectionRowHeight, kind: b.kind,
		})
	}
	p.displayBtnRects = rects
}

// handleDisplayButtonClick toggles the matching layer flag on hit.
func (p *Panel) handleDisplayButtonClick(mx, my int) bool {
	for _, r := range p.displayBtnRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			switch r.kind {
			case displayOrganisms:
				p.grid.showOrganisms = !p.grid.showOrganisms
			case displayPh:
				p.grid.showPh = !p.grid.showPh
			case displayFood:
				p.grid.showFood = !p.grid.showFood
			case displayWalls:
				p.grid.showWalls = !p.grid.showWalls
			}
			p.grid.doRefresh = true
			return true
		}
	}
	return false
}

// renderOrgColor paints the ORGANISM COLOR radio row: TRUE / PH EFFECT
// / HEALTH, exclusive choice, only meaningful when ORGANISMS is on.
func (p *Panel) renderOrgColor(panelImage *ebiten.Image, yOff int) {
	y := orgColorYOffset + yOff
	drawRowLabel(panelImage, "ORGANISM COLOR", y)

	rects := make([]orgColorBtnHitbox, 0, len(orgColorButtons))
	for i, b := range orgColorButtons {
		x, w := sectionRowButtonRect(i, len(orgColorButtons))
		active := p.grid.orgColor == b.color
		drawGraphButton(panelImage, x, y, w, sectionRowHeight, b.label, active)
		rects = append(rects, orgColorBtnHitbox{
			x: x, y: y, w: w, h: sectionRowHeight, color: b.color,
		})
	}
	p.orgColorBtnRects = rects
}

// handleOrgColorButtonClick sets the active organism colour mode.
func (p *Panel) handleOrgColorButtonClick(mx, my int) bool {
	for _, r := range p.orgColorBtnRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			p.grid.orgColor = r.color
			p.grid.doRefresh = true
			return true
		}
	}
	return false
}

// renderFindMost paints the FIND MOST radio row: the auto-select
// criteria that drive the grid's highlighted organism.
func (p *Panel) renderFindMost(panelImage *ebiten.Image, yOff int) {
	y := findMostYOffset + yOff
	drawRowLabel(panelImage, "FIND MOST", y)

	rects := make([]selectButtonHitbox, 0, len(selectModeButtons))
	for i, b := range selectModeButtons {
		x, w := sectionRowButtonRect(i, len(selectModeButtons))
		active := p.grid.selectMode == b.sel
		drawGraphButton(panelImage, x, y, w, sectionRowHeight, b.label, active)
		rects = append(rects, selectButtonHitbox{
			x: x, y: y, w: w, h: sectionRowHeight, sel: b.sel,
		})
	}
	p.selectButtonRects = rects
}

// handleSelectButtonClick consumes a click at (mx, my) if it landed on
// any select-mode button. Returns true on hit. Caller must have
// scroll-adjusted my already.
func (p *Panel) handleSelectButtonClick(mx, my int) bool {
	for _, r := range p.selectButtonRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			p.grid.selectMode = r.sel
			p.grid.doRefresh = true
			return true
		}
	}
	return false
}

// SetReplayController enables replay controls in the panel.
func (p *Panel) SetReplayController(ctrl *replay.Controller) {
	p.replayCtrl = ctrl
}

// HandleScroll processes mouse wheel input when the cursor is over the panel.
func (p *Panel) HandleScroll() {
	mx, _ := ebiten.CursorPosition()
	if mx >= 0 && mx < panelWidth {
		_, wy := ebiten.Wheel()
		p.scrollY -= wy * 20
		p.clampScroll()
	}
}

func (p *Panel) clampScroll() {
	screenH := config.ScreenHeight()
	maxScroll := float64(p.contentHeight - screenH)
	p.scrollY = max(0, min(p.scrollY, maxScroll))
}

func (p *Panel) replayYOffset() int {
	if p.replayCtrl != nil {
		return replayCtrlHeight
	}
	return 0
}

func (p *Panel) Render() *ebiten.Image {
	screenH := config.ScreenHeight()

	// Render all content onto a tall inner image
	innerImage := ebiten.NewImage(panelWidth, panelInnerH)

	p.renderDividingLine(innerImage, screenH)
	p.renderTitle(innerImage)
	p.renderKeyBindingText(innerImage)
	if p.replayCtrl != nil {
		p.renderReplayControls(innerImage)
	}
	yOff := p.replayYOffset()
	p.renderStats(innerImage, yOff)
	p.renderMode(innerImage, yOff)
	p.renderPhColor(innerImage, yOff)
	p.renderDisplay(innerImage, yOff)
	p.renderOrgColor(innerImage, yOff)
	p.renderFindMost(innerImage, yOff)
	p.renderGraph(innerImage, yOff)
	contentBottom := p.renderSelected(innerImage, yOff)
	p.contentHeight = contentBottom + padding

	// Extract visible portion based on scroll
	p.clampScroll()
	panelImage := ebiten.NewImage(panelWidth, screenH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, -p.scrollY)
	panelImage.DrawImage(innerImage, op)

	// Draw dividing line on the output (not scrolled)
	ebitenutil.DrawRect(panelImage, float64(panelWidth)-1, 0, 1, float64(screenH), themedForeground())

	// Draw scrollbar if content overflows
	if p.contentHeight > screenH {
		p.drawScrollbar(panelImage, screenH)
	}

	return panelImage
}

func (p *Panel) drawScrollbar(panelImage *ebiten.Image, screenH int) {
	scrollbarWidth := 4.0
	scrollbarX := float64(panelWidth) - scrollbarWidth - 3
	trackH := float64(screenH)

	// Thumb size proportional to visible fraction
	visibleFraction := float64(screenH) / float64(p.contentHeight)
	thumbH := max(20, trackH*visibleFraction)

	// Thumb position proportional to scroll position
	maxScroll := float64(p.contentHeight - screenH)
	scrollFraction := 0.0
	if maxScroll > 0 {
		scrollFraction = p.scrollY / maxScroll
	}
	thumbY := scrollFraction * (trackH - thumbH)

	// Track
	ebitenutil.DrawRect(panelImage, scrollbarX, 0, scrollbarWidth, trackH,
		chrome(
			color.RGBA{R: 40, G: 40, B: 40, A: 150},
			color.RGBA{R: 200, G: 200, B: 205, A: 150},
		))
	// Thumb
	ebitenutil.DrawRect(panelImage, scrollbarX, thumbY, scrollbarWidth, thumbH,
		chrome(
			color.RGBA{R: 120, G: 120, B: 120, A: 200},
			color.RGBA{R: 130, G: 130, B: 140, A: 200},
		))
}

func (p *Panel) renderDividingLine(panelImage *ebiten.Image, screenH int) {
	// Drawn on the final output instead, see Render()
}

func (p *Panel) renderReplayControls(panelImage *ebiten.Image) {
	ctrl := p.replayCtrl
	cycle := ctrl.Cycle()
	finalCycle := ctrl.FinalCycle
	paused := p.simulation.IsPaused()

	arrowFg := chrome(
		color.RGBA{R: 180, G: 180, B: 180, A: 255},
		color.RGBA{R: 70, G: 70, B: 80, A: 255},
	)
	mutedFg := chrome(
		color.RGBA{R: 150, G: 150, B: 150, A: 255},
		color.RGBA{R: 100, G: 100, B: 110, A: 255},
	)

	// Scrubber track
	scrubY := replayCtrlY
	ebitenutil.DrawRect(panelImage, float64(scrubberX), float64(scrubY), float64(scrubberW), float64(scrubberH),
		chrome(
			color.RGBA{R: 50, G: 50, B: 60, A: 255},
			color.RGBA{R: 200, G: 205, B: 215, A: 255},
		))

	// Scrubber fill (progress)
	progress := 0.0
	if finalCycle > 0 {
		progress = float64(cycle) / float64(finalCycle)
	}
	fillW := progress * float64(scrubberW)
	ebitenutil.DrawRect(panelImage, float64(scrubberX), float64(scrubY), fillW, float64(scrubberH),
		chrome(
			color.RGBA{R: 80, G: 80, B: 120, A: 255},
			color.RGBA{R: 130, G: 150, B: 200, A: 255},
		))

	// Scrubber handle
	handleX := float64(scrubberX) + fillW - float64(scrubberHandleW)/2
	ebitenutil.DrawRect(panelImage, handleX, float64(scrubY-2), float64(scrubberHandleW), float64(scrubberH+4),
		chrome(
			color.RGBA{R: 180, G: 180, B: 220, A: 255},
			color.RGBA{R: 70, G: 90, B: 150, A: 255},
		))

	// Snapshot markers on the scrubber
	for _, snapCycle := range ctrl.SnapshotCycles() {
		if finalCycle > 0 {
			mx := float64(scrubberX) + float64(snapCycle)/float64(finalCycle)*float64(scrubberW)
			ebitenutil.DrawRect(panelImage, mx, float64(scrubY), 1, float64(scrubberH),
				chrome(
					color.RGBA{R: 150, G: 150, B: 150, A: 100},
					color.RGBA{R: 80, G: 80, B: 90, A: 100},
				))
		}
	}

	// Buttons row below scrubber
	btnY := scrubY + scrubberH + 6
	btnH := 16
	btnGap := 4
	bx := scrubberX

	// Prev cycle
	p.drawButton(panelImage, bx, btnY, 24, btnH, "<<", arrowFg)
	bx += 24 + btnGap

	// Play/Pause
	if paused {
		p.drawButton(panelImage, bx, btnY, 36, btnH, "PLAY", chrome(
			color.RGBA{R: 100, G: 200, B: 100, A: 255},
			color.RGBA{R: 50, G: 140, B: 70, A: 255},
		))
	} else {
		p.drawButton(panelImage, bx, btnY, 36, btnH, "STOP", chrome(
			color.RGBA{R: 200, G: 200, B: 100, A: 255},
			color.RGBA{R: 160, G: 130, B: 30, A: 255},
		))
	}
	bx += 36 + btnGap

	// Next cycle
	p.drawButton(panelImage, bx, btnY, 24, btnH, ">>", arrowFg)
	bx += 24 + btnGap + 8

	// Speed down / up
	p.drawButton(panelImage, bx, btnY, 16, btnH, "-", arrowFg)
	bx += 16 + btnGap

	// Speed indicator
	speedLabel := formatReplaySpeed(ctrl.Speed)
	text.Draw(panelImage, speedLabel, r.FontSourceCodePro10, bx, btnY+btnH-4, chrome(
		color.RGBA{R: 200, G: 200, B: 200, A: 255},
		color.RGBA{R: 60, G: 60, B: 70, A: 255},
	))
	bx += 28

	p.drawButton(panelImage, bx, btnY, 16, btnH, "+", arrowFg)
	bx += 16 + btnGap

	// AUTO toggle — green label when on, dim grey when off. When on, the
	// replay speed is driven by the camera's zoom; when off the speed
	// responds only to the - / + buttons.
	autoLabel := chrome(
		color.RGBA{R: 110, G: 110, B: 110, A: 255},
		color.RGBA{R: 140, G: 140, B: 150, A: 255},
	)
	if ctrl.AutoSpeed {
		autoLabel = chrome(
			color.RGBA{R: 110, G: 200, B: 120, A: 255},
			color.RGBA{R: 50, G: 140, B: 70, A: 255},
		)
	}
	p.drawButton(panelImage, bx, btnY, 32, btnH, "AUTO", autoLabel)
	bx += 32 + btnGap + 8

	// Cycle counter
	cycleLabel := fmt.Sprintf("Cycle %d / %d", cycle, finalCycle)
	text.Draw(panelImage, cycleLabel, r.FontSourceCodePro10, bx, btnY+btnH-4, mutedFg)
}

func (p *Panel) drawButton(img *ebiten.Image, x, y, w, h int, label string, col color.RGBA) {
	// Button background
	ebitenutil.DrawRect(img, float64(x), float64(y), float64(w), float64(h),
		chrome(
			color.RGBA{R: 40, G: 40, B: 50, A: 255},
			color.RGBA{R: 220, G: 220, B: 225, A: 255},
		))
	// Border
	ebitenutil.DrawRect(img, float64(x), float64(y), float64(w), 1,
		chrome(
			color.RGBA{R: 70, G: 70, B: 80, A: 255},
			color.RGBA{R: 200, G: 200, B: 210, A: 255},
		))
	ebitenutil.DrawRect(img, float64(x), float64(y+h-1), float64(w), 1,
		chrome(
			color.RGBA{R: 30, G: 30, B: 35, A: 255},
			color.RGBA{R: 180, G: 180, B: 190, A: 255},
		))
	// Label centered
	bounds := boundString(r.FontSourceCodePro8, label)
	tx := x + (w-bounds.Dx())/2
	ty := y + (h+bounds.Dy())/2
	text.Draw(img, label, r.FontSourceCodePro8, tx, ty, col)
}

// HandleReplayClick handles clicks on replay controls and the graph
// mode button strip. Returns true if consumed.
func (p *Panel) HandleReplayClick(mx, my int) bool {
	// Adjust for scroll once so all hit-tests share the same coords.
	scrolledY := my + int(p.scrollY)

	// Graph, theme, and section-row buttons are independent of replay
	// state — handle them first so they work in non-replay mode too.
	if p.handleSelectButtonClick(mx, scrolledY) {
		return true
	}
	if p.handleDisplayButtonClick(mx, scrolledY) {
		return true
	}
	if p.handleOrgColorButtonClick(mx, scrolledY) {
		return true
	}
	if p.handleThemeButtonClick(mx, scrolledY) {
		return true
	}
	if p.handlePhColorButtonClick(mx, scrolledY) {
		return true
	}
	if p.handleGraphButtonClick(mx, scrolledY) {
		return true
	}
	if p.handleDetailTabClick(mx, scrolledY) {
		return true
	}

	if p.replayCtrl == nil {
		return false
	}

	my = scrolledY

	// Click anywhere in the graph area → seek to the cycle the cursor
	// maps to. Same range math as the hover overlay.
	gY := graphYOffset + p.replayYOffset()
	graphTop := gY + 10
	graphBot := gY + 10 + graphHeight
	if mx >= graphXOffset && mx < graphXOffset+graphWidth && my >= graphTop && my < graphBot {
		startCycle, endCycle := p.graphCycleRange()
		if endCycle > startCycle {
			rel := float64(mx-graphXOffset) / float64(graphWidth)
			target := startCycle + int(rel*float64(endCycle-startCycle))
			p.simulation.Pause(true)
			if err := p.replayCtrl.SeekToCycle(target); err == nil {
				p.grid.doRefresh = true
			}
			return true
		}
	}

	// Check scrubber click (with expanded hit area for easier clicking)
	scrubY := replayCtrlY
	scrubHitPad := 4
	if my >= scrubY-scrubHitPad && my < scrubY+scrubberH+scrubHitPad && mx >= scrubberX && mx < scrubberX+scrubberW {
		progress := float64(mx-scrubberX) / float64(scrubberW)
		targetCycle := int(progress * float64(p.replayCtrl.FinalCycle))
		p.replayCtrl.SeekToCycle(targetCycle)
		p.grid.doRefresh = true
		return true
	}

	// Check button clicks
	btnY := scrubY + scrubberH + 6
	btnH := 16
	btnGap := 4
	bx := scrubberX

	// Prev cycle (width 24). Routes through StepBackward, which uses
	// the controller's in-memory state ring for O(1) rewind in the
	// recent-past window and falls back to SeekToCycle for older
	// cycles outside the ring.
	if p.clickInRect(mx, my, bx, btnY, 24, btnH) {
		p.simulation.Pause(true)
		if p.replayCtrl.StepBackward() {
			p.grid.doRefresh = true
		}
		return true
	}
	bx += 24 + btnGap

	// Play/Pause (width 36)
	if p.clickInRect(mx, my, bx, btnY, 36, btnH) {
		p.simulation.Pause(!p.simulation.IsPaused())
		return true
	}
	bx += 36 + btnGap

	// Next cycle (width 24)
	if p.clickInRect(mx, my, bx, btnY, 24, btnH) {
		p.simulation.Pause(true)
		p.replayCtrl.StepForward()
		return true
	}
	bx += 24 + btnGap + 8

	// Speed down (width 16)
	if p.clickInRect(mx, my, bx, btnY, 16, btnH) {
		speed := p.replayCtrl.Speed / 2
		if speed < replay.MinReplaySpeed {
			speed = replay.MinReplaySpeed
		}
		p.replayCtrl.SetSpeed(speed)
		return true
	}
	bx += 16 + btnGap + 28 // skip speed label

	// Speed up (width 16). Clamp to the per-zoom cap so 16x16 / 8x8
	// sprites can't be pushed past the speeds where they animate cleanly.
	if p.clickInRect(mx, my, bx, btnY, 16, btnH) {
		speed := p.replayCtrl.Speed * 2
		if speed > replay.MaxReplaySpeed {
			speed = replay.MaxReplaySpeed
		}
		if zoomCap := replay.MaxSpeedForUnitSize(p.grid.Camera.GridUnitSize()); speed > zoomCap {
			speed = zoomCap
		}
		p.replayCtrl.SetSpeed(speed)
		return true
	}
	bx += 16 + btnGap

	// AUTO toggle (width 32). Toggles auto-speed on/off; turning on
	// immediately re-anchors speed to the camera's current zoom.
	if p.clickInRect(mx, my, bx, btnY, 32, btnH) {
		if p.replayCtrl.AutoSpeed {
			p.replayCtrl.AutoSpeed = false
		} else {
			p.replayCtrl.EnableAutoSpeed(p.grid.Camera.GridUnitSize())
		}
		return true
	}

	return false
}

func (p *Panel) clickInRect(mx, my, bx, by, w, h int) bool {
	return mx >= bx && mx < bx+w && my >= by && my < by+h
}

// formatReplaySpeed renders a Speed value as a compact display string.
// Sub-1 speeds use fraction notation (1/2x, 1/4x) so they read at a
// glance; integer speeds use the plain "Nx" form.
func formatReplaySpeed(speed float64) string {
	switch speed {
	case 0.25:
		return "1/4x"
	case 0.5:
		return "1/2x"
	}
	if speed == float64(int(speed)) {
		return fmt.Sprintf("%dx", int(speed))
	}
	return fmt.Sprintf("%.2gx", speed)
}

func (p *Panel) renderTitle(panelImage *ebiten.Image) {
	bounds := boundString(r.FontInversionz40, "protozoa")
	// Title borrows the *opposite* theme's inactive-button fill — light
	// grey on dark, dark grey on light — so it reads as a soft accent
	// against the panel chrome instead of disappearing into it.
	titleColor := chrome(
		color.RGBA{R: 215, G: 215, B: 220, A: 255},
		color.RGBA{R: 35, G: 35, B: 45, A: 255},
	)
	text.Draw(panelImage, "protozoa", r.FontInversionz40, titleXOffset, titleYOffset+bounds.Dy(), titleColor)
}

func (p *Panel) renderKeyBindingText(panelImage *ebiten.Image) {
	var lines []string
	if p.simulation.IsPaused() {
		lines = []string{"[Space] to Resume"}
	} else {
		lines = []string{"[Space] to Pause"}
	}

	lineHeight := r.FontSourceCodePro10.Metrics().Height.Round()
	y := titleYOffset + lineHeight
	for _, line := range lines {
		bounds := boundString(r.FontSourceCodePro10, line)
		x := panelWidth - playXOffset - bounds.Dx()
		text.Draw(panelImage, line, r.FontSourceCodePro10, x, y, themedForeground())
		y += lineHeight
	}
}

func (p *Panel) renderStats(panelImage *ebiten.Image, yOff int) {
	// Cycle is shown in the timeline (when present), so we don't repeat
	// it here. In live mode there's no timeline; the cycle is implicit
	// from playback time.
	// Left-pad each count to a fixed width so the ORGANISMS / DEAD / AVG PH
	// labels don't jitter as the values shrink or grow by a digit.
	statsString := fmt.Sprintf("ORGANISMS: %8d   DEAD: %8d   AVG PH: %4.1f",
		p.simulation.OrganismCount(), p.simulation.GetDeadCount(), p.simulation.AveragePh())
	text.Draw(panelImage, statsString, r.FontSourceCodePro12, statsXOffset, statsYOffset+yOff, themedForeground())
}

func (p *Panel) renderGraph(panelImage *ebiten.Image, yOff int) {
	// Graph mode is panel-owned and changes only via the buttons below
	// the graph — not via the grid's view-mode key.
	p.graph.SetMode(p.graphMode)
	p.graph.SetShowSelected(p.graphShowSelected)

	graphMode := p.graphMode
	label := graphModeLabel(graphMode, p.graphShowSelected)

	// Append selected organism ID to sub-tree population titles.
	if p.graphShowSelected && p.graph.HasSelection() && graphMode == graph.ModePopulation {
		label = fmt.Sprintf("%s (ORG ID: %d)", label, p.simulation.GetSelected())
	}

	gY := graphYOffset + yOff

	text.Draw(panelImage, label, r.FontSourceCodePro12, graphXOffset, gY, themedForeground())
	graphImage := p.graph.Render()
	if graphImage == nil {
		return
	}
	graphOptions := &ebiten.DrawImageOptions{}
	scaleX := float64(graphWidth) / float64(graphImage.Bounds().Dx())
	scaleY := float64(graphHeight) / float64(graphImage.Bounds().Dy())
	graphOptions.GeoM.Scale(scaleX, scaleY)
	graphOptions.GeoM.Translate(float64(graphXOffset), float64(gY+10))

	panelImage.DrawImage(graphImage, graphOptions)

	// Draw avg pH label on the pH graph at panel resolution
	if graphMode == graph.ModePh {
		avgPh := p.graph.LastAvgPh()
		if avgPh >= 0 {
			phLabel := fmt.Sprintf("avg: %.1f", avgPh)
			// Map pH to Y within the graph area: MaxPh=top, MinPh=bottom
			phRange := config.MaxPh() - config.MinPh()
			lineY := float64(gY+10) + float64(graphHeight)*(1.0-(avgPh-config.MinPh())/phRange)
			bounds := boundString(r.FontSourceCodePro8, phLabel)
			textX := graphXOffset + graphWidth - bounds.Dx() - 2
			textY := int(lineY) - 2
			if textY < gY+10+bounds.Dy() {
				textY = gY + 10 + bounds.Dy()
			}
			text.Draw(panelImage, phLabel, r.FontSourceCodePro8, textX, textY, themedForeground())
		}
	}

	// Draw start cycle label for selected sub-tree graphs
	if p.graph.HasSelection() && graphMode == graph.ModePopulation {
		startCycle := p.graph.SelectedStartCycle()
		if startCycle >= 0 {
			cycleLabel := fmt.Sprintf("cycle %d", startCycle)
			text.Draw(panelImage, cycleLabel, r.FontSourceCodePro8, graphXOffset+2, gY+10+8, themedForeground())
		}
	}

	// draw border around graph
	left, top, right, bottom := float64(graphXOffset), float64(gY+10), float64(graphXOffset+graphWidth), float64(gY+graphHeight+10)
	ebitenutil.DrawLine(panelImage, left, top, right, top, themedForeground())
	ebitenutil.DrawLine(panelImage, right, top, right, bottom, themedForeground())
	ebitenutil.DrawLine(panelImage, left, bottom, right, bottom, themedForeground())
	ebitenutil.DrawLine(panelImage, left, top, left, bottom, themedForeground())

	// Hover overlay: vertical line + cycle label when the cursor is
	// over the graph. The cursor's panel-inner Y is screen Y +
	// scrollY; X is the same as screen X since the panel sits at
	// screen 0.
	mx, my := ebiten.CursorPosition()
	panelMy := my + int(p.scrollY)
	graphTop := gY + 10
	graphBot := gY + 10 + graphHeight
	if mx >= graphXOffset && mx < graphXOffset+graphWidth && panelMy >= graphTop && panelMy < graphBot {
		startCycle, endCycle := p.graphCycleRange()
		if endCycle > startCycle {
			ebitenutil.DrawRect(panelImage,
				float64(mx), float64(graphTop),
				1, float64(graphHeight),
				themedForegroundDim())
			rel := float64(mx-graphXOffset) / float64(graphWidth)
			cycle := startCycle + int(rel*float64(endCycle-startCycle))
			label := fmt.Sprintf("%d", cycle)
			lb := boundString(r.FontSourceCodePro8, label)
			tx := mx + 3
			if tx+lb.Dx() > graphXOffset+graphWidth {
				tx = mx - 3 - lb.Dx()
			}
			text.Draw(panelImage, label, r.FontSourceCodePro8,
				tx, graphTop+lb.Dy()+2, themedForeground())
		}
	}

	// Graph mode buttons in a 2×3 grid below the graph.
	p.renderGraphButtons(panelImage, gY+graphHeight+14)
}

// graphCycleRange returns the [start, end) cycle range the graph
// currently covers. start depends on whether a sub-tree selection is
// in play (otherwise 0); end is the simulation's current cycle. Used
// to map cursor X position over the graph back to a sim cycle for
// hover labels and click-to-seek.
func (p *Panel) graphCycleRange() (start, end int) {
	start = 0
	if p.graphShowSelected && p.graph.HasSelection() {
		if s := p.graph.SelectedStartCycle(); s >= 0 {
			start = s
		}
	}
	end = p.simulation.Cycle()
	return start, end
}

// renderSelected draws the selected organism info and the active
// detail tab (decision tree or descendant tree). When the selected
// organism is alive, its info is captured into the cache so we can keep
// showing their stats — dimmed — after they die. In replay mode, dead
// organisms with no cached stats trigger a background reconstruction
// from the nearest snapshot. Returns the Y position after the last
// line of content.
func (p *Panel) renderSelected(panelImage *ebiten.Image, yOff int) int {
	p.drainReconstruction()

	sY := selectedYOffset + yOff
	titleHeight := r.FontSourceCodePro12.Metrics().Height.Round()
	infoY := sY + titleHeight + 4

	id := p.simulation.GetSelected()

	// Detect outside-the-tree selection changes so descRootID can reset
	// to follow them. Tree-internal row clicks set skipNextRootReset to
	// suppress this reset (so users can browse siblings within a family
	// without losing the displayed root).
	if id != p.lastObservedSelectedID {
		if !p.skipNextRootReset {
			p.descRootID = 0
		}
		p.skipNextRootReset = false
		p.lastObservedSelectedID = id
	}

	if id < 0 {
		// Nothing selected — skip the section entirely so the panel
		// doesn't show an orphaned title with empty space below.
		p.detailTabRects = nil
		p.descRowRects = nil
		p.showParentRect = nil
		p.cachedSelID = -1
		p.cachedInfo = nil
		p.cachedTraitsValid = false
		p.cachedDecisionTree = nil
		return sY
	}

	// Section title — drawn only when there's something to show below.
	titleStr := fmt.Sprintf("SELECTED ORG: %d", id)
	text.Draw(panelImage, titleStr, r.FontSourceCodePro12, selectedXOffset, sY, themedForeground())

	liveInfo := p.simulation.GetOrganismInfoByID(id)
	liveTraits, traitsFound := p.simulation.GetOrganismTraitsByID(id)
	liveTree := p.simulation.GetOrganismDecisionTreeByID(id)
	alive := liveInfo != nil && traitsFound && liveTree != nil

	// Refresh the cache from the live snapshot whenever it's available.
	// This naturally re-populates when the user steps backward in
	// replay mode and the organism comes back to life.
	if alive {
		p.cachedSelID = id
		p.cachedInfo = liveInfo
		p.cachedTraits = liveTraits
		p.cachedTraitsValid = true
		p.cachedDecisionTree = liveTree
	}

	// Pick the data to render: live values when present, otherwise the
	// last-seen cache for the same id. If neither matches, we have no
	// stats to show (e.g. clicking a long-dead organism we never
	// observed alive in this session).
	var (
		info         *organism.Info
		traits       organism.Traits
		decisionTree *d.Tree
		dim          = !alive
	)
	switch {
	case alive:
		info, traits, decisionTree = liveInfo, liveTraits, liveTree
	case p.cachedSelID == id && p.cachedInfo != nil && p.cachedTraitsValid && p.cachedDecisionTree != nil:
		info, traits, decisionTree = p.cachedInfo, p.cachedTraits, p.cachedDecisionTree
	}

	// The portrait + stats block is always 96px tall regardless of
	// whether we're showing live data, cached/dim data, or a placeholder
	// message — keeps the detail tabs and tree below from jumping when
	// reconstruction completes ~half a second after a click.
	statsLineHeight := r.FontSourceCodePro12.Metrics().Height.Round()
	statsBottom := infoY + portraitSize

	if info == nil || decisionTree == nil {
		// We have a selection but no stats to show. In replay mode we
		// can reconstruct from the nearest snapshot; in live mode (or
		// after a failed reconstruction attempt) we just say so. Either
		// way the descendant tree tab still renders so the user can
		// navigate the family.
		p.detailTabRects = nil
		p.descRowRects = nil
		p.showParentRect = nil

		// cachedSelID == id means we already attempted reconstruction
		// for this organism (success would have populated info above;
		// reaching here means the attempt failed to find them).
		triedThisOrg := p.cachedSelID == id
		canReconstruct := p.replayCtrl != nil && !triedThisOrg
		if canReconstruct && !p.reconstructInFlight {
			p.startReconstruction(id)
		}

		msg := "(no stats — try stepping back to when this organism was alive)"
		if canReconstruct || (p.reconstructInFlight && p.reconstructForID == id) {
			msg = "Reconstructing stats from snapshot..."
		}
		// Centre the message inside the reserved block so the
		// transition from placeholder to real stats doesn't shift the
		// rest of the panel.
		bounds := boundString(r.FontSourceCodePro10, msg)
		msgX := selectedXOffset + (graphWidth-bounds.Dx())/2
		msgY := infoY + portraitSize/2 + bounds.Dy()/2
		text.Draw(panelImage, msg, r.FontSourceCodePro10, msgX, msgY, themedForegroundDim())

		offsetY := statsBottom + 6
		offsetY = p.renderDetailTabs(panelImage, offsetY) + 14
		switch p.selectedTab {
		case 1:
			offsetY = p.renderDescendantTreeTab(panelImage, p.effectiveDescRoot(id), id, offsetY)
		}
		return offsetY
	}

	// Portrait — right column, flush against the right padding.
	p.renderPortrait(panelImage, info, dim, selectedPortraitX, infoY)

	infoColor := themedForeground()
	if dim {
		infoColor = themedForegroundDim()
	}

	// Two stat columns to the left of the portrait. Each line uses
	// `%-Ns %Ms` with N = label-pad width and M = value-pad width, so
	// labels are left-aligned at the column's left edge and values
	// are right-aligned at the column's right edge, lining up across
	// rows.
	const (
		col1LabelW = 9 // "CHILDREN:" / "TRAVELED:" are the longest col1 labels
		col1ValueW = 7 // "X.X-X.X" PH TOL / 7-digit ints / "12.34" health
		col2LabelW = 10 // "PH EFFECT:" is the longest col2 label
		col2ValueW = 8  // "+0.00100" PH EFFECT / "100/200" attacks / "12.34" health
	)

	healthVal := fmt.Sprintf("%5.2f", info.Health)
	ageVal := fmt.Sprintf("%d", info.Age)
	if dim {
		ageVal = fmt.Sprintf("%d (dead)", info.Age)
	}
	tolerance := config.PhTolerance()
	phTolVal := fmt.Sprintf("%1.1f-%1.1f", traits.IdealPh-tolerance, traits.IdealPh+tolerance)
	col1 := fmt.Sprintf("%-*s %*s", col1LabelW, "HEALTH:", col1ValueW, healthVal)
	col1 += fmt.Sprintf("\n%-*s %*s", col1LabelW, "AGE:", col1ValueW, ageVal)
	col1 += fmt.Sprintf("\n%-*s %*d", col1LabelW, "CHILDREN:", col1ValueW, info.Children)
	col1 += fmt.Sprintf("\n%-*s %*d", col1LabelW, "TRAVELED:", col1ValueW, info.TraveledDist)
	col1 += fmt.Sprintf("\n%-*s %*s", col1LabelW, "PH TOL:", col1ValueW, phTolVal)

	sizeVal := fmt.Sprintf("%5.2f", info.Size)
	spawnVal := fmt.Sprintf("%5.2f", traits.MinHealthToSpawn)
	attacksVal := fmt.Sprintf("%d/%d", info.AttackHits, info.AttackTotal)
	// Net cumulative pH push: positive = base-leaning (eating-driven),
	// negative = acid-leaning (chemo-driven).
	phEffVal := fmt.Sprintf("%+.2f", info.PhPositive-info.PhNegative)
	col2 := fmt.Sprintf("%-*s %*s", col2LabelW, "SIZE:", col2ValueW, sizeVal)
	col2 += fmt.Sprintf("\n%-*s %*s", col2LabelW, "SPAWN HP:", col2ValueW, spawnVal)
	col2 += fmt.Sprintf("\n%-*s %*s", col2LabelW, "HITS/ATK:", col2ValueW, attacksVal)
	col2 += fmt.Sprintf("\n%-*s %*s", col2LabelW, "PH EFFECT:", col2ValueW, phEffVal)

	// First baseline shifted down one line height so visual top of
	// line 1 aligns with the top of the portrait beside it.
	statsTextY := infoY + statsLineHeight
	text.Draw(panelImage, col1, r.FontSourceCodePro12, selectedCol1X, statsTextY, infoColor)
	text.Draw(panelImage, col2, r.FontSourceCodePro12, selectedCol2X, statsTextY, infoColor)

	// Overlay coloured health, pH-tolerance, and pH-effect values.
	// Skipped in dim mode (dead organism) — the cached pH at a stale
	// location wouldn't say anything useful, and the consistent dim
	// treatment reads as "snapshot, not live".
	if !dim {
		col1ValuePrefix := fmt.Sprintf("%-*s ", col1LabelW, "")
		col2ValuePrefix := fmt.Sprintf("%-*s ", col2LabelW, "")
		col1ValueX := selectedCol1X + textAdvance(r.FontSourceCodePro12, col1ValuePrefix)
		col2ValueX := selectedCol2X + textAdvance(r.FontSourceCodePro12, col2ValuePrefix)

		// HEALTH — col 1 line 0
		healthOver := fmt.Sprintf("%*s", col1ValueW, healthVal)
		text.Draw(panelImage, healthOver, r.FontSourceCodePro12, col1ValueX, statsTextY, healthColor(info.Health, info.Size))

		// PH TOL — col 1 line 4
		phTolOver := fmt.Sprintf("%*s", col1ValueW, phTolVal)
		phTolY := statsTextY + 4*statsLineHeight
		text.Draw(panelImage, phTolOver, r.FontSourceCodePro12, col1ValueX, phTolY, phIdealTextColor(traits.IdealPh))

		// PH EFFECT — col 2 line 3 — colour derived from the
		// positive/negative imbalance ratio, not the displayed net.
		phEffOver := fmt.Sprintf("%*s", col2ValueW, phEffVal)
		phEffY := statsTextY + 3*statsLineHeight
		text.Draw(panelImage, phEffOver, r.FontSourceCodePro12, col2ValueX, phEffY, phEffectTextColor(info.PhPositive, info.PhNegative))
	}

	offsetY := statsBottom + 6
	offsetY = p.renderFeatures(panelImage, traits.Features, dim, selectedXOffset, offsetY) + 6

	offsetY = p.renderDetailTabs(panelImage, offsetY)
	offsetY += 14

	switch p.selectedTab {
	case 1:
		p.descRowRects = nil
		offsetY = p.renderDescendantTreeTab(panelImage, p.effectiveDescRoot(id), id, offsetY)
	default:
		p.descRowRects = nil
		p.showParentRect = nil
		offsetY = p.renderDecisionTreeTab(panelImage, decisionTree, traits.Features, dim, offsetY)
	}
	return offsetY
}

// renderFeatures draws a small block summarising the organism's
// evolved physiology: one row per modality tree showing the held
// root → advanced path, or a dash for trees the organism hasn't
// entered. Always renders the same number of rows so the detail
// tabs below don't shift as features are gained.
func (p *Panel) renderFeatures(panelImage *ebiten.Image, features physiology.Set, dim bool, x, y int) int {
	col := themedForeground()
	if dim {
		col = themedForegroundDim()
	}
	dimCol := themedForegroundDim()

	lineH := r.FontSourceCodePro12.Metrics().Height.Round()
	cur := y + lineH
	text.Draw(panelImage, "FEATURES:", r.FontSourceCodePro12, x, cur, col)
	cur += lineH

	for _, tree := range physiology.AllTrees {
		path := features.Path(tree)
		label := fmt.Sprintf("  %-10s ", tree.Name()+":")
		text.Draw(panelImage, label, r.FontSourceCodePro12, x, cur, col)
		valueX := x + textAdvance(r.FontSourceCodePro12, label)
		if len(path) == 0 {
			text.Draw(panelImage, "—", r.FontSourceCodePro12, valueX, cur, dimCol)
		} else {
			names := make([]string, len(path))
			for i, f := range path {
				names[i] = physiology.Specs[f].Name
			}
			text.Draw(panelImage, strings.Join(names, " → "), r.FontSourceCodePro12, valueX, cur, col)
		}
		cur += lineH
	}
	return cur
}

// renderPortrait draws an animated 96x96 spotlight of the selected
// organism — a 4x-scaled 16x16 sprite centred on its base cell, picked
// for the organism's current size, action, direction, and animation
// frame. xl multi-cell sprites extend past the frame in their facing
// direction; cropping is OK and clipped at the portrait edge by
// rendering into a dedicated buffer image.
func (p *Panel) renderPortrait(panelImage *ebiten.Image, info *organism.Info, dim bool, x, y int) {
	if p.portraitImg == nil {
		p.portraitImg = ebiten.NewImage(portraitSize, portraitSize)
	}
	// Background: pH colour of the organism's current cell, computed
	// the same way the grid env layer paints it (theme bg blended
	// toward PhTargetColorRGB by distance from neutral).
	p.portraitImg.Fill(portraitPhBackground(p.simulation.GetPhAtPoint(info.Location)))

	// Size-to-role mapping mirrors the grid renderer so the portrait
	// shows the same sprite the grid uses.
	maxSize := config.MaximumMaxSize()
	var role r.ImageRole
	switch {
	case info.Size < maxSize*(1.0/3.0):
		role = r.RoleOrganismSmall
	case info.Size < maxSize*(2.0/3.0):
		role = r.RoleOrganismMedium
	default:
		role = r.RoleOrganismLarge
	}

	// Live organisms get the action's animation and a frame index
	// driven by playback. Dead organisms (dim) freeze on frame 0 of
	// their last action so the portrait reads as a snapshot.
	anim := animation.ForStatus(info.Status)
	frameIdx := 0
	direction := info.Direction
	if !dim && p.grid.animState != nil {
		if frame, ok := p.grid.animState.Frames[info.ID]; ok {
			anim = animation.ForFrame(frame)
			direction = frame.Direction
		}
		// 16x16 sprite set always uses 4 frames per cycle.
		frameIdx = p.grid.animState.SpriteFrameIndex(4)
	}

	// Centre the base cell at the portrait centre. Multi-cell xl
	// sprites extend up from the base in their authored orientation;
	// drawAnimatedSprite rotates around the base anchor, so the
	// extension swings to follow direction. Anything past the
	// portrait edge is clipped by drawing into the dedicated buffer.
	const cellSize = portraitSpriteCell
	const scale = float64(portraitScale)
	baseX := float64(portraitSize)/2 - float64(cellSize)*scale/2
	baseY := float64(portraitSize)/2 - float64(cellSize)*scale/2
	// Portrait is always the 16x16 (high-res) set, so iterate the
	// physiology-driven layers like the grid renderer does. Falls
	// back to the bare default sprite when nothing in that organism's
	// layer list has a PNG on disk yet.
	layers := r.OrganismLayersFor(info.Features)
	stampedAny := false
	for _, layer := range layers {
		sprite := r.SpriteLayerAtZoom(2, role, layer, anim, frameIdx)
		if sprite == nil {
			continue
		}
		drawAnimatedSprite(p.portraitImg, baseX, baseY, sprite, direction, info.Color, float64(cellSize), scale)
		stampedAny = true
	}
	if !stampedAny {
		sprite := r.SpriteAtZoom(2, role, anim, frameIdx)
		drawAnimatedSprite(p.portraitImg, baseX, baseY, sprite, direction, info.Color, float64(cellSize), scale)
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	panelImage.DrawImage(p.portraitImg, op)

	// 1px border around the portrait window. Uses the dim foreground
	// so it reads as a thin frame rather than a heavy outline against
	// either theme.
	border := themedForegroundDim()
	fx, fy, fw, fh := float64(x), float64(y), float64(portraitSize), float64(portraitSize)
	ebitenutil.DrawRect(panelImage, fx, fy, fw, 1, border)
	ebitenutil.DrawRect(panelImage, fx, fy+fh-1, fw, 1, border)
	ebitenutil.DrawRect(panelImage, fx, fy, 1, fh, border)
	ebitenutil.DrawRect(panelImage, fx+fw-1, fy, 1, fh, border)
}

// portraitPhBackground returns the colour the grid env layer would
// paint a cell at the given pH — theme bg blended toward
// PhTargetColorRGB by distance from neutral. Used as a flat fill
// behind the portrait sprite so the spotlight matches the org's
// surrounding pH at a glance.
func portraitPhBackground(ph float64) color.Color {
	neutral := (config.MaxPh() + config.MinPh()) / 2.0
	halfRange := (config.MaxPh() - config.MinPh()) / 2.0
	weight := 0.0
	if halfRange > 0 {
		diff := ph - neutral
		if diff < 0 {
			diff = -diff
		}
		weight = diff / halfRange
		if weight > 1 {
			weight = 1
		}
	}
	bgR, bgG, bgB := config.ThemeBackgroundRGB()
	tR, tG, tB := config.PhTargetColorRGB(ph)
	return color.RGBA{
		R: uint8((weight*tR + (1-weight)*bgR) * 255),
		G: uint8((weight*tG + (1-weight)*bgG) * 255),
		B: uint8((weight*tB + (1-weight)*bgB) * 255),
		A: 255,
	}
}

// effectiveDescRoot returns the ID of the node currently being shown
// as the descendant-tree root. descRootID > 0 wins (set by Show
// Parent); otherwise it tracks the live selection.
func (p *Panel) effectiveDescRoot(selectedID int) int {
	if p.descRootID > 0 {
		return p.descRootID
	}
	return selectedID
}

// startReconstruction launches a background goroutine that rebuilds
// the simulation at the cycle just before the named organism died and
// reads back its stats. No-op if we're not in replay mode, the org's
// tree node can't be found, or it doesn't have a death cycle.
func (p *Panel) startReconstruction(orgID int) {
	if p.replayCtrl == nil {
		return
	}
	node := p.simulation.GetTreeNodeByID(orgID)
	if node == nil || node.EndCycle <= 0 {
		return
	}
	target := node.EndCycle - 1
	if target < 0 {
		target = 0
	}

	p.reconstructInFlight = true
	p.reconstructForID = orgID

	ctrl := p.replayCtrl
	ch := p.reconstructResult
	go func() {
		ch <- reconstructPayload{
			orgID:  orgID,
			result: ctrl.ReconstructOrganismAt(target, orgID),
		}
	}()
}

// drainReconstruction empties the goroutine result channel, applying
// payloads that match the live selection and discarding any others.
// Caching the result (success or failure) on cachedSelID prevents the
// next render from re-triggering the same reconstruction.
func (p *Panel) drainReconstruction() {
	for {
		select {
		case payload := <-p.reconstructResult:
			p.reconstructInFlight = false
			if payload.orgID != p.simulation.GetSelected() {
				// Stale: the user moved on while we were working. Drop
				// the result entirely so it doesn't poison the cache.
				continue
			}
			p.cachedSelID = payload.orgID
			p.cachedInfo = payload.result.Info
			p.cachedTraits = payload.result.Traits
			p.cachedTraitsValid = payload.result.TraitsValid
			p.cachedDecisionTree = payload.result.DecisionTree
		default:
			return
		}
	}
}

// renderDetailTabs draws the two-tab strip (DECISION / DESCENDANT)
// just below the organism info block. Returns the Y after the tabs.
func (p *Panel) renderDetailTabs(panelImage *ebiten.Image, topY int) int {
	tabH := 18
	tabGap := 4
	totalW := graphWidth
	tabW := (totalW - tabGap) / 2
	labels := [2]string{"DECISION TREE", "DESCENDANT TREE"}

	rects := make([]detailTabHitbox, 0, 2)
	for i := 0; i < 2; i++ {
		x := selectedXOffset + i*(tabW+tabGap)
		drawGraphButton(panelImage, x, topY, tabW, tabH, labels[i], i == p.selectedTab)
		rects = append(rects, detailTabHitbox{x: x, y: topY, w: tabW, h: tabH, tab: i})
	}
	p.detailTabRects = rects
	return topY + tabH
}

// renderDecisionTreeTab draws the decision-tree text block in three
// tiers:
//   - active   — UsedLastCycle, the path chooseAction just took
//   - travelled — WasTravelled, ever-visited but not on current path,
//                 so the user can see which branches the organism has
//                 explored over its lifetime distinct from dead code
//   - dim      — never-visited
//
// When dim==true the organism is dead and the cached tree's per-cycle
// flags are stale; everything collapses to the dim foreground.
//
// Nodes whose action / condition is gated by a feature the organism
// doesn't currently hold are rendered with a horizontal strikethrough
// so the user can see "junk DNA" the lineage is still carrying. When
// such a gated node is also on the current path, its line colour is
// replaced by a muted red — a clear visual signal that the tree
// picked a node which fell back to idle / false this cycle.
func (p *Panel) renderDecisionTreeTab(panelImage *ebiten.Image, decisionTree *d.Tree, features physiology.Set, dim bool, topY int) int {
	activeColor := themedForeground()
	// Travelled nodes keep the original "dim" tone so they read as
	// noticeably distinct from never-visited branches.
	travelledColor := chrome(
		color.RGBA{R: 80, G: 80, B: 80, A: 255},
		color.RGBA{R: 170, G: 170, B: 180, A: 255},
	)
	// Untravelled nodes step further toward the background so the
	// "dead branches" of the tree fade out rather than competing with
	// travelled lines for visual weight.
	dimColor := chrome(
		color.RGBA{R: 50, G: 50, B: 55, A: 255},
		color.RGBA{R: 205, G: 205, B: 215, A: 255},
	)
	// Muted red for gated nodes on the active path — the organism
	// chose this node but couldn't actually use it.
	gatedActiveColor := chrome(
		color.RGBA{R: 180, G: 70, B: 70, A: 255},
		color.RGBA{R: 200, G: 90, B: 90, A: 255},
	)
	face := r.FontSourceCodePro10
	lineHeight := face.Metrics().Height.Round()
	// Strike line sits roughly through the x-height middle: half the
	// ascent above the baseline, then nudged down a couple of pixels
	// so it crosses the visual centre of lowercase glyphs rather than
	// floating high on the cap line.
	strikeOffset := face.Metrics().Ascent.Round()/2 - 2
	offsetY := topY
	for _, line := range decisionTree.PrintLines() {
		gated := false
		switch nt := line.NodeType.(type) {
		case d.Action:
			gated = !features.ActionAvailable(nt)
		case d.Condition:
			gated = !features.ConditionAvailable(nt)
		}

		var clr color.Color = dimColor
		if !dim {
			switch {
			case line.UsedLastCycle && gated:
				clr = gatedActiveColor
			case line.UsedLastCycle:
				clr = activeColor
			case line.WasTravelled:
				clr = travelledColor
			}
		}
		text.Draw(panelImage, line.Text, face, selectedXOffset, offsetY, clr)
		if gated {
			labelStart := selectedXOffset + textAdvance(face, line.Prefix)
			labelEnd := selectedXOffset + textAdvance(face, line.Text)
			y := float64(offsetY - strikeOffset)
			ebitenutil.DrawLine(panelImage,
				float64(labelStart), y,
				float64(labelEnd), y,
				clr)
		}
		offsetY += lineHeight
	}
	return offsetY
}

// renderDescendantTreeTab draws an indented, scrollable view of the
// rooted-at-rootID subtree. selectedID is the live selection — its row
// gets a gold highlight so the user can see where they sit in the tree
// even when Show Parent has walked the displayed root above them. Only
// nodes whose StartCycle is at or before the current sim cycle are
// shown. Live nodes use the themed foreground; dead nodes are dimmed.
func (p *Panel) renderDescendantTreeTab(panelImage *ebiten.Image, rootID, selectedID, topY int) int {
	root := p.simulation.GetTreeNodeByID(rootID)
	if root == nil {
		text.Draw(panelImage, "(no descendant tree)", r.FontSourceCodePro10, selectedXOffset, topY+12, themedForegroundDim())
		p.showParentRect = nil
		return topY + 16
	}

	// Show Parent button at the top, only when the displayed root has
	// somewhere to walk up to (root descendant tree nodes have nil
	// Parent).
	y := topY
	p.showParentRect = nil
	if root.Parent != nil {
		btnH := 16
		btnW := 90
		drawGraphButton(panelImage, selectedXOffset, y, btnW, btnH, "SHOW PARENT", false)
		p.showParentRect = &showParentHitbox{
			x: selectedXOffset, y: y, w: btnW, h: btnH,
			parentID: root.Parent.ID,
		}
		y += btnH + 4
	}

	currentCycle := p.simulation.Cycle()
	rows := make([]descRowHitbox, 0, 64)
	rowH := r.FontSourceCodePro10.Metrics().Height.Round() + 2
	expandW := 14
	const maxRows = 1000 // hard cap to keep huge subtrees from blocking the main thread

	var live, dead color.Color = themedForeground(), color.RGBA{R: 90, G: 90, B: 100, A: 255}
	if config.IsLightTheme() {
		dead = color.RGBA{R: 170, G: 170, B: 180, A: 255}
	}

	// Visited guard against malformed trees (cycles or shared sub-trees
	// would otherwise infinitely recurse and crash the render loop).
	visited := make(map[int]struct{})

	truncated := false
	var walk func(n *organism.DescendantNode, depth int)
	walk = func(n *organism.DescendantNode, depth int) {
		if truncated || n == nil {
			return
		}
		if _, seen := visited[n.ID]; seen {
			return
		}
		visited[n.ID] = struct{}{}

		// Skip the entire subtree if this node hasn't been born yet at
		// the current replay cycle. Nodes are appended in spawn order,
		// so a not-yet-born child means siblings born later are also
		// skipped on subsequent recursion (each is its own check).
		if n.StartCycle > currentCycle {
			return
		}

		if len(rows) >= maxRows {
			truncated = true
			return
		}

		indent := depth * 12
		x := selectedXOffset + indent

		// Count born children (those visible at the current cycle).
		bornChildren := 0
		n.ForEachChild(func(c *organism.DescendantNode) {
			if c != nil && c.StartCycle <= currentCycle {
				bornChildren++
			}
		})
		hasChildren := bornChildren > 0

		// [+]/[-] expand toggle.
		var btnX, btnW int
		if hasChildren {
			btnX = x
			btnW = expandW
			label := "-"
			if p.descCollapsed[n.ID] {
				label = "+"
			}
			drawGraphButton(panelImage, btnX, y, btnW, rowH-2, label, false)
		}
		labelX := x + expandW + 4

		alive := p.simulation.GetOrganismInfoByID(n.ID) != nil
		clr := dead
		if alive {
			clr = live
		}
		// Gold pill marks the live selection so the user can spot where
		// they are after Show Parent has walked above them.
		if n.ID == selectedID {
			ebitenutil.DrawRect(panelImage, float64(x), float64(y),
				float64(graphWidth-(x-selectedXOffset)), float64(rowH-2),
				chrome(
					color.RGBA{R: 220, G: 175, B: 60, A: 130},
					color.RGBA{R: 230, G: 180, B: 40, A: 160},
				))
		}

		nodeLabel := fmt.Sprintf("id %d", n.ID)
		if !alive {
			nodeLabel = fmt.Sprintf("id %d (died %d)", n.ID, n.EndCycle)
		}
		text.Draw(panelImage, nodeLabel, r.FontSourceCodePro10, labelX, y+rowH-4, clr)
		// Fake-bold nodes on the most-successful lineage by drawing the
		// label a second time one pixel to the right.
		if p.simulation.IsMostSuccessful(n.ID) {
			text.Draw(panelImage, nodeLabel, r.FontSourceCodePro10, labelX+1, y+rowH-4, clr)
		}

		rows = append(rows, descRowHitbox{
			rowX: x, rowY: y, rowW: graphWidth - (x - selectedXOffset), rowH: rowH,
			expandX: btnX, expandW: btnW,
			nodeID: n.ID,
		})
		y += rowH

		if p.descCollapsed[n.ID] {
			return
		}
		n.ForEachChild(func(c *organism.DescendantNode) {
			walk(c, depth+1)
		})
	}
	walk(root, 0)

	if truncated {
		text.Draw(panelImage, fmt.Sprintf("(truncated at %d rows)", maxRows),
			r.FontSourceCodePro10, selectedXOffset, y+rowH-4, themedForegroundDim())
		y += rowH
	}

	p.descRowRects = rows
	return y
}

// handleDetailTabClick consumes a click on the detail-view tab strip,
// the Show Parent button, or a row in the descendant-tree tab.
// Returns true if consumed.
func (p *Panel) handleDetailTabClick(mx, my int) bool {
	for _, t := range p.detailTabRects {
		if mx >= t.x && mx < t.x+t.w && my >= t.y && my < t.y+t.h {
			p.selectedTab = t.tab
			return true
		}
	}
	if sp := p.showParentRect; sp != nil &&
		mx >= sp.x && mx < sp.x+sp.w && my >= sp.y && my < sp.y+sp.h {
		p.descRootID = sp.parentID
		return true
	}
	for _, row := range p.descRowRects {
		if my < row.rowY || my >= row.rowY+row.rowH {
			continue
		}
		// Expand-toggle takes priority over row-select where they overlap.
		if row.expandW > 0 && mx >= row.expandX && mx < row.expandX+row.expandW {
			p.descCollapsed[row.nodeID] = !p.descCollapsed[row.nodeID]
			return true
		}
		if mx >= row.rowX && mx < row.rowX+row.rowW {
			// Tree-internal selection — keep the displayed root pinned
			// so users can browse siblings/children without losing the
			// view.
			p.skipNextRootReset = true
			p.simulation.Select(row.nodeID)
			p.grid.SetManualSelection()
			p.grid.doRefresh = true
			// Smooth-pan to the picked organism if they're alive on
			// the grid (dead nodes have no location to centre on).
			if info := p.simulation.GetOrganismInfoByID(row.nodeID); info != nil {
				p.grid.Camera.PanTo(info.Location.X, info.Location.Y, time.Second)
			}
			return true
		}
	}
	return false
}
