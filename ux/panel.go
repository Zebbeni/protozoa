package ux

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
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

	// gridRender / highlight are rendered side by side, each in its own
	// column with a comfortable gap between them. The button width is
	// derived from panelWidth so the columns split the available area
	// evenly minus padding and the inter-column gap.
	sectionColumnGap  = 16
	gridRenderXOffset = padding
	gridRenderYOffset = 300
	sectionColumnWidth = (panelWidth - 2*padding - sectionColumnGap) / 2
	selectionsXOffset = padding + sectionColumnWidth + sectionColumnGap
	selectionsYOffset = 300

	// 4 single-column buttons (16 + 4) per row, 4 rows = 76, plus
	// header line (~14) and a gap = ~95. selectedYOffset gives a bit
	// more breathing room before the per-organism Selected Statistics
	// section that follows.
	selectedXOffset = padding
	selectedYOffset = 415

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

	// viewModeButtonRects caches the on-screen hitboxes of the
	// grid-render mode buttons.
	viewModeButtonRects []viewModeButtonHitbox

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

// selectButtonHitbox associates a select-mode button's screen rect
// with the auto-select mode it activates.
type selectButtonHitbox struct {
	x, y, w, h int
	sel        mode
}

// viewModeButtonHitbox associates a grid-render mode button's screen
// rect with the view mode it activates.
type viewModeButtonHitbox struct {
	x, y, w, h int
	view       mode
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
		simulation:    sim,
		grid:          grid,
		graph:         graph.NewGraph(sim),
		graphMode:     graph.ModePopulation,
		descCollapsed: make(map[int]bool),
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
	{label: "EFF (all)", mode: graph.ModePopulationPhEffect, showSelected: false},
	{label: "EFF (sel)", mode: graph.ModePopulationPhEffect, showSelected: true},
	{label: "EFF HIST", mode: graph.ModePhEffect, showSelected: false},
}

const (
	graphButtonRowHeight = 16
	graphButtonGap       = 4
	graphButtonsPerRow   = 3
)

// selectModeButton is one entry in the SELECTIONS button strip. The
// last entry (MANUAL) doubles as the "deselect" state — clicking it
// stops auto-following the current criterion. We expose it as a button
// so the user can clear an active follow without having to click on the
// grid first.
type selectModeButton struct {
	label string
	sel   mode
}

var selectModeButtons = [...]selectModeButton{
	{label: "OLDEST", sel: selectOldest},
	{label: "MOST CHILDREN", sel: selectMostChildren},
	{label: "MOST TRAVELED", sel: selectMostTraveled},
	{label: "MOST SUCCESSFUL", sel: selectMostSuccessful},
}

const (
	selectButtonRowHeight = 16
	selectButtonGap       = 4
	selectButtonsPerRow   = 2
)

// viewModeButton is one entry in the GRID RENDER button strip.
type viewModeButton struct {
	label string
	view  mode
}

var viewModeButtons = [...]viewModeButton{
	{label: "ORGANISMS & PH", view: orgsPhMode},
	{label: "ORGANISMS ONLY", view: organismsOnlyMode},
	{label: "ORGANISM PH EFFECTS", view: phEffectsOnlyMode},
	{label: "PH ONLY", view: phOnlyMode},
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
	case graph.ModePopulationPhEffect:
		if showSelected {
			return "PH EFFECT POP (SELECTED)"
		}
		return "PH EFFECT POPULATION"
	case graph.ModePhEffect:
		return "PH EFFECT HISTORY"
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
// reads at a glance.
func drawGraphButton(img *ebiten.Image, x, y, w, h int, label string, active bool) {
	bg := color.RGBA{R: 35, G: 35, B: 45, A: 255}
	fg := color.RGBA{R: 170, G: 170, B: 180, A: 255}
	if active {
		bg = color.RGBA{R: 70, G: 90, B: 130, A: 255}
		fg = color.RGBA{R: 235, G: 235, B: 245, A: 255}
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

// renderSelections paints the HIGHLIGHT section in the right column
// (paired side-by-side with GRID RENDER on the left). Single column
// of buttons, one per row. Refreshes p.selectButtonRects.
func (p *Panel) renderSelections(panelImage *ebiten.Image, yOff int) {
	y := selectionsYOffset + yOff
	text.Draw(panelImage, "HIGHLIGHT", r.FontSourceCodePro12, selectionsXOffset, y, themedForeground())

	btnW := sectionColumnWidth
	topY := y + 8

	rects := make([]selectButtonHitbox, 0, len(selectModeButtons))
	for i, b := range selectModeButtons {
		x := selectionsXOffset
		by := topY + i*(selectButtonRowHeight+selectButtonGap)

		active := p.grid.selectMode == b.sel
		drawGraphButton(panelImage, x, by, btnW, selectButtonRowHeight, b.label, active)

		rects = append(rects, selectButtonHitbox{
			x: x, y: by, w: btnW, h: selectButtonRowHeight, sel: b.sel,
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

// renderGridRender paints the GRID RENDER section in the left column
// (paired side-by-side with HIGHLIGHT on the right). Single column of
// buttons, one per row. Refreshes p.viewModeButtonRects.
func (p *Panel) renderGridRender(panelImage *ebiten.Image, yOff int) {
	y := gridRenderYOffset + yOff
	text.Draw(panelImage, "GRID RENDER", r.FontSourceCodePro12, gridRenderXOffset, y, themedForeground())

	btnW := sectionColumnWidth
	topY := y + 8

	rects := make([]viewModeButtonHitbox, 0, len(viewModeButtons))
	for i, b := range viewModeButtons {
		x := gridRenderXOffset
		by := topY + i*(selectButtonRowHeight+selectButtonGap)

		active := p.grid.viewMode == b.view
		drawGraphButton(panelImage, x, by, btnW, selectButtonRowHeight, b.label, active)

		rects = append(rects, viewModeButtonHitbox{
			x: x, y: by, w: btnW, h: selectButtonRowHeight, view: b.view,
		})
	}
	p.viewModeButtonRects = rects
}

// handleViewModeButtonClick consumes a click on a view-mode button.
// Returns true on hit. Caller must have scroll-adjusted my already.
func (p *Panel) handleViewModeButtonClick(mx, my int) bool {
	for _, r := range p.viewModeButtonRects {
		if mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h {
			p.grid.viewMode = r.view
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
	p.renderGridRender(innerImage, yOff)
	p.renderSelections(innerImage, yOff)
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
		color.RGBA{R: 40, G: 40, B: 40, A: 150})
	// Thumb
	ebitenutil.DrawRect(panelImage, scrollbarX, thumbY, scrollbarWidth, thumbH,
		color.RGBA{R: 120, G: 120, B: 120, A: 200})
}

func (p *Panel) renderDividingLine(panelImage *ebiten.Image, screenH int) {
	// Drawn on the final output instead, see Render()
}

func (p *Panel) renderReplayControls(panelImage *ebiten.Image) {
	ctrl := p.replayCtrl
	cycle := ctrl.Cycle()
	finalCycle := ctrl.FinalCycle
	paused := p.simulation.IsPaused()

	// Scrubber track
	scrubY := replayCtrlY
	ebitenutil.DrawRect(panelImage, float64(scrubberX), float64(scrubY), float64(scrubberW), float64(scrubberH),
		color.RGBA{R: 50, G: 50, B: 60, A: 255})

	// Scrubber fill (progress)
	progress := 0.0
	if finalCycle > 0 {
		progress = float64(cycle) / float64(finalCycle)
	}
	fillW := progress * float64(scrubberW)
	ebitenutil.DrawRect(panelImage, float64(scrubberX), float64(scrubY), fillW, float64(scrubberH),
		color.RGBA{R: 80, G: 80, B: 120, A: 255})

	// Scrubber handle
	handleX := float64(scrubberX) + fillW - float64(scrubberHandleW)/2
	ebitenutil.DrawRect(panelImage, handleX, float64(scrubY-2), float64(scrubberHandleW), float64(scrubberH+4),
		color.RGBA{R: 180, G: 180, B: 220, A: 255})

	// Snapshot markers on the scrubber
	for _, snapCycle := range ctrl.SnapshotCycles() {
		if finalCycle > 0 {
			mx := float64(scrubberX) + float64(snapCycle)/float64(finalCycle)*float64(scrubberW)
			ebitenutil.DrawRect(panelImage, mx, float64(scrubY), 1, float64(scrubberH),
				color.RGBA{R: 150, G: 150, B: 150, A: 100})
		}
	}

	// Buttons row below scrubber
	btnY := scrubY + scrubberH + 6
	btnH := 16
	btnGap := 4
	bx := scrubberX

	// Prev cycle
	p.drawButton(panelImage, bx, btnY, 24, btnH, "<<", color.RGBA{R: 180, G: 180, B: 180, A: 255})
	bx += 24 + btnGap

	// Play/Pause
	if paused {
		p.drawButton(panelImage, bx, btnY, 36, btnH, "PLAY", color.RGBA{R: 100, G: 200, B: 100, A: 255})
	} else {
		p.drawButton(panelImage, bx, btnY, 36, btnH, "STOP", color.RGBA{R: 200, G: 200, B: 100, A: 255})
	}
	bx += 36 + btnGap

	// Next cycle
	p.drawButton(panelImage, bx, btnY, 24, btnH, ">>", color.RGBA{R: 180, G: 180, B: 180, A: 255})
	bx += 24 + btnGap + 8

	// Speed down / up
	p.drawButton(panelImage, bx, btnY, 16, btnH, "-", color.RGBA{R: 180, G: 180, B: 180, A: 255})
	bx += 16 + btnGap

	// Speed indicator
	speedLabel := formatReplaySpeed(ctrl.Speed)
	text.Draw(panelImage, speedLabel, r.FontSourceCodePro10, bx, btnY+btnH-4, color.RGBA{R: 200, G: 200, B: 200, A: 255})
	bx += 28

	p.drawButton(panelImage, bx, btnY, 16, btnH, "+", color.RGBA{R: 180, G: 180, B: 180, A: 255})
	bx += 16 + btnGap

	// AUTO toggle — green label when on, dim grey when off. When on, the
	// replay speed is driven by the camera's zoom; when off the speed
	// responds only to the - / + buttons.
	autoLabel := color.RGBA{R: 110, G: 110, B: 110, A: 255}
	if ctrl.AutoSpeed {
		autoLabel = color.RGBA{R: 110, G: 200, B: 120, A: 255}
	}
	p.drawButton(panelImage, bx, btnY, 32, btnH, "AUTO", autoLabel)
	bx += 32 + btnGap + 8

	// Cycle counter
	cycleLabel := fmt.Sprintf("Cycle %d / %d", cycle, finalCycle)
	text.Draw(panelImage, cycleLabel, r.FontSourceCodePro10, bx, btnY+btnH-4, color.RGBA{R: 150, G: 150, B: 150, A: 255})
}

func (p *Panel) drawButton(img *ebiten.Image, x, y, w, h int, label string, col color.RGBA) {
	// Button background
	ebitenutil.DrawRect(img, float64(x), float64(y), float64(w), float64(h),
		color.RGBA{R: 40, G: 40, B: 50, A: 255})
	// Border
	ebitenutil.DrawRect(img, float64(x), float64(y), float64(w), 1, color.RGBA{R: 70, G: 70, B: 80, A: 255})
	ebitenutil.DrawRect(img, float64(x), float64(y+h-1), float64(w), 1, color.RGBA{R: 30, G: 30, B: 35, A: 255})
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

	// Graph and selection buttons are independent of replay state —
	// handle them first so they work in non-replay mode too.
	if p.handleSelectButtonClick(mx, scrolledY) {
		return true
	}
	if p.handleViewModeButtonClick(mx, scrolledY) {
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
	text.Draw(panelImage, "protozoa", r.FontInversionz40, titleXOffset, titleYOffset+bounds.Dy(), themedForeground())
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
	// Left-pad each count to a fixed width so the LIVING / DEAD
	// labels don't jitter as the values shrink or grow by a digit.
	// DEAD reaches the millions on long runs, so reserve 10 digits
	// for both so they're easy to compare visually.
	statsString := fmt.Sprintf("LIVING: %10d        DEAD: %10d",
		p.simulation.OrganismCount(), p.simulation.GetDeadCount())
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
	if p.graphShowSelected && p.graph.HasSelection() &&
		(graphMode == graph.ModePopulation || graphMode == graph.ModePopulationPhEffect) {
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
	if p.graph.HasSelection() && (graphMode == graph.ModePopulation || graphMode == graph.ModePopulationPhEffect) {
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

	// Graph mode buttons in a 2×3 grid below the graph.
	p.renderGraphButtons(panelImage, gY+graphHeight+14)
}

// renderSelected draws the selected organism info and the active
// detail tab (decision tree or descendant tree). Returns the Y
// position after the last line of content.
func (p *Panel) renderSelected(panelImage *ebiten.Image, yOff int) int {
	sY := selectedYOffset + yOff
	// Section title sits above the per-organism info.
	titleHeight := r.FontSourceCodePro12.Metrics().Height.Round()
	text.Draw(panelImage, "SELECTED STATISTICS", r.FontSourceCodePro12, selectedXOffset, sY, themedForeground())
	infoY := sY + titleHeight + 4

	id := p.simulation.GetSelected()
	info := p.simulation.GetOrganismInfoByID(id)
	traits, found := p.simulation.GetOrganismTraitsByID(id)

	decisionTree := p.simulation.GetOrganismDecisionTreeByID(id)
	if info == nil || decisionTree == nil || found == false {
		p.detailTabRects = nil
		p.descRowRects = nil
		return infoY
	}
	infoString := fmt.Sprintf("ORGANISM ID:    %7d       HEALTH:       %[4]*.[3]*[2]f", info.ID, info.Health, 2, 5)
	infoString += fmt.Sprintf("\nANCESTOR ID:    %7d       SIZE:         %5.2f", info.AncestorID, info.Size)
	infoString += fmt.Sprintf("\nAGE:            %7d       CHILDREN:   %7d", info.Age, info.Children)
	infoString += fmt.Sprintf("\nMUTATE CHANCE:     %3.0f%%       SPAWN HEALTH: %[4]*.[3]*[2]f", traits.ChanceToMutateDecisionTree*100.0, traits.MinHealthToSpawn, 2, 5)
	infoString += fmt.Sprintf("\nPH TOLERANCE:   %1.1f-%1.1f       PH EFFECT: %+1.5f", traits.IdealPh-traits.PhTolerance, traits.IdealPh+traits.PhTolerance, traits.PhGrowthEffect)
	infoLineCount := strings.Count(infoString, "\n") + 1
	infoHeight := infoLineCount * r.FontSourceCodePro12.Metrics().Height.Round()
	// Small gap between info text and the tab strip below.
	offsetY := infoY + infoHeight + 6

	text.Draw(panelImage, infoString, r.FontSourceCodePro12, selectedXOffset, infoY, themedForeground())

	// Tab buttons row, with a bit of breathing room before the
	// content below so the first tab content line doesn't kiss the
	// button edges.
	offsetY = p.renderDetailTabs(panelImage, offsetY)
	offsetY += 14

	switch p.selectedTab {
	case 1:
		p.descRowRects = nil
		offsetY = p.renderDescendantTreeTab(panelImage, id, offsetY)
	default:
		p.descRowRects = nil
		offsetY = p.renderDecisionTreeTab(panelImage, decisionTree, offsetY)
	}
	return offsetY
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

// renderDecisionTreeTab draws the existing decision-tree text block.
func (p *Panel) renderDecisionTreeTab(panelImage *ebiten.Image, decisionTree *d.Tree, topY int) int {
	var activeColor, dimColor color.Color = themedForeground(), color.RGBA{R: 80, G: 80, B: 80, A: 255}
	if config.IsLightTheme() {
		dimColor = color.RGBA{R: 170, G: 170, B: 180, A: 255}
	}
	lineHeight := r.FontSourceCodePro10.Metrics().Height.Round()
	offsetY := topY
	for _, line := range decisionTree.PrintLines() {
		clr := dimColor
		if line.WasTravelled {
			clr = activeColor
		}
		text.Draw(panelImage, line.Text, r.FontSourceCodePro10, selectedXOffset, offsetY, clr)
		offsetY += lineHeight
	}
	return offsetY
}

// renderDescendantTreeTab draws an indented, scrollable view of the
// selected organism's descendant subtree. Only nodes whose StartCycle
// is at or before the current sim cycle are shown — the loaded
// replay tree contains descendants from the full recording, including
// future births that haven't happened yet at the current playhead.
// Live nodes use the themed foreground; dead nodes are dimmed. Each
// row is clickable (selects that organism) and shows a [+]/[-] toggle
// when it has children that have already been born.
func (p *Panel) renderDescendantTreeTab(panelImage *ebiten.Image, selectedID, topY int) int {
	root := p.simulation.GetOrganismTreeNode(selectedID)
	if root == nil {
		text.Draw(panelImage, "(no descendant tree)", r.FontSourceCodePro10, selectedXOffset, topY+12, themedForegroundDim())
		return topY + 16
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

	y := topY
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
		if n.ID == selectedID {
			ebitenutil.DrawRect(panelImage, float64(x), float64(y),
				float64(graphWidth-(x-selectedXOffset)), float64(rowH-2),
				color.RGBA{R: 70, G: 90, B: 130, A: 100})
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

// handleDetailTabClick consumes a click on the detail-view tab strip
// or a row in the descendant-tree tab. Returns true if consumed.
func (p *Panel) handleDetailTabClick(mx, my int) bool {
	for _, t := range p.detailTabRects {
		if mx >= t.x && mx < t.x+t.w && my >= t.y && my < t.y+t.h {
			p.selectedTab = t.tab
			return true
		}
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
			p.simulation.Select(row.nodeID)
			p.grid.SetManualSelection()
			p.grid.doRefresh = true
			return true
		}
	}
	return false
}
