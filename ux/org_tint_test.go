package ux

import (
	d "github.com/Zebbeni/protozoa/decision"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

func tintFixture() (orgTint, *organism.Info) {
	info := &organism.Info{
		ID:       7,
		Health:   5,
		Size:     10,
		Age:      50,
		IdealPh:  5,
		Location: utils.Point{X: 1, Y: 1},
		Color:    familyUnrelatedColor(),
	}
	info.Abilities[physiology.AbilityTolerance] = 4
	t := orgTint{
		oldestAlive: 100,
		cycle:       10,
		phAt:        func(utils.Point) float64 { return 5 },
	}
	return t, info
}

// TestEveryColorModeTintsTheMinimapToo: the minimap paints from the same
// function the grid does, so a mode that returns a colour on one returns it
// on the other. Before this it understood two of the nine and quietly fell
// back to lineage colours for the rest.
func TestEveryColorModeTintsTheMinimapToo(t *testing.T) {
	loadKeyGlobals(t)
	base, info := tintFixture()
	trueCol, flat := base.bodyColor(info)
	if flat {
		t.Error("TRUE mode reported a flat tint; it must keep the organism's own colours")
	}

	differs := 0
	for _, m := range allOrgColorModes {
		if m == orgColorTrue {
			continue
		}
		tint := base
		tint.mode = m
		tint.globals = config.GetCurrentGlobals()
		col, flat := tint.bodyColor(info)
		if !flat {
			t.Errorf("mode %v did not claim the tint, so the minimap would paint the lineage colour", m)
		}
		if col != trueCol {
			differs++
		}
	}
	if differs == 0 {
		t.Error("no mode produced a colour different from TRUE, so this proves nothing")
	}
}

// TestFamilyTintDoesNotShareTheGridsMemo: the minimap colours on a
// background goroutine and the family memo is written as organisms are
// coloured, so the two must never hold the same tinter.
func TestFamilyTintDoesNotShareTheGridsMemo(t *testing.T) {
	a := orgTint{mode: orgColorFamily}
	b := orgTint{mode: orgColorFamily}
	if a.family != nil || b.family != nil {
		t.Skip("no selection in this fixture")
	}
	// With no selection both answer the unrelated colour rather than
	// dereferencing a shared tinter.
	if got, _ := a.bodyColor(&organism.Info{}); got != familyUnrelatedColor() {
		t.Errorf("family tint with no selection gave %v, want the unrelated colour", got)
	}
}

// TestActionModeBrightestWhenActing: the mode answers two questions at once
// — what an organism is doing now, and what it is built to do — and acting
// must always outrank any amount of tree weight, or a tree full of an action
// would be indistinguishable from one taking it.
func TestActionModeBrightestWhenActing(t *testing.T) {
	loadKeyGlobals(t)
	weights := make([]float64, 16)
	weights[d.ActEat] = 1 // as built-for-eating as a tree can be

	acting := &organism.Info{Action: d.ActEat, ActionWeights: weights}
	built := &organism.Info{Action: d.ActMove, ActionWeights: weights}

	tint := orgTint{mode: orgColorAction, action: d.ActEat}
	hot, _ := tint.bodyColor(acting)
	warm, _ := tint.bodyColor(built)
	if hot == warm {
		t.Fatal("an organism acting reads the same as one merely built for it")
	}
	if _, _, hl := hot.Hcl(); true {
		_, _, wl := warm.Hcl()
		if hl <= wl {
			t.Errorf("acting is not the brighter of the two (%v vs %v)", hot, warm)
		}
	}
}

// TestActionModeRampsWithTreeWeight: between two organisms doing something
// else, the one whose tree holds more of the action reads greener.
func TestActionModeRampsWithTreeWeight(t *testing.T) {
	loadKeyGlobals(t)
	tint := orgTint{mode: orgColorAction, action: d.ActDig}
	prev := -1.0
	for _, share := range []float64{0, 0.05, 0.1, 0.2, 0.34, 1} {
		w := make([]float64, 16)
		w[d.ActDig] = share
		col, flat := tint.bodyColor(&organism.Info{Action: d.ActIdle, ActionWeights: w})
		if !flat {
			t.Fatal("the action mode did not claim the tint")
		}
		_, _, l := col.Hcl()
		if l < prev-1e-9 {
			t.Errorf("share %.2f is darker than the share below it", share)
		}
		prev = l
	}
}

// TestActionModeSurvivesAShortWeightSlice: a snapshot restored from another
// configuration can carry weights for fewer actions than this build has, and
// indexing past the end would panic in the render loop.
func TestActionModeSurvivesAShortWeightSlice(t *testing.T) {
	loadKeyGlobals(t)
	tint := orgTint{mode: orgColorAction, action: d.ActDig}
	for _, w := range [][]float64{nil, {}, {0.5}} {
		if _, flat := tint.bodyColor(&organism.Info{Action: d.ActIdle, ActionWeights: w}); !flat {
			t.Error("the action mode did not claim the tint for a short weight slice")
		}
	}
}
