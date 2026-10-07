package config

import (
	"encoding/json"
	"testing"
)

func TestAbsentNodeListGetsTheDefaults(t *testing.T) {
	var g Globals
	if err := json.Unmarshal([]byte(`{"seed": 7}`), &g); err != nil {
		t.Fatal(err)
	}
	if g.DisabledDecisionNodes != nil {
		t.Fatal("absent JSON should leave the slice nil; the repair depends on it")
	}
	g.repairDecisionNodes()
	if len(g.DisabledDecisionNodes) != len(DefaultDisabledNodes) {
		t.Errorf("an old file got %d hold-outs, want the %d defaults",
			len(g.DisabledDecisionNodes), len(DefaultDisabledNodes))
	}
}

func TestEmptyNodeListMeansNothingDisabled(t *testing.T) {
	var g Globals
	if err := json.Unmarshal([]byte(`{"disabled_decision_nodes": []}`), &g); err != nil {
		t.Fatal(err)
	}
	if g.DisabledDecisionNodes == nil {
		t.Fatal("an explicit [] decoded to nil; absent and empty would be indistinguishable")
	}
	g.repairDecisionNodes()
	if len(g.DisabledDecisionNodes) != 0 {
		t.Errorf("an explicit empty list was overwritten with %v", g.DisabledDecisionNodes)
	}
}

// TestAChosenNodeListIsLeftAlone: a user who has picked their own set must not have the defaults merged back in.
func TestAChosenNodeListIsLeftAlone(t *testing.T) {
	var g Globals
	if err := json.Unmarshal([]byte(`{"disabled_decision_nodes": ["ActDig"]}`), &g); err != nil {
		t.Fatal(err)
	}
	g.repairDecisionNodes()
	if len(g.DisabledDecisionNodes) != 1 || g.DisabledDecisionNodes[0] != "ActDig" {
		t.Errorf("a chosen list became %v", g.DisabledDecisionNodes)
	}
}

func TestRepairDoesNotAliasTheDefaults(t *testing.T) {
	var g Globals
	g.repairDecisionNodes()
	if len(g.DisabledDecisionNodes) == 0 {
		t.Fatal("no defaults to test with")
	}
	g.DisabledDecisionNodes[0] = "MUTATED"
	if DefaultDisabledNodes[0] == "MUTATED" {
		t.Error("editing a repaired list wrote through to DefaultDisabledNodes")
	}
}
