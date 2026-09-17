package effects

import (
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// digGlobals is the shipped configuration, for tests that want the real
// numbers rather than ones they invented.
func digGlobals(t *testing.T) *config.Globals {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return &g
}

// TestDigEffectsHitBothEndpoints: each whole-unit dig effect starts at
// its *AtZero setting and reaches its size-class value at 100 Digging.
// Those two settings are the whole contract — everything between them is
// the curve.
func TestDigEffectsHitBothEndpoints(t *testing.T) {
	g := digGlobals(t)
	g.FoodFromDiggingAtZero, g.WallStrengthDeltaAtZero = 2, 4
	g.FoodFromDiggingSmall, g.WallStrengthDeltaSmall = 9, 20

	const small = 1.0
	if got := DigFood(g, 0, small); got != 2 {
		t.Errorf("food at 0 Digging = %d, want the at-zero setting (2)", got)
	}
	if got := DigFood(g, 100, small); got != 9 {
		t.Errorf("food at 100 Digging = %d, want the size-class setting (9)", got)
	}
	if got := DigWallRemoved(g, 0, small); got != 4 {
		t.Errorf("wall strength at 0 Digging = %d, want the at-zero setting (4)", got)
	}
	if got := DigWallRemoved(g, 100, small); got != 20 {
		t.Errorf("wall strength at 100 Digging = %d, want the size-class setting (20)", got)
	}

	// An at-zero above the full value still reads as an endpoint, so a
	// setting that inverts the curve does something predictable rather
	// than clamping to nothing.
	g.FoodFromDiggingAtZero, g.FoodFromDiggingSmall = 6, 1
	if lo, hi := DigFood(g, 0, small), DigFood(g, 100, small); lo != 6 || hi != 1 {
		t.Errorf("inverted endpoints gave %d → %d, want 6 → 1", lo, hi)
	}
}

// TestDigEffectsRiseWithDigging: between the endpoints both effects only
// ever climb, in whole units.
func TestDigEffectsRiseWithDigging(t *testing.T) {
	g := digGlobals(t)
	for _, size := range []float64{1, 20, 50, 90} {
		prevFood, prevWall := DigFood(g, 0, size), DigWallRemoved(g, 0, size)
		for score := 1; score <= 100; score++ {
			food, wall := DigFood(g, score, size), DigWallRemoved(g, score, size)
			if food < prevFood {
				t.Fatalf("size %g: food fell from %d to %d at Digging %d", size, prevFood, food, score)
			}
			if wall < prevWall {
				t.Fatalf("size %g: wall strength fell from %d to %d at Digging %d", size, prevWall, wall, score)
			}
			prevFood, prevWall = food, wall
		}
		if prevFood != SizeFoodFromDigging(g, size) || prevWall != SizeStrengthDelta(g, size) {
			t.Errorf("size %g ends at food %d / wall %d, want the size-class values %d / %d",
				size, prevFood, prevWall, SizeFoodFromDigging(g, size), SizeStrengthDelta(g, size))
		}
	}
}

// TestADigAlwaysMovesTerrain: on the shipped settings no dig is wasted
// motion — it costs health, so moving nothing would read as a bug.
func TestADigAlwaysMovesTerrain(t *testing.T) {
	g := digGlobals(t)
	for _, size := range []float64{1, 20, 50, 90} {
		for score := 0; score <= 100; score += 5 {
			if got := DigWallRemoved(g, score, size); got < 1 {
				t.Errorf("size %g at Digging %d moves %d wall strength", size, score, got)
			}
		}
	}
}

// TestSuccessfulEatPaysForItsAttempt is the invariant behind the eat
// attempt cost: a bite that fills an organism's capacity must be worth
// more than the attempt cost, at every Eating score that can eat at all.
// Below that line a *successful* eat is a net loss and the ability is a
// trap — the organism does the right thing and dies of it.
//
// The relationship is what matters, not the numbers: cost scales with
// size and so does capacity, so this holds for every size or none.
func TestSuccessfulEatPaysForItsAttempt(t *testing.T) {
	g := digGlobals(t)
	cost := func(size float64) float64 { return -g.HealthChangeFromEatingAttempt * size }

	for _, size := range []float64{1, 2, 10, 40, 100} {
		for score := 1; score <= 100; score++ {
			gain := HealthFromFood(g, MaxFoodPerEat(g, score, size))
			if gain <= cost(size) {
				t.Fatalf("size %g at Eating %d: a full bite gains %.3f health but the attempt costs %.3f",
					size, score, gain, cost(size))
			}
		}
	}

	// The margin at the lowest score that can eat: capacity there is
	// roughly the organism's own size, so this is health_per_food_unit
	// against the attempt cost. Thin here and the ability only pays for
	// organisms that have already invested in it.
	const size = 10
	margin := HealthFromFood(g, MaxFoodPerEat(g, 1, size)) / cost(size)
	if margin < 2 {
		t.Errorf("at Eating 1 a full bite is only %.1fx its attempt cost; raise health_per_food_unit "+
			"or lower health_change_from_eating_attempt", margin)
	}
	t.Logf("at Eating 1, a full bite is %.1fx the attempt cost", margin)

	// Eating 0 is the one dead case, and it's deliberate: no capacity,
	// so every attempt is a miss that pays the cost for nothing.
	if got := MaxFoodPerEat(g, 0, size); got != 0 {
		t.Errorf("Eating 0 has capacity %v, want none", got)
	}
}

// TestActionCostsNeverReachZero: each cost curve runs between two
// configured ends, so a maxed-out ability buys a discount and never an
// exemption. A cost curve alone runs to nothing, which would let the best
// movers travel for free and the best diggers reshape terrain for free —
// and a free action is one selection can no longer price.
func TestActionCostsNeverReachZero(t *testing.T) {
	g := digGlobals(t)
	const size = 10.0

	for _, tc := range []struct {
		name          string
		cost          func(*config.Globals, int, float64) float64
		atZero, atMax float64
	}{
		{"move", MoveCost, g.HealthChangeFromMoving, g.HealthChangeFromMovingAtMax},
		{"turn", TurnCost, g.HealthChangeFromTurning, g.HealthChangeFromTurningAtMax},
		{"dig", DigCost, g.HealthChangeFromDigging, g.HealthChangeFromDiggingAtMax},
	} {
		if tc.atMax >= 0 {
			t.Errorf("%s at full ability is configured at %v; it has to stay a cost", tc.name, tc.atMax)
		}
		if got, want := tc.cost(g, 0, size), tc.atZero*size; math.Abs(got-want) > 1e-12 {
			t.Errorf("%s at ability 0 = %v, want the configured %v", tc.name, got, want)
		}
		if got, want := tc.cost(g, physiology.MaxAbilityScore, size), tc.atMax*size; math.Abs(got-want) > 1e-12 {
			t.Errorf("%s at full ability = %v, want the configured %v", tc.name, got, want)
		}

		// Cheaper with every point, and never free anywhere along it.
		prev := tc.cost(g, 0, size)
		for score := 1; score <= physiology.MaxAbilityScore; score++ {
			got := tc.cost(g, score, size)
			if got <= prev {
				t.Errorf("%s at ability %d costs %v, not less than the %v before it", tc.name, score, got, prev)
			}
			if got >= 0 {
				t.Errorf("%s at ability %d costs %v; the action is never free", tc.name, score, got)
			}
			prev = got
		}
	}
}

// TestWallCreationIsItsOwnCurve: what a dig clears ahead and what it
// raises beside it are separate settings on separate curves, so an
// organism can be able to tunnel without being able to build — a small
// digger slowly clearing a path shouldn't be walling itself in as it
// goes.
func TestWallCreationIsItsOwnCurve(t *testing.T) {
	g := digGlobals(t)
	const small, large = 1.0, 90.0

	// The shipped settings are the case that motivated the split.
	if got := SizeWallCreated(g, small); got != 0 {
		t.Errorf("a small organism raises %d wall strength per dig, want none", got)
	}
	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		if got := DigWallCreated(g, score, small); got != 0 {
			t.Errorf("a small organism at Digging %d raised %d, want none at any score", score, got)
		}
		if got := DigWallRemoved(g, score, small); got < 1 {
			t.Errorf("a small organism at Digging %d cleared %d; it should still be able to tunnel", score, got)
		}
	}

	// The two are independent: moving one endpoint doesn't move the other.
	wide := *g
	wide.WallCreatedSmall, wide.WallCreatedAtZero = 6, 2
	if got := DigWallCreated(&wide, 0, small); got != 2 {
		t.Errorf("creation at 0 Digging = %d, want the at-zero setting 2", got)
	}
	if got := DigWallCreated(&wide, physiology.MaxAbilityScore, small); got != 6 {
		t.Errorf("creation at full Digging = %d, want the size-class setting 6", got)
	}
	if got := DigWallRemoved(&wide, physiology.MaxAbilityScore, small); got != SizeStrengthDelta(g, small) {
		t.Errorf("changing creation moved removal to %d", got)
	}

	// And the curves are separate too: each reads its own shape.
	wide.DiggingCreationCurveShape = string(physiology.ShapeLinear)
	wide.DiggingStrengthCurveShape = string(physiology.ShapeQuadratic)
	creation := Multiplier(&wide, physiology.CurveDiggingCreate, physiology.MaxAbilityScore/2)
	removal := Multiplier(&wide, physiology.CurveDiggingStrength, physiology.MaxAbilityScore/2)
	if creation == removal {
		t.Errorf("both curves gave %v at half score; they should read their own shapes", creation)
	}

	// A large organism still builds, so creation isn't off globally.
	if got := DigWallCreated(g, physiology.MaxAbilityScore, large); got < 1 {
		t.Errorf("a large organism at full Digging raised %d, want at least 1", got)
	}
}
