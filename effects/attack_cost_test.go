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
	// A sloped shape, because this test is about the cost being CARRIED
	// between the two ends. The shipped shape is flat, which on a cost
	// curve means the multiplier is 0 and every score pays the at-max end.
	g.AttackCostCurveShape = string(physiology.ShapeLinear)
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

// TestTheShippedAttackCostIsDeliberatelyFlat.
//
// The curve exists and works (TestAttackCostFallsWithTheScore), and the
// shipped settings switch it off: both ends are the same value, which
// TestEqualAttackCostEndsAreInert pins as producing a flat cost. This
// guards that the two ways of saying "flat" AGREE, so the cost cannot be
// flat by one and sloped by the other.
//
// Separating the ends re-engages it, and that is the whole change.
func TestTheShippedAttackCostIsDeliberatelyFlat(t *testing.T) {
	g := digGlobals(t)
	config.SetGlobals(g)
	if g.HealthChangeFromAttacking != g.HealthChangeFromAttackingAtMax {
		t.Fatalf("the shipped attack cost ends differ (%v and %v); this test is about them being equal",
			g.HealthChangeFromAttacking, g.HealthChangeFromAttackingAtMax)
	}
	atZero := AttackCost(g, 0, 10)
	atMax := AttackCost(g, physiology.MaxAbilityScore, 10)
	if atZero != atMax {
		t.Errorf("equal ends give %v at score 0 and %v at max; the shape is overriding them", atZero, atMax)
	}
	// The shape is ShapeFlat, which on a COST curve means the multiplier is
	// 1-1 = 0 and costBetween returns the AT-MAX end. That is the same
	// number only while the ends are equal: separate them and flat means
	// always the cheapest, not always the dearest.
	if want := g.HealthChangeFromAttackingAtMax * 10; atZero != want {
		t.Errorf("the flat cost is %v, want the at-max end scaled by size, %v", atZero, want)
	}
}
