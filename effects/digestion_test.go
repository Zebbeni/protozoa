package effects

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// engagedDigestion is the shipped settings with the digestion curve turned
// on, which is what every test here is about: the curve SHIPS FLAT (see
// TestTheShippedDigestionCurveIsDeliberatelyFlat), so a test run against the
// defaults would be measuring the inert case and passing for the wrong
// reason.
func engagedDigestion(t *testing.T) *config.Globals {
	t.Helper()
	g := digGlobals(t)
	g.EatingDigestionCurveShape = string(physiology.ShapeSaturating)
	g.EatingDigestionSaturatingK = 4
	g.HealthPerFoodUnitAtZero = 0.5
	return g
}

// TestABetterEaterDigestsMore is what the curve is for: the same food is
// worth more health to an organism that has invested in Eating.
func TestABetterEaterDigestsMore(t *testing.T) {
	g := engagedDigestion(t)

	if got, want := HealthFromFood(g, 0, 1), g.HealthPerFoodUnitAtZero; got != want {
		t.Errorf("one unit of food at Eating 0 is worth %g, want the at-zero setting %g", got, want)
	}
	if got, want := HealthFromFood(g, physiology.MaxAbilityScore, 1), g.HealthPerFoodUnit; got != want {
		t.Errorf("one unit of food at full Eating is worth %g, want %g", got, want)
	}

	prev := -1.0
	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		per := HealthFromFood(g, score, 1)
		if per <= prev {
			t.Errorf("Eating %d digests one unit of food into %.4f health, no more than Eating %d's %.4f",
				score, per, score-1, prev)
		}
		prev = per
		t.Logf("Eating %2d: %.3f health per food unit", score, per)
	}
}

// TestCapacityAndDigestionMultiply: the Eating score buys how much one attempt
// takes AND how much of it is absorbed, so what a meal is worth rises faster
// than either curve alone. That product is why the first point of Eating can
// turn a losing bite into a paying one — see
// TestSuccessfulEatPaysForItsAttempt, which is the balance half of this.
func TestCapacityAndDigestionMultiply(t *testing.T) {
	g := engagedDigestion(t)
	const size = 10

	meal := func(score int) float64 { return HealthFromFood(g, score, MaxFoodPerEat(g, score, size)) }
	capacityOnly := func(score int) float64 {
		return MaxFoodPerEat(g, score, size) * HealthFromFood(g, 0, 1)
	}

	for score := 1; score <= physiology.MaxAbilityScore; score++ {
		if meal(score) <= capacityOnly(score) {
			t.Errorf("at Eating %d a full bite is worth %.3f, no more than the %.3f capacity alone buys",
				score, meal(score), capacityOnly(score))
		}
	}
	t.Logf("a full bite at size %d: Eating 0 %.3f, Eating 1 %.3f, Eating %d %.3f",
		size, meal(0), meal(1), physiology.MaxAbilityScore, meal(physiology.MaxAbilityScore))
}

// TestFlatDigestionIsTheValueItWasBeforeTheCurve is the compatibility claim,
// and it has to be BIT-identical rather than close: health per food unit was a
// plain constant, and a settings file or replay header written before this
// curve existed carries neither the shape nor the at-zero end. The shape key
// decodes to "", which is not a valid shape name, so the curve falls back to
// its default — and its default is flat for exactly this reason.
func TestFlatDigestionIsTheValueItWasBeforeTheCurve(t *testing.T) {
	for _, shape := range []string{"", string(physiology.ShapeFlat)} {
		g := digGlobals(t)
		g.EatingDigestionCurveShape = shape
		g.HealthPerFoodUnitAtZero = 0 // what an absent key decodes to

		for _, food := range []float64{0, 0.01, 0.1, 1, 7.5, 100, 1000} {
			for score := 0; score <= physiology.MaxAbilityScore; score++ {
				got := HealthFromFood(g, score, food)
				want := food * g.HealthPerFoodUnit
				if got != want {
					t.Fatalf("shape %q, Eating %d, %g food: %v, want exactly %v",
						shape, score, food, got, want)
				}
			}
		}
	}
}

// TestEqualDigestionEndsAreInert is what to revert to if the curve turns out
// to be unwanted: ends that match make the score decide nothing, whatever
// shape is in use. The same property the attack cost curve has.
func TestEqualDigestionEndsAreInert(t *testing.T) {
	for _, shape := range physiology.AllShapeKinds {
		g := digGlobals(t)
		g.EatingDigestionCurveShape = string(shape)
		g.HealthPerFoodUnitAtZero = g.HealthPerFoodUnit

		for score := 0; score <= physiology.MaxAbilityScore; score++ {
			if got, want := HealthFromFood(g, score, 3), 3*g.HealthPerFoodUnit; got != want {
				t.Errorf("shape %s, Eating %d: 3 food is worth %v with equal ends, want %v",
					shape, score, got, want)
			}
		}
	}
}

// TestTheShippedDigestionEndpointsRise: digestion is an EFFECT curve, so its
// at-zero end must not sit above its at-max end — that would make a better
// eater digest WORSE, which no shape can correct and which reads as the
// ability working backwards. Deliberately independent of WHICH shape ships,
// so engaging or flattening the curve is a balance decision that does not
// break a test. TestDamageGraphsRiseWithScore caught exactly this for thorns,
// where an at-zero of 0.29286 against an at-max of 0.01 ran the curve
// backwards in the shipped file.
func TestTheShippedDigestionEndpointsRise(t *testing.T) {
	g := digGlobals(t)
	if g.HealthPerFoodUnitAtZero > g.HealthPerFoodUnit {
		t.Errorf("food is worth %g a unit at Eating 0 and %g at full, so investing in Eating digests worse",
			g.HealthPerFoodUnitAtZero, g.HealthPerFoodUnit)
	}
	shape := physiology.ShapeFor(g, physiology.CurveEatingDigestion)
	t.Logf("the shipped digestion curve is %q, %g to %g health per food unit",
		shape, HealthFromFood(g, 0, 1), HealthFromFood(g, physiology.MaxAbilityScore, 1))
	// Whatever the shape, the ends are what the ends say.
	if got := HealthFromFood(g, 0, 1); shape != physiology.ShapeFlat && got != g.HealthPerFoodUnitAtZero {
		t.Errorf("one unit of food at Eating 0 is worth %v, want the at-zero setting %v",
			got, g.HealthPerFoodUnitAtZero)
	}
	if got := HealthFromFood(g, physiology.MaxAbilityScore, 1); got != g.HealthPerFoodUnit {
		t.Errorf("one unit of food at full Eating is worth %v, want %v", got, g.HealthPerFoodUnit)
	}
}
