package ux

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"

	d "github.com/Zebbeni/protozoa/decision"
	r "github.com/Zebbeni/protozoa/resources"
)

// nodeTypeFields builds the section: the actions first, then the conditions, in the pools' declaration order.
func nodeTypeFields() []configField {
	fields := []configField{{
		label:    "  Nodes mutation may use — Actions",
		textOnly: true,
	}}
	for _, a := range d.MutableActions {
		fields = append(fields, configField{
			label:    d.Map[a],
			jsonTag:  d.NodeName(a),
			row:      rowNodeType,
			fieldIdx: -1,
		})
	}
	fields = append(fields, configField{
		label:    "  Condition families — basic read, and whether it can refine",
		textOnly: true,
	})
	for _, root := range d.BasicConditionsAll() {
		fields = append(fields, configField{
			label:    d.Map[root],
			jsonTag:  d.NodeName(root),
			row:      rowNodeType,
			fieldIdx: -1,
		})
		if adv := d.AdvancedConditions(root); len(adv) > 0 {
			fields = append(fields, configField{
				label:      fmt.Sprintf("    include advanced (%d)", len(adv)),
				familyRoot: d.NodeName(root),
				row:        rowConditionFamily,
				fieldIdx:   -1,
			})
		}
	}
	return fields
}

// nodeDisabled reports whether a node name appears in a hold-out list.
func nodeDisabled(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

func (cs *ConfigScreen) toggleNodeType(name string) {
	if name == "" {
		return
	}
	for i, n := range cs.globals.DisabledDecisionNodes {
		if n == name {
			cs.globals.DisabledDecisionNodes = append(
				cs.globals.DisabledDecisionNodes[:i],
				cs.globals.DisabledDecisionNodes[i+1:]...)
			return
		}
	}
	cs.globals.DisabledDecisionNodes = append(cs.globals.DisabledDecisionNodes, name)
}

// toggleConditionFamily switches a family's advanced reads on and off.
func (cs *ConfigScreen) toggleConditionFamily(rootName string) {
	if rootName == "" {
		return
	}
	for i, n := range cs.globals.BasicOnlyConditionFamilies {
		if n == rootName {
			cs.globals.BasicOnlyConditionFamilies = append(
				cs.globals.BasicOnlyConditionFamilies[:i],
				cs.globals.BasicOnlyConditionFamilies[i+1:]...)
			return
		}
	}
	cs.globals.BasicOnlyConditionFamilies = append(
		cs.globals.BasicOnlyConditionFamilies, rootName)
}

// drawConditionFamilyRow paints the indented "include advanced" tick.
func (cs *ConfigScreen) drawConditionFamilyRow(screen *ebiten.Image, px, py int, field configField) {
	rootOff := nodeDisabled(cs.globals.DisabledDecisionNodes, field.familyRoot)
	included := !nodeDisabled(cs.globals.BasicOnlyConditionFamilies, field.familyRoot)

	box := themedControlFill()
	mark := ""
	if included && !rootOff {
		box = color.RGBA{R: 70, G: 130, B: 90, A: 255}
		mark = "x"
	}
	ebitenutil.DrawRect(screen, float64(px+16), float64(py), 14, 14, box)
	if mark != "" {
		text.Draw(screen, mark, r.FontSourceCodePro10, px+19, py+11, color.White)
	}
	ink := themedForeground()
	if rootOff {
		ink = themedMuted()
	}
	text.Draw(screen, field.label, r.FontSourceCodePro10, px+38, py+11, ink)

	if rootOff {
		text.Draw(screen, "family is off", r.FontSourceCodePro8,
			px+cfgLabelWidth, py+11, themedMuted())
	} else if !included {
		text.Draw(screen, "basic read only", r.FontSourceCodePro8,
			px+cfgLabelWidth, py+11, themedMuted())
	}
}

// drawNodeTypeRow paints one action or condition with a tick box.
func (cs *ConfigScreen) drawNodeTypeRow(screen *ebiten.Image, px, py int, field configField) {
	enabled := !nodeDisabled(cs.globals.DisabledDecisionNodes, field.jsonTag)

	box := themedControlFill()
	mark := ""
	if enabled {
		box = color.RGBA{R: 70, G: 130, B: 90, A: 255}
		mark = "x"
	}
	ebitenutil.DrawRect(screen, float64(px+4), float64(py), 14, 14, box)
	if mark != "" {
		// White in both themes: the mark sits on the box's own saturated green rather than on the row.
		text.Draw(screen, mark, r.FontSourceCodePro10, px+7, py+11, color.White)
	}
	text.Draw(screen, field.label, r.FontSourceCodePro10, px+26, py+11, themedForeground())

	// The note says what an unticked row means.
	if !enabled {
		text.Draw(screen, "not offered to mutation", r.FontSourceCodePro8,
			px+cfgLabelWidth, py+11, themedMuted())
	}
}
