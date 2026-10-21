package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

// orgTabWidth is the width drawTabStrip gives each of the four tabs across the panel.
func orgTabWidth() int {
	n := len(orgTabLabels())
	return (graphWidth - tabStripGap*(n-1)) / n
}

// TestOrgTabLabelsFitTheirTabs measures every label against its button in the
// font drawGraphButton draws with. Four tabs across the panel leave each 89px
// where two left 183, so a label that fit the old strip need not fit this one.
//
// Both worlds are measured, since the third tab is named for whichever brain
// the simulation runs on.
func TestOrgTabLabelsFitTheirTabs(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	tabW := orgTabWidth()
	for _, nets := range []bool{false, true} {
		g := *config.GetCurrentGlobals()
		g.UseNeuralNets = nets
		config.SetGlobals(&g)
		for _, label := range orgTabLabels() {
			if w := textAdvance(r.FontSourceCodePro8, label); w > tabW {
				t.Errorf("with use_neural_nets=%v, tab %q measures %dpx in a %dpx tab",
					nets, label, w, tabW)
			}
		}
	}
}

// TestEveryOrgTabHasALabel ties the tab constants to the strip they index.
func TestEveryOrgTabHasALabel(t *testing.T) {
	for _, tab := range []int{orgTabTraits, orgTabStats, orgTabDecision, orgTabDescendant} {
		if tab < 0 || tab >= len(orgTabLabels()) {
			t.Errorf("tab %d has no label in a strip of %d", tab, len(orgTabLabels()))
		}
	}
}

// TestTabContentStaysLeftOfThePortrait pins the layout the merged strip rests
// on: the tabs span the panel, the content they switch does not reach the
// portrait, and the TRAITS tab's two columns both sit inside that content area.
func TestTabContentStaysLeftOfThePortrait(t *testing.T) {
	if right := selectedXOffset + infoAreaWidth; right > selectedPortraitX {
		t.Errorf("content area ends at %d, the portrait starts at %d", right, selectedPortraitX)
	}
	if abilitiesColX+abilitiesColWidth > selectedXOffset+infoAreaWidth {
		t.Errorf("abilities column ends at %d, the content area at %d",
			abilitiesColX+abilitiesColWidth, selectedXOffset+infoAreaWidth)
	}
	if selectedCol1X+statsColWidth > abilitiesColX {
		t.Errorf("traits column ends at %d, the abilities column starts at %d",
			selectedCol1X+statsColWidth, abilitiesColX)
	}
	if statsCol2X+statsTabColWidth > selectedXOffset+infoAreaWidth {
		t.Errorf("second stats column ends at %d, the content area at %d",
			statsCol2X+statsTabColWidth, selectedXOffset+infoAreaWidth)
	}
}

// TestAbilityBarsHoldTheirOwnLabels measures the ability block at whatever
// abilityFace is set to: the rotated name is drawn down the middle of its bar,
// so the bar has to be at least a line height wide, and the bars have to fit
// their slots without touching.
func TestAbilityBarsHoldTheirOwnLabels(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	lineH := abilityFace().Metrics().Height.Round()
	if abilityBarWidth < lineH {
		t.Errorf("ability bar is %dpx wide, its rotated label %dpx tall", abilityBarWidth, lineH)
	}
	slotW := abilitiesColWidth / len(physiology.AllAbilities)
	if slotW <= abilityBarWidth {
		t.Errorf("ability slot is %dpx for a %dpx bar, so the bars touch", slotW, abilityBarWidth)
	}
}
