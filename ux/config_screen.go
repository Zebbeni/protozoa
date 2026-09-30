package ux

import (
	"fmt"
	"image/color"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

const (
	cfgPanelWidth   = 700
	cfgLabelWidth   = 340
	cfgSliderWidth  = 200
	cfgValueWidth   = 120
	cfgRowHeight    = 18
	cfgPadding      = 10
	cfgHeaderHeight = 30
	cfgButtonHeight = 30
	cfgButtonWidth  = 200
)

type configField struct {
	label    string
	jsonTag  string
	kind     reflect.Kind // Int, Float64, Bool
	fieldIdx int          // index into Globals struct
	// textOnly disables the slider/toggle for this row and shows just the value as text.
	textOnly bool
	// row selects special rows that aren't a plain Globals field.
	row     rowType
	ability int
	// familyRoot is the Go name of a condition family's root, for rowConditionFamily.
	familyRoot  string
	graphToggle bool
	curve       physiology.CurveID
	// curveAbility is the ability a header or block row belongs to: its block holds every curve that ability drives.
	curveAbility physiology.Ability
	// shape is the curve shape a K field belongs to.
	shape physiology.ShapeKind
	// hidden keeps a field in the section.
	hidden bool
}

// rowType distinguishes the config screen's special rows from ordinary field rows.
type rowType int

const (
	rowField rowType = iota
	rowAbilityScore
	rowAbilityTotal
	// rowCurveGraph is a curve's graph block, zero-height until its toggle is expanded.
	rowCurveGraph
	rowCurveHeader
	// rowWallPhGraph is the wall-permeability curve, drawn under the two settings that shape it.
	rowWallPhGraph
	// rowDesign is one saved organism design, toggled on or off as a founder of the simulation.
	rowDesign
	// rowNodeType is one decision-tree action or condition, toggled on or off as something mutation may put into a tree.
	rowNodeType
	// rowConditionFamily is the indented "include advanced" tick under a condition family's basic read.
	rowConditionFamily
)

type configSection struct {
	title     string
	fields    []configField
	collapsed bool
}

type ConfigScreen struct {
	globals    *c.Globals
	initValues c.Globals // snapshot of initial values for stable slider ranges
	// defaults is the shipped default configuration (settings/default.json), used by the per-row reset buttons and RESTORE ALL DEFAULTS.
	defaults c.Globals
	sections []configSection

	scrollY      float64
	selectedRow  int // -1 for none
	editingValue string
	// designs is the saved organism designs the INITIAL DESIGNS rows were built from, read once when the screen opens so a row index keeps meaning the same design.
	designs  []organism.Design
	accepted bool

	draggingSlider bool
	dragField      *configField
	dragSlider     *sliderRect

	// Embedded mode: when true the screen skips painting its own background and START button (the popup owns those).
	embedded bool
	// readOnly shows the settings without letting them change: no sliders, toggles, text entry or reset buttons.
	readOnly bool
	// graphExpanded records which abilities' blocks are showing.
	graphExpanded map[physiology.Ability]bool
	graphCanvas   *ebiten.Image
	// hoverKey / hoverSince track which row the mouse is resting on and since when.
	hoverKey       string
	hoverSince     time.Time
	viewportLeft   int
	viewportTop    int
	viewportRight  int
	viewportBottom int
}

func NewConfigScreen(globals *c.Globals) *ConfigScreen {
	globals.InitialAbilityScores = normalizedInitialScores(globals.InitialAbilityScores)
	defaults := loadConfigDefaults()
	defaults.InitialAbilityScores = normalizedInitialScores(defaults.InitialAbilityScores)
	cs := &ConfigScreen{
		globals:       globals,
		initValues:    *globals,
		defaults:      defaults,
		selectedRow:   -1,
		graphExpanded: map[physiology.Ability]bool{},
	}
	cs.buildSections()
	return cs
}

func (cs *ConfigScreen) Globals() *c.Globals {
	return cs.globals
}

// SetEmbedded switches the screen into popup-body mode and binds the scroll viewport to the given screen-coordinate range.
func (cs *ConfigScreen) SetEmbedded(left, top, right, bottom int) {
	cs.embedded = true
	cs.viewportLeft = left
	cs.viewportTop = top
	cs.viewportRight = right
	cs.viewportBottom = bottom
}

// SetReadOnly makes the screen a viewer for its settings; see readOnly.
func (cs *ConfigScreen) SetReadOnly() {
	cs.readOnly = true
	cs.selectedRow = -1
	cs.editingValue = ""
}

func (cs *ConfigScreen) panelWidth() int {
	if !cs.embedded {
		return cfgPanelWidth
	}
	avail := cs.viewportRight - cs.viewportLeft
	if avail > cfgPanelWidth {
		return cfgPanelWidth
	}
	if avail < cfgPanelWidth/2 {
		// Pathologically narrow popup — clamp so layout math stays sane.
		return cfgPanelWidth / 2
	}
	return avail
}

// panelX returns the form's left edge in screen coordinates.
func (cs *ConfigScreen) panelX() int {
	pw := cs.panelWidth()
	if cs.embedded {
		return cs.viewportLeft + (cs.viewportRight-cs.viewportLeft-pw)/2
	}
	return (c.ScreenWidth() - pw) / 2
}

// sliderColWidth returns the slider column's pixel width given the current panel width.
func (cs *ConfigScreen) sliderColWidth() int {
	const interColGap = 20 // total horizontal padding between label/slider/value
	sw := cs.panelWidth() - cfgLabelWidth - cfgValueWidth - interColGap
	if sw < 50 {
		sw = 50
	}
	if sw > cfgSliderWidth {
		sw = cfgSliderWidth
	}
	return sw
}

// Update handles input; returns true when the user accepts the config.
func (cs *ConfigScreen) Update() bool {
	if cs.accepted {
		return true
	}

	_, wy := ebiten.Wheel()
	cs.scrollY -= wy * 30
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		cs.scrollY += float64(cfgRowHeight)
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		cs.scrollY -= float64(cfgRowHeight)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		cs.scrollY += float64(cfgRowHeight * 10)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		cs.scrollY -= float64(cfgRowHeight * 10)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		cs.scrollY = float64(cs.contentHeight())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		cs.scrollY = 0
	}

	maxScroll := cs.maxScroll()
	if cs.scrollY > float64(maxScroll) {
		cs.scrollY = float64(maxScroll)
	}
	if cs.scrollY < 0 {
		cs.scrollY = 0
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		cs.handleClick()
	} else if !cs.readOnly && cs.draggingSlider && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		cs.handleSliderDrag()
	}

	// Clamp scroll once more in case the click moved the layout.
	maxScroll = cs.maxScroll()
	if cs.scrollY > float64(maxScroll) {
		cs.scrollY = float64(maxScroll)
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		cs.draggingSlider = false
		cs.dragField = nil
		cs.dragSlider = nil
	}

	if cs.selectedRow >= 0 {
		cs.handleKeyboard()
	}

	return false
}

// contentHeight returns the total pixel height of all sections, fields, inter-section gaps.
func (cs *ConfigScreen) contentHeight() int {
	h := cfgHeaderHeight
	for _, section := range cs.sections {
		h += cfgRowHeight + 4 // header
		if !section.collapsed {
			for _, field := range section.fields {
				h += cs.rowHeight(field)
			}
		}
		h += 6 // gap between sections
	}
	if !cs.embedded {
		h += cfgPadding + cfgButtonHeight
	}
	return h
}

// maxScroll returns the largest valid scrollY value given the current viewport.
func (cs *ConfigScreen) maxScroll() int {
	var visible int
	if cs.embedded {
		visible = cs.viewportBottom - cs.viewportTop
	} else {
		visible = c.ScreenHeight() - cfgButtonHeight - cfgPadding*2
	}
	maxScroll := cs.contentHeight() - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

func (cs *ConfigScreen) Draw(screen *ebiten.Image) {
	if !cs.embedded {
		fillThemeBackground(screen)
	}

	panelX := cs.panelX()
	panelW := cs.panelWidth()
	panelTop := cs.panelTop()
	clipTop, clipBottom := cs.clipRange()

	if !cs.embedded {
		title := "SIMULATION SETTINGS"
		titleBounds := boundString(r.FontSourceCodePro12, title)
		titleX := panelX + (panelW-titleBounds.Dx())/2
		text.Draw(screen, title, r.FontSourceCodePro12, titleX, panelTop+titleBounds.Dy(), themedForeground())
	}

	if bx, by, bw, bh := cs.restoreAllRect(); cs.readOnly {
		if by+bh > clipTop && by < clipBottom {
			note := "Highlighted values differ from the defaults"
			text.Draw(screen, note, r.FontSourceCodePro10, panelX, by+bh-5, changedFromDefaultInk())
		}
	} else if by+bh > clipTop && by < clipBottom {
		cs.drawSmallButton(screen, bx, by, bw, bh, "RESTORE ALL DEFAULTS", !cs.allDefaults())
	}

	y := panelTop + cfgHeaderHeight - int(cs.scrollY)
	rowIdx := 0

	for _, section := range cs.sections {
		// Section header: prefix with ▼ when expanded, ▶ when collapsed so the user can see at a glance which sections have visible fields below them.
		if y+cfgRowHeight > clipTop && y < clipBottom {
			marker := "▶" // ▶
			if !section.collapsed {
				marker = "▼" // ▼
			}
			titleColor := color.Color(themedSectionTitle())
			if cs.sectionChanged(section) {
				titleColor = themedChanged()
			}
			text.Draw(screen, marker+" "+section.title, r.FontSourceCodePro12, panelX, y+12, titleColor)
		}
		y += cfgRowHeight + 4

		if section.collapsed {
			rowIdx += len(section.fields)
		} else {
			for _, field := range section.fields {
				rowH := cs.rowHeight(field)
				if field.row == rowCurveGraph {
					cs.drawGraphRow(screen, panelX, y, field)
				} else if rowH == 0 {
					// A row that isn't in play — another shape's K — takes no space and draws nothing.
					rowIdx++
					continue
				} else if y+rowH > clipTop && y < clipBottom {
					switch field.row {
					case rowWallPhGraph:
						cs.drawWallPhGraphRow(screen, panelX, y)
					case rowCurveHeader:
						cs.drawCurveHeaderRow(screen, panelX, y, field)
					case rowAbilityScore:
						cs.drawAbilityScoreRow(screen, panelX, y, rowIdx, field)
					case rowAbilityTotal:
						cs.drawAbilityTotalRow(screen, panelX, y)
					case rowDesign:
						cs.drawDesignRow(screen, panelX, y, field)
					case rowNodeType:
						cs.drawNodeTypeRow(screen, panelX, y, field)
					case rowConditionFamily:
						cs.drawConditionFamilyRow(screen, panelX, y, field)
					default:
						cs.drawRow(screen, panelX, y, rowIdx, field)
					}
					if cs.canReset(field) && !cs.readOnly {
						bx, by, bw, bh := cs.resetRect(panelX, y)
						cs.drawSmallButton(screen, bx, by, bw, bh, "reset", true)
					}
					if field.graphToggle {
						cs.drawGraphToggle(screen, panelX, y, field.curveAbility)
					}
				}
				y += rowH
				rowIdx++
			}
		}
		y += 6 // gap between sections
	}

	// Standalone START button — popup mode hides this.
	if !cs.embedded {
		defer cs.DrawTooltip(screen)
		btnX := panelX + (panelW-cfgButtonWidth)/2
		btnY := y + cfgPadding
		if btnY > -cfgButtonHeight && btnY < c.ScreenHeight() {
			cs.drawButton(screen, btnX, btnY, cfgButtonWidth, cfgButtonHeight, "START SIMULATION")
		}
	}
}

// panelTop returns the y-coordinate of the form's top edge.
func (cs *ConfigScreen) panelTop() int {
	if cs.embedded {
		return cs.viewportTop
	}
	return 20
}

// clipRange is the screen-coord band rows must overlap to be drawn.
func (cs *ConfigScreen) clipRange() (int, int) {
	if cs.embedded {
		return cs.viewportTop, cs.viewportBottom
	}
	return 0, c.ScreenHeight()
}

func (cs *ConfigScreen) drawRow(screen *ebiten.Image, px, py, rowIdx int, field configField) {
	v := reflect.ValueOf(cs.globals).Elem()
	fv := v.Field(field.fieldIdx)

	isSelected := rowIdx == cs.selectedRow
	labelColor := themedLabel()
	valueColor := themedValue()
	if cs.canReset(field) {
		valueColor = themedChanged()
	}
	if isSelected {
		ebitenutil.DrawRect(screen, float64(px), float64(py-2), float64(cs.panelWidth()), float64(cfgRowHeight), themedSelectedRow())
		valueColor = themedSelectedInk()
	}

	// Label (shifted right on "@0" curve rows to make room for the graph toggle drawn over the left edge).
	labelX := px
	if field.graphToggle {
		labelX += graphToggleW + 2
	}
	text.Draw(screen, field.label, r.FontSourceCodePro10, labelX, py+10, labelColor)

	if cs.readOnly {
		cs.drawReadOnlyValue(screen, px+cfgLabelWidth, py, cs.getValueStr(fv, field, false), field)
		return
	}

	sliderW := cs.sliderColWidth()
	valueStr := cs.getValueStr(fv, field, isSelected)
	text.Draw(screen, valueStr, r.FontSourceCodePro10, px+cfgLabelWidth+sliderW+10, py+10, valueColor)

	if field.textOnly {
		return
	}

	if field.kind == reflect.Float64 || field.kind == reflect.Int {
		cs.drawSlider(screen, px+cfgLabelWidth, py, fv, field)
	}

	if field.kind == reflect.Bool {
		cs.drawToggle(screen, px+cfgLabelWidth, py, fv.Bool())
	}
}

// changedFromDefaultInk marks values that differ from the defaults, in the editor as well as the read-only viewer.
func changedFromDefaultInk() color.RGBA { return themedChanged() }

// drawReadOnlyValue draws a read-only row's value at x, highlighted with the default beside it when it differs from the default.
func (cs *ConfigScreen) drawReadOnlyValue(screen *ebiten.Image, x, py int, value string, field configField) {
	if !cs.canReset(field) {
		text.Draw(screen, value, r.FontSourceCodePro10, x, py+10, themedValue())
		return
	}
	text.Draw(screen, value, r.FontSourceCodePro10, x, py+10, changedFromDefaultInk())
	def := ""
	if field.row == rowAbilityScore {
		def = strconv.Itoa(cs.defaults.InitialAbilityScores[field.ability])
	} else {
		def = cs.getValueStr(reflect.ValueOf(cs.defaults).Field(field.fieldIdx), field, false)
	}
	vb := boundString(r.FontSourceCodePro10, value)
	text.Draw(screen, "(default "+def+")", r.FontSourceCodePro10, x+vb.Dx()+12, py+10, themedMuted())
}

func (cs *ConfigScreen) getValueStr(fv reflect.Value, field configField, isEditing bool) string {
	if isEditing && cs.editingValue != "" {
		return cs.editingValue + "_"
	}
	switch field.kind {
	case reflect.Int:
		return strconv.Itoa(int(fv.Int()))
	case reflect.Float64:
		return formatConfigValue(fv.Float())
	case reflect.Bool:
		if fv.Bool() {
			return "true"
		}
		return "false"
	}
	return "?"
}

// configValueDecimals is how much of a float the config screen shows, and the precision a user edit is held to.
const configValueDecimals = 5

func roundConfigValue(v float64) float64 {
	pow := math.Pow(10, configValueDecimals)
	return math.Round(v*pow) / pow
}

func formatConfigValue(v float64) string {
	s := strconv.FormatFloat(v, 'f', configValueDecimals, 64)
	s = strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
	if v != 0 && (s == "0" || s == "-0") {
		return strconv.FormatFloat(v, 'g', 3, 64)
	}
	return s
}

func (cs *ConfigScreen) drawSlider(screen *ebiten.Image, x, y int, fv reflect.Value, field configField) {
	sx, sy := float64(x), float64(y+4)
	sw := float64(cs.sliderColWidth())
	sh := float64(cfgRowHeight - 8)

	ebitenutil.DrawRect(screen, sx, sy, sw, sh, themedTrack())

	// Fill based on value ratio (estimate range from value magnitude)
	ratio := cs.getSliderRatio(fv, field)
	fillW := sw * ratio
	ebitenutil.DrawRect(screen, sx, sy, fillW, sh, themedTrackFill())

	handleX := sx + fillW - 2
	if handleX < sx {
		handleX = sx
	}
	ebitenutil.DrawRect(screen, handleX, sy, 4, sh, themedTrackHandle())
}

func (cs *ConfigScreen) drawToggle(screen *ebiten.Image, x, y int, val bool) {
	sx, sy := float64(x), float64(y+2)
	w, h := 40.0, float64(cfgRowHeight-4)

	if val {
		ebitenutil.DrawRect(screen, sx, sy, w, h, color.RGBA{R: 60, G: 140, B: 60, A: 255})
		ebitenutil.DrawRect(screen, sx+w-h, sy, h, h, color.White)
	} else {
		ebitenutil.DrawRect(screen, sx, sy, w, h, color.RGBA{R: 80, G: 80, B: 80, A: 255})
		ebitenutil.DrawRect(screen, sx, sy, h, h, color.RGBA{R: 160, G: 160, B: 160, A: 255})
	}
}

func (cs *ConfigScreen) drawButton(screen *ebiten.Image, x, y, w, h int, label string) {
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h),
		color.RGBA{R: 40, G: 100, B: 40, A: 255})
	bounds := boundString(r.FontSourceCodePro12, label)
	tx := x + (w-bounds.Dx())/2
	ty := y + (h+bounds.Dy())/2
	// Button label stays white regardless of theme since the button has its own coloured background.
	text.Draw(screen, label, r.FontSourceCodePro12, tx, ty, color.White)
}

func (cs *ConfigScreen) handleClick() {
	mx, my := ebiten.CursorPosition()
	panelX := cs.panelX()
	panelW := cs.panelWidth()
	sliderW := cs.sliderColWidth()
	panelTop := cs.panelTop()
	clipTop, clipBottom := cs.clipRange()

	// In embedded mode, ignore clicks outside the body region.
	if cs.embedded && (my < clipTop || my >= clipBottom) {
		return
	}

	cs.commitEdit()

	if bx, by, bw, bh := cs.restoreAllRect(); !cs.readOnly && mx >= bx && mx < bx+bw && my >= by && my < by+bh {
		cs.restoreAllDefaults()
		return
	}

	y := panelTop + cfgHeaderHeight - int(cs.scrollY)
	rowIdx := 0

	for i := range cs.sections {
		section := &cs.sections[i]
		// Header hitbox: clicking anywhere across the full panel width on the header row toggles the section's collapsed state.
		headerY := y
		if my >= headerY && my < headerY+cfgRowHeight+4 && mx >= panelX && mx < panelX+panelW {
			section.collapsed = !section.collapsed
			cs.selectedRow = -1
			cs.editingValue = ""
			return
		}
		y += cfgRowHeight + 4

		if section.collapsed {
			rowIdx += len(section.fields)
			y += 6
			continue
		}
		for _, field := range section.fields {
			rowH := cs.rowHeight(field)
			if rowH > 0 && my >= y-2 && my < y+rowH-2 {
				if field.row == rowCurveGraph {
					cs.handleCurveBlockClick(panelX, y, mx, my, field.curveAbility)
					return
				}

				if field.graphToggle && mx >= panelX && mx < panelX+graphToggleW {
					cs.graphExpanded[field.curveAbility] = !cs.graphExpanded[field.curveAbility]
					return
				}
				if cs.readOnly {
					return
				}
				if bx, by, bw, bh := cs.resetRect(panelX, y); cs.canReset(field) &&
					mx >= bx && mx < bx+bw && my >= by && my < by+bh {
					cs.resetField(field)
					return
				}
				sliderX := panelX + cfgLabelWidth
				switch field.row {
				case rowAbilityTotal:
					return
				case rowDesign:
					if !cs.readOnly {
						cs.toggleDesign(field.ability)
					}
					return
				case rowNodeType:
					if !cs.readOnly {
						cs.toggleNodeType(field.jsonTag)
					}
					return
				case rowConditionFamily:
					if !cs.readOnly {
						cs.toggleConditionFamily(field.familyRoot)
					}
					return
				case rowCurveHeader:
					// Anywhere on the row opens or closes the curve's block; the controls live inside it.
					cs.graphExpanded[field.curveAbility] = !cs.graphExpanded[field.curveAbility]
					return
				case rowAbilityScore:
					if cs.globals.RandomInitialAbilities {
						return // greyed out while Random is on
					}
					step := 1
					if ebiten.IsKeyPressed(ebiten.KeyShift) {
						step = 10
					}
					switch {
					case mx >= sliderX && mx < sliderX+abilityBtnW:
						cs.adjustAbilityScore(field.ability, -step)
						return
					case mx >= sliderX+sliderW-abilityBtnW && mx < sliderX+sliderW:
						cs.adjustAbilityScore(field.ability, step)
						return
					}
				}
				// Check if clicked on slider area (skipped for text-only fields)
				if !field.textOnly {
					if field.kind == reflect.Bool && mx >= sliderX && mx < sliderX+40 {
						cs.toggleBool(field)
						return
					}
					if (field.kind == reflect.Float64 || field.kind == reflect.Int) &&
						mx >= sliderX && mx < sliderX+sliderW {
						cs.handleSliderClick(mx-sliderX, field)
						return
					}
				}
				cs.selectedRow = rowIdx
				cs.editingValue = ""
				return
			}
			y += rowH
			rowIdx++
		}
		y += 6
	}

	if !cs.embedded {
		btnX := panelX + (panelW-cfgButtonWidth)/2
		btnY := y + cfgPadding
		if mx >= btnX && mx < btnX+cfgButtonWidth && my >= btnY && my < btnY+cfgButtonHeight {
			if cs.StartBlockedReason() == "" {
				cs.accepted = true
			}
			return
		}
	}

	cs.selectedRow = -1
}

func (cs *ConfigScreen) handleKeyboard() {
	chars := ebiten.AppendInputChars(nil)
	for _, ch := range chars {
		s := string(ch)
		if strings.ContainsAny(s, "0123456789.-+eE") {
			cs.editingValue += s
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if len(cs.editingValue) > 0 {
			cs.editingValue = cs.editingValue[:len(cs.editingValue)-1]
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		cs.commitEdit()
		cs.selectedRow = -1
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		cs.editingValue = ""
		cs.selectedRow = -1
	}
}

func (cs *ConfigScreen) commitEdit() {
	if cs.selectedRow < 0 || cs.editingValue == "" {
		return
	}
	field := cs.getFieldByRow(cs.selectedRow)
	if field == nil {
		return
	}
	if field.row == rowAbilityScore {
		if val, err := strconv.Atoi(cs.editingValue); err == nil && !cs.globals.RandomInitialAbilities {
			cs.setAbilityScore(field.ability, val)
		}
		cs.editingValue = ""
		return
	}
	v := reflect.ValueOf(cs.globals).Elem()
	fv := v.Field(field.fieldIdx)

	b, bounded := fixedBounds[field.jsonTag]
	switch field.kind {
	case reflect.Int:
		if val, err := strconv.Atoi(cs.editingValue); err == nil {
			if bounded {
				val = min(int(b[1]), max(int(b[0]), val))
			}
			fv.SetInt(int64(val))
		}
	case reflect.Float64:
		if val, err := strconv.ParseFloat(cs.editingValue, 64); err == nil {
			if bounded {
				val = min(b[1], max(b[0], val))
			}
			// Typing "-650" into a damage field means 650 damage, not a heal.
			fv.SetFloat(c.NormalizeSign(roundConfigValue(val), field.jsonTag))
		}
	}
	cs.editingValue = ""
}

func (cs *ConfigScreen) toggleBool(field configField) {
	v := reflect.ValueOf(cs.globals).Elem()
	fv := v.Field(field.fieldIdx)
	fv.SetBool(!fv.Bool())
}

func (cs *ConfigScreen) handleSliderClick(relX int, field configField) {
	cs.draggingSlider = true
	cs.dragField = &field
	ratio := float64(relX) / float64(cs.sliderColWidth())
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	cs.setValueFromSliderRatio(ratio, field)
}

// sliderRect is where a drag-in-progress slider sits: its left edge and width, in screen coordinates.
type sliderRect struct{ x, w int }

func (cs *ConfigScreen) handleSliderDrag() {
	if cs.dragField == nil {
		return
	}
	mx, _ := ebiten.CursorPosition()
	sliderX, sliderW := cs.panelX()+cfgLabelWidth, cs.sliderColWidth()
	if cs.dragSlider != nil {
		sliderX, sliderW = cs.dragSlider.x, cs.dragSlider.w
	}
	relX := mx - sliderX
	ratio := float64(relX) / float64(sliderW)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	cs.setValueFromSliderRatio(ratio, *cs.dragField)
}

func (cs *ConfigScreen) getSliderRatio(fv reflect.Value, field configField) float64 {
	min, max := cs.getSliderRange(field)
	if max == min {
		return 0.5
	}
	var val float64
	switch field.kind {
	case reflect.Int:
		val = float64(fv.Int())
	case reflect.Float64:
		val = fv.Float()
	}
	r := (val - min) / (max - min)
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	return r
}

func (cs *ConfigScreen) setValueFromSliderRatio(ratio float64, field configField) {
	min, max := cs.getSliderRange(field)
	val := min + ratio*(max-min)
	v := reflect.ValueOf(cs.globals).Elem()
	fv := v.Field(field.fieldIdx)
	switch field.kind {
	case reflect.Int:
		fv.SetInt(int64(math.Round(val)))
	case reflect.Float64:
		fv.SetFloat(c.NormalizeSign(roundConfigValue(val), field.jsonTag))
	}
}

// getSliderRange returns sensible min/max for the slider based on the initial default value (stable.
func (cs *ConfigScreen) getSliderRange(field configField) (float64, float64) {
	iv := reflect.ValueOf(cs.initValues)
	fv := iv.Field(field.fieldIdx)
	var initial float64
	switch field.kind {
	case reflect.Int:
		initial = float64(fv.Int())
	case reflect.Float64:
		initial = fv.Float()
	}

	// Chemo efficiency multipliers represent "fraction of base chemo rate" — semantically bounded to [0, 1].
	if strings.HasSuffix(field.jsonTag, "_chemo_efficiency_mult") {
		return 0, 1
	}
	// Initial and ideal pH values are points on the fixed pH scale.
	switch field.jsonTag {
	case "ideal_ph_range":
		// A range, not a point on the scale.
		return 0, c.MaxPh() - c.MinPh()
	}
	if b, ok := fixedBounds[field.jsonTag]; ok {
		return b[0], b[1]
	}
	// A K row's slider runs over the range its own shape uses.
	if tag, ok := curveKTags[field.jsonTag]; ok {
		lo, hi := tag.shape.KRange()
		return lo, hi
	}
	return centeredSliderRange(field.jsonTag, initial)
}

var fixedBounds = map[string][2]float64{
	// A one-node tree is a lone action.
	"max_decision_tree_size": {1, 32},
	// Chemosynthesis uses the saturating curve, whose K is a score scale.
	"chemosynthesis_curve_k": {physiology.MinSaturatingK, physiology.MaxSaturatingK},
	// No unlimited option on purpose.
	"max_replay_size_mb": {50, 1000},
	// A fraction of the diffusion a full-strength wall takes away.
	"wall_ph_block_at_max":  {0, 1},
	"wall_ph_block_curve":   {0, 4},
	"burrow_spoil_fraction": {0, 1},
	// The size ratios are bounded where their names stop meaning what they say.
	"much_bigger_size_ratio":  {1, 10},
	"much_smaller_size_ratio": {0, 1},
}

func centeredSliderRange(jsonTag string, initial float64) (float64, float64) {
	// A setting that only makes sense with one sign gets a slider that can't cross zero.
	if sign := c.SettingSign(jsonTag); sign != 0 {
		span := 2 * math.Abs(initial)
		if span == 0 {
			span = 1
		}
		if sign < 0 {
			return -span, 0
		}
		return 0, span
	}
	if strings.Contains(jsonTag, "health_change") {
		if initial == 0 {
			return -1, 1
		}
		spread := 2 * math.Abs(initial)
		return initial - spread, initial + spread
	}
	if initial <= 0 {
		return 0, 1
	}
	return 0, 2 * initial
}

func (cs *ConfigScreen) getFieldByRow(rowIdx int) *configField {
	idx := 0
	for _, section := range cs.sections {
		for i := range section.fields {
			if idx == rowIdx {
				return &section.fields[i]
			}
			idx++
		}
	}
	return nil
}

func (cs *ConfigScreen) buildSections() {
	gType := reflect.TypeOf(c.Globals{})
	byTag := map[string]int{}
	for i := 0; i < gType.NumField(); i++ {
		tag := gType.Field(i).Tag.Get("json")
		byTag[tag] = i
	}

	field := func(label, jsonTag string) configField {
		idx, ok := byTag[jsonTag]
		if !ok {
			panic("config screen: no Globals field with json tag " + jsonTag)
		}
		return configField{
			label:    label,
			jsonTag:  jsonTag,
			kind:     gType.Field(idx).Type.Kind(),
			fieldIdx: idx,
		}
	}
	textField := func(label, jsonTag string) configField {
		f := field(label, jsonTag)
		f.textOnly = true
		return f
	}

	cs.sections = []configSection{
		{title: "— SIMULATION —", fields: []configField{
			textField("Seed (0 = random)", "seed"),
		}},
		{title: "— DISPLAY —", fields: []configField{
			field("Grid Units Wide", "grid_units_wide"),
			field("Grid Units High", "grid_units_high"),
			field("Screen Width", "screen_width"),
			field("Screen Height", "screen_height"),
		}},
		{title: "— ENVIRONMENT —", fields: []configField{
			field("Initial Organisms", "initial_organisms"),
			field("Initial Food", "initial_food"),
			field("Initial Walls", "initial_walls"),
			field("Chance to Add Food", "chance_to_add_food_item"),
			field("Min Food Value", "min_food_value"),
			field("Max Food Value", "max_food_value"),
		}},
		{title: "— PH —", fields: []configField{
			field("Ideal pH Range", "ideal_ph_range"),
			field("Ideal pH Mutation Step", "ideal_ph_mutation_step"),
			field("pH Diffuse Factor", "ph_diffuse_factor"),
			field("Wall pH Block at Max Strength", "wall_ph_block_at_max"),
			field("Wall pH Block Curve", "wall_ph_block_curve"),
			{label: "", jsonTag: "", row: rowWallPhGraph, fieldIdx: -1},
			field("pH Increment to Display", "ph_increment_to_display"),
			// Currents: how hard the flow field skews pH diffusion, and how fast an un-tended current fades.
		}},
		{title: "— ORGANISMS —", fields: []configField{
			field("Min Organisms", "min_organisms"),
			field("Max Cycles (0=unlimited)", "max_cycles"),
			field("Max Replay Size (MB)", "max_replay_size_mb"),
			field("Max Organisms", "max_organisms"),
			field("Growth Factor", "growth_factor"),
			field("Maximum Max Size", "maximum_max_size"),
			field("Minimum Max Size", "minimum_max_size"),
			field("Max Initial Size", "maximum_initial_size"),
			field("Max Initial Spawn Health", "maximum_initial_spawn_health"),
			field("Max Cycles Between Spawns", "max_cycles_between_spawns"),
			field("Max Initial Cycles Between Spawns", "max_initial_cycles_between_spawns"),
			field("Min Spawn Health", "min_spawn_health"),
			field("Max Spawn Health Percent", "max_spawn_health_percent"),
			field("Max Lifespan (0=off)", "max_lifespan"),
		}},
		{title: "— DECISION TREES —", fields: append([]configField{
			field("Initial Mutations", "initial_organism_decision_tree_mutations"),
			field("Chance to Mutate", "chance_to_mutate_decision_tree"),
			field("Max Tree Size", "max_decision_tree_size"),
			field("Smart Mutation", "smart_tree_mutation"),
			field("Tiered Conditions", "tiered_condition_mutation"),
			field("Weight: Swap Action", "mutation_weight_swap_action"),
			field("Weight: Grow Branch", "mutation_weight_grow_branch"),
			field("Weight: Swap Condition", "mutation_weight_swap_condition"),
			field("Weight: Prune Branch", "mutation_weight_prune_branch"),
			field("\"Much Bigger\" Size Ratio", "much_bigger_size_ratio"),
			field("\"Much Smaller\" Size Ratio", "much_smaller_size_ratio"),
			field("\"Much Food\" Per Size", "much_food_per_size"),
		}, nodeTypeFields()...)},
		{title: "— HEALTH CHANGES —", fields: []configField{
			// Costs and gains that belong to no single ability.
			field("Idle", "health_change_from_idle"),
			field("Spawning", "health_change_from_spawning"),
			field("Blocked Move", "health_change_from_blocked_move"),
			field("Corpse Food Multiplier", "corpse_food_multiplier"),
			field("Burial Amount", "burial_amount"),
			field("Burial Interval (0=off)", "burial_interval"),
			field("Initial Buried Cells", "initial_buried_food"),
			field("Min Initial Buried Value", "min_initial_buried_value"),
			field("Max Initial Buried Value", "max_initial_buried_value"),
		}},
		{title: "— INITIAL DESIGNS —", fields: initialDesignFields()},
		{title: "— INITIAL ABILITIES —", fields: initialAbilityFields(field("Random Initial Abilities", "random_initial_abilities"))},
		{title: "— ABILITIES —", fields: withCurveGraphs([]configField{
			field("Chance to Mutate", "chance_to_mutate_abilities"),
			// One row per curve, opening onto its graphs, its shape and every knob that curve scales.
			field("Chemosynthesis K", "chemosynthesis_cosine_k"),
			field("Chemosynthesis K", "chemosynthesis_saturating_k"),
			field("Chemosynthesis pH push K", "chemo_ph_effect_cosine_k"),
			field("Chemosynthesis pH push K", "chemo_ph_effect_saturating_k"),
			field("Eating K", "eating_cosine_k"),
			field("Eating K", "eating_saturating_k"),
			field("Eating pH push K", "eating_ph_effect_cosine_k"),
			field("Eating pH push K", "eating_ph_effect_saturating_k"),
			field("Eating cost K", "eating_cost_cosine_k"),
			field("Eating cost K", "eating_cost_saturating_k"),
			field("Movement cost K", "movement_cost_cosine_k"),
			field("Movement cost K", "movement_cost_saturating_k"),
			field("Digging cost K", "digging_cost_cosine_k"),
			field("Digging cost K", "digging_cost_saturating_k"),
			field("Digging removal K", "digging_strength_cosine_k"),
			field("Digging removal K", "digging_strength_saturating_k"),
			field("Digging creation K", "digging_creation_cosine_k"),
			field("Digging creation K", "digging_creation_saturating_k"),
			field("Attack K", "attack_cosine_k"),
			field("Attack K", "attack_saturating_k"),
			field("Damage taken K", "damage_taken_cosine_k"),
			field("Damage taken K", "damage_taken_saturating_k"),
			field("Thorns K", "thorns_cosine_k"),
			field("Thorns K", "thorns_saturating_k"),
			field("pH Tolerance K", "ph_tolerance_cosine_k"),
			field("pH Tolerance K", "ph_tolerance_saturating_k"),
		})},
		{title: "— APPEARANCE —", fields: []configField{
			// Score at which each sprite overlay starts being drawn.
			field("Shell Body", "shell_body_threshold"),
			field("Spikes Body", "spikes_body_threshold"),
			field("Pili Motor", "pili_motor_threshold"),
			field("Flagella Motor", "flagella_motor_threshold"),
			field("Teeth Mouth", "teeth_mouth_threshold"),
			field("Fangs Mouth", "fangs_mouth_threshold"),
			field("Tusks Mouth", "tusks_mouth_threshold"),
			field("Sensor Min Conditions", "sensor_min_conditions"),
		}},
		{title: "— STATISTICS —", fields: []configField{
			field("Population Update Interval", "population_update_interval"),
		}},
	}
	for i := range cs.sections {
		cs.sections[i].collapsed = true
	}
}

// initialDesignFields builds the INITIAL DESIGNS section: one toggle per saved design.
func initialDesignFields() []configField {
	designs := organism.LoadDesigns(organism.DesignsDir)
	if len(designs) == 0 {
		return []configField{{label: "  (none saved — build one in the Organism Designer)", textOnly: true}}
	}
	fields := make([]configField, 0, len(designs))
	for i, ds := range designs {
		fields = append(fields, configField{label: ds.Name, row: rowDesign, ability: i})
	}
	return fields
}

// designNames lists the saved designs in the same order the rows are built.
func (cs *ConfigScreen) designNames() []string {
	if cs.designs == nil {
		cs.designs = organism.LoadDesigns(organism.DesignsDir)
	}
	out := make([]string, len(cs.designs))
	for i, ds := range cs.designs {
		out[i] = ds.Name
	}
	return out
}

// designChosen reports whether the design at index i is one of the simulation's founders.
func (cs *ConfigScreen) designChosen(i int) bool {
	names := cs.designNames()
	if i < 0 || i >= len(names) {
		return false
	}
	for _, n := range cs.globals.InitialDesigns {
		if n == names[i] {
			return true
		}
	}
	return false
}

func (cs *ConfigScreen) toggleDesign(i int) {
	names := cs.designNames()
	if i < 0 || i >= len(names) {
		return
	}
	name := names[i]
	for j, n := range cs.globals.InitialDesigns {
		if n == name {
			cs.globals.InitialDesigns = append(cs.globals.InitialDesigns[:j], cs.globals.InitialDesigns[j+1:]...)
			return
		}
	}
	cs.globals.InitialDesigns = append(cs.globals.InitialDesigns, name)
}

// designInList reports whether the design at index i appears in a list of founder names.
func designInList(list, names []string, i int) bool {
	if i < 0 || i >= len(names) {
		return false
	}
	for _, n := range list {
		if n == names[i] {
			return true
		}
	}
	return false
}

func (cs *ConfigScreen) drawDesignRow(screen *ebiten.Image, px, py int, field configField) {
	chosen := cs.designChosen(field.ability)
	box := themedControlFill()
	mark := ""
	if chosen {
		box = color.RGBA{R: 70, G: 130, B: 90, A: 255}
		mark = "x"
	}
	ebitenutil.DrawRect(screen, float64(px+4), float64(py), 14, 14, box)
	if mark != "" {
		// White in both themes: the mark sits on the ticked box's own saturated green, not on the row.
		text.Draw(screen, mark, r.FontSourceCodePro10, px+7, py+11, color.White)
	}
	text.Draw(screen, field.label, r.FontSourceCodePro10, px+26, py+11, themedForeground())

	note, noteCol := "", themedForegroundDim()
	if chosen {
		note = "founder"
	}
	if i := field.ability; i >= 0 && i < len(cs.designs) && cs.designs[i].ExceedsTreeLimit(cs.treeLimit()) {
		note = fmt.Sprintf("%d nodes > %d limit", cs.designs[i].TreeSize(), cs.treeLimit())
		noteCol = themedBad()
	}
	if note != "" {
		text.Draw(screen, note, r.FontSourceCodePro8, px+cfgLabelWidth, py+11, noteCol)
	}
}

const abilityBtnW = 22

func initialAbilityFields(random configField) []configField {
	fields := []configField{random}
	for _, a := range physiology.AllAbilities {
		fields = append(fields, configField{label: a.Name(), row: rowAbilityScore, ability: int(a)})
	}
	return append(fields, configField{label: "Total", row: rowAbilityTotal})
}

// normalizedInitialScores returns a fresh copy of scores with exactly one entry per ability, falling back to the balanced distribution when the setting is missing or the wrong length.
func normalizedInitialScores(scores []int) []int {
	out := make([]int, len(physiology.AllAbilities))
	if len(scores) != len(out) {
		balanced := physiology.BalancedScores()
		for _, a := range physiology.AllAbilities {
			out[a] = balanced[a]
		}
		return out
	}
	copy(out, scores)
	return out
}

func (cs *ConfigScreen) initialScoresTotal() int {
	total := 0
	for _, v := range cs.globals.InitialAbilityScores {
		total += v
	}
	return total
}

func (cs *ConfigScreen) adjustAbilityScore(ability, delta int) {
	cs.setAbilityScore(ability, cs.globals.InitialAbilityScores[ability]+delta)
}

func (cs *ConfigScreen) setAbilityScore(ability, value int) {
	if ability < 0 || ability >= len(cs.globals.InitialAbilityScores) {
		return
	}
	cs.globals.InitialAbilityScores[ability] = min(physiology.MaxAbilityScore, max(0, value))
}

// StartBlockedReason explains why the simulation can't start with the current settings, or returns "" when it can.
func (cs *ConfigScreen) StartBlockedReason() string {
	if cs.globals.RandomInitialAbilities {
		return ""
	}
	if total := cs.initialScoresTotal(); total != physiology.PointTotal {
		return fmt.Sprintf("Initial abilities total %d; they must add up to %d", total, physiology.PointTotal)
	}
	return cs.startBlockedByDesigns(organism.LoadDesigns(organism.DesignsDir))
}

// startBlockedByDesigns explains why the ticked founders can't start the simulation, or "".
func (cs *ConfigScreen) startBlockedByDesigns(available []organism.Design) string {
	chosen := map[string]bool{}
	for _, n := range cs.globals.InitialDesigns {
		chosen[n] = true
	}
	var over []string
	for _, ds := range available {
		if chosen[ds.Name] && ds.ExceedsTreeLimit(cs.treeLimit()) {
			over = append(over, ds.Name)
		}
	}
	if len(over) == 0 {
		return ""
	}
	return fmt.Sprintf("%s has a decision tree over the %d-node limit; untick it or raise Max Tree Size",
		strings.Join(over, ", "), cs.treeLimit())
}

func (cs *ConfigScreen) treeLimit() int {
	return cs.globals.MaxDecisionTreeSize
}

// drawAbilityScoreRow draws one initial ability row: the ability name, then - value + in the slider column.
func (cs *ConfigScreen) drawAbilityScoreRow(screen *ebiten.Image, px, py, rowIdx int, field configField) {
	disabled := cs.globals.RandomInitialAbilities
	labelColor := themedLabel()
	valueColor := themedValue()
	btnColor := themedControlFill()
	if disabled {
		labelColor = themedMuted()
		valueColor = labelColor
		btnColor = themedControlDim()
	}
	if cs.canReset(field) && !disabled {
		valueColor = themedChanged()
	}
	isSelected := rowIdx == cs.selectedRow && !disabled
	if isSelected {
		ebitenutil.DrawRect(screen, float64(px), float64(py-2), float64(cs.panelWidth()), float64(cfgRowHeight), themedSelectedRow())
		valueColor = themedSelectedInk()
	}

	text.Draw(screen, "  "+field.label, r.FontSourceCodePro10, px, py+10, labelColor)

	sliderX := px + cfgLabelWidth
	if cs.readOnly {
		cs.drawReadOnlyValue(screen, sliderX, py, strconv.Itoa(cs.globals.InitialAbilityScores[field.ability]), field)
		return
	}
	sliderW := cs.sliderColWidth()
	btnH := float64(cfgRowHeight - 4)
	for _, b := range []struct {
		x     int
		label string
	}{{sliderX, "-"}, {sliderX + sliderW - abilityBtnW, "+"}} {
		ebitenutil.DrawRect(screen, float64(b.x), float64(py), abilityBtnW, btnH, btnColor)
		lb := boundString(r.FontSourceCodePro12, b.label)
		text.Draw(screen, b.label, r.FontSourceCodePro12, b.x+(abilityBtnW-lb.Dx())/2, py+11, valueColor)
	}

	value := strconv.Itoa(cs.globals.InitialAbilityScores[field.ability])
	if isSelected && cs.editingValue != "" {
		value = cs.editingValue + "_"
	}
	vb := boundString(r.FontSourceCodePro10, value)
	text.Draw(screen, value, r.FontSourceCodePro10, sliderX+(sliderW-vb.Dx())/2, py+10, valueColor)
}

func (cs *ConfigScreen) drawCurveHeaderRow(screen *ebiten.Image, px, py int, field configField) {
	labelColor := themedLabel()
	valueColor := color.Color(themedValue())
	if cs.curveShapeChanged(field.curve) {
		valueColor = themedChanged()
	}
	text.Draw(screen, field.label, r.FontSourceCodePro10, px+graphToggleW+2, py+10, labelColor)

	summary := string(physiology.ShapeFor(cs.globals, field.curve))
	if physiology.ShapeFor(cs.globals, field.curve).UsesK() {
		summary += "  K " + formatConfigValue(curveKValue(cs.globals, field.curve))
	}
	text.Draw(screen, summary, r.FontSourceCodePro10, px+cfgLabelWidth, py+10, valueColor)
}

func (cs *ConfigScreen) handleCurveBlockClick(px, py, mx, my int, a physiology.Ability) {
	if cs.readOnly {
		return
	}
	// An ability with two curves stacks a section per curve; the click belongs to whichever it landed in.
	curve, sectionTop, ok := curveSectionAt(a, py, my)
	if !ok {
		return
	}
	ctrlX, _ := curveCtrlX(px, cs.panelWidth())
	top := sectionTop + curveHeadingH

	if kind, ok := curveShapeButtonAt(ctrlX, top, mx, my); ok {
		cs.setCurveShape(curve, kind)
		return
	}

	fields := cs.curveSliderFields(curve)
	i, onLabel, ok := curveSliderAt(ctrlX, top, len(fields), mx, my)
	if !ok {
		return
	}
	field := fields[i]
	if onLabel {
		// Click a slider's line to type an exact value, the same editing path the form's own rows use.
		cs.selectedRow = cs.rowIndexOfTag(field.jsonTag)
		cs.editingValue = ""
		return
	}
	sx, _, sw, _ := curveSliderRect(ctrlX, top, i)
	cs.draggingSlider = true
	cs.dragField = &field
	cs.dragSlider = &sliderRect{x: sx, w: sw}
	cs.setValueFromSliderRatio(float64(mx-sx)/float64(sw), field)
}

// curveSliderFields are the config fields a curve's controls column edits.
func (cs *ConfigScreen) curveSliderFields(curve physiology.CurveID) []configField {
	var out []configField
	if physiology.ShapeFor(cs.globals, curve).UsesK() {
		if f, ok := cs.curveKField(curve); ok {
			out = append(out, f)
		}
	}
	for _, tag := range curveSettingTags[curve] {
		if f, ok := cs.fieldByTag(tag); ok {
			out = append(out, f)
		}
	}
	return out
}

// curveSliders is what a curve's controls column draws: one entry per field in curveSliderFields.
func (cs *ConfigScreen) curveSliders(curve physiology.CurveID) []curveSlider {
	fields := cs.curveSliderFields(curve)
	out := make([]curveSlider, 0, len(fields))
	for _, f := range fields {
		fv := reflect.ValueOf(cs.globals).Elem().Field(f.fieldIdx)
		label := f.label
		if f.shape != "" {
			label = "K"
		}
		editing, selected := "", cs.selectedRow >= 0 && cs.selectedRow == cs.rowIndexOfTag(f.jsonTag)
		if selected {
			editing = cs.editingValue
		}
		out = append(out, curveSlider{
			label:    label,
			ratio:    cs.getSliderRatio(fv, f),
			value:    cs.getValueStr(fv, f, false),
			editing:  editing,
			selected: selected,
		})
	}
	return out
}

func (cs *ConfigScreen) fieldByTag(tag string) (configField, bool) {
	for _, section := range cs.sections {
		for _, f := range section.fields {
			if f.jsonTag == tag {
				return f, true
			}
		}
	}
	return configField{}, false
}

func (cs *ConfigScreen) rowIndexOfTag(tag string) int {
	idx := 0
	for _, section := range cs.sections {
		for _, f := range section.fields {
			if f.jsonTag == tag {
				return idx
			}
			idx++
		}
	}
	return -1
}

// curveKField is the config field behind the coefficient the curve's current shape uses.
func (cs *ConfigScreen) curveKField(curve physiology.CurveID) (configField, bool) {
	tag := curveKTag(cs.globals, curve)
	for _, section := range cs.sections {
		for _, f := range section.fields {
			if f.jsonTag == tag {
				return f, true
			}
		}
	}
	return configField{}, false
}

// curveKRatio is where the curve's coefficient sits in its slider range, for drawing the block's slider.
func (cs *ConfigScreen) curveKRatio(curve physiology.CurveID) float64 {
	field, ok := cs.curveKField(curve)
	if !ok {
		return 0
	}
	fv := reflect.ValueOf(cs.globals).Elem().Field(field.fieldIdx)
	return cs.getSliderRatio(fv, field)
}

// cycleCurveShape steps a curve's shape through physiology.AllShapeKinds.
func (cs *ConfigScreen) cycleCurveShape(curve physiology.CurveID, step int) {
	kinds := physiology.AllShapeKinds
	current := physiology.ShapeFor(cs.globals, curve)
	idx := 0
	for i, k := range kinds {
		if k == current {
			idx = i
		}
	}
	cs.setCurveShape(curve, kinds[((idx+step)%len(kinds)+len(kinds))%len(kinds)])
}

func (cs *ConfigScreen) setCurveShape(curve physiology.CurveID, kind physiology.ShapeKind) {
	setCurveShape(cs.globals, curve, kind)
	cs.selectedRow = -1
	cs.editingValue = ""
}

// curveShapeChanged reports whether a curve's shape differs from the shipped default.
func (cs *ConfigScreen) curveShapeChanged(curve physiology.CurveID) bool {
	return physiology.ShapeFor(cs.globals, curve) != physiology.ShapeFor(&cs.defaults, curve)
}

// drawAbilityTotalRow shows the running total of the initial ability scores, green when it adds up to the budget and red otherwise, or a note that scores are random per organism.
func (cs *ConfigScreen) drawAbilityTotalRow(screen *ebiten.Image, px, py int) {
	label := fmt.Sprintf("  Total: %d / %d", cs.initialScoresTotal(), physiology.PointTotal)
	col := themedOK()
	switch {
	case cs.globals.RandomInitialAbilities:
		label = "  Total: random split of 100 per organism"
		col = themedMuted()
	case cs.initialScoresTotal() != physiology.PointTotal:
		col = themedBad()
	}
	text.Draw(screen, label, r.FontSourceCodePro10, px, py+10, col)
}

// tooltipDelay is how long the mouse must rest on a row before its tooltip appears.
const tooltipDelay = 350 * time.Millisecond

// tooltipMaxWidth is the widest a tooltip box's text runs before wrapping.
const tooltipMaxWidth = 300

// rowAt returns the field row under screen point (mx, my), or false if the point isn't on a visible field row.
func (cs *ConfigScreen) rowAt(mx, my int) (configField, bool) {
	panelX, panelW := cs.panelX(), cs.panelWidth()
	clipTop, clipBottom := cs.clipRange()
	if mx < panelX || mx >= panelX+panelW || my < clipTop || my >= clipBottom {
		return configField{}, false
	}
	y := cs.panelTop() + cfgHeaderHeight - int(cs.scrollY)
	for _, section := range cs.sections {
		y += cfgRowHeight + 4
		if !section.collapsed {
			for _, field := range section.fields {
				rowH := cs.rowHeight(field)
				if rowH > 0 && my >= y-2 && my < y+rowH-2 {
					return field, true
				}
				y += rowH
			}
		}
		y += 6
	}
	return configField{}, false
}

// DrawTooltip draws the explanation for the row under the mouse once it has rested there for tooltipDelay.
func (cs *ConfigScreen) DrawTooltip(screen *ebiten.Image) {
	mx, my := ebiten.CursorPosition()
	field, ok := cs.rowAt(mx, my)
	tip := ""
	if ok {
		tip = tooltipFor(field)
	}
	key := field.jsonTag + "|" + field.label
	if tip == "" {
		cs.hoverKey = ""
		return
	}
	if key != cs.hoverKey {
		cs.hoverKey = key
		cs.hoverSince = time.Now()
		return
	}
	if time.Since(cs.hoverSince) < tooltipDelay {
		return
	}
	drawTooltipBox(screen, tip, mx, my)
}

// drawTooltipBox draws word-wrapped text in a bordered box near the cursor, flipping to the other side of the cursor when it would run off the screen.
func drawTooltipBox(screen *ebiten.Image, tip string, mx, my int) {
	face := r.FontSourceCodePro10
	lines := wrapText(tip, tooltipMaxWidth, func(s string) int { return textAdvance(face, s) })
	lineH := face.Metrics().Height.Round()
	ascent := face.Metrics().Ascent.Round()
	const pad = 6
	textW := 0
	for _, l := range lines {
		textW = max(textW, textAdvance(face, l))
	}
	w, h := textW+2*pad, len(lines)*lineH+2*pad

	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	x, y := mx+14, my+18
	if x+w > sw {
		x = mx - 14 - w
	}
	if y+h > sh {
		y = my - 10 - h
	}
	x, y = max(0, x), max(0, y)

	bg := chrome(color.RGBA{R: 25, G: 25, B: 35, A: 245}, color.RGBA{R: 250, G: 250, B: 252, A: 245})
	border := chrome(color.RGBA{R: 120, G: 120, B: 170, A: 255}, color.RGBA{R: 140, G: 140, B: 160, A: 255})
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), bg)
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, border)
	ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, border)
	ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), border)
	ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), border)
	for i, l := range lines {
		text.Draw(screen, l, face, x+pad, y+pad+ascent+i*lineH, themedForeground())
	}
}

// wrapText splits text into lines no wider than maxWidth, as measured by width, breaking between words.
func wrapText(s string, maxWidth int, width func(string) int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if line != "" && width(candidate) > maxWidth {
			lines = append(lines, line)
			line = word
			continue
		}
		line = candidate
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

var loadConfigDefaults = c.GetDefaultGlobals

const resetBtnW = 34

// resetRect is the reset button's rect for the row at y: right-aligned in the panel, past the value column.
func (cs *ConfigScreen) resetRect(panelX, y int) (x, top, w, h int) {
	return panelX + cs.panelWidth() - resetBtnW, y, resetBtnW, cfgRowHeight - 4
}

func (cs *ConfigScreen) restoreAllRect() (x, y, w, h int) {
	const bw, bh = 150, 18
	return cs.panelX() + cs.panelWidth() - bw, cs.panelTop() + (cfgHeaderHeight-bh)/2 - int(cs.scrollY), bw, bh
}

// canReset reports whether a row has a value that differs from its default.
func (cs *ConfigScreen) canReset(field configField) bool {
	if field.textOnly && field.jsonTag == "" {
		return false // a section's explanatory line has nothing to reset
	}
	switch field.row {
	case rowAbilityTotal, rowCurveGraph, rowWallPhGraph:
		// Rows that draw rather than hold a value.
		return false
	case rowNodeType:
		// Built from the decision pools, not a Globals field.
		return nodeDisabled(cs.globals.DisabledDecisionNodes, field.jsonTag) !=
			nodeDisabled(cs.defaults.DisabledDecisionNodes, field.jsonTag)
	case rowConditionFamily:
		return nodeDisabled(cs.globals.BasicOnlyConditionFamilies, field.familyRoot) !=
			nodeDisabled(cs.defaults.BasicOnlyConditionFamilies, field.familyRoot)
	case rowDesign:
		// Rows built from the designs directory carry no Globals field.
		return cs.designChosen(field.ability) != designInList(cs.defaults.InitialDesigns, cs.designNames(), field.ability)
	case rowAbilityScore:
		return cs.globals.InitialAbilityScores[field.ability] != cs.defaults.InitialAbilityScores[field.ability]
	}
	current := reflect.ValueOf(cs.globals).Elem().Field(field.fieldIdx)
	def := reflect.ValueOf(cs.defaults).Field(field.fieldIdx)
	return !reflect.DeepEqual(current.Interface(), def.Interface())
}

// resetField sets one row back to its default value.
func (cs *ConfigScreen) resetField(field configField) {
	cs.selectedRow = -1
	cs.editingValue = ""
	switch field.row {
	case rowAbilityTotal, rowCurveGraph, rowWallPhGraph:
		return
	case rowNodeType:
		if nodeDisabled(cs.defaults.DisabledDecisionNodes, field.jsonTag) !=
			nodeDisabled(cs.globals.DisabledDecisionNodes, field.jsonTag) {
			cs.toggleNodeType(field.jsonTag)
		}
		return
	case rowConditionFamily:
		if cs.canReset(field) {
			cs.toggleConditionFamily(field.familyRoot)
		}
		return
	case rowDesign:
		if cs.canReset(field) {
			cs.toggleDesign(field.ability)
		}
		return
	case rowAbilityScore:
		cs.globals.InitialAbilityScores[field.ability] = cs.defaults.InitialAbilityScores[field.ability]
		return
	}
	def := reflect.ValueOf(cs.defaults).Field(field.fieldIdx)
	reflect.ValueOf(cs.globals).Elem().Field(field.fieldIdx).Set(def)
}

func (cs *ConfigScreen) sectionChanged(section configSection) bool {
	for _, field := range section.fields {
		if cs.canReset(field) {
			return true
		}
	}
	return false
}

// allDefaults reports whether every setting already matches its default.
func (cs *ConfigScreen) allDefaults() bool {
	for _, section := range cs.sections {
		if cs.sectionChanged(section) {
			return false
		}
	}
	return true
}

// restoreAllDefaults sets every setting on the screen back to the shipped defaults.
func (cs *ConfigScreen) restoreAllDefaults() {
	theme, phScheme := cs.globals.Theme, cs.globals.PhColorScheme
	*cs.globals = cs.defaults
	cs.globals.Theme, cs.globals.PhColorScheme = theme, phScheme
	cs.globals.InitialAbilityScores = normalizedInitialScores(cs.defaults.InitialAbilityScores)
	cs.selectedRow = -1
	cs.editingValue = ""
}

func (cs *ConfigScreen) drawSmallButton(screen *ebiten.Image, x, y, w, h int, label string, active bool) {
	bg := themedControlFill()
	fg := themedValue()
	if !active {
		bg = themedControlDim()
		fg = themedMuted()
	}
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), bg)
	lb := boundString(r.FontSourceCodePro8, label)
	text.Draw(screen, label, r.FontSourceCodePro8, x+(w-lb.Dx())/2, y+(h+lb.Dy())/2, fg)
}

func (cs *ConfigScreen) rowHeight(field configField) int {
	if field.hidden {
		return 0
	}
	if field.row == rowCurveGraph {
		if !cs.graphExpanded[field.curveAbility] {
			return 0
		}
		return abilityBlockHeight(field.curveAbility)
	}
	if field.row == rowWallPhGraph {
		return wallPhGraphHeight
	}
	return cfgRowHeight
}
