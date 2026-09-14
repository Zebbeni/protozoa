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

// configField describes one editable config value
type configField struct {
	label    string
	jsonTag  string
	kind     reflect.Kind // Int, Float64, Bool
	fieldIdx int          // index into Globals struct
	// textOnly disables the slider/toggle for this row and shows just
	// the value as text — clicking selects the row for direct
	// keyboard entry. Used for fields whose value range isn't a
	// natural slider (e.g. RNG seed).
	textOnly bool
	// row selects special rows that aren't a plain Globals field: the
	// initial ability scores (one row per ability, with -/+ buttons) and
	// their running total. ability is the score's index for
	// rowAbilityScore.
	row     rowType
	ability int
}

// rowType distinguishes the config screen's special rows from ordinary
// field rows.
type rowType int

const (
	rowField rowType = iota
	rowAbilityScore
	rowAbilityTotal
)

// configSection groups fields under a heading. Sections are
// collapsible: clicking the header toggles `collapsed`, and when
// collapsed only the header row renders (fields are skipped and
// don't contribute to contentHeight). All sections start collapsed
// so the editor opens to a navigable overview rather than a wall
// of values.
type configSection struct {
	title     string
	fields    []configField
	collapsed bool
}

// ConfigScreen is the pre-simulation config editor UI. Used either as
// a full-screen panel (legacy path / fallback) or, when SetEmbedded
// is called, as the body of the New Simulation popup — the popup
// supplies its own chrome, Cancel/Start buttons, and viewport bounds,
// and the screen renders only the scrollable form within those bounds.
type ConfigScreen struct {
	globals    *c.Globals
	initValues c.Globals // snapshot of initial values for stable slider ranges
	sections   []configSection

	scrollY      float64
	selectedRow  int // -1 for none
	editingValue string
	accepted     bool

	// Slider drag state
	draggingSlider bool
	dragField      *configField

	// Embedded mode: when true the screen skips painting its own
	// background and START button (the popup owns those). The
	// viewport rect bounds the scroll area in screen coords — rows
	// outside this range are clipped, the form panel centres
	// horizontally inside [Left, Right], and the slider column
	// shrinks if the popup is narrower than the standalone-mode
	// default. accepted never goes true in embedded mode; the popup
	// decides when Start fires.
	embedded bool
	// hoverKey / hoverSince track which row the mouse is resting on and
	// since when, so its tooltip only appears after a short pause.
	hoverKey       string
	hoverSince     time.Time
	viewportLeft   int
	viewportTop    int
	viewportRight  int
	viewportBottom int
}

func NewConfigScreen(globals *c.Globals) *ConfigScreen {
	// The ability scores are a slice, so copying Globals shares their
	// backing array with whatever the caller passed in — editing them
	// here would silently change the active config before Start.
	// Give the form its own copy, padded to one entry per ability.
	globals.InitialAbilityScores = normalizedInitialScores(globals.InitialAbilityScores)
	cs := &ConfigScreen{
		globals:     globals,
		initValues:  *globals,
		selectedRow: -1,
	}
	cs.buildSections()
	return cs
}

func (cs *ConfigScreen) Globals() *c.Globals {
	return cs.globals
}

// SetEmbedded switches the screen into popup-body mode and binds the
// scroll viewport to the given screen-coordinate range. Pass the
// inner bounds of the popup body — the form centres within
// [left, right] and scrolls within [top, bottom]. Rows outside the
// vertical range are clipped; the slider column shrinks
// automatically when right-left is narrower than the standalone
// panel width.
func (cs *ConfigScreen) SetEmbedded(left, top, right, bottom int) {
	cs.embedded = true
	cs.viewportLeft = left
	cs.viewportTop = top
	cs.viewportRight = right
	cs.viewportBottom = bottom
}

// panelWidth returns the form's content-column width. Standalone
// mode uses the fixed default; embedded mode shrinks to fit the
// popup body when narrower (capped at the default).
func (cs *ConfigScreen) panelWidth() int {
	if !cs.embedded {
		return cfgPanelWidth
	}
	avail := cs.viewportRight - cs.viewportLeft
	if avail > cfgPanelWidth {
		return cfgPanelWidth
	}
	if avail < cfgPanelWidth/2 {
		// Pathologically narrow popup — clamp so layout math stays
		// sane. The form will overflow the popup body in this case
		// but at least won't go negative.
		return cfgPanelWidth / 2
	}
	return avail
}

// panelX returns the form's left edge in screen coordinates.
// Embedded mode centres within the popup viewport; standalone mode
// centres on screen.
func (cs *ConfigScreen) panelX() int {
	pw := cs.panelWidth()
	if cs.embedded {
		return cs.viewportLeft + (cs.viewportRight-cs.viewportLeft-pw)/2
	}
	return (c.ScreenWidth() - pw) / 2
}

// sliderColWidth returns the slider column's pixel width given the
// current panel width. The label and value columns keep their fixed
// widths for legibility; the slider absorbs whatever space remains
// (with min/max clamps).
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

	// Scroll. Mouse wheel + keyboard. Keyboard fallback exists because
	// browsers (wasm build) sometimes don't deliver wheel deltas to the
	// canvas if focus is elsewhere — Down / Up step a row at a time,
	// PageDown / PageUp / Space jump in larger increments. End jumps to
	// the bottom (where the START SIMULATION button lives).
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

	// Clamp to [0, max]. Outside embedded mode max accounts for the
	// START SIMULATION button at the bottom; inside embedded mode the
	// popup owns the button area and the form ends at the viewport's
	// bottom edge.
	maxScroll := cs.maxScroll()
	if cs.scrollY > float64(maxScroll) {
		cs.scrollY = float64(maxScroll)
	}
	if cs.scrollY < 0 {
		cs.scrollY = 0
	}

	// Mouse click / drag
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		cs.handleClick()
	} else if cs.draggingSlider && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
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
	}

	// Keyboard input for editing
	if cs.selectedRow >= 0 {
		cs.handleKeyboard()
	}

	return false
}

// contentHeight returns the total pixel height of all sections, fields,
// inter-section gaps, and the START SIMULATION button — i.e. the full
// scrollable extent. Computed from the static section/field counts so
// it matches what Draw lays out. Collapsed sections contribute only
// their header row.
func (cs *ConfigScreen) contentHeight() int {
	h := cfgHeaderHeight
	for _, section := range cs.sections {
		h += cfgRowHeight + 4 // header
		if !section.collapsed {
			h += len(section.fields) * cfgRowHeight
		}
		h += 6 // gap between sections
	}
	if !cs.embedded {
		h += cfgPadding + cfgButtonHeight
	}
	return h
}

// maxScroll returns the largest valid scrollY value given the current
// viewport. Standalone mode targets full screen height; embedded mode
// targets the popup body height and stops one row above the bottom so
// the popup's buttons aren't covered by trailing content.
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

// Draw renders the config screen
func (cs *ConfigScreen) Draw(screen *ebiten.Image) {
	if !cs.embedded {
		fillThemeBackground(screen)
	}

	panelX := cs.panelX()
	panelW := cs.panelWidth()
	panelTop := cs.panelTop()
	clipTop, clipBottom := cs.clipRange()

	// Title — only in standalone mode; the popup paints its own title
	// bar so it stays anchored while the form scrolls.
	if !cs.embedded {
		title := "SIMULATION SETTINGS"
		titleBounds := boundString(r.FontSourceCodePro12, title)
		titleX := panelX + (panelW-titleBounds.Dx())/2
		text.Draw(screen, title, r.FontSourceCodePro12, titleX, panelTop+titleBounds.Dy(), themedForeground())
	}

	y := panelTop + cfgHeaderHeight - int(cs.scrollY)
	rowIdx := 0

	for _, section := range cs.sections {
		// Section header: prefix with ▼ when expanded, ▶ when
		// collapsed so the user can see at a glance which sections
		// have visible fields below them.
		if y+cfgRowHeight > clipTop && y < clipBottom {
			marker := "▶" // ▶
			if !section.collapsed {
				marker = "▼" // ▼
			}
			text.Draw(screen, marker+" "+section.title, r.FontSourceCodePro12, panelX, y+12, color.RGBA{R: 180, G: 180, B: 255, A: 255})
		}
		y += cfgRowHeight + 4

		if section.collapsed {
			rowIdx += len(section.fields)
		} else {
			for _, field := range section.fields {
				if y+cfgRowHeight > clipTop && y < clipBottom {
					switch field.row {
					case rowAbilityScore:
						cs.drawAbilityScoreRow(screen, panelX, y, rowIdx, field)
					case rowAbilityTotal:
						cs.drawAbilityTotalRow(screen, panelX, y)
					default:
						cs.drawRow(screen, panelX, y, rowIdx, field)
					}
				}
				y += cfgRowHeight
				rowIdx++
			}
		}
		y += 6 // gap between sections
	}

	// Standalone START button — popup mode hides this; the popup
	// renders its own Cancel/Start row in fixed footer position.
	if !cs.embedded {
		defer cs.DrawTooltip(screen)
		btnX := panelX + (panelW-cfgButtonWidth)/2
		btnY := y + cfgPadding
		if btnY > -cfgButtonHeight && btnY < c.ScreenHeight() {
			cs.drawButton(screen, btnX, btnY, cfgButtonWidth, cfgButtonHeight, "START SIMULATION")
		}
	}
}

// panelTop returns the y-coordinate of the form's top edge. Standalone
// mode hardcodes 20px (room for the in-form title); embedded mode uses
// the viewport top supplied by the popup.
func (cs *ConfigScreen) panelTop() int {
	if cs.embedded {
		return cs.viewportTop
	}
	return 20
}

// clipRange is the screen-coord band rows must overlap to be drawn.
// In embedded mode it's the popup's body bounds; otherwise the full
// screen.
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
	labelColor := color.RGBA{R: 180, G: 180, B: 180, A: 255}
	valueColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	if isSelected {
		// highlight row
		ebitenutil.DrawRect(screen, float64(px), float64(py-2), float64(cs.panelWidth()), float64(cfgRowHeight), color.RGBA{R: 40, G: 40, B: 60, A: 255})
		valueColor = color.RGBA{R: 100, G: 255, B: 100, A: 255}
	}

	// Label
	text.Draw(screen, field.label, r.FontSourceCodePro10, px, py+10, labelColor)

	// Value
	sliderW := cs.sliderColWidth()
	valueStr := cs.getValueStr(fv, field, isSelected)
	text.Draw(screen, valueStr, r.FontSourceCodePro10, px+cfgLabelWidth+sliderW+10, py+10, valueColor)

	if field.textOnly {
		return
	}

	// Slider (for numeric types)
	if field.kind == reflect.Float64 || field.kind == reflect.Int {
		cs.drawSlider(screen, px+cfgLabelWidth, py, fv, field)
	}

	// Toggle for bool
	if field.kind == reflect.Bool {
		cs.drawToggle(screen, px+cfgLabelWidth, py, fv.Bool())
	}
}

func (cs *ConfigScreen) getValueStr(fv reflect.Value, field configField, isEditing bool) string {
	if isEditing && cs.editingValue != "" {
		return cs.editingValue + "_"
	}
	switch field.kind {
	case reflect.Int:
		return strconv.Itoa(int(fv.Int()))
	case reflect.Float64:
		val := fv.Float()
		if math.Abs(val) < 0.001 && val != 0 {
			return fmt.Sprintf("%.6f", val)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case reflect.Bool:
		if fv.Bool() {
			return "true"
		}
		return "false"
	}
	return "?"
}

func (cs *ConfigScreen) drawSlider(screen *ebiten.Image, x, y int, fv reflect.Value, field configField) {
	sx, sy := float64(x), float64(y+4)
	sw := float64(cs.sliderColWidth())
	sh := float64(cfgRowHeight - 8)

	// Track
	ebitenutil.DrawRect(screen, sx, sy, sw, sh, color.RGBA{R: 50, G: 50, B: 60, A: 255})

	// Fill based on value ratio (estimate range from value magnitude)
	ratio := cs.getSliderRatio(fv, field)
	fillW := sw * ratio
	ebitenutil.DrawRect(screen, sx, sy, fillW, sh, color.RGBA{R: 80, G: 80, B: 120, A: 255})

	// Handle
	handleX := sx + fillW - 2
	if handleX < sx {
		handleX = sx
	}
	ebitenutil.DrawRect(screen, handleX, sy, 4, sh, color.RGBA{R: 150, G: 150, B: 200, A: 255})
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
	// Button label stays white regardless of theme since the button has
	// its own coloured background.
	text.Draw(screen, label, r.FontSourceCodePro12, tx, ty, color.White)
}

func (cs *ConfigScreen) handleClick() {
	mx, my := ebiten.CursorPosition()
	panelX := cs.panelX()
	panelW := cs.panelWidth()
	sliderW := cs.sliderColWidth()
	panelTop := cs.panelTop()
	clipTop, clipBottom := cs.clipRange()

	// In embedded mode, ignore clicks outside the body region — the
	// popup chrome lives there and we don't want a click on the title
	// bar to fall through and land on a row.
	if cs.embedded && (my < clipTop || my >= clipBottom) {
		return
	}

	// Commit any current edit
	cs.commitEdit()

	y := panelTop + cfgHeaderHeight - int(cs.scrollY)
	rowIdx := 0

	for i := range cs.sections {
		section := &cs.sections[i]
		// Header hitbox: clicking anywhere across the full panel
		// width on the header row toggles the section's collapsed
		// state. Use a generous vertical band so the click feels
		// forgiving.
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
			if my >= y-2 && my < y+cfgRowHeight-2 {
				sliderX := panelX + cfgLabelWidth
				switch field.row {
				case rowAbilityTotal:
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
				// Select row for text editing
				cs.selectedRow = rowIdx
				cs.editingValue = ""
				return
			}
			y += cfgRowHeight
			rowIdx++
		}
		y += 6
	}

	// Standalone mode owns the START button; embedded mode delegates
	// that role to the popup so we skip the hit-test entirely.
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
	// Text input
	chars := ebiten.AppendInputChars(nil)
	for _, ch := range chars {
		s := string(ch)
		if strings.ContainsAny(s, "0123456789.-+eE") {
			cs.editingValue += s
		}
	}

	// Backspace
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if len(cs.editingValue) > 0 {
			cs.editingValue = cs.editingValue[:len(cs.editingValue)-1]
		}
	}

	// Enter to commit
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		cs.commitEdit()
		cs.selectedRow = -1
	}

	// Escape to cancel
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

	switch field.kind {
	case reflect.Int:
		if val, err := strconv.Atoi(cs.editingValue); err == nil {
			fv.SetInt(int64(val))
		}
	case reflect.Float64:
		if val, err := strconv.ParseFloat(cs.editingValue, 64); err == nil {
			fv.SetFloat(val)
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

func (cs *ConfigScreen) handleSliderDrag() {
	if cs.dragField == nil {
		return
	}
	mx, _ := ebiten.CursorPosition()
	sliderX := cs.panelX() + cfgLabelWidth
	relX := mx - sliderX
	ratio := float64(relX) / float64(cs.sliderColWidth())
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
		fv.SetFloat(val)
	}
}

// getSliderRange returns sensible min/max for the slider based on the
// initial default value (stable — doesn't shift as the value is edited).
// Except for fields with a fixed natural range, the default sits at the
// centre of its slider so it can be nudged either way by the same amount.
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

	// Chemo efficiency multipliers represent "fraction of base chemo
	// rate" — semantically bounded to [0, 1]. A 1.0 value means no
	// penalty; 0 means the feature can't chemosynthesise at all.
	// Bounding here prevents the slider from suggesting values >1
	// that would be a buff rather than a tradeoff.
	if strings.HasSuffix(field.jsonTag, "_chemo_efficiency_mult") {
		return 0, 1
	}
	// Initial and ideal pH values are points on the fixed pH scale, so
	// their sliders span exactly that scale rather than a multiple of the
	// default.
	switch field.jsonTag {
	case "min_initial_ph", "max_initial_ph", "min_ideal_ph", "max_ideal_ph":
		return c.MinPh(), c.MaxPh()
	}
	return centeredSliderRange(field.jsonTag, initial)
}

// nonPositiveSliders lists fields whose slider is capped at zero: values
// that are always damage, where a positive setting would turn a penalty
// into healing.
var nonPositiveSliders = map[string]bool{
	"health_change_per_unhealthy_ph": true,
}

// centeredSliderRange puts initial at the centre of its slider.
//
// Health changes are signed (negative costs, positive gains), so their
// slider spans twice the default's magnitude either side of it and can
// cross zero — a cost can be turned into a gain. Everything else is a
// non-negative quantity, so its slider runs from 0 to twice the default.
// A zero default has no scale to centre on and gets a unit range instead.
func centeredSliderRange(jsonTag string, initial float64) (float64, float64) {
	// Some health changes only make sense as damage. Their sliders stop at
	// zero instead of crossing into a gain, and still centre on a
	// negative default.
	if nonPositiveSliders[jsonTag] {
		if initial >= 0 {
			return -1, 0
		}
		return 2 * initial, 0
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
		idx := byTag[jsonTag]
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
			field("Min Initial pH", "min_initial_ph"),
			field("Max Initial pH", "max_initial_ph"),
			field("Min Ideal pH", "min_ideal_ph"),
			field("Max Ideal pH", "max_ideal_ph"),
			field("pH Tolerance", "ph_tolerance"),
			field("Chemosynthesis Tolerance", "chemosynthesis_tolerance"),
			field("Chemo pH Falloff", "chemo_ph_falloff"),
			field("Chemo Curve Exponent", "chemo_curve_exponent"),
			field("Chemo pH Effect / Size", "chemosynthesis_ph_effect_per_size"),
			field("Eating pH Effect / Food", "eating_ph_effect_per_food"),
			field("pH Diffuse Factor", "ph_diffuse_factor"),
			field("pH Increment to Display", "ph_increment_to_display"),
			// Currents: how hard the flow field skews pH diffusion,
			// and how fast an un-tended current fades.
		}},
		{title: "— ORGANISMS —", fields: []configField{
			field("Min Organisms", "min_organisms"),
			field("Max Organisms", "max_organisms"),
			field("Growth Factor", "growth_factor"),
			field("Eating Growth Factor", "eating_growth_factor"),
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
		{title: "— DECISION TREES —", fields: []configField{
			field("Initial Mutations", "initial_organism_decision_tree_mutations"),
			field("Chance to Mutate", "chance_to_mutate_decision_tree"),
			field("Max Tree Size", "max_decision_tree_size"),
		}},
		{title: "— HEALTH CHANGES —", fields: []configField{
			// Per-action costs/gains paid by the actor. All values
			// are signed absolute deltas (typically size-scaled at
			// the apply site); convention is negative = cost,
			// positive = gain. One row per action.
			field("Chemosynthesis", "health_change_from_chemosynthesis"),
			field("Failed Chemosynthesis", "health_change_from_failed_chemosynthesis"),
			field("Idle", "health_change_from_idle"),
			field("Turning", "health_change_from_turning"),
			field("Moving", "health_change_from_moving"),
			field("Eating Attempt", "health_change_from_eating_attempt"),
			field("Spawning", "health_change_from_spawning"),
			field("Attacking", "health_change_from_attacking"),
			field("Digging", "health_change_from_digging"),
			// Damage delivered to targets (signed, always negative).
			field("Inflicted by Attack", "health_change_inflicted_by_attack"),
			field("Inflicted by Shoving", "health_change_inflicted_by_shoving"),
			field("Attack Health Gain", "attack_health_gain"),
			field("Chemo Crowding Penalty", "chemo_crowding_penalty"),
			field("Corpse Food Multiplier", "corpse_food_multiplier"),
			// Environmental health changes.
			field("Per Unhealthy pH Cycle", "health_change_per_unhealthy_ph"),
		}},
		{title: "— TERRAIN —", fields: []configField{
			// Per-size-class wall strength delta for ActDig, before
			// the digger's Digging ability scales it.
			field("Wall Strength Delta (S)", "wall_strength_delta_small"),
			field("Wall Strength Delta (M)", "wall_strength_delta_medium"),
			field("Wall Strength Delta (L)", "wall_strength_delta_large"),
			// Ability score at which a blocked move (Digging) or an
			// attack (Attack) removes one point of wall strength per
			// hit, per size class.
			field("Wall Break Score (S)", "wall_break_score_small"),
			field("Wall Break Score (M)", "wall_break_score_medium"),
			field("Wall Break Score (L)", "wall_break_score_large"),
		}},
		{title: "— INITIAL ABILITIES —", fields: initialAbilityFields(field("Random Initial Abilities", "random_initial_abilities"))},
		{title: "— ABILITY SCORES —", fields: []configField{
			// Where each ability's multiplier curve crosses 1.0; starting
			// scores are set under INITIAL ABILITIES.
			field("Curve Pivot Minor Score", "genesis_minor_ability_score"),
			field("Chance to Mutate", "chance_to_mutate_abilities"),
			field("Max Point Shift", "max_ability_shift"),
			field("Specialization Span", "ability_specialization_span"),
			field("Thorns Threshold", "thorns_threshold"),
			field("Thorns Damage Per Point", "thorns_damage_per_point"),
			field("Defense pH Protection", "defense_ph_protection"),
			// Each pair is (multiplier at score 0, multiplier at 100);
			// the curve passes through 1.0 at the genesis allocation.
			// Movement and Defense are COST multipliers, so their
			// endpoints run the other way round.
			field("Chemo @0", "chemo_mult_at_zero"),
			field("Chemo @100", "chemo_mult_at_max"),
			field("Eating @0", "eating_mult_at_zero"),
			field("Eating @100", "eating_mult_at_max"),
			field("Movement cost @0", "movement_mult_at_zero"),
			field("Movement cost @100", "movement_mult_at_max"),
			field("Digging @0", "digging_mult_at_zero"),
			field("Digging @100", "digging_mult_at_max"),
			field("Attack @0", "attack_mult_at_zero"),
			field("Attack @100", "attack_mult_at_max"),
			field("Dmg taken @0", "defense_mult_at_zero"),
			field("Dmg taken @100", "defense_mult_at_max"),
		}},
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
	// Sections collapsed by default — the editor opens to a list of
	// section headers that the user can expand as they need.
	for i := range cs.sections {
		cs.sections[i].collapsed = true
	}
}

// abilityBtnW is the width of the - / + buttons on an initial ability row.
const abilityBtnW = 22

// initialAbilityFields builds the INITIAL ABILITIES section: the Random
// toggle, one score row per ability, and the running total.
func initialAbilityFields(random configField) []configField {
	fields := []configField{random}
	for _, a := range physiology.AllAbilities {
		fields = append(fields, configField{label: a.Name(), row: rowAbilityScore, ability: int(a)})
	}
	return append(fields, configField{label: "Total", row: rowAbilityTotal})
}

// normalizedInitialScores returns a fresh copy of scores with exactly one
// entry per ability, falling back to the genesis distribution when the
// setting is missing or the wrong length.
func normalizedInitialScores(scores []int) []int {
	out := make([]int, len(physiology.AllAbilities))
	if len(scores) != len(out) {
		genesis := physiology.GenesisScores()
		for _, a := range physiology.AllAbilities {
			out[a] = genesis[a]
		}
		return out
	}
	copy(out, scores)
	return out
}

// initialScoresTotal is the sum of the configured initial ability scores.
func (cs *ConfigScreen) initialScoresTotal() int {
	total := 0
	for _, v := range cs.globals.InitialAbilityScores {
		total += v
	}
	return total
}

// adjustAbilityScore changes one initial ability score by delta.
func (cs *ConfigScreen) adjustAbilityScore(ability, delta int) {
	cs.setAbilityScore(ability, cs.globals.InitialAbilityScores[ability]+delta)
}

// setAbilityScore sets one initial ability score, clamped to [0, 100].
// The total isn't forced to 100 here — the user rebalances by hand, and
// StartBlockedReason holds the start back until it adds up.
func (cs *ConfigScreen) setAbilityScore(ability, value int) {
	if ability < 0 || ability >= len(cs.globals.InitialAbilityScores) {
		return
	}
	cs.globals.InitialAbilityScores[ability] = min(physiology.PointTotal, max(0, value))
}

// StartBlockedReason explains why the simulation can't start with the
// current settings, or returns "" when it can. The initial ability scores
// must add up to the full budget unless Random is on.
func (cs *ConfigScreen) StartBlockedReason() string {
	if cs.globals.RandomInitialAbilities {
		return ""
	}
	if total := cs.initialScoresTotal(); total != physiology.PointTotal {
		return fmt.Sprintf("Initial abilities total %d; they must add up to %d", total, physiology.PointTotal)
	}
	return ""
}

// drawAbilityScoreRow draws one initial ability row: the ability name,
// then - value + in the slider column. Greyed out while Random is on.
func (cs *ConfigScreen) drawAbilityScoreRow(screen *ebiten.Image, px, py, rowIdx int, field configField) {
	disabled := cs.globals.RandomInitialAbilities
	labelColor := color.RGBA{R: 180, G: 180, B: 180, A: 255}
	valueColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	btnColor := color.RGBA{R: 70, G: 70, B: 95, A: 255}
	if disabled {
		labelColor = color.RGBA{R: 90, G: 90, B: 90, A: 255}
		valueColor = labelColor
		btnColor = color.RGBA{R: 45, G: 45, B: 50, A: 255}
	}
	isSelected := rowIdx == cs.selectedRow && !disabled
	if isSelected {
		ebitenutil.DrawRect(screen, float64(px), float64(py-2), float64(cs.panelWidth()), float64(cfgRowHeight), color.RGBA{R: 40, G: 40, B: 60, A: 255})
		valueColor = color.RGBA{R: 100, G: 255, B: 100, A: 255}
	}

	text.Draw(screen, "  "+field.label, r.FontSourceCodePro10, px, py+10, labelColor)

	sliderX := px + cfgLabelWidth
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

// drawAbilityTotalRow shows the running total of the initial ability
// scores, green when it adds up to the budget and red otherwise, or a
// note that scores are random per organism.
func (cs *ConfigScreen) drawAbilityTotalRow(screen *ebiten.Image, px, py int) {
	label := fmt.Sprintf("  Total: %d / %d", cs.initialScoresTotal(), physiology.PointTotal)
	col := color.RGBA{R: 100, G: 220, B: 100, A: 255}
	switch {
	case cs.globals.RandomInitialAbilities:
		label = "  Total: random split of 100 per organism"
		col = color.RGBA{R: 90, G: 90, B: 90, A: 255}
	case cs.initialScoresTotal() != physiology.PointTotal:
		col = color.RGBA{R: 235, G: 90, B: 90, A: 255}
	}
	text.Draw(screen, label, r.FontSourceCodePro10, px, py+10, col)
}

// tooltipDelay is how long the mouse must rest on a row before its
// tooltip appears, so tooltips don't flicker while scrolling past.
const tooltipDelay = 350 * time.Millisecond

// tooltipMaxWidth is the widest a tooltip box's text runs before wrapping.
const tooltipMaxWidth = 300

// rowAt returns the field row under screen point (mx, my), or false if
// the point isn't on a visible field row. Mirrors Draw's layout.
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
				if my >= y-2 && my < y+cfgRowHeight-2 {
					return field, true
				}
				y += cfgRowHeight
			}
		}
		y += 6
	}
	return configField{}, false
}

// DrawTooltip draws the explanation for the row under the mouse once it
// has rested there for tooltipDelay. The popup calls this after its
// footer so the tooltip sits on top of everything; standalone Draw calls
// it itself.
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

// drawTooltipBox draws word-wrapped text in a bordered box near the
// cursor, flipping to the other side of the cursor when it would run off
// the screen.
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

// wrapText splits text into lines no wider than maxWidth, as measured by
// width, breaking between words. A single word wider than maxWidth gets a
// line of its own.
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
