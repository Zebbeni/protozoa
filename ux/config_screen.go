package ux

import (
	"fmt"
	"image/color"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

const (
	cfgPanelWidth    = 700
	cfgLabelWidth    = 340
	cfgSliderWidth   = 200
	cfgValueWidth    = 120
	cfgRowHeight     = 18
	cfgPadding       = 10
	cfgHeaderHeight  = 30
	cfgButtonHeight  = 30
	cfgButtonWidth   = 200
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
}

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
	embedded       bool
	viewportLeft   int
	viewportTop    int
	viewportRight  int
	viewportBottom int
}

func NewConfigScreen(globals *c.Globals) *ConfigScreen {
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
					cs.drawRow(screen, panelX, y, rowIdx, field)
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
				// Check if clicked on slider area (skipped for text-only fields)
				sliderX := panelX + cfgLabelWidth
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
			cs.accepted = true
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
	// For health change values that can be negative
	if strings.Contains(field.jsonTag, "health_change") {
		absMax := math.Max(math.Abs(initial)*3, 1)
		return -absMax, absMax
	}
	// General positive values
	if initial <= 0 {
		return 0, 1
	}
	return 0, initial * 3
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
			field("Chance to Add Food", "chance_to_add_food_item"),
			field("Min Food Value", "min_food_value"),
			field("Max Food Value", "max_food_value"),
		}},
		{title: "— PH —", fields: []configField{
			field("Min pH", "min_ph"),
			field("Max pH", "max_ph"),
			field("Min Initial pH", "min_initial_ph"),
			field("Max Initial pH", "max_initial_ph"),
			field("Min Ideal pH", "min_ideal_ph"),
			field("Max Ideal pH", "max_ideal_ph"),
			field("pH Tolerance", "ph_tolerance"),
			field("Chemosynthesis Tolerance", "chemosynthesis_tolerance"),
			field("Chemo pH Effect / Size", "chemosynthesis_ph_effect_per_size"),
			field("Eating pH Effect / Food", "eating_ph_effect_per_food"),
			field("pH Diffuse Factor", "ph_diffuse_factor"),
			field("pH Increment to Display", "ph_increment_to_display"),
		}},
		{title: "— ORGANISMS —", fields: []configField{
			field("Min Organisms", "min_organisms"),
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
			field("Stinging", "health_change_from_stinging"),
			field("Digging", "health_change_from_digging"),
			field("Burrowing", "health_change_from_burrowing"),
			field("Hunkering", "health_change_from_hunkering"),
			field("Flaring", "health_change_from_flaring"),
			field("Hiding", "health_change_from_hiding"),
			// Damage delivered to targets (signed, always negative).
			field("Inflicted by Attack", "health_change_inflicted_by_attack"),
			field("Inflicted by Sting", "health_change_inflicted_by_sting"),
			// Environmental health changes.
			field("Per Unhealthy pH Cycle", "health_change_per_unhealthy_ph"),
		}},
		{title: "— PHYSIOLOGY —", fields: []configField{
			// Feature-evolution rates.
			field("Chance to Gain Feature", "chance_to_gain_feature"),
			field("Chance to Lose Feature", "chance_to_lose_feature"),
			// Posture-state modifiers — multipliers (unitless,
			// 1.0 = no effect) and additive deltas applied during
			// the cycle an organism holds the matching posture.
			field("Hunker Damage Taken Mult", "hunker_damage_taken_mult"),
			field("Flare Damage Dealt Mult", "flare_damage_dealt_mult"),
			field("Flare Perceived Size +", "flare_perceived_size_add"),
			// Per-size-class wall strength delta for dig / burrow.
			field("Wall Strength Delta (S)", "wall_strength_delta_small"),
			field("Wall Strength Delta (M)", "wall_strength_delta_medium"),
			field("Wall Strength Delta (L)", "wall_strength_delta_large"),
		}},
		{title: "— CHEMO EFFICIENCY —", fields: []configField{
			// Grouped together because every feature has one and
			// they're easier to balance side-by-side than scattered
			// across per-tree sections.
			field("Flagellae", "flagellae_chemo_efficiency_mult"),
			field("Cilia", "cilia_chemo_efficiency_mult"),
			field("Stinger", "stinger_chemo_efficiency_mult"),
			field("Antennae", "antennae_chemo_efficiency_mult"),
			field("Feelers", "feelers_chemo_efficiency_mult"),
			field("Tasters", "tasters_chemo_efficiency_mult"),
			field("Shell", "shell_chemo_efficiency_mult"),
			field("Spikes", "spikes_chemo_efficiency_mult"),
			field("Camouflage", "camouflage_chemo_efficiency_mult"),
			field("Teeth", "teeth_chemo_efficiency_mult"),
			field("Fangs", "fangs_chemo_efficiency_mult"),
			field("Tusks", "tusks_chemo_efficiency_mult"),
		}},
		{title: "— FLAGELLAE TREE —", fields: []configField{
			field("Cilia Move Cost", "cilia_move_cost_mult"),
		}},
		{title: "— DEFENSE TREE —", fields: []configField{
			field("Shell Move Cost", "shell_move_cost_mult"),
			field("Shell Damage Taken", "shell_damage_taken_mult"),
			field("Spikes Move Cost", "spikes_move_cost_mult"),
			field("Spikes Damage Taken", "spikes_damage_taken_mult"),
			field("Spikes Damage Dealt", "spikes_damage_dealt_mult"),
			field("Spikes Perceived Size +", "spikes_perceived_size_add"),
			field("Camouflage Move Cost", "camouflage_move_cost_mult"),
			field("Camouflage Damage Taken", "camouflage_damage_taken_mult"),
		}},
		{title: "— TEETH TREE —", fields: []configField{
			field("Fangs Damage Dealt", "fangs_damage_dealt_mult"),
			field("Tusks Move Cost", "tusks_move_cost_mult"),
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
