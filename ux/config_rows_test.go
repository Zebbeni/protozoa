package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestNoSettingHasTwoRows: a setting scaled by a curve gets a hidden row generated into the ABILITIES section by withCurveGraphs.
func TestNoSettingHasTwoRows(t *testing.T) {
	loadKeyGlobals(t)
	g := *config.GetCurrentGlobals()
	defaults := g
	defaults.InitialAbilityScores = append([]int(nil), g.InitialAbilityScores...)
	old := loadConfigDefaults
	loadConfigDefaults = func() config.Globals { return defaults }
	defer func() { loadConfigDefaults = old }()
	form := g
	cs := NewConfigScreen(&form)
	count := map[string]int{}
	where := map[string][]string{}
	for _, sec := range cs.sections {
		for _, f := range sec.fields {
			if f.jsonTag == "" {
				continue
			}
			count[f.jsonTag]++
			vis := "visible"
			if f.hidden {
				vis = "hidden"
			}
			where[f.jsonTag] = append(where[f.jsonTag], sec.title+"/"+vis)
		}
	}
	for tag, n := range count {
		if n > 1 {
			t.Errorf("%s appears %d times: %v", tag, n, where[tag])
		}
	}
}
