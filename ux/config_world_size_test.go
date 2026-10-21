package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// worldSizeRow finds the row the buttons are drawn on, and fails rather than
// returning a zero value: every test below is about that row existing at the
// top of the form.
func worldSizeRow(t *testing.T, cs *ConfigScreen) (section int, row configField) {
	t.Helper()
	for si, s := range cs.sections {
		for _, f := range s.fields {
			if f.row == rowWorldSize {
				return si, f
			}
		}
	}
	t.Fatal("no rowWorldSize on the config screen")
	return 0, configField{}
}

// TestTheWorldSizeButtonsAreTheFirstThingOnTheForm: the row sets the grid and
// both food stocks, and the rows it overrides are spread across two sections
// further down, so a reader who meets those first has already been handed the
// harder version of the same decision.
func TestTheWorldSizeButtonsAreTheFirstThingOnTheForm(t *testing.T) {
	cs, _ := abilityConfigScreen(t)

	si, _ := worldSizeRow(t, cs)
	if si != 0 {
		t.Errorf("the world size row is in section %d (%q), want the first",
			si, cs.sections[si].title)
	}
	if first := cs.sections[0].fields[0]; first.row != rowWorldSize {
		t.Errorf("the first row of the first section is %q, want the world size buttons", first.label)
	}
}

// TestClickingASizeButtonSetsThatWorld walks every button, clicks its own
// rectangle, and requires the four settings to land on that size. It is the
// only test that exercises the geometry and the action together — a button
// drawn where nothing is clickable passes every other check here.
func TestClickingASizeButtonSetsThatWorld(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	_, row := worldSizeRow(t, cs)
	_ = row

	const px, py = 100, 200
	for i, size := range config.AllWorldSizes {
		x, y, w, h := cs.worldSizeButtonRect(px, py, i)
		cs.clickWorldSize(px, py, x+w/2, y+h/2)

		got, ok := config.WorldSizeOf(cs.globals)
		if !ok || got != size {
			t.Errorf("clicking %s left the form on %v (known: %v)", size.Name(), got, ok)
		}
		wantW, wantH := size.Grid()
		if cs.globals.GridUnitsWide != wantW || cs.globals.GridUnitsHigh != wantH {
			t.Errorf("clicking %s gave a %dx%d grid, want %dx%d", size.Name(),
				cs.globals.GridUnitsWide, cs.globals.GridUnitsHigh, wantW, wantH)
		}
	}
}

// TestTheSizeButtonsDoNotOverlapAndStayInTheSliderColumn: they are laid out
// across the column a slider would occupy, so three of them have to divide it
// without touching each other or running into the value beside them.
func TestTheSizeButtonsDoNotOverlapAndStayInTheSliderColumn(t *testing.T) {
	cs, _ := abilityConfigScreen(t)

	const px, py = 0, 0
	colLeft := px + cfgLabelWidth
	colRight := colLeft + cs.sliderColWidth()

	prevRight := colLeft
	for i, size := range config.AllWorldSizes {
		x, _, w, h := cs.worldSizeButtonRect(px, py, i)
		if x < prevRight {
			t.Errorf("%s starts at %d, overlapping the button before it (ends %d)", size.Name(), x, prevRight)
		}
		if x+w > colRight {
			t.Errorf("%s ends at %d, past the slider column's right edge %d", size.Name(), x+w, colRight)
		}
		if h > cfgRowHeight {
			t.Errorf("%s is %dpx tall against a %dpx row", size.Name(), h, cfgRowHeight)
		}
		prevRight = x + w
	}
}

// TestEverySizeLabelFitsItsButton measures against the face drawSmallButton
// actually draws with. A label wider than its button is centred and overhangs
// on both sides, which reads as two buttons running together.
func TestEverySizeLabelFitsItsButton(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	cs, _ := abilityConfigScreen(t)

	for i, size := range config.AllWorldSizes {
		_, _, w, _ := cs.worldSizeButtonRect(0, 0, i)
		if lw := boundString(r.FontSourceCodePro8, size.Name()).Dx(); lw > w {
			t.Errorf("%q measures %dpx against a %dpx button", size.Name(), lw, w)
		}
	}
}

// TestAClickBesideTheButtonsChangesNothing: the row spans the full width of
// the form, so most of it is label and value rather than a control, and a
// click there must not pick a size.
func TestAClickBesideTheButtonsChangesNothing(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	const px, py = 100, 200

	cs.clickWorldSize(px, py, px, py+5) // on the label
	before := *cs.globals

	cs.clickWorldSize(px, py, px+cfgLabelWidth+cs.sliderColWidth()+30, py+5) // on the value
	if cs.globals.GridUnitsWide != before.GridUnitsWide ||
		cs.globals.GridUnitsHigh != before.GridUnitsHigh ||
		cs.globals.InitialFood != before.InitialFood {
		t.Error("a click outside the buttons changed the world size")
	}
}

// TestAReadOnlyScreenIgnoresTheSizeButtons: the replay menu's View Settings
// shows this screen without letting it change, and every other control checks
// readOnly before acting.
func TestAReadOnlyScreenIgnoresTheSizeButtons(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	cs.readOnly = true

	const px, py = 100, 200
	before := *cs.globals
	x, y, w, h := cs.worldSizeButtonRect(px, py, len(config.AllWorldSizes)-1)
	cs.clickWorldSize(px, py, x+w/2, y+h/2)

	if cs.globals.GridUnitsWide != before.GridUnitsWide ||
		cs.globals.InitialFood != before.InitialFood {
		t.Error("a read-only screen let a size button change the world")
	}
}
