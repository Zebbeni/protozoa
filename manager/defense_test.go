package manager

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// scoresWithDefense builds a valid distribution with the given Defense,
// balancing the budget out of Chemosynthesis.
func scoresWithDefense(t *testing.T, defense int) physiology.Scores {
	t.Helper()
	s := physiology.GenesisScores()
	s[physiology.AbilityChemosynthesis] -= defense - s[physiology.AbilityDefense]
	s[physiology.AbilityDefense] = defense
	if err := s.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}
	return s
}

// TestDefenseDamageMultNeverGoesNegative is the regression test for the
// sign flip: the Defense curve is linear past full specialisation, so
// unfloored it crosses zero at a score of about 96, where attacks (and
// unhealthy pH) would heal the defender instead of hurting it.
func TestDefenseDamageMultNeverGoesNegative(t *testing.T) {
	loadDefaultGlobals(t)

	for defense := 0; defense <= 50; defense += 5 {
		if got := defenseDamageMult(scoresWithDefense(t, defense)); got < 0 {
			t.Errorf("defense %d: damage multiplier %.3f is negative — damage would heal", defense, got)
		}
	}

	// A maximal defense specialist. Assert the unfloored curve really
	// is negative here, so the floor check guards something reachable
	// rather than a no-op.
	maxed := physiology.Scores{0, 0, 0, 0, 0, 100}
	if err := maxed.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}
	if raw := maxed.Multiplier(physiology.AbilityDefense); raw >= 0 {
		t.Fatalf("expected the unfloored multiplier to be negative at defense 100, got %.3f", raw)
	}
	if got := defenseDamageMult(maxed); got != 0 {
		t.Errorf("defense 100: damage multiplier %.3f, want floored at 0", got)
	}
	if got := defensePhMult(defenseDamageMult(maxed)); got < 0 {
		t.Errorf("defense 100: pH multiplier %.3f is negative — unhealthy pH would heal", got)
	}
}

// TestDefenseShieldsPh pins the pH protection's shape.
func TestDefenseShieldsPh(t *testing.T) {
	loadDefaultGlobals(t)

	genesis := physiology.GenesisScores()
	if got := defensePhMult(defenseDamageMult(genesis)); got != 1 {
		t.Errorf("a genesis organism should take normal pH damage, got multiplier %.3f", got)
	}

	specialist := scoresWithDefense(t, physiology.SpecialistScore(physiology.AbilityDefense))
	attackMult := defenseDamageMult(specialist)
	phMult := defensePhMult(attackMult)
	if !(phMult < 1) {
		t.Errorf("a defense specialist should take less pH damage, got %.3f", phMult)
	}
	if protection := config.DefensePhProtection(); protection < 1 && !(phMult > attackMult) {
		t.Errorf("with protection %.2f, pH should be shielded less than attacks: pH %.3f vs attack %.3f",
			protection, phMult, attackMult)
	}

	undefended := scoresWithDefense(t, 0)
	if got := defensePhMult(defenseDamageMult(undefended)); !(got > 1) {
		t.Errorf("an organism below its genesis Defense should take more pH damage, got %.3f", got)
	}
}
