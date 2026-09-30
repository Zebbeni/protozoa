package effects

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

func TestBurrowingIsGatedOnSizeAndScore(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 1

	// Neither alone is enough: a big organism that never invested in Digging gets through nothing at all.
	if CanBreakWall(g, 0, 100, 1) {
		t.Error("a Digging score of 0 broke a wall; the ability should gate it entirely")
	}
	if CanBreakWall(g, physiology.MaxAbilityScore, 1, 50) {
		t.Error("a size-1 specialist broke a strength-50 wall")
	}
	if !CanBreakWall(g, physiology.MaxAbilityScore, 1, 5) {
		t.Error("a size-1 specialist should still manage a weak wall")
	}

	if !CanBreakWall(g, physiology.MaxAbilityScore, 50, 100) {
		t.Error("a large specialist should get through the strongest wall there is")
	}
}

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

func TestBurrowingOffByMultiplier(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 0

	if CanBreakWall(g, physiology.MaxAbilityScore, 1000, 1) {
		t.Error("a 0 multiplier still let the biggest possible specialist through the weakest possible wall")
	}
}

func TestBurrowingNeverBreaksEvenOnAZeroWall(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 1

	if CanBreakWall(g, 0, 1, 0) {
		t.Error("a 0-score organism broke a 0-strength wall")
	}
}

func TestMaxBreakableWallAgreesWithCanBreakWall(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 1

	for _, size := range []float64{0, 0.5, 1, 7.5, 12, 37.4, 100} {
		for score := 0; score <= physiology.MaxAbilityScore; score++ {
			most := MaxBreakableWall(g, score, size)

			// Everything it claims must actually break...
			if most > 0 && !CanBreakWall(g, score, size, most) {
				t.Errorf("size %v score %d: claims strength %d, CanBreakWall says no", size, score, most)
			}
			// ...and the next one up must not.
			if CanBreakWall(g, score, size, most+1) {
				t.Errorf("size %v score %d: claims strength %d, but %d breaks too", size, score, most, most+1)
			}
		}
	}
}

func TestMaxBreakableWallIsZeroWhenNothingBreaks(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 1

	if got := MaxBreakableWall(g, 0, 100); got != 0 {
		t.Errorf("a Digging score of 0 reports %d, want 0", got)
	}
	g.WallBreakMultiplier = 0
	if got := MaxBreakableWall(g, physiology.MaxAbilityScore, 100); got != 0 {
		t.Errorf("burrowing switched off reports %d, want 0", got)
	}
}

func TestBurrowEndpointsDefaultToTheOldFormula(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakAtZeroDigging = 0
	g.WallBreakAtMaxDigging = config.DefaultWallBreakAtMax

	for _, mult := range []float64{0, 0.5, 1, 2.5} {
		g.WallBreakMultiplier = mult
		for score := 0; score <= physiology.MaxAbilityScore; score++ {
			for _, size := range []float64{0, 1, 2.5, 7.5, 37.4, 100} {
				got := WallBreakStrength(g, score, size)
				want := size * float64(score) * mult
				if got != want {
					t.Fatalf("mult %v score %d size %v: %v, old formula %v", mult, score, size, got, want)
				}
			}
		}
	}
}

func TestBurrowEndsAreWhatTheScoresGet(t *testing.T) {
	g := digGlobals(t)
	g.WallBreakMultiplier = 1
	g.WallBreakAtZeroDigging = 4
	g.WallBreakAtMaxDigging = 20

	const size = 2.0
	if got, want := WallBreakStrength(g, 0, size), size*4.0; got != want {
		t.Errorf("0 Digging burrows %v, want %v", got, want)
	}
	if got, want := WallBreakStrength(g, physiology.MaxAbilityScore, size), size*20.0; got != want {
		t.Errorf("full Digging burrows %v, want %v", got, want)
	}
	// A floor at zero Digging is the thing "score x size" could not express.
	if !CanBreakWall(g, 0, size, 7) {
		t.Error("with an at-zero end of 4 and size 2, a strength-7 wall should still give")
	}
	g.WallBreakMultiplier = 0
	if CanBreakWall(g, physiology.MaxAbilityScore, size, 1) {
		t.Error("a multiplier of 0 should stop even a full specialist")
	}
}

func TestBurrowRepairKeepsOldFilesWorking(t *testing.T) {
	old := digGlobals(t)
	old.WallBreakMultiplier = 1
	old.WallBreakAtZeroDigging = 0
	old.WallBreakAtMaxDigging = 0 // as absent JSON decodes

	config.SetGlobals(old)
	repaired := config.GetCurrentGlobals()
	if repaired.WallBreakAtMaxDigging != config.DefaultWallBreakAtMax {
		t.Fatalf("an old file kept an at-max of %v; burrowing would be off",
			repaired.WallBreakAtMaxDigging)
	}
	if got, want := WallBreakStrength(repaired, physiology.MaxAbilityScore, 3), 3*float64(physiology.MaxAbilityScore); got != want {
		t.Errorf("after repair a full specialist burrows %v, want the old %v", got, want)
	}

	// A file that genuinely wants burrowing off says so with the multiplier, and the repair must leave that alone.
	off := digGlobals(t)
	off.WallBreakMultiplier = 0
	off.WallBreakAtMaxDigging = 0
	config.SetGlobals(off)
	if CanBreakWall(config.GetCurrentGlobals(), physiology.MaxAbilityScore, 100, 1) {
		t.Error("the repair switched burrowing back on for a file that turned it off")
	}
}

func TestWallBreakDefaultMatchesTheAbilityCap(t *testing.T) {
	if config.DefaultWallBreakAtMax != physiology.MaxAbilityScore {
		t.Errorf("config.DefaultWallBreakAtMax is %d, physiology.MaxAbilityScore is %d",
			config.DefaultWallBreakAtMax, physiology.MaxAbilityScore)
	}
}
