package ux

import (
	"reflect"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// settingsWithTheirOwnControl are the settings the config screen reaches through something other than a labelled row.
var settingsWithTheirOwnControl = map[string]string{
	"theme":                         "the panel's theme buttons, not a row in the settings list",
	"ph_color_scheme":               "the panel's palette buttons",
	"initial_designs":               "the INITIAL DESIGNS section, whose rows are built from the designs directory rather than from a Globals field",
	"disabled_decision_nodes":       "the DECISION TREE NODES section, whose rows are built from the decision package's mutable pools rather than from a Globals field",
	"basic_only_condition_families": "the \"include advanced\" tick under each condition family in the DECISION TREES section, whose rows are built from the decision package's family forest rather than from a Globals field",
	"initial_ability_scores":        "its own row kind — one slider per ability against a fixed budget, which a single value row can't express",
}

func TestEverySettingIsReachableOnScreen(t *testing.T) {
	loadKeyGlobals(t)

	// NewConfigScreen reads the shipped defaults out of the embedded bundle.
	g := *config.GetCurrentGlobals()
	defaults := g
	defaults.InitialAbilityScores = append([]int(nil), g.InitialAbilityScores...)
	old := loadConfigDefaults
	loadConfigDefaults = func() config.Globals { return defaults }
	defer func() { loadConfigDefaults = old }()

	form := g
	cs := NewConfigScreen(&form)

	reachable := map[string]bool{}
	for _, section := range cs.sections {
		for _, f := range section.fields {
			reachable[f.jsonTag] = true
		}
	}
	// A curve's shape isn't a value row.
	for _, tag := range curveShapeTags {
		reachable[tag] = true
	}

	tt := reflect.TypeOf(config.Globals{})
	for i := 0; i < tt.NumField(); i++ {
		tag := tt.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if reachable[tag] || settingsWithTheirOwnControl[tag] != "" {
			continue
		}
		t.Errorf("setting %q (Globals.%s) is on no config screen row: add a field() for it, "+
			"or list it in settingsWithTheirOwnControl with the control that does reach it",
			tag, tt.Field(i).Name)
	}
}

func TestExemptSettingsStillExist(t *testing.T) {
	tt := reflect.TypeOf(config.Globals{})
	real := map[string]bool{}
	for i := 0; i < tt.NumField(); i++ {
		real[tt.Field(i).Tag.Get("json")] = true
	}
	for tag := range settingsWithTheirOwnControl {
		if !real[tag] {
			t.Errorf("settingsWithTheirOwnControl exempts %q, which is not a setting any more", tag)
		}
	}
}
