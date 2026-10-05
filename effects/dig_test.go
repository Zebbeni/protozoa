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

// digGlobals is the shipped configuration, for tests that want the real numbers rather than ones they invented.
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

func TestDigEffectsHitBothEndpoints(t *testing.T) {
	g := digGlobals(t)
	// A shape, set here rather than taken from the settings: ShapeFlat is 1
	// at every score, so it reaches the at-max endpoint even at 0 and the
	// at-zero is unreachable by definition. That is what flat MEANS, so a
	// test about endpoints has to engage the curve first.
	g.DiggingStrengthCurveShape = string(physiology.ShapeLinear)
	g.FoodFromDiggingAtZero = 2
	g.FoodFromDiggingSmall = 9

	const small = 1.0
	if got := DigFood(g, 0, small); got != 2 {
		t.Errorf("food at 0 Digging = %d, want the at-zero setting (2)", got)
	}
	if got := DigFood(g, 100, small); got != 9 {
		t.Errorf("food at 100 Digging = %d, want the size-class setting (9)", got)
	}

	// An at-zero above the full value still reads as an endpoint.
	g.FoodFromDiggingAtZero, g.FoodFromDiggingSmall = 6, 1
	if lo, hi := DigFood(g, 0, small), DigFood(g, 100, small); lo != 6 || hi != 1 {
		t.Errorf("inverted endpoints gave %d → %d, want 6 → 1", lo, hi)
	}
}

// TestDigFoodRisesWithDigging: between the endpoints the yield only ever climbs, in whole units.
func TestDigFoodRisesWithDigging(t *testing.T) {
	g := digGlobals(t)
	for _, size := range []float64{1, 20, 50, 90} {
		prevFood := DigFood(g, 0, size)
		for score := 1; score <= 100; score++ {
			food := DigFood(g, score, size)
			if food < prevFood {
				t.Fatalf("size %g: food fell from %d to %d at Digging %d", size, prevFood, food, score)
			}
			prevFood = food
		}
		if prevFood != SizeFoodFromDigging(g, size) {
			t.Errorf("size %g ends at food %d, want the size-class value %d",
				size, prevFood, SizeFoodFromDigging(g, size))
		}
	}
}

// TestSuccessfulEatPaysForItsAttempt is the invariant behind the eat attempt cost.
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

	const size = 10
	margin := HealthFromFood(g, MaxFoodPerEat(g, 1, size)) / cost(size)
	if margin < 2 {
		t.Errorf("at Eating 1 a full bite is only %.1fx its attempt cost; raise health_per_food_unit "+
			"or lower health_change_from_eating_attempt", margin)
	}
	t.Logf("at Eating 1, a full bite is %.1fx the attempt cost", margin)

	// Eating 0 must not pay for itself, which is the property that matters
	// rather than its capacity being exactly 0. A floor of 0 says a zero
	// score cannot eat AT ALL; a small floor says it can and should not
	// bother. Either is fine; what is not is a free bite, which removes the
	// reason to invest in the ability.
	if zero := HealthFromFood(g, MaxFoodPerEat(g, 0, size)); zero > cost(size) {
		t.Errorf("at Eating 0 a full bite returns %.4f against a %.4f attempt cost, "+
			"so eating pays without investing in it", zero, cost(size))
	}
}

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

func TestWallCreationIsItsOwnCurve(t *testing.T) {
	g := digGlobals(t)
	const small, large = 1.0, 90.0

	// Both endpoints are the settings, whichever way round they run.
	if got, want := DigWallCreated(g, 0, small), g.WallCreatedAtZero; got != want {
		t.Errorf("creation at 0 Digging = %d, want the at-zero setting %d", got, want)
	}
	if got, want := DigWallCreated(g, physiology.MaxAbilityScore, small), SizeWallCreated(g, small); got != want {
		t.Errorf("creation at full Digging = %d, want the size-class setting %d", got, want)
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
	if got := DigFood(&wide, physiology.MaxAbilityScore, small); got != SizeFoodFromDigging(g, small) {
		t.Errorf("changing wall creation moved the food yield to %d", got)
	}

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

func TestAnyDiggerRaisesSomethingFromAStockedCell(t *testing.T) {
	g := digGlobals(t)
	if g.FoodFromDiggingAtZero < 1 {
		t.Fatalf("food_from_digging_at_zero is %d; a scratch should raise something",
			g.FoodFromDiggingAtZero)
	}
	for _, size := range []float64{1, 5, 20, 50, 90} {
		if got := DigFood(g, 0, size); got != g.FoodFromDiggingAtZero {
			t.Errorf("size %g at Digging 0 raises %d, want the at-zero setting %d",
				size, got, g.FoodFromDiggingAtZero)
		}
	}
}
