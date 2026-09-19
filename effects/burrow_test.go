package effects

import (
	"testing"

	"github.com/Zebbeni/protozoa/physiology"
)

// TestBurrowingIsGatedOnSizeAndScore: burrowing is the fast way past a
// wall, so what gets through has to be earned twice over — by the
// ability, and by the size an organism only reaches by surviving long
// enough to grow. A founder should get through the weakest walls and
// nothing more.
func TestBurrowingIsGatedOnSizeAndScore(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 1

	// Neither alone is enough: a big organism that never invested in
	// Digging gets through nothing at all.
	if CanBreakWall(g, 0, 100, 1) {
		t.Error("a Digging score of 0 broke a wall; the ability should gate it entirely")
	}
	// And a specialist that is still tiny gets through very little.
	if CanBreakWall(g, physiology.MaxAbilityScore, 1, 50) {
		t.Error("a size-1 specialist broke a strength-50 wall")
	}
	if !CanBreakWall(g, physiology.MaxAbilityScore, 1, 5) {
		t.Error("a size-1 specialist should still manage a weak wall")
	}

	// Both together get through anything.
	if !CanBreakWall(g, physiology.MaxAbilityScore, 50, 100) {
		t.Error("a large specialist should get through the strongest wall there is")
	}
}

// TestBurrowStrengthRisesWithBoth pins the shape: more size or more
// score always gets you through more, never less.
func TestBurrowStrengthRisesWithBoth(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 1

	prev := -1.0
	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		got := WallBreakStrength(g, score, 10)
		if got < prev {
			t.Errorf("score %d gets through %v, less than score %d's %v", score, got, score-1, prev)
		}
		prev = got
	}
	prev = -1
	for size := 1.0; size <= 100; size += 10 {
		got := WallBreakStrength(g, 5, size)
		if got < prev {
			t.Errorf("size %v gets through %v, less than the size below it", size, got)
		}
		prev = got
	}
}

// TestBurrowingOffByMultiplier: 0 has to switch the mechanic off
// entirely, leaving digging as the only way through a wall — the same
// escape hatch every other multiplier has.
func TestBurrowingOffByMultiplier(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 0

	if CanBreakWall(g, physiology.MaxAbilityScore, 1000, 1) {
		t.Error("a 0 multiplier still let the biggest possible specialist through the weakest possible wall")
	}
}

// TestBurrowingNeverBreaksEvenOnAZeroWall: the comparison is strictly
// greater, so the threshold can't be met by a wall that isn't there.
// Belt and braces — callers check for a wall first — but a >= here would
// let a 0-score organism "break" a 0-strength wall and read as burrowing
// through open ground.
func TestBurrowingNeverBreaksEvenOnAZeroWall(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 1

	if CanBreakWall(g, 0, 1, 0) {
		t.Error("a 0-score organism broke a 0-strength wall")
	}
}
