package ux

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	r "github.com/Zebbeni/protozoa/resources"
)

func TestDisplayTogglesRoundTrip(t *testing.T) {
	for _, tc := range displayToggles {
		t.Run(tc.label, func(t *testing.T) {
			g := &Grid{}

			tc.set(g, true)
			if !tc.get(g) {
				t.Fatalf("set(true) then get() = false — accessors disagree")
			}
			tc.set(g, false)
			if tc.get(g) {
				t.Fatalf("set(false) then get() = true — accessors disagree")
			}
		})
	}
}

func TestDisplayTogglesAreDistinct(t *testing.T) {
	for i, a := range displayToggles {
		for j, b := range displayToggles {
			if i >= j {
				continue
			}
			g := &Grid{}
			a.set(g, true)
			if b.get(g) {
				t.Errorf("%q and %q share a Grid field: setting %q lit %q",
					a.label, b.label, a.label, b.label)
			}
		}
	}
}

func TestDisplayToggleLabelsFitTheirButtons(t *testing.T) {
	// Fonts come from resources.Init, not from package-level vars.
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	const padding = 4
	for i, tc := range displayToggles {
		_, w := sectionRowButtonRect(i, len(displayToggles))
		got := boundString(r.FontSourceCodePro8, tc.label).Dx()
		if got+padding > w {
			t.Errorf("%q is %dpx wide in a %dpx button (%dpx of padding wanted); "+
				"shorten the label or give the row fewer toggles",
				tc.label, got, w, padding)
		}
	}
}

func TestGraphModeButtonsShareOneRow(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	p := &Panel{}
	img := ebiten.NewImage(graphWidth+2*graphXOffset, 200)
	p.renderGraphButtons(img, graphXOffset, 0, graphWidth)

	if len(p.graphButtonRects) != len(graphModeButtons) {
		t.Fatalf("%d hitboxes for %d buttons", len(p.graphButtonRects), len(graphModeButtons))
	}

	// The population modes are on one row, everything else on another.
	popY := p.graphButtonRects[0].y
	for i, rect := range p.graphButtonRects[:graphPopulationButtons] {
		if rect.y != popY {
			t.Errorf("population button %d is at y=%d, want %d with the rest of its row", i, rect.y, popY)
		}
	}
	modeY := p.graphButtonRects[graphPopulationButtons].y
	if modeY == popY {
		t.Fatal("the mode buttons are on the same row as the population buttons")
	}
	for i, rect := range p.graphButtonRects[graphPopulationButtons:] {
		if rect.y != modeY {
			t.Errorf("%s is at y=%d, want %d — every mode button shares one row",
				graphModeButtons[graphPopulationButtons+i].label, rect.y, modeY)
		}
	}

	// The ABILITY toggle completes the top row.
	if p.graphAbilityToggleRect == nil {
		t.Fatal("no ABILITY toggle hitbox")
	}
	if p.graphAbilityToggleRect.y != popY {
		t.Errorf("the ABILITY toggle is at y=%d, want the population row at %d",
			p.graphAbilityToggleRect.y, popY)
	}

	// Nothing overlaps, and nothing hangs off either edge.
	for i, a := range p.graphButtonRects {
		if a.x < graphXOffset || a.x+a.w > graphXOffset+graphWidth {
			t.Errorf("button %d spans %d..%d, outside the graph's %d..%d",
				i, a.x, a.x+a.w, graphXOffset, graphXOffset+graphWidth)
		}
		for j, b := range p.graphButtonRects {
			if i >= j || a.y != b.y {
				continue
			}
			if a.x < b.x+b.w && b.x < a.x+a.w {
				t.Errorf("buttons %d and %d overlap on row y=%d", i, j, a.y)
			}
		}
	}

	for i, rect := range p.graphButtonRects {
		label := graphModeButtons[i].label
		if got := boundString(r.FontSourceCodePro8, label).Dx(); got+4 > rect.w {
			t.Errorf("%q is %dpx in a %dpx button", label, got, rect.w)
		}
	}
}
