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
	fieldIdx int         // index into Globals struct
}

// configSection groups fields under a heading
type configSection struct {
	title  string
	fields []configField
}

// ConfigScreen is the pre-simulation config editor UI
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

// Update handles input; returns true when the user accepts the config.
func (cs *ConfigScreen) Update() bool {
	if cs.accepted {
		return true
	}

	// Scroll
	_, wy := ebiten.Wheel()
	cs.scrollY -= wy * 30
	if cs.scrollY < 0 {
		cs.scrollY = 0
	}

	// Mouse click / drag
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		cs.handleClick()
	} else if cs.draggingSlider && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		cs.handleSliderDrag()
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

// Draw renders the config screen
func (cs *ConfigScreen) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 20, G: 20, B: 25, A: 255})

	sh := c.ScreenHeight()
	panelX := (c.ScreenWidth() - cfgPanelWidth) / 2
	panelTop := 20

	// Title
	title := "SIMULATION SETTINGS"
	titleBounds := boundString(r.FontSourceCodePro12, title)
	titleX := panelX + (cfgPanelWidth-titleBounds.Dx())/2
	text.Draw(screen, title, r.FontSourceCodePro12, titleX, panelTop+titleBounds.Dy(), color.White)

	y := panelTop + cfgHeaderHeight - int(cs.scrollY)
	rowIdx := 0

	for _, section := range cs.sections {
		// Section header
		if y > -cfgRowHeight && y < sh {
			text.Draw(screen, section.title, r.FontSourceCodePro12, panelX, y+12, color.RGBA{R: 180, G: 180, B: 255, A: 255})
		}
		y += cfgRowHeight + 4

		for _, field := range section.fields {
			if y > -cfgRowHeight && y < sh {
				cs.drawRow(screen, panelX, y, rowIdx, field)
			}
			y += cfgRowHeight
			rowIdx++
		}
		y += 6 // gap between sections
	}

	// Accept button
	btnX := panelX + (cfgPanelWidth-cfgButtonWidth)/2
	btnY := y + cfgPadding
	if btnY > -cfgButtonHeight && btnY < sh {
		cs.drawButton(screen, btnX, btnY, cfgButtonWidth, cfgButtonHeight, "START SIMULATION")
	}
}

func (cs *ConfigScreen) drawRow(screen *ebiten.Image, px, py, rowIdx int, field configField) {
	v := reflect.ValueOf(cs.globals).Elem()
	fv := v.Field(field.fieldIdx)

	isSelected := rowIdx == cs.selectedRow
	labelColor := color.RGBA{R: 180, G: 180, B: 180, A: 255}
	valueColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	if isSelected {
		// highlight row
		ebitenutil.DrawRect(screen, float64(px), float64(py-2), float64(cfgPanelWidth), float64(cfgRowHeight), color.RGBA{R: 40, G: 40, B: 60, A: 255})
		valueColor = color.RGBA{R: 100, G: 255, B: 100, A: 255}
	}

	// Label
	text.Draw(screen, field.label, r.FontSourceCodePro10, px, py+10, labelColor)

	// Value
	valueStr := cs.getValueStr(fv, field, isSelected)
	text.Draw(screen, valueStr, r.FontSourceCodePro10, px+cfgLabelWidth+cfgSliderWidth+10, py+10, valueColor)

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
	sw := float64(cfgSliderWidth)
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
	text.Draw(screen, label, r.FontSourceCodePro12, tx, ty, color.White)
}

func (cs *ConfigScreen) handleClick() {
	mx, my := ebiten.CursorPosition()
	panelX := (c.ScreenWidth() - cfgPanelWidth) / 2
	panelTop := 20

	// Commit any current edit
	cs.commitEdit()

	y := panelTop + cfgHeaderHeight - int(cs.scrollY)
	rowIdx := 0

	for _, section := range cs.sections {
		y += cfgRowHeight + 4 // section header
		for _, field := range section.fields {
			if my >= y-2 && my < y+cfgRowHeight-2 {
				// Check if clicked on slider area
				sliderX := panelX + cfgLabelWidth
				if field.kind == reflect.Bool && mx >= sliderX && mx < sliderX+40 {
					cs.toggleBool(field)
					return
				}
				if (field.kind == reflect.Float64 || field.kind == reflect.Int) &&
					mx >= sliderX && mx < sliderX+cfgSliderWidth {
					cs.handleSliderClick(mx-sliderX, field)
					return
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

	// Check accept button
	btnX := panelX + (cfgPanelWidth-cfgButtonWidth)/2
	btnY := y + cfgPadding
	if mx >= btnX && mx < btnX+cfgButtonWidth && my >= btnY && my < btnY+cfgButtonHeight {
		cs.accepted = true
		return
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
	ratio := float64(relX) / float64(cfgSliderWidth)
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
	panelX := (c.ScreenWidth() - cfgPanelWidth) / 2
	sliderX := panelX + cfgLabelWidth
	relX := mx - sliderX
	ratio := float64(relX) / float64(cfgSliderWidth)
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

	cs.sections = []configSection{
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
			field("Min pH Tolerance Range", "min_ph_tolerance_range"),
			field("Max pH Tolerance Range", "max_ph_tolerance_range"),
			field("Max pH Growth Effect", "max_organism_ph_growth_effect"),
			field("Max pH Effect Change", "max_ph_effect_change"),
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
		}},
		{title: "— DECISION TREES —", fields: []configField{
			field("Initial Mutations", "initial_organism_decision_tree_mutations"),
			field("Min Chance to Mutate", "min_chance_to_mutate_decision_tree"),
			field("Max Chance to Mutate", "max_chance_to_mutate_decision_tree"),
			field("Max Tree Size", "max_decision_tree_size"),
		}},
		{title: "— HEALTH CHANGES —", fields: []configField{
			field("Chemosynthesis", "health_change_from_chemosynthesis"),
			field("Turning", "health_change_from_turning"),
			field("Moving", "health_change_from_moving"),
			field("Eating Attempt", "health_change_from_eating_attempt"),
			field("Attacking", "health_change_from_attacking"),
			field("Spawning", "health_change_from_spawning"),
			field("Idle", "health_change_from_idle"),
			field("Inflicted by Attack", "health_change_inflicted_by_attack"),
			field("Per Unhealthy pH Cycle", "health_change_per_unhealthy_ph"),
		}},
		{title: "— POOLS —", fields: []configField{
			field("Use Pools", "use_pools"),
			field("Pool Width", "pool_width"),
			field("Pool Height", "pool_height"),
		}},
		{title: "— STATISTICS —", fields: []configField{
			field("Population Update Interval", "population_update_interval"),
		}},
	}
}
