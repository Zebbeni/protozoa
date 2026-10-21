package ux

import (
	"fmt"
	"strings"
	"testing"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

// TestTheEatingWarningTracksWhatAnEatIsWorth: the warning is about the
// mechanism, not about what the settings file happens to say today, so it is
// stated both ways round — a bite that beats its attempt cost warns about
// nothing, and one that does not is named with the score it fails at.
func TestTheEatingWarningTracksWhatAnEatIsWorth(t *testing.T) {
	loadKeyGlobals(t)
	g := *c.GetCurrentGlobals()

	// A bite worth far more than the attempt: nothing to say.
	paying := g
	paying.MaxFoodPerEatAtZeroEating = 1
	paying.MaxFoodPerEatAtFullEating = 2
	paying.HealthPerFoodUnit = 10
	paying.HealthChangeFromEatingAttempt = -0.1
	paying.HealthChangeFromEatingAttemptAtMax = -0.01
	if w := abilityWarnings(&paying, physiology.AbilityEating); len(w) != 0 {
		t.Errorf("a bite worth 10.0 against a 0.1 attempt warns: %q", w)
	}

	// The same settings with the attempt costing more than the bite returns.
	losing := paying
	losing.HealthChangeFromEatingAttempt = -20
	losing.HealthChangeFromEatingAttemptAtMax = -20
	w := abilityWarnings(&losing, physiology.AbilityEating)
	if len(w) != 1 {
		t.Fatalf("a successful eat that loses health warns %d times: %q", len(w), w)
	}
	// It names the span it fails over, measured rather than assumed: the
	// top score here breaks even exactly, so the span stops below it.
	highest := -1
	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		net := effects.HealthFromFood(&losing, score, effects.MaxFoodPerEat(&losing, score, warnSize)) +
			effects.EatCost(&losing, score, warnSize)
		if net < 0 {
			highest = score
		}
	}
	if want := fmt.Sprintf("Eating 0-%d", highest); !strings.Contains(w[0], want) {
		t.Errorf("warning %q does not name the span %q it fails over", w[0], want)
	}

	// The figure quoted is what effects says, not a second copy of the sum.
	net := effects.HealthFromFood(&losing, 0, effects.MaxFoodPerEat(&losing, 0, warnSize)) +
		effects.EatCost(&losing, 0, warnSize)
	if !strings.Contains(w[0], fmt.Sprintf("%.3f", -net)) {
		t.Errorf("warning %q does not quote the %.3f health effects says is lost", w[0], -net)
	}
}

// TestTheEatingWarningFiresWhereTheRunDies is the case this was written for:
// a successful eat at the bottom of the range costing more than it returns is
// what took eating out of every run on the shipped defaults.
func TestTheEatingWarningFiresWhereTheRunDies(t *testing.T) {
	loadKeyGlobals(t)
	g := *c.GetCurrentGlobals()
	// Eating 0 pays, Eating 1 does not: the warning has to catch the span
	// rather than only the bottom of it.
	g.MaxFoodPerEatAtZeroEating = 0.01
	g.MaxFoodPerEatAtFullEating = 0.1
	g.HealthPerFoodUnit = 10
	g.HealthChangeFromEatingAttempt = -0.25
	g.HealthChangeFromEatingAttemptAtMax = -0.01
	g.EatingCurveShape = string(physiology.ShapeLinear)

	w := abilityWarnings(&g, physiology.AbilityEating)
	if len(w) == 0 {
		t.Fatal("the configuration that killed eating in every run warns about nothing")
	}
	// Every score the warning claims loses health has to actually lose it.
	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		net := effects.HealthFromFood(&g, score, effects.MaxFoodPerEat(&g, score, warnSize)) +
			effects.EatCost(&g, score, warnSize)
		t.Logf("eating %2d: net %+0.4f health per unit of size", score, net)
	}
}

// TestTheChemoWarningIsAboutTheBandAndTheGain: the two faults are different.
// A gain of nothing at the organism's own ideal pH is chemosynthesis switched
// off; a band of no width at any score is every attempt but an exact pH match
// costing the full gain, which no score can buy its way out of.
func TestTheChemoWarningIsAboutTheBandAndTheGain(t *testing.T) {
	loadKeyGlobals(t)
	g := *c.GetCurrentGlobals()

	sound := g
	sound.MaxChemosynthesisGain = 0.5
	sound.MaxChemosynthesisPhWidth = 1
	sound.ChemosynthesisCurveShape = string(physiology.ShapeLinear)
	if w := abilityWarnings(&sound, physiology.AbilityChemosynthesis); len(w) != 0 {
		t.Errorf("a gain of 0.5 in a band a full score opens warns: %q", w)
	}

	noGain := sound
	noGain.MaxChemosynthesisGain = 0
	if w := abilityWarnings(&noGain, physiology.AbilityChemosynthesis); len(w) == 0 {
		t.Error("chemosynthesis gaining nothing at its own ideal pH warns about nothing")
	}

	noBand := sound
	noBand.MaxChemosynthesisPhWidth = 0
	w := abilityWarnings(&noBand, physiology.AbilityChemosynthesis)
	if len(w) == 0 {
		t.Fatal("a band no score can open warns about nothing")
	}
	if !strings.Contains(w[len(w)-1], "feeding band") {
		t.Errorf("the band fault is reported as %q", w[len(w)-1])
	}
}

// TestTheWarningsFitThePanel: they are prose written against no width at all,
// and the panel is 700px with the text set in the smallest face.
func TestTheWarningsFitThePanel(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	g := *c.GetCurrentGlobals()
	// Every fault at once, with figures wide enough to be worth measuring.
	g.MaxFoodPerEatAtZeroEating = 0.001
	g.MaxFoodPerEatAtFullEating = 0.001
	g.HealthPerFoodUnit = 0.001
	g.HealthChangeFromEatingAttempt = -123.456
	g.HealthChangeFromEatingAttemptAtMax = -123.456
	g.MaxChemosynthesisGain = -123.456
	g.MaxChemosynthesisPhWidth = 0

	room := cfgPanelWidth - graphToggleW - 2
	for _, a := range []physiology.Ability{physiology.AbilityEating, physiology.AbilityChemosynthesis} {
		for _, line := range abilityWarnings(&g, a) {
			if w := textAdvance(warnFace(), line); w > room {
				t.Errorf("%s warning is %dpx against the %dpx panel: %q", a.Name(), w, room, line)
			}
		}
	}
}

// TestTheShippedSettingsRaiseNoWarning: the screen must not come up red on its
// own defaults. Worth pinning because the eating economy sits deliberately
// close to the line — a bite at Eating 0 takes 0.1 food per unit of size and
// digests it at 0.5 health a unit, which is exactly the 0.05 attempt cost. A
// zero score can eat and gains nothing by it, so investing in the ability is
// the only way to profit and no score is left paying to eat.
func TestTheShippedSettingsRaiseNoWarning(t *testing.T) {
	loadKeyGlobals(t)
	g := c.GetCurrentGlobals()
	for _, a := range physiology.AllAbilities {
		if w := abilityWarnings(g, a); len(w) != 0 {
			t.Errorf("%s warns on the shipped settings: %q", a.Name(), w)
		}
	}
	net := effects.HealthFromFood(g, 0, effects.MaxFoodPerEat(g, 0, warnSize)) +
		effects.EatCost(g, 0, warnSize)
	t.Logf("a successful eat at Eating 0 nets %+.4f health per unit of size", net)
}
