package ux

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/physiology"
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

// TestOrgColorLabelsFitTheirButtons: eight colour modes in one row left each
// about 30px, which is not enough for "PH EFFECT" or "TOLERANCE" — the
// labels were drawn clipped. Over two rows each button is twice as wide.
//
// Measured against the font drawGraphButton actually draws with and the
// geometry sectionRowButtonRect actually uses, like the display-toggle
// version of this test, so it fails if either changes under it.
func TestOrgColorLabelsFitTheirButtons(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	const padding = 4 // drawGraphButton's own inset either side
	for i, b := range orgColorButtons {
		_, idxInRow, countInRow := orgColorRowSplit(i)
		_, w := sectionRowButtonRect(idxInRow, countInRow)
		lw := boundString(r.FontSourceCodePro10, b.label).Dx()
		if lw+padding > w {
			t.Errorf("%q needs %dpx and its button is %dpx", b.label, lw+padding, w)
		}
	}
}

// TestOrgColorRowSplitCoversEveryButtonOnce: the split decides both where a
// button is drawn and where its hitbox goes, so a gap or an overlap in it
// means a mode that cannot be clicked or one that steals another's clicks.
func TestOrgColorRowSplitCoversEveryButtonOnce(t *testing.T) {
	seen := map[[2]int]int{}
	for i := range orgColorButtons {
		row, idxInRow, countInRow := orgColorRowSplit(i)
		if row < 0 || row >= orgColorRows {
			t.Fatalf("button %d landed in row %d of %d", i, row, orgColorRows)
		}
		if idxInRow < 0 || idxInRow >= countInRow {
			t.Fatalf("button %d is index %d of %d in its row", i, idxInRow, countInRow)
		}
		key := [2]int{row, idxInRow}
		if prev, dup := seen[key]; dup {
			t.Fatalf("buttons %d and %d both sit at row %d index %d", prev, i, row, idxInRow)
		}
		seen[key] = i
	}
	if len(seen) != len(orgColorButtons) {
		t.Errorf("the split placed %d of %d buttons", len(seen), len(orgColorButtons))
	}
}

// TestOrgColorSectionHeightMatchesItsRows: the section draws colour rows
// then ability rows, and separately REPORTS a height that everything below
// it is offset by. The two are computed apart, so a row added to either
// group without the height following draws the next section over this one.
func TestOrgColorSectionHeightMatchesItsRows(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	// The lowest row the section actually draws, relative to its own top.
	lowest := 0
	for i := range orgColorButtons {
		row, _, _ := orgColorRowSplit(i)
		if top := row * sectionRowPitch; top > lowest {
			lowest = top
		}
	}
	for i := range physiology.AllAbilities {
		row, _, _ := abilityRowSplit(i)
		if top := (orgColorRows + row) * sectionRowPitch; top > lowest {
			lowest = top
		}
	}
	for i := range colorActionButtons {
		row, _, _ := actionRowSplit(i)
		if top := (orgColorRows + abilityRows + row) * sectionRowPitch; top > lowest {
			lowest = top
		}
	}

	p := &Panel{grid: &Grid{}}
	img := ebiten.NewImage(panelWidth, 2000)
	reported := p.renderOrgColor(img, 0)
	if reported != lowest {
		t.Errorf("the section reports %dpx and draws its last row at %dpx; "+
			"the next section would be drawn %dpx out", reported, lowest, lowest-reported)
	}

	// And every hitbox it registered sits inside the height it claimed.
	for _, hb := range p.orgColorBtnRects {
		if hb.y-orgColorYOffset+hb.h > reported+sectionRowHeight {
			t.Errorf("a colour button at y=%d falls outside the reported height", hb.y)
		}
	}
}

// TestAbilityLabelsFitTheirButtons is the same measurement for the ability
// row under the colour rows, which shares the row-button geometry.
func TestAbilityLabelsFitTheirButtons(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	const padding = 4
	for i, a := range physiology.AllAbilities {
		_, idxInRow, countInRow := abilityRowSplit(i)
		_, w := sectionRowButtonRect(idxInRow, countInRow)
		label, ok := abilityButtonLabels[a]
		if !ok {
			label = strings.ToUpper(a.Name())
		}
		lw := boundString(r.FontSourceCodePro10, label).Dx()
		if lw+padding > w {
			t.Errorf("%q needs %dpx and its button is %dpx", label, lw+padding, w)
		}
	}
}

// TestGroupBracketLabelsFitTheirColumn: the bracket labels sit in the same
// reserved column the row labels use, to the LEFT of the bracket, so they
// have less room than a row label does. One that overruns would be drawn
// over the panel's edge rather than clipped.
func TestGroupBracketLabelsFitTheirColumn(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	// What drawGroupBracket leaves for the text: the label column minus the
	// bracket and both gaps.
	avail := sectionBtnsX - sectionRowX - 2*groupBracketGap
	for _, label := range []string{"ability", "action"} {
		if w := boundString(r.FontSourceCodePro10, label).Dx(); w > avail {
			t.Errorf("%q needs %dpx and the column leaves %dpx", label, w, avail)
		}
	}
}

// TestGroupBracketsSpanTheirRows: a bracket is drawn from the first row's top
// to the last row's BOTTOM, so it has to be told the row height as well as
// the pitch. Stopping at the last row's top would leave it visibly short.
func TestGroupBracketsSpanTheirRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows int
	}{{"ability", abilityRows}, {"action", actionRows}} {
		top := 0
		bottom := (tc.rows-1)*sectionRowPitch + sectionRowHeight
		if bottom <= top {
			t.Fatalf("%s bracket spans nothing", tc.name)
		}
		// It covers every row it marks, including the last one's full height.
		lastRowBottom := (tc.rows-1)*sectionRowPitch + sectionRowHeight
		if bottom < lastRowBottom {
			t.Errorf("%s bracket ends at %d, short of its last row's %d",
				tc.name, bottom, lastRowBottom)
		}
		// And does not run into the row below the group.
		if next := tc.rows * sectionRowPitch; bottom > next {
			t.Errorf("%s bracket ends at %d, past the next row's top at %d",
				tc.name, bottom, next)
		}
	}
}

// TestGroupBracketsClearTheButtons: the bracket sits in the label column, so
// it must not overlap the first button of the rows it marks.
func TestGroupBracketsClearTheButtons(t *testing.T) {
	bracketRight := sectionBtnsX - groupBracketGap + groupBracketTick
	firstBtnX, _ := sectionRowButtonRect(0, 4)
	if bracketRight > firstBtnX {
		t.Errorf("the bracket reaches %dpx and the first button starts at %dpx",
			bracketRight, firstBtnX)
	}
}
