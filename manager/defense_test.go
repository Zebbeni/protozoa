package manager

import (
	"testing"

	"github.com/Zebbeni/protozoa/physiology"
)

// scoresWithDefense builds a valid distribution with the given Defense,
// balancing the budget out of Chemosynthesis.
func scoresWithDefense(t *testing.T, defense int) physiology.Scores {
	t.Helper()
	s := physiology.Scores{8, 2, 2, 2, 2, 2, 2}
	s[physiology.AbilityChemosynthesis] -= defense - s[physiology.AbilityDefense]
	s[physiology.AbilityDefense] = defense
	if err := s.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}
	return s
}

// TestDefenseDamageMultNeverGoesNegative: damage taken can shrink to zero
// (immunity) but never below, where attacks and unhealthy pH would heal
// the defender.
func TestDefenseDamageMultNeverGoesNegative(t *testing.T) {
	loadDefaultGlobals(t)

	for defense := 0; defense <= 10; defense += 2 {
		if got := defenseDamageMult(scoresWithDefense(t, defense)); got < 0 {
			t.Errorf("defense %d: damage multiplier %.3f is negative — damage would heal", defense, got)
		}
	}
	maxed := physiology.Scores{0, 0, 0, 0, 0, 10, 10}
	if got := defenseDamageMult(maxed); got != 0 {
		t.Errorf("defense at the cap: damage multiplier %.3f, want 0", got)
	}
}
