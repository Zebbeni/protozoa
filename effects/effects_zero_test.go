package effects

import (
	"testing"

	"github.com/Zebbeni/protozoa/physiology"
)

func TestAtZeroEndsDefaultToTheOldFloor(t *testing.T) {
	g := digGlobals(t)
	g.MaxFoodPerEatAtZeroEating = 0
	g.AttackDamageAtZeroAttack = 0
	g.ThornsDamageAtZeroDefense = 0

	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		for _, size := range []float64{1, 7.5, 40} {
			if got, want := MaxFoodPerEat(g, score, size),
				size*g.MaxFoodPerEatAtFullEating*Multiplier(g, physiology.CurveEating, score); got != want {
				t.Fatalf("bite at score %d size %v: %v, old formula gives %v", score, size, got, want)
			}
			if got, want := AttackDamage(g, score, size),
				size*g.AttackDamageAtFullAttack*Multiplier(g, physiology.CurveAttack, score); got != want {
				t.Fatalf("attack at score %d size %v: %v, old formula gives %v", score, size, got, want)
			}
			if got, want := ThornsDamage(g, score, size),
				size*g.ThornsDamageAtFullDefense*Multiplier(g, physiology.CurveThorns, score); got != want {
				t.Fatalf("thorns at score %d size %v: %v, old formula gives %v", score, size, got, want)
			}
		}
	}
}

func TestAtZeroEndIsWhatAZeroScoreGets(t *testing.T) {
	g := digGlobals(t)
	g.MaxFoodPerEatAtZeroEating = 2
	g.MaxFoodPerEatAtFullEating = 10

	const size = 3.0
	if got, want := MaxFoodPerEat(g, 0, size), size*2.0; got != want {
		t.Errorf("a 0-Eating bite is %v, want %v", got, want)
	}
	if got, want := MaxFoodPerEat(g, physiology.MaxAbilityScore, size), size*10.0; got != want {
		t.Errorf("a full-Eating bite is %v, want %v", got, want)
	}
	prev := -1.0
	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		got := MaxFoodPerEat(g, score, size)
		if got < prev {
			t.Errorf("bite fell from %v to %v at score %d", prev, got, score)
		}
		prev = got
	}
}

func TestAZeroScoreCanStillEatWhenGivenAFloor(t *testing.T) {
	g := digGlobals(t)
	g.MaxFoodPerEatAtZeroEating = 0
	if got := MaxFoodPerEat(g, 0, 10); got != 0 {
		t.Errorf("with no floor a 0-Eating organism takes %v, want nothing", got)
	}
	g.MaxFoodPerEatAtZeroEating = 1
	if got := MaxFoodPerEat(g, 0, 10); got <= 0 {
		t.Errorf("with a floor of 1 a 0-Eating organism still takes %v", got)
	}
}
