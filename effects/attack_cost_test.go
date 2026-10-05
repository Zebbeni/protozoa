package effects

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// TestAttackCostFallsWithTheScore: attacking is priced like moving, turning
// and digging now — both ends named, carried between them along a cost
// curve, so specialising makes it cheaper without making it free.
func TestAttackCostFallsWithTheScore(t *testing.T) {
	g := digGlobals(t)
	g.HealthChangeFromAttacking = -0.05
	g.HealthChangeFromAttackingAtMax = -0.02
	config.SetGlobals(g)

	const size = 10
	prev := AttackCost(g, 0, size)
	if prev != size*g.HealthChangeFromAttacking {
		t.Errorf("at score 0 the cost is %v, want %v", prev, size*g.HealthChangeFromAttacking)
	}
	for score := 1; score <= physiology.MaxAbilityScore; score++ {
		got := AttackCost(g, score, size)
		if got < prev-1e-12 {
			t.Fatalf("score %d costs %v, more than score %d's %v", score, got, score-1, prev)
		}
		prev = got
	}
	if want := size * g.HealthChangeFromAttackingAtMax; prev != want {
		t.Errorf("at a full score the cost is %v, want %v", prev, want)
	}
}

// TestAttackCostNeverReachesZero: a cost curve running to nothing would
// make a maxed ability exempt from paying, which is why both ends are
// settings rather than one end and a curve to zero.
func TestAttackCostNeverReachesZero(t *testing.T) {
	g := digGlobals(t)
	g.HealthChangeFromAttacking = -0.05
	g.HealthChangeFromAttackingAtMax = -0.02
	config.SetGlobals(g)
	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		if got := AttackCost(g, score, 10); got >= 0 {
			t.Errorf("score %d attacks for %v; attacking is never free", score, got)
		}
	}
}

// TestEqualAttackCostEndsAreInert reproduces the flat cost attacking had
// before the curve, which is what to revert to if the curve is unwanted.
func TestEqualAttackCostEndsAreInert(t *testing.T) {
	g := digGlobals(t)
	g.HealthChangeFromAttacking = -0.05
	g.HealthChangeFromAttackingAtMax = -0.05
	config.SetGlobals(g)
	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		if got, want := AttackCost(g, score, 10), 10*-0.05; got != want {
			t.Errorf("score %d: %v, want exactly %v", score, got, want)
		}
	}
}

// TestShippedAttackCostEngagesTheCurve: the request was for the cost to
// SCALE, so equal ends in the shipped settings would satisfy the letter of
// it and not the point.
func TestShippedAttackCostEngagesTheCurve(t *testing.T) {
	g := digGlobals(t)
	config.SetGlobals(g)
	atZero := AttackCost(g, 0, 10)
	atMax := AttackCost(g, physiology.MaxAbilityScore, 10)
	if atZero == atMax {
		t.Errorf("the shipped attack cost is flat at %v; the curve is inert", atZero)
	}
	if atMax <= atZero {
		t.Errorf("shipped cost at max (%v) is not cheaper than at 0 (%v)", atMax, atZero)
	}
}
