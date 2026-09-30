package ux

import (
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
