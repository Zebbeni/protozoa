package ux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

func abilityConfigScreen(t *testing.T) (*ConfigScreen, *config.Globals) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	config.SetGlobals(&g)
	defaults := g
	defaults.InitialAbilityScores = append([]int(nil), g.InitialAbilityScores...)
	loadConfigDefaults = func() config.Globals { return defaults }
	form := g // what the popup edits: a copy of the active globals
	return NewConfigScreen(&form), &g
}

func TestInitialAbilitiesBlockStartUntilTheyTotalTheBudget(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	if reason := cs.StartBlockedReason(); reason != "" {
		t.Fatalf("default scores should be startable, got %q", reason)
	}

	cs.adjustAbilityScore(int(physiology.AbilityAttack), 2)
	if cs.StartBlockedReason() == "" {
		t.Errorf("a total of %d should block the start", physiology.PointTotal+2)
	}
	cs.adjustAbilityScore(int(physiology.AbilityChemosynthesis), -2)
	if reason := cs.StartBlockedReason(); reason != "" {
		t.Errorf("rebalanced to %d should start, got %q", physiology.PointTotal, reason)
	}

	cs.adjustAbilityScore(int(physiology.AbilityDefense), -1)
	cs.globals.RandomInitialAbilities = true
	if reason := cs.StartBlockedReason(); reason != "" {
		t.Errorf("Random ignores the fixed scores, got %q", reason)
	}
}

func TestInitialAbilityScoresClamp(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	cs.adjustAbilityScore(int(physiology.AbilityEating), -1000)
	cs.adjustAbilityScore(int(physiology.AbilityMovement), 1000)
	if got := cs.globals.InitialAbilityScores[physiology.AbilityEating]; got != 0 {
		t.Errorf("score below 0 should clamp to 0, got %d", got)
	}
	if got := cs.globals.InitialAbilityScores[physiology.AbilityMovement]; got != physiology.MaxAbilityScore {
		t.Errorf("score above 100 should clamp to 100, got %d", got)
	}
}

// TestConfigScreenDoesNotEditActiveScores: the form's globals are a copy,
// but a slice copies by reference; editing must not reach the active
// config before Start.
func TestConfigScreenDoesNotEditActiveScores(t *testing.T) {
	cs, active := abilityConfigScreen(t)
	before := active.InitialAbilityScores[physiology.AbilityAttack]
	cs.adjustAbilityScore(int(physiology.AbilityAttack), 7)
	if active.InitialAbilityScores[physiology.AbilityAttack] != before {
		t.Error("editing the form changed the active config's ability scores")
	}
}
