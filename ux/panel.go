package ux

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/replay"
	r "github.com/Zebbeni/protozoa/resources"
	s "github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/ux/graph"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

const (
	padding     = 15
	panelWidth  = 400
	panelInnerH = 2000

	titleXOffset = padding
	titleYOffset = padding
	playXOffset  = padding
	playYOffset  = 0

	replayCtrlY      = 55 // Y offset for replay controls (below title)
	replayCtrlHeight = 35 // height of the replay control bar

	graphXOffset = padding
	graphYOffset = 75 // graph sits closely under the timeline / replay controls
	graphWidth   = 370
	graphHeight  = 120

	// statsYOffset positions the single organisms/food/pH row below the
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
	modeYOffset       = 295
	phColorYOffset    = modeYOffset + sectionRowHeight + sectionRowGap
	displayYOffset    = phColorYOffset + sectionRowHeight + sectionRowGap
	orgColorYOffset   = displayYOffset + sectionRowHeight + sectionRowGap
	findMostYOffset   = orgColorYOffset + sectionRowHeight + sectionRowGap

	// sectionRowPitch is the vertical step between inline rows. The
	// ability sub-row under ORGANISM COLOR uses it to push everything
	// below down by exactly one row while it's showing.
	sectionRowPitch = sectionRowHeight + sectionRowGap

	// Selected Statistics section starts below the Find Most row, with
	// a comfortable gap so it reads as its own section rather than a
	// fifth row.
	selectedXOffset = padding
	selectedYOffset = findMostYOffset + sectionRowHeight + 28

	// Portrait window: an animated 96x96 spotlight of the selected
	// organism's sprite, drawn as the third column to the right of two
	// stat columns. The 16x16 sprite — the highest resolution the
	// project authors — is scaled 4x (nearest-neighbour) and centred
	// on its base cell, for a 64px on-screen sprite.
	portraitSize       = 96
	portraitSpriteCell = 16
	portraitScale      = 4
	// Health bar under the portrait: healthBarGap below it, healthBarH
	// tall, one segment per healthBarSegment points of size.
	healthBarGap     = 4
	healthBarH       = 8
	healthBarSegment = 10.0
	// Asymmetric gaps: a tight 8px between the two stat columns, then
	// a larger 16px before the portrait so it has visible breathing
	// room from the column 2 values.
	selectedColInnerGap = 8
	selectedPortraitGap = 16
	selectedColWidth    = (panelWidth - 2*padding - portraitSize - selectedColInnerGap - selectedPortraitGap) / 2
	selectedCol1X       = padding
	selectedPortraitX   = panelWidth - padding - portraitSize

	// Tab strips (the DECISION / DESCENDANT strip below the portrait) are
	// drawn by drawTabStrip with these dimensions.
	tabStripHeight = 18
	tabStripGap    = 4
	// infoAreaWidth spans both text columns: everything left of the
	// portrait.
	infoAreaWidth = 2*selectedColWidth + selectedColInnerGap
	// The stats column (label + right-aligned value per row) takes the
	// left of the info area and the abilities column the right, sized at
	// the panel font's 7px per character: "TRAVELED:" beside a 7-digit
	// value, and "Movement" beside "100".
	statsRowCount     = 7
	statsColWidth     = 119
	abilitiesColWidth = 110
	abilitiesColX     = selectedCol1X + infoAreaWidth - abilitiesColWidth
	// The abilities block is a row of vertical bars, one per ability,
	// each with its score above it and its name rotated below.
	abilityBarWidth  = 12
	abilityBarLength = 60
	abilityLabelGap  = 4
	// selectedTitleGap is the space between the SELECTED ORG line and the
	// block below it.
	selectedTitleGap = -6

	// Scrubber dimensions
	scrubberX       = padding
	scrubberW       = panelWidth - padding*2
	scrubberH       = 8
	scrubberHandleW = 4

	// MENU button, top right of the panel in replay mode.
	menuBtnW = 56
	menuBtnH = 16
	menuBtnX = panelWidth - padding - menuBtnW
	menuBtnY = titleYOffset
)

type Panel struct {
	simulation         *s.Simulation
	grid               *Grid
	replayCtrl         *replay.Controller
	previousPanelImage *ebiten.Image
	graph              *graph.Graph
	scrollY            float64
	contentHeight      int // actual height of rendered content

	// menuRequested is set by a click on the MENU button and cleared by
	// TakeMenuRequest.
	menuRequested bool

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

	// lastGraphImage is what the last Render produced, kept so the
	// expanded popup can draw it without asking for another.
	lastGraphImage *ebiten.Image

	// graphExpandRect is the expand button's hitbox, set while the
	// cursor is over the graph and nil otherwise — it only appears on
	// hover, so it is only clickable then.
	graphExpandRect *graphRect

	// graphExpandRequested is set by a click on the expand control and
	// taken by the runner, so the panel doesn't need to know what a
	// popup is.
	graphExpandRequested bool

	// graphByAbility colours the population graphs gray→green by
	// graphColorAbility instead of by lineage. Toggled by the ABILITY
	// button under the graph; the ability choice is kept across toggles.
	graphByAbility    bool
	graphColorAbility physiology.Ability
	// graphAbilityToggleRect / graphAbilityBtnRects are the hitboxes for
	// the ABILITY toggle and, while it's on, its ability sub-row. The
	// sub-row slice is nil while hidden so it can't swallow clicks.
	graphAbilityToggleRect *graphButtonHitbox

	// graphView is the zoomed/panned window of the graph. graphTop is the
	// graph area's top edge in panel-inner coordinates, recorded each
	// render so input handling can hit-test it (0 until first render).
	// graphDragging / graphDragLastX track a pan drag in progress,
	// lastGraphClick detects a double-click reset, and graphCursor is
	// the cursor shape the graph last set, so it only changes the cursor
	// when it needs to.
	// lastTreesGeneration identifies the descendant trees the graph's
	// cached alive sets were built from. A replay reinstalls the same
	// decoded trees on every seek, so this normally never changes; when
	// it does, the cached sets point at nodes that are gone.
	lastTreesGeneration int

	graphView      graphView
	graphTop       int
	graphDragging  bool
	graphDragLastX int
	lastGraphClick time.Time
	graphCursor    ebiten.CursorShapeType
	// graphPressX is where the current press on the graph started, and
	// graphDragMoved whether it has moved far enough to be a pan rather
	// than a click. A click's seek waits in pendingGraphSeek (a fraction
	// of the full range) until pendingGraphSeekAt, so a double-click can
	// reset the zoom without also seeking.
	graphPressX          int
	graphDragMoved       bool
	pendingGraphSeek     float64
	pendingGraphSeekAt   time.Time
	graphAbilityBtnRects []abilityBtnHitbox

	// selectButtonRects caches the on-screen hitboxes of the auto-
	// select buttons in the SELECTIONS section above the graph.
	selectButtonRects []selectButtonHitbox

	// displayBtnRects caches hitboxes for the GRID DISPLAY toggle row
	// (ORGANISMS / PH / FOOD).
	displayBtnRects []displayBtnHitbox

	// orgColorBtnRects caches hitboxes for the ORGANISM COLOR radio row
	// (TRUE / PH EFFECT / HEALTH).
	orgColorBtnRects []orgColorBtnHitbox
	// abilityBtnRects caches hitboxes for the ability sub-row, which
	// only exists while the ABILITY colour mode is active. Nil otherwise,
	// so a hidden row can't swallow clicks.
	abilityBtnRects []abilityBtnHitbox

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
// rect with the toggle it drives.
type displayBtnHitbox struct {
	x, y, w, h int
	toggle     displayToggle
}

// abilityBtnHitbox associates an ability sub-row button's screen rect
// with the ability it selects for the ABILITY colour mode.
type abilityBtnHitbox struct {
	x, y, w, h int
	ability    physiology.Ability
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
	{label: "FOOD", mode: graph.ModeFood, showSelected: false},
	{label: "WALLS", mode: graph.ModeWalls, showSelected: false},
	{label: "PH HIST", mode: graph.ModePh, showSelected: false},
}

// graphPopulationButtons is how many of graphModeButtons sit on the first
// row, beside the ABILITY toggle that recolours them. The rest fill the
// row below — pushed down by the ability picker when it is showing.
const graphPopulationButtons = 2

// graphButtonPitch is the vertical step between graph button rows. The
// ability sub-row under the graph uses it to push everything below down
// by one row while it's showing.
const graphButtonPitch = graphButtonRowHeight + graphButtonGap

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

// displayToggle is one button in the GRID DISPLAY row. Each entry
// carries its own read and write accessors for the Grid flag it owns,
// so adding a layer means adding exactly one table row.
//
// The accessors replaced a pair of parallel switch statements — one in
// renderDisplay deciding whether a button looks active, one in
// handleDisplayButtonClick doing the toggle. Adding FLOW to only the
// second of those shipped a button that toggled its layer correctly
// but never lit up, which is the precise failure a single source of
// truth per toggle prevents.
type displayToggle struct {
	label string
	get   func(*Grid) bool
	set   func(*Grid, bool)
}

var displayToggles = [...]displayToggle{
	{
		label: "ORGANISMS",
		get:   func(g *Grid) bool { return g.showOrganisms },
		set:   func(g *Grid, v bool) { g.showOrganisms = v },
	},
	{
		label: "PH",
		get:   func(g *Grid) bool { return g.showPh },
		set:   func(g *Grid, v bool) { g.showPh = v },
	},
	{
		label: "FOOD",
		get:   func(g *Grid) bool { return g.showFood },
		set:   func(g *Grid, v bool) { g.showFood = v },
	},
	{
		label: "WALLS",
		get:   func(g *Grid) bool { return g.showWalls },
		set:   func(g *Grid, v bool) { g.showWalls = v },
	},
}

var orgColorButtons = [...]struct {
	label string
	color mode
}{
	{label: "TRUE", color: orgColorTrue},
	{label: "PH EFFECT", color: orgColorPhEffect},
	{label: "HEALTH", color: orgColorHealth},
	{label: "TOLERANCE", color: orgColorTolerance},
	{label: "SUCCESS", color: orgColorSuccess},
	{label: "FAMILY", color: orgColorFamily},
	{label: "AGE", color: orgColorAge},
	{label: "SIZE", color: orgColorSize},
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
	case graph.ModeFood:
		return "FOOD HISTORY"
	case graph.ModeWalls:
		return "WALL HISTORY"
	}
	return ""
}

// renderGraphButtons paints the buttons below the graph and refreshes their
// hitboxes. Layout:
//
//	POP (all)  POP (sel)  ABILITY
//	[ability picker, only while ABILITY is on]
//	FOOD       WALLS      PH HIST
//
// ABILITY is a colouring toggle rather than a graph mode, so it sits with
// the population buttons it recolours, and its picker opens directly
// beneath them. Returns how far the sections below must shift: one row
// pitch while the picker is showing, zero otherwise.
// x and totalWidth are passed rather than taken from the panel's own
// geometry so the expanded popup can draw the same buttons at its own
// size. The hitboxes it leaves behind are in whatever space it was
// drawn in, which is safe because only one of the two is interactive at
// a time — the popup is modal and takes input before the panel sees it.
func (p *Panel) renderGraphButtons(panelImage *ebiten.Image, x, topY, totalWidth int) int {
	btnW := (totalWidth - graphButtonGap*(graphButtonsPerRow-1)) / graphButtonsPerRow
	colX := func(col int) int { return x + col*(btnW+graphButtonGap) }

	rects := make([]graphButtonHitbox, 0, len(graphModeButtons))
	drawMode := func(b graphModeButton, x, y int) {
		active := b.mode == p.graphMode && b.showSelected == p.graphShowSelected
		drawGraphButton(panelImage, x, y, btnW, graphButtonRowHeight, b.label, active)
		rects = append(rects, graphButtonHitbox{
			x: x, y: y, w: btnW, h: graphButtonRowHeight,
			mode:         b.mode,
			showSelected: b.showSelected,
		})
	}

	// First row: the population buttons, then the ABILITY toggle.
	for i, b := range graphModeButtons[:graphPopulationButtons] {
		drawMode(b, colX(i), topY)
	}
	abilityX := colX(graphPopulationButtons)
	drawGraphButton(panelImage, abilityX, topY, btnW, graphButtonRowHeight, "ABILITY", p.graphByAbility)
	p.graphAbilityToggleRect = &graphButtonHitbox{x: abilityX, y: topY, w: btnW, h: graphButtonRowHeight}

	// Ability picker directly beneath, spanning the graph width.
	shift := 0
	p.graphAbilityBtnRects = nil
	if p.graphByAbility {
		rowY := topY + graphButtonPitch
		n := len(physiology.AllAbilities)
		abW := (totalWidth - graphButtonGap*(n-1)) / n
		abilityRects := make([]abilityBtnHitbox, 0, n)
		for i, a := range physiology.AllAbilities {
			ax := x + i*(abW+graphButtonGap)
			label, ok := abilityButtonLabels[a]
			if !ok {
				label = strings.ToUpper(a.Name())
			}
			drawGraphButton(panelImage, ax, rowY, abW, graphButtonRowHeight, label, p.graphColorAbility == a)
			abilityRects = append(abilityRects, abilityBtnHitbox{
				x: ax, y: rowY, w: abW, h: graphButtonRowHeight, ability: a,
			})
		}
		p.graphAbilityBtnRects = abilityRects
		shift = graphButtonPitch
	}

	// Remaining modes fill the rows below the picker.
	for i, b := range graphModeButtons[graphPopulationButtons:] {
		row := 1 + i/graphButtonsPerRow
		drawMode(b, colX(i%graphButtonsPerRow), topY+shift+row*graphButtonPitch)
	}
	p.graphButtonRects = rects
	return shift
}

// graphRect is a plain hitbox in whatever space it was drawn in.
type graphRect struct{ x, y, w, h int }

func (r graphRect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// expandButtonSize is the side of the square expand control in the
// graph's top-right corner.
const expandButtonSize = 14

// drawExpandButton puts a small expand control in the graph's top-right
// corner while the cursor is over the graph, and records its hitbox.
//
// Only on hover: it sits over the graph itself, and a control permanently
// covering the corner of a plot is in the way of the thing it is meant to
// help you read.
func (p *Panel) drawExpandButton(dst *ebiten.Image, gx, gy, gw int, hovered bool) {
	if !hovered {
		p.graphExpandRect = nil
		return
	}
	rect := graphRect{x: gx + gw - expandButtonSize - 2, y: gy + 2, w: expandButtonSize, h: expandButtonSize}
	p.graphExpandRect = &rect

	ebitenutil.DrawRect(dst, float64(rect.x), float64(rect.y), float64(rect.w), float64(rect.h),
		chrome(color.RGBA{R: 40, G: 40, B: 52, A: 220}, color.RGBA{R: 235, G: 235, B: 242, A: 220}))

	// Two corner brackets, which read as "make this bigger" without
	// needing a glyph the font may not have.
	ink := themedValue()
	const pad, arm = 3, 5
	l, t := float64(rect.x+pad), float64(rect.y+pad)
	rgt, b := float64(rect.x+rect.w-pad), float64(rect.y+rect.h-pad)
	ebitenutil.DrawRect(dst, l, t, arm, 1, ink)
	ebitenutil.DrawRect(dst, l, t, 1, arm, ink)
	ebitenutil.DrawRect(dst, rgt-arm, b-1, arm, 1, ink)
	ebitenutil.DrawRect(dst, rgt-1, b-arm, 1, arm, ink)
}

// drawGraphInto draws the graph image into a rect at any size: the
// horizontal zoom window, cropped vertically to the band the data
// actually uses, with the playhead over it.
//
// Extracted so the expanded popup is the same graph at a different size
// rather than a second implementation of it — the crop and the zoom
// window are where a copy would quietly diverge.
func (p *Panel) drawGraphInto(dst *ebiten.Image, graphImage *ebiten.Image, x, y, w, h int) {
	imgW := graphImage.Bounds().Dx()
	imgH := graphImage.Bounds().Dy()
	viewStart, viewEnd := p.graphView.bounds()
	x0 := int(viewStart * float64(imgW))
	x1 := max(x0+1, int(math.Ceil(viewEnd*float64(imgW))))
	x1 = min(x1, imgW)

	// Crop vertically to the band the data actually uses: the image's
	// y-axis covers the whole run's peak, so a live run that ends far
	// larger than it starts would show its early cycles as a flat line
	// along the bottom. A replay draws the whole run, so this is the
	// whole height.
	height := p.graph.HeightFraction(p.graphLastBar())
	y0 := min(imgH-1, int(float64(imgH)*(1-height)))

	visible := graphImage.SubImage(image.Rect(x0, y0, x1, imgH)).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w)/float64(x1-x0), float64(h)/float64(imgH-y0))
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(visible, op)

	p.drawGraphPlayhead(dst, x, y, w, h)
}

// drawGraphProgress paints the graph area blank with a centred progress
// bar and percentage, shown in place of the graph while a slow render is
// in flight.
func drawGraphProgress(img *ebiten.Image, x, y, w, h int, fraction float64) {
	ebitenutil.DrawRect(img, float64(x), float64(y), float64(w), float64(h), gh.GraphBackground())

	const barH = 6
	barW := w * 3 / 5
	barX := x + (w-barW)/2
	barY := y + h/2

	label := fmt.Sprintf("RENDERING GRAPH  %d%%", int(fraction*100))
	bounds := boundString(r.FontSourceCodePro8, label)
	text.Draw(img, label, r.FontSourceCodePro8, x+(w-bounds.Dx())/2, barY-6, themedForeground())

	track := chrome(
		color.RGBA{R: 55, G: 55, B: 65, A: 255},
		color.RGBA{R: 200, G: 200, B: 208, A: 255},
	)
	fill := chrome(
		color.RGBA{R: 70, G: 90, B: 130, A: 255},
		color.RGBA{R: 110, G: 140, B: 200, A: 255},
	)
	ebitenutil.DrawRect(img, float64(barX), float64(barY), float64(barW), barH, track)
	ebitenutil.DrawRect(img, float64(barX), float64(barY), float64(barW)*fraction, barH, fill)

	left, top, right, bottom := float64(x), float64(y), float64(x+w), float64(y+h)
	ebitenutil.DrawLine(img, left, top, right, top, themedForeground())
	ebitenutil.DrawLine(img, right, top, right, bottom, themedForeground())
	ebitenutil.DrawLine(img, left, bottom, right, bottom, themedForeground())
	ebitenutil.DrawLine(img, left, top, left, bottom, themedForeground())
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
	if t := p.graphAbilityToggleRect; t != nil &&
		mx >= t.x && mx < t.x+t.w && my >= t.y && my < t.y+t.h {
		p.graphByAbility = !p.graphByAbility
		if p.graphByAbility {
			p.showPopulationForAbility()
		}
		return true
	}
	for _, r := range p.graphAbilityBtnRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			p.graphColorAbility = r.ability
			p.showPopulationForAbility()
			return true
		}
	}
	for _, r := range p.graphButtonRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			p.graphMode = r.mode
			p.graphShowSelected = r.showSelected
			return true
		}
	}
	return false
}

// showPopulationForAbility switches the graph to POP (all) when an ability
// colouring is picked while a non-population graph (PH HIST, FOOD, WALLS)
// is showing, since only the population graphs are coloured by ability.
// POP (sel) stays as it is: it is already a population graph.
func (p *Panel) showPopulationForAbility() {
	if p.graphMode != graph.ModePopulation {
		p.graphMode = graph.ModePopulation
		p.graphShowSelected = false
	}
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
		drawGraphButton(panelImage, x, y, w, sectionRowHeight, b.label, b.get(p.grid))
		rects = append(rects, displayBtnHitbox{
			x: x, y: y, w: w, h: sectionRowHeight, toggle: b,
		})
	}
	p.displayBtnRects = rects
}

// handleDisplayButtonClick toggles the matching layer flag on hit.
func (p *Panel) handleDisplayButtonClick(mx, my int) bool {
	for _, r := range p.displayBtnRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			r.toggle.set(p.grid, !r.toggle.get(p.grid))
			p.grid.doRefresh = true
			return true
		}
	}
	return false
}

// abilityButtonLabels are the short names for the ability sub-row. Six
// buttons share one row, so the full names ("CHEMOSYNTHESIS") would
// overflow; an ability missing here falls back to its full name.
var abilityButtonLabels = map[physiology.Ability]string{
	physiology.AbilityChemosynthesis: "CHEMO",
	physiology.AbilityEating:         "EAT",
	physiology.AbilityMovement:       "MOVE",
	physiology.AbilityDigging:        "DIG",
	physiology.AbilityAttack:         "ATTACK",
	physiology.AbilityDefense:        "DEFENSE",
	physiology.AbilityTolerance:      "PH TOL",
}

// renderOrgColor paints the ORGANISM COLOR radio row — TRUE / PH EFFECT
// / HEALTH / ABILITY / SUCCESS, exclusive choice, only meaningful when ORGANISMS is
// on — and, while ABILITY is active, a second row picking which ability
// to colour by.
//
// Returns how far the rows below must shift: one row pitch while the
// ability row is showing, zero otherwise. The row genuinely appears and
// disappears rather than reserving blank space, so the panel is no
// taller than it needs to be; the shift only ever follows a click on
// this row, so nothing moves under a pending click.
func (p *Panel) renderOrgColor(panelImage *ebiten.Image, yOff int) int {
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

	// The ability buttons stay visible whatever the colour mode: each one
	// is a colour mode in its own right, and a row that came and went
	// shifted everything below it.
	abilityY := y + sectionRowPitch
	abilityRects := make([]abilityBtnHitbox, 0, len(physiology.AllAbilities))
	for i, a := range physiology.AllAbilities {
		x, w := sectionRowButtonRect(i, len(physiology.AllAbilities))
		label, ok := abilityButtonLabels[a]
		if !ok {
			label = strings.ToUpper(a.Name())
		}
		active := p.grid.orgColor == orgColorAbility && p.grid.colorAbility == a
		drawGraphButton(panelImage, x, abilityY, w, sectionRowHeight, label, active)
		abilityRects = append(abilityRects, abilityBtnHitbox{
			x: x, y: abilityY, w: w, h: sectionRowHeight, ability: a,
		})
	}
	p.abilityBtnRects = abilityRects
	return sectionRowPitch
}

// handleOrgColorButtonClick sets the active organism colour mode, or
// the ability the ABILITY mode colours by.
func (p *Panel) handleOrgColorButtonClick(mx, my int) bool {
	for _, r := range p.abilityBtnRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			// Picking an ability is also how the ability colour mode is
			// chosen; the ORGANISM COLOR row has no button for it.
			p.grid.orgColor = orgColorAbility
			p.grid.colorAbility = r.ability
			p.grid.doRefresh = true
			return true
		}
	}
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

// TakeMenuRequest reports whether the MENU button was clicked since the
// last call.
func (p *Panel) TakeMenuRequest() bool {
	requested := p.menuRequested
	p.menuRequested = false
	return requested
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
	// The graph comes first because everything under it shifts down
	// while its ability sub-row is showing; likewise the rows under
	// ORGANISM COLOR while that sub-row is showing.
	below := yOff + p.renderGraph(innerImage, yOff)
	p.renderStats(innerImage, below)
	p.renderMode(innerImage, below)
	p.renderPhColor(innerImage, below)
	p.renderDisplay(innerImage, below)
	below += p.renderOrgColor(innerImage, below)
	p.renderFindMost(innerImage, below)
	contentBottom := p.renderSelected(innerImage, below)
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

// panelButtonFill is the panel button's background, and the one place
// its colour is written down — the title borrows it, so the two can't
// drift apart.
func panelButtonFill() color.RGBA {
	return chrome(
		color.RGBA{R: 40, G: 40, B: 50, A: 255},
		color.RGBA{R: 220, G: 220, B: 225, A: 255},
	)
}

func (p *Panel) drawButton(img *ebiten.Image, x, y, w, h int, label string, col color.RGBA) {
	// Button background
	ebitenutil.DrawRect(img, float64(x), float64(y), float64(w), float64(h), panelButtonFill())
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

	if p.clickInRect(mx, my, menuBtnX, menuBtnY, menuBtnW, menuBtnH) {
		p.menuRequested = true
		return true
	}

	// Clicks on the graph itself are handled by HandleGraphInput, which
	// runs first and tells a click-to-seek apart from a pan or a
	// double-click reset.

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

	// Speed up (width 16). Bounded only by MaxReplaySpeed, at any zoom.
	if p.clickInRect(mx, my, bx, btnY, 16, btnH) {
		speed := p.replayCtrl.Speed * 2
		if speed > replay.MaxReplaySpeed {
			speed = replay.MaxReplaySpeed
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
	// On the dark theme the title borrows the *light* theme's button fill
	// — a pale grey that reads as a soft accent against the panel chrome
	// instead of glaring like a heading. The light theme takes the button
	// fill it actually uses, rather than the mirror-image near-black: at
	// 40 point that much dark ink stopped reading as chrome and started
	// reading as the loudest thing on the screen.
	titleColor := chrome(
		color.RGBA{R: 215, G: 215, B: 220, A: 255},
		panelButtonFill(),
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
	if p.replayCtrl != nil {
		// The MENU button takes the top line.
		p.drawButton(panelImage, menuBtnX, menuBtnY, menuBtnW, menuBtnH, "MENU", chrome(color.RGBA{R: 220, G: 220, B: 225, A: 255}, color.RGBA{R: 40, G: 40, B: 50, A: 255}))
		y = menuBtnY + menuBtnH + lineHeight - 2
	}
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
	// Left-pad each count to a fixed width so the ORGANISMS / FOOD / AVG PH
	// labels don't jitter as the values shrink or grow by a digit.
	statsString := fmt.Sprintf("ORGANISMS: %8d   FOOD: %8d   AVG PH: %4.1f",
		p.simulation.OrganismCount(), p.simulation.FoodCount(), p.simulation.AveragePh())
	text.Draw(panelImage, statsString, r.FontSourceCodePro12, statsXOffset, statsYOffset+yOff, themedForeground())
}

// renderGraph draws the graph and the buttons below it. Returns how far
// the sections below must shift: one button row while the ability
// sub-row is showing, zero otherwise.
func (p *Panel) renderGraph(panelImage *ebiten.Image, yOff int) int {
	// Graph mode is panel-owned and changes only via the buttons below
	// the graph — not via the grid's view-mode key.
	p.graph.SetMode(p.graphMode)
	p.graph.SetShowSelected(p.graphShowSelected)
	p.graph.SetPopulationColor(graph.PopulationColor{
		ByAbility: p.graphByAbility,
		Ability:   p.graphColorAbility,
	})

	if gen := p.simulation.TreesGeneration(); gen != p.lastTreesGeneration {
		p.lastTreesGeneration = gen
		p.graph.InvalidateTrees()
	}

	graphMode := p.graphMode
	label := graphModeLabel(graphMode, p.graphShowSelected)

	// Append selected organism ID to sub-tree population titles.
	if p.graphShowSelected && p.graph.HasSelection() && graphMode == graph.ModePopulation {
		label = fmt.Sprintf("%s (ORG ID: %d)", label, p.simulation.GetSelected())
	}

	gY := graphYOffset + yOff

	text.Draw(panelImage, label, r.FontSourceCodePro12, graphXOffset, gY, themedForeground())
	graphImage := p.graph.Render()
	// Stashed for the expanded popup, which draws the same image at a
	// larger size. It must not call Render itself: that would schedule a
	// second render per frame.
	p.lastGraphImage = graphImage
	if fraction, show := p.graph.RenderProgress(); show {
		// A slow render is in flight: clear the stale graph and show
		// how far along the new one is, rather than leaving an old
		// image up that looks like the answer.
		drawGraphProgress(panelImage, graphXOffset, gY+10, graphWidth, graphHeight, fraction)
		return p.renderGraphButtons(panelImage, graphXOffset, gY+graphHeight+14, graphWidth)
	}
	if graphImage == nil {
		// Still draw the buttons so the controls don't vanish while the
		// first render is in flight.
		return p.renderGraphButtons(panelImage, graphXOffset, gY+graphHeight+14, graphWidth)
	}
	p.graphTop = gY + 10
	p.graphView.syncRange(p.graphCycleRange())

	// Draw only the visible window of the rendered image — the zoom
	// window, and nothing else. Replaying a recording, the image covers
	// the whole run and stays on show whatever the playhead has reached:
	// the history is what the graph is for, and a line marks where the
	// playhead sits in it.
	p.drawGraphInto(panelImage, graphImage, graphXOffset, gY+10, graphWidth, graphHeight)
	// On hover, and in the panel's own inner coordinates — the cursor
	// has to be scroll-adjusted to match.
	hoverX, hoverY := ebiten.CursorPosition()
	hoverInner := hoverY + int(p.scrollY)
	overGraph := hoverX >= graphXOffset && hoverX < graphXOffset+graphWidth &&
		hoverInner >= gY+10 && hoverInner < gY+10+graphHeight
	p.drawExpandButton(panelImage, graphXOffset, gY+10, graphWidth, overGraph)

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

	// POP (sel) doesn't start at cycle 0 — it starts at the birth of the
	// selected organism's family root — so label where its left edge
	// sits: that birth cycle, or the zoomed window's first cycle.
	if p.graphShowSelected && p.graph.HasSelection() && graphMode == graph.ModePopulation {
		startCycle, endCycle := p.graphCycleRange()
		leftCycle := startCycle + int(p.graphView.toFull(0)*float64(endCycle-startCycle))
		cycleLabel := fmt.Sprintf("from cycle %d", leftCycle)
		text.Draw(panelImage, cycleLabel, r.FontSourceCodePro8, graphXOffset+2, gY+10+8, themedForeground())
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
	hintRight := p.drawGraphZoomHint(panelImage, graphTop, graphBot, mx, panelMy)
	if mx >= graphXOffset && mx < graphXOffset+graphWidth && panelMy >= graphTop && panelMy < graphBot {
		startCycle, endCycle := p.graphCycleRange()
		if endCycle > startCycle {
			ebitenutil.DrawRect(panelImage,
				float64(mx), float64(graphTop),
				1, float64(graphHeight),
				themedForegroundDim())
			rel := p.graphView.toFull(float64(mx-graphXOffset) / float64(graphWidth))
			cycle := startCycle + int(rel*float64(endCycle-startCycle))
			label := fmt.Sprintf("%d", cycle)
			lb := boundString(r.FontSourceCodePro8, label)
			tx := mx + 3
			if tx+lb.Dx() > graphXOffset+graphWidth {
				tx = mx - 3 - lb.Dx()
			}
			// Drop below the zoom hint rather than overlap it.
			ty := graphTop + lb.Dy() + 2
			if tx < hintRight {
				ty += lb.Dy() + 3
			}
			text.Draw(panelImage, label, r.FontSourceCodePro8, tx, ty, themedForeground())
		}
	}

	// Graph mode buttons in a 2×3 grid below the graph.
	return p.renderGraphButtons(panelImage, graphXOffset, gY+graphHeight+14, graphWidth)
}

// drawGraphZoomHint labels the graph's zoom controls in its top-left
// corner: the zoom level and how to pan or reset while zoomed, or how to
// zoom while hovering an unzoomed graph. Returns the hint's right edge,
// or 0 when there's no hint.
func (p *Panel) drawGraphZoomHint(img *ebiten.Image, top, bottom, mx, panelMy int) int {
	hovered := mx >= graphXOffset && mx < graphXOffset+graphWidth && panelMy >= top && panelMy < bottom
	var hint string
	switch {
	case p.graphView.zoomed():
		hint = fmt.Sprintf("%.1fx  drag to pan, double-click to reset", p.graphView.zoomFactor())
	case hovered:
		hint = "scroll to zoom"
	default:
		return 0
	}
	hb := boundString(r.FontSourceCodePro8, hint)
	text.Draw(img, hint, r.FontSourceCodePro8, graphXOffset+3, top+hb.Dy()+2, themedForegroundDim())
	return graphXOffset + 3 + hb.Dx() + 3
}

// HandleGraphInput handles the graph's mouse controls: the wheel zooms
// around the cursor, a drag pans, a single click seeks a replay to the
// cycle under the cursor, and a double-click resets the zoom. It also
// sets the cursor while over the graph: a crosshair, or a horizontal
// resize arrow when there is a zoomed view to drag.
//
// Returns true when it consumed this frame's mouse input, so the caller
// skips panel scrolling and grid mouse handling: the wheel shouldn't also
// scroll the panel, and a graph drag shouldn't also pan the grid.
func (p *Panel) HandleGraphInput() bool {
	mx, my := ebiten.CursorPosition()
	innerY := my + int(p.scrollY)
	// The expand control sits over the graph, so it has to take the
	// click before the graph's own click-to-seek sees it.
	if r := p.graphExpandRect; r != nil && r.contains(mx, innerY) {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			p.graphExpandRequested = true
		}
		p.setGraphCursor(ebiten.CursorShapePointer)
		return true
	}
	over := p.graphTop > 0 && mx >= graphXOffset && mx < graphXOffset+graphWidth &&
		innerY >= p.graphTop && innerY < p.graphTop+graphHeight

	// A click's seek fires once the double-click window has passed
	// without a second click.
	if !p.pendingGraphSeekAt.IsZero() && time.Now().After(p.pendingGraphSeekAt) {
		p.pendingGraphSeekAt = time.Time{}
		p.seekGraphTo(p.pendingGraphSeek)
	}

	if p.graphDragging {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			if dx := mx - p.graphPressX; dx > graphClickSlop || dx < -graphClickSlop {
				p.graphDragMoved = true
			}
			if p.graphDragMoved {
				// Dragging right pulls earlier cycles into view.
				p.graphView.panBy(-float64(mx-p.graphDragLastX) / float64(graphWidth))
			}
			p.graphDragLastX = mx
		} else {
			p.graphDragging = false
			if !p.graphDragMoved && !p.lastGraphClick.IsZero() {
				// A click, not a pan: seek there unless a second click
				// turns it into a double-click first.
				visible := float64(p.graphPressX-graphXOffset) / float64(graphWidth)
				p.pendingGraphSeek = p.graphView.toFull(visible)
				p.pendingGraphSeekAt = p.lastGraphClick.Add(graphDoubleClickWindow)
			}
		}
		p.setGraphCursor(ebiten.CursorShapeEWResize)
		return true
	}

	if !over {
		p.setGraphCursor(ebiten.CursorShapeDefault)
		return false
	}

	consumed := false
	if _, wy := ebiten.Wheel(); wy != 0 {
		anchor := float64(mx-graphXOffset) / float64(graphWidth)
		p.graphView.zoomAt(anchor, math.Pow(graphZoomStep, wy))
		consumed = true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		now := time.Now()
		if now.Sub(p.lastGraphClick) < graphDoubleClickWindow {
			// Double-click: reset the zoom and drop the first click's
			// pending seek.
			p.graphView.reset()
			p.lastGraphClick = time.Time{}
			p.pendingGraphSeekAt = time.Time{}
		} else {
			p.lastGraphClick = now
		}
		p.graphDragging = true
		p.graphDragMoved = false
		p.graphPressX = mx
		p.graphDragLastX = mx
		consumed = true
	}

	if p.graphView.zoomed() {
		p.setGraphCursor(ebiten.CursorShapeEWResize)
	} else {
		p.setGraphCursor(ebiten.CursorShapeCrosshair)
	}
	return consumed
}

// graphDoubleClickWindow is how quickly a second click must follow the
// first to reset the graph's zoom. A single click's seek waits this long.
const graphDoubleClickWindow = 350 * time.Millisecond

// graphClickSlop is how far, in pixels, a press can move and still count
// as a click rather than a pan.
const graphClickSlop = 3

// seekGraphTo pauses a replay and seeks it to the cycle at fraction of the
// graph's full time range. Does nothing outside a replay.
func (p *Panel) seekGraphTo(fraction float64) {
	if p.replayCtrl == nil {
		return
	}
	startCycle, endCycle := p.graphCycleRange()
	if endCycle <= startCycle {
		return
	}
	target := startCycle + int(fraction*float64(endCycle-startCycle))
	p.simulation.Pause(true)
	if err := p.replayCtrl.SeekToCycle(target); err == nil {
		p.grid.doRefresh = true
	}
}

// setGraphCursor changes the mouse cursor only when the shape differs from
// what the graph last set, so the graph doesn't reset a cursor every frame
// or override one it never set.
func (p *Panel) setGraphCursor(shape ebiten.CursorShapeType) {
	if shape == p.graphCursor {
		return
	}
	p.graphCursor = shape
	ebiten.SetCursorShape(shape)
}

// graphPlayheadW is how wide the playhead line is drawn, and
// graphPlayheadColor what colour: warm, so it reads as "now" against the
// cool graph lines and stays clear of the dim grey hover line.
const graphPlayheadW = 2

var graphPlayheadColor = color.RGBA{R: 255, G: 190, B: 90, A: 255}

// graphLastBar is the last graph bar the image covers.
func (p *Panel) graphLastBar() int {
	_, end := p.graphCycleRange()
	return end / config.PopulationUpdateInterval()
}

// graphPlayheadFraction is where the playhead sits in the graph's time
// range, 0 at its left edge and 1 at its right. Only meaningful while
// replaying: a live run's graph ends at the cycle being played, so the
// playhead is always the right edge.
func (p *Panel) graphPlayheadFraction() (float64, bool) {
	if p.simulation.RecordedEndCycle() == 0 {
		return 0, false
	}
	start, end := p.graphCycleRange()
	return playheadFraction(start, p.simulation.Cycle(), end)
}

// playheadFraction is where current sits in [start, end], clamped to the
// ends. Not ok when the range is empty — nothing to sit in.
func playheadFraction(start, current, end int) (float64, bool) {
	if end <= start {
		return 0, false
	}
	return min(1, max(0, float64(current-start)/float64(end-start))), true
}

// drawGraphPlayhead marks the cycle being played on a graph that shows
// the whole recording. Without it the graph and the grid would be
// telling the viewer about different moments with nothing to connect
// them. Skipped when the playhead is scrolled outside a zoomed window.
func (p *Panel) drawGraphPlayhead(dst *ebiten.Image, x, y, w, h int) {
	full, ok := p.graphPlayheadFraction()
	if !ok {
		return
	}
	visible := p.graphView.toVisible(full)
	if visible < 0 || visible > 1 {
		return
	}
	px := float64(x) + visible*float64(w)
	// Kept inside the frame at either end so the line reads as part of
	// the graph rather than as its border.
	px = min(px, float64(x+w)-graphPlayheadW)
	ebitenutil.DrawRect(dst, px, float64(y), graphPlayheadW, float64(h), graphPlayheadColor)
}

// graphCycleRange returns the [start, end) cycle range the graph
// currently covers. start depends on whether a sub-tree selection is
// in play (otherwise 0); end is the cycle the image on show was rendered
// up to (the simulation's cycle before the first render). Used to map
// cursor X position over the graph back to a sim cycle for hover labels
// and click-to-seek, and to hold a zoomed window on its cycles.
func (p *Panel) graphCycleRange() (start, end int) {
	start = 0
	if p.graphShowSelected && p.graph.HasSelection() {
		if s := p.graph.SelectedStartCycle(); s >= 0 {
			start = s
		}
	}
	end = p.simulation.Cycle()
	if recorded := p.simulation.RecordedEndCycle(); recorded > 0 {
		// Replaying: the graphs cover the whole recording and are shown
		// in full whatever the playhead, so the axis is the whole run.
		// Seeking therefore reaches anywhere in the run, forward or back.
		end = recorded
	} else if rendered := p.graph.RenderedEndCycle(); rendered > start {
		// A live run's graph image lags the simulation by up to one
		// render, so map positions against what it actually shows.
		end = rendered
	}
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
	infoY := sY + titleHeight + selectedTitleGap

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

	// Section title — drawn only when there's something to show below.
	// A dead organism's stats are its last-seen values, drawn dim.
	titleStr := fmt.Sprintf("SELECTED ORG: %d", id)
	if info != nil && dim {
		titleStr += " (dead)"
	}
	text.Draw(panelImage, titleStr, r.FontSourceCodePro12, selectedXOffset, sY, themedForeground())

	// The info block is the taller of the stats column, the abilities
	// column and the portrait. Its height is fixed whether we have live
	// data, cached/dim data, or only a placeholder message — so a
	// reconstruction finishing half a second after a click never shifts
	// the trees below.
	statsLineHeight := r.FontSourceCodePro12.Metrics().Height.Round()
	statsBottom := max(infoY+portraitSize+healthBarGap+healthBarH,
		infoY+statsRowCount*statsLineHeight,
		infoY+abilitiesHeight())

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
		msgY := infoY + (statsBottom-infoY)/2 + bounds.Dy()/2
		text.Draw(panelImage, msg, r.FontSourceCodePro10, msgX, msgY, themedForegroundDim())

		offsetY := statsBottom + detailTabsGap
		offsetY = p.renderDetailTabs(panelImage, offsetY) + 14
		switch p.selectedTab {
		case 1:
			offsetY = p.renderDescendantTreeTab(panelImage, p.effectiveDescRoot(id), id, offsetY)
		}
		return offsetY
	}

	// Portrait — right column, flush against the right padding — with
	// the health bar directly beneath it.
	p.renderPortrait(panelImage, info, dim, selectedPortraitX, infoY)
	drawPortraitHealth(panelImage, info.Health, info.Size, dim, selectedPortraitX, infoY)
	drawHealthBar(panelImage, info.Health, info.Size, dim, selectedPortraitX, infoY+portraitSize+healthBarGap)

	// Stats and abilities side by side, level with the top of the portrait.
	p.renderOrganismStats(panelImage, info, traits, dim, infoY)
	p.renderAbilities(panelImage, traits.Abilities, dim, abilitiesColX, infoY, abilitiesColWidth)

	offsetY := statsBottom + detailTabsGap
	offsetY = p.renderDetailTabs(panelImage, offsetY)
	offsetY += 14

	switch p.selectedTab {
	case 1:
		p.descRowRects = nil
		offsetY = p.renderDescendantTreeTab(panelImage, p.effectiveDescRoot(id), id, offsetY)
	default:
		p.descRowRects = nil
		p.showParentRect = nil
		offsetY = p.renderDecisionTreeTab(panelImage, decisionTree, dim, offsetY)
	}
	return offsetY
}

// detailTabsGap is the space between the selected organism's info block
// (portrait, stats or abilities) and the DECISION / DESCENDANT tabs below.
const detailTabsGap = 16

// phComfortColor tints the IDEAL PH value green→red by how much the water
// the organism is actually sitting in costs it, matching the grid's
// TOLERANCE view. Colouring by the ideal itself said only which end of
// the scale a lineage liked, which in an acid world was every organism on
// screen — the same red whether it was thriving or dying in it.
func (p *Panel) phComfortColor(info *organism.Info, traits organism.Traits) color.Color {
	distance := math.Abs(traits.IdealPh - p.simulation.GetPhAtPoint(info.Location))
	damage := effects.PhDamage(config.GetCurrentGlobals(), traits.Abilities[physiology.AbilityTolerance], info.Size, distance)
	return phToleranceColor(damage)
}

// renderOrganismStats draws the selected organism's current state as two
// columns of label/value rows in the info area, with health, pH comfort
// and pH effect overlaid in their meaning-coded colours.
func (p *Panel) renderOrganismStats(panelImage *ebiten.Image, info *organism.Info, traits organism.Traits, dim bool, contentTop int) {
	face := r.FontSourceCodePro12
	lineH := face.Metrics().Height.Round()

	infoColor := themedForeground()
	if dim {
		infoColor = themedForegroundDim()
	}

	// Net cumulative pH push: positive = base-leaning (eating-driven),
	// negative = acid-leaning (chemo-driven).
	rows := []struct {
		label, value string
		// valueColor, when set, recolours the value. Skipped when dim:
		// the cached pH at a stale location wouldn't say anything useful,
		// and the consistent dim treatment reads as "snapshot, not live".
		valueColor color.Color
	}{
		{"AGE:", fmt.Sprintf("%d", info.Age), nil},
		{"CHILDREN:", fmt.Sprintf("%d", info.Children), nil},
		{"TRAVELED:", fmt.Sprintf("%d", info.TraveledDist), nil},
		{"IDEAL PH:", fmt.Sprintf("%1.1f", traits.IdealPh), p.phComfortColor(info, traits)},
		{"SPAWN HP:", fmt.Sprintf("%.2f", traits.MinHealthToSpawn), nil},
		{"HITS/ATK:", fmt.Sprintf("%d/%d", info.AttackHits, info.AttackTotal), nil},
		{"PH EFF:", fmt.Sprintf("%+.2f", info.PhPositive-info.PhNegative), phEffectTextColor(info.PhPositive, info.PhNegative)},
	}

	// Labels left-aligned at the column's left edge, values right-aligned
	// at its right edge. (Health and size are drawn on the portrait
	// instead; see drawPortraitHealth.)
	y := contentTop + lineH
	right := selectedCol1X + statsColWidth
	for _, row := range rows {
		text.Draw(panelImage, row.label, face, selectedCol1X, y, infoColor)
		col := infoColor
		if row.valueColor != nil && !dim {
			col = row.valueColor
		}
		text.Draw(panelImage, row.value, face, right-textAdvance(face, row.value), y, col)
		y += lineH
	}
}

// renderAbilities draws the organism's ability distribution in the info
// area: one row per ability with its score and a proportional bar.
//
// Every row's label is formatted to the same width in a monospace face,
// so all bars start at the same x and share one scale — the whole point
// budget maps to the remaining width. Drawn to a common scale rather
// than each normalised to its own maximum, so the shape of a
// distribution reads at a glance, which is what a fixed budget is for.
func (p *Panel) renderAbilities(panelImage *ebiten.Image, scores physiology.Scores, dim bool, x, top, width int) {
	col := themedForeground()
	if dim {
		col = themedForegroundDim()
	}
	barCol := col
	if !dim {
		barCol = fadedForeground(0xB0)
	}

	valueFace := r.FontSourceCodePro8
	valueH := valueFace.Metrics().Height.Round()
	labelFace := r.FontSourceCodePro8
	slotW := width / len(physiology.AllAbilities)
	barsTop := top + valueH + 2
	labelsTop := barsTop + abilityBarLength + abilityLabelGap

	for i, a := range physiology.AllAbilities {
		slotX := x + i*slotW
		barX := slotX + (slotW-abilityBarWidth)/2

		// Score above its bar, centred on the bar.
		value := fmt.Sprintf("%d", scores[a])
		vb := textAdvance(valueFace, value)
		text.Draw(panelImage, value, valueFace, barX+(abilityBarWidth-vb)/2, top+valueH, col)

		// A faint full-height track (a score of 100) filled from the
		// bottom, so bars read against each other as well as on their own.
		ebitenutil.DrawRect(panelImage, float64(barX), float64(barsTop), abilityBarWidth, abilityBarLength, fadedForeground(0x30))
		h := float64(scores[a]) / float64(physiology.MaxAbilityScore) * abilityBarLength
		if h > 0 {
			ebitenutil.DrawRect(panelImage, float64(barX), float64(barsTop)+abilityBarLength-h, abilityBarWidth, h, barCol)
		}

		// Label below, rotated to read bottom-to-top so it fits the
		// column, ending at the bar so the names line up under the bars
		// however long they are.
		name := a.Name()
		if short, ok := abilityStatLabels[a]; ok {
			name = short
		}
		drawTextBottomUp(panelImage, name, labelFace, barX+(abilityBarWidth-labelFace.Metrics().Height.Round())/2, labelsTop, col)
	}
}

// abilitiesHeight is how tall the abilities block is: the score line, the
// bars, and the longest rotated label below them.
func abilitiesHeight() int {
	longest := 0
	for _, a := range physiology.AllAbilities {
		name := a.Name()
		if short, ok := abilityStatLabels[a]; ok {
			name = short
		}
		longest = max(longest, textAdvance(r.FontSourceCodePro8, name))
	}
	return r.FontSourceCodePro8.Metrics().Height.Round() + 2 + abilityBarLength + abilityLabelGap + longest
}

// abilityStatLabels shortens ability names too long for the abilities
// column beside the stats; the rest use their full names.
var abilityStatLabels = map[physiology.Ability]string{
	physiology.AbilityChemosynthesis: "Chemo",
	physiology.AbilityTolerance:      "pH Tol",
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
		// The high-res sprite set (16x16) uses 4 frames per cycle.
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
	// Portrait always renders from the 16x16 (highest-res) set, so
	// iterate the physiology-driven layers like the grid renderer
	// does. Falls back to the bare default sprite when nothing in
	// that organism's layer list has a PNG on disk yet.
	const portraitZoom = 2 // 0:4x4, 1:8x8, 2:16x16
	layers := r.OrganismLayersFor(info.Appearance)
	stampedAny := false
	for _, layer := range layers {
		sprite := r.SpriteLayerAtZoom(portraitZoom, role, layer, anim, frameIdx)
		if sprite == nil {
			continue
		}
		col := info.SecondaryColor
		if r.UsesPrimaryColor(layer) {
			col = info.Color
		}
		drawAnimatedSprite(p.portraitImg, baseX, baseY, sprite, direction, col, float64(cellSize), scale)
		stampedAny = true
	}
	if !stampedAny {
		sprite := r.SpriteAtZoom(portraitZoom, role, anim, frameIdx)
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

// drawPortraitHealth labels the portrait's bottom-right corner with the
// organism's health and size as "health / size", in the theme's foreground
// colour — white under the dark UI, dark grey under the light — so it stays
// readable over any pH tint behind the portrait. Dim for a dead organism.
func drawPortraitHealth(img *ebiten.Image, health, size float64, dim bool, x, y int) {
	face := r.FontSourceCodePro10
	label := fmt.Sprintf("%.1f / %.1f", max(0, health), size)

	const pad = 3
	lineH := face.Metrics().Height.Round()
	ascent := face.Metrics().Ascent.Round()
	left := x + portraitSize - pad - textAdvance(face, label)
	baseline := y + portraitSize - pad - lineH + ascent

	col := themedForeground()
	if dim {
		col = themedForegroundDim()
	}
	text.Draw(img, label, face, left, baseline, col)
}

// healthBarGeometry lays out the health bar. The bar always spans the
// full width; the organism's size sets how many healthBarSegment-point
// segments it is divided into, and its health sets how much of it is
// filled. Returns the filled width and the x offsets of the dividers
// between segments.
//
// For example size 35 and health 21 give 3.5 segments (dividers at 10, 20
// and 30 of 35 points, the last segment half-width) and 60% of the width
// filled: the first two segments full and 10% of the third.
func healthBarGeometry(health, size, width float64) (fill float64, dividers []float64) {
	if size <= 0 || width <= 0 {
		return 0, nil
	}
	fill = min(max(health, 0), size) / size * width
	for pts := healthBarSegment; pts < size; pts += healthBarSegment {
		dividers = append(dividers, pts/size*width)
	}
	return fill, dividers
}

// drawHealthBar draws the selected organism's health as a segmented bar
// the width of the portrait: one segment per healthBarSegment points of
// size, filled to its share of health in the health colour.
func drawHealthBar(img *ebiten.Image, health, size float64, dim bool, x, y int) {
	if size <= 0 {
		return
	}
	fill, dividers := healthBarGeometry(health, size, portraitSize)
	capacity := float64(portraitSize)
	fx, fy := float64(x), float64(y)

	// Unfilled segments are dark grey under both themes.
	track := color.RGBA{R: 64, G: 64, B: 64, A: 255}
	var fillCol color.Color = healthColor(health, size)
	if dim {
		fillCol = themedForegroundDim()
	}
	ebitenutil.DrawRect(img, fx, fy, capacity, healthBarH, track)
	if fill > 0 {
		ebitenutil.DrawRect(img, fx, fy, fill, healthBarH, fillCol)
	}
	// Dividers are 1px gaps in the background colour, so the segments
	// read as separate cells across both the filled and empty parts.
	bg := themeBackgroundColor()
	for _, d := range dividers {
		ebitenutil.DrawRect(img, fx+math.Round(d), fy, 1, healthBarH, bg)
	}
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

// drawTabStrip draws a row of equal-width tab buttons spanning width and
// returns their hitboxes. Shared by the info tabs (STATS / ABILITIES)
// and the detail tabs (DECISION / DESCENDANT) so the two strips look
// and behave identically.
func drawTabStrip(img *ebiten.Image, x, y, width int, labels []string, selected int) []detailTabHitbox {
	n := len(labels)
	tabW := (width - tabStripGap*(n-1)) / n
	rects := make([]detailTabHitbox, 0, n)
	for i, label := range labels {
		tx := x + i*(tabW+tabStripGap)
		drawGraphButton(img, tx, y, tabW, tabStripHeight, label, i == selected)
		rects = append(rects, detailTabHitbox{x: tx, y: y, w: tabW, h: tabStripHeight, tab: i})
	}
	return rects
}

// renderDetailTabs draws the DECISION / DESCENDANT tab strip just below
// the organism info block. Returns the Y after the tabs.
func (p *Panel) renderDetailTabs(panelImage *ebiten.Image, topY int) int {
	p.detailTabRects = drawTabStrip(panelImage, selectedXOffset, topY, graphWidth,
		[]string{"DECISION TREE", "DESCENDANT TREE"}, p.selectedTab)
	return topY + tabStripHeight
}

// renderDecisionTreeTab draws the decision-tree text block in three
// tiers:
//   - active   — UsedLastCycle, the path chooseAction just took
//   - travelled — WasTravelled, ever-visited but not on current path,
//     so the user can see which branches the organism has
//     explored over its lifetime distinct from dead code
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
func (p *Panel) renderDecisionTreeTab(panelImage *ebiten.Image, decisionTree *d.Tree, dim bool, topY int) int {
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
	// No gated styling: under ability scores every organism can express
	// every node, so a tree can no longer contain something its owner
	// is unable to perform. A low score makes the action weak or
	// expensive, which the ability panel shows — not something to
	// strike out here.
	face := r.FontSourceCodePro10
	lineHeight := face.Metrics().Height.Round()
	offsetY := topY
	for _, line := range decisionTree.PrintLines() {
		var clr color.Color = dimColor
		if !dim {
			switch {
			case line.UsedLastCycle:
				clr = activeColor
			case line.WasTravelled:
				clr = travelledColor
			}
		}
		text.Draw(panelImage, line.Text, face, selectedXOffset, offsetY, clr)
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

// TakeGraphExpandRequest reports and clears a click on the graph's
// expand control, so each click opens the popup exactly once.
func (p *Panel) TakeGraphExpandRequest() bool {
	req := p.graphExpandRequested
	p.graphExpandRequested = false
	return req
}
