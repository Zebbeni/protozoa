package config

import (
	"encoding/json"
	"testing"
)

// TestNormalizeSignsRepairsOldFiles: attack and thorns damage were
// stored as negative health changes before they became positive
// magnitudes subtracted where they land. Their json tags didn't change,
// so a settings file or replay header written back then still loads —
// and has to mean the same amount of damage, not the same number.
func TestNormalizeSignsRepairsOldFiles(t *testing.T) {
	var g Globals
	old := `{"health_change_inflicted_by_attack": -650,
	         "health_change_inflicted_by_thorns": -0.3,
	         "health_change_from_moving": 0.03125,
	         "max_chemosynthesis_gain": -0.15,
	         "growth_factor": -0.5}`
	if err := json.Unmarshal([]byte(old), &g); err != nil {
		t.Fatal(err)
	}
	g.NormalizeSigns()

	for _, tc := range []struct {
		name string
		got  float64
		want float64
	}{
		{"attack damage", g.AttackDamageAtFullAttack, 650},
		{"thorns damage", g.ThornsDamageAtFullDefense, 0.3},
		{"move cost", g.HealthChangeFromMoving, -0.03125},
		{"chemosynthesis gain", g.MaxChemosynthesisGain, 0.15},
		// Not sign-bound: a setting nobody classified keeps what it was
		// given, however odd, rather than being quietly rewritten.
		{"growth factor", g.GrowthFactor, -0.5},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// TestEverySignedSettingExists guards the table against a typo or a
// renamed tag: a sign recorded for a tag no field carries does nothing,
// silently.
func TestEverySignedSettingExists(t *testing.T) {
	tags := map[string]bool{}
	data, err := json.Marshal(Globals{})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for tag := range fields {
		tags[tag] = true
	}
	for tag := range settingSigns {
		if !tags[tag] {
			t.Errorf("settingSigns has %q, which no Globals field carries", tag)
		}
	}
}
