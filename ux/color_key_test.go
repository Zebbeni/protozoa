package ux

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

// testKeyWidth is the width the key is measured at: the widest a minimap
// gets, which is the widest the key ever is. A narrower world gives a
// narrower minimap and therefore a narrower key, but text that fits the
// widest plate is the thing worth pinning — a narrower one just wraps to
// more lines, which height() accounts for.
const testKeyWidth = minimapMaxW

// loadKeyGlobals installs the shipped defaults, which the pH-effect ramp
// reads for its hues and the theme background.
func loadKeyGlobals(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	config.SetGlobals(&g)
}

// TestTrueColorShowsNoKey: True's colours are a lineage's inherited
// identity, not a measurement, so there is no scale to explain and the
// corner is left alone rather than filled with a plate saying so.
func TestTrueColorShowsNoKey(t *testing.T) {
	loadKeyGlobals(t)

	if k := colorKeyFor(orgColorTrue, physiology.AbilityDigging); !k.empty() {
		t.Errorf("True should show no key, got %q", k.title)
	}
}

// TestEveryColorModeHasAKey: a colour mode that measures something and
// says nothing about it is a gradient with no scale — "greener" with no
// answer to "than what". Every mode but True therefore needs a populated
// key, and this is also what catches a *new* mode added without one:
// colorKeyFor falls through to the empty key, which would ship silently
// showing nothing at all.
func TestEveryColorModeHasAKey(t *testing.T) {
	loadKeyGlobals(t)

	for _, m := range allOrgColorModes {
		if m == orgColorTrue {
			continue
		}
		k := colorKeyFor(m, physiology.AbilityChemosynthesis)
		if k.empty() {
			t.Errorf("colour mode %d has no key at all", m)
			continue
		}
		if k.note == "" {
			t.Errorf("colour mode %d (%s) says nothing about what it measures", m, k.title)
		}
		if len(k.bars) == 0 {
			t.Errorf("%s paints no scale at all", k.title)
		}
		for n, bar := range k.bars {
			if bar.ramp == nil {
				t.Errorf("%s: bar %d has no ramp", k.title, n)
			}
			if bar.subtitle == "" {
				t.Errorf("%s: bar %d has no subtitle saying what it measures", k.title, n)
			}
			for i, label := range bar.axis {
				if label == "" {
					t.Errorf("%s: bar %d axis label %d is blank", k.title, n, i)
				}
			}
		}
	}
}

// TestColorKeyTextFitsItsPlate measures every key's wrapped note against
// the width it is drawn in, and the end labels against each other. The
// notes are authored as prose in colorKeyFor with no width in sight, so
// this is what catches one that has grown past the box — which would
// otherwise run out over the grid, or in the labels' case collide in the
// middle.
func TestColorKeyTextFitsItsPlate(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	maxW := keyInnerWidth(testKeyWidth)
	for _, m := range allOrgColorModes {
		k := colorKeyFor(m, physiology.AbilityChemosynthesis)
		if k.empty() {
			continue
		}

		for _, line := range k.noteLines(testKeyWidth) {
			if w := boundString(r.FontSourceCodePro8, line).Dx(); w > maxW {
				t.Errorf("%s: note line %q is %dpx wide, plate holds %d", k.title, line, w, maxW)
			}
		}
		for _, line := range k.titleLines(testKeyWidth) {
			if w := boundString(r.FontSourceCodePro10, line).Dx(); w > maxW {
				t.Errorf("%s: title line %q is %dpx wide, plate holds %d", k.title, line, w, maxW)
			}
		}
		for _, bar := range k.bars {
			for _, line := range bar.subtitleLines(testKeyWidth) {
				if w := boundString(r.FontSourceCodePro8, line).Dx(); w > maxW {
					t.Errorf("%s: subtitle line %q is %dpx wide, plate holds %d", k.title, line, w, maxW)
				}
			}
			// The three axis labels share one line — flush left, centred,
			// flush right — so each needs clear air either side of the
			// centred one, not merely room for all three end to end.
			lw := boundString(r.FontSourceCodePro8, bar.axis[0]).Dx()
			mw := boundString(r.FontSourceCodePro8, bar.axis[1]).Dx()
			hw := boundString(r.FontSourceCodePro8, bar.axis[2]).Dx()
			gap := 4
			if lw+gap > (maxW-mw)/2 || hw+gap > (maxW-mw)/2 {
				t.Errorf("%s: axis labels %q %q %q collide on one line (%d %d %d in %d)",
					k.title, bar.axis[0], bar.axis[1], bar.axis[2], lw, mw, hw, maxW)
			}
		}
	}
}

// TestColorKeyTextIsFlushLeft: every wrapped line has to start at the
// left margin. The key first shared the rules screen's splitWords, which
// keeps a word's leading whitespace on purpose so indented bullets keep
// their indent — here that put a space on the front of each wrapped line
// and stepped the whole paragraph rightwards one line at a time.
func TestColorKeyTextIsFlushLeft(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	for _, m := range allOrgColorModes {
		k := colorKeyFor(m, physiology.AbilityDigging)
		var blocks [][]string
		blocks = append(blocks, k.titleLines(testKeyWidth), k.noteLines(testKeyWidth))
		for _, bar := range k.bars {
			blocks = append(blocks, bar.subtitleLines(testKeyWidth))
		}
		for _, lines := range blocks {
			for _, line := range lines {
				if line != strings.TrimSpace(line) {
					t.Errorf("%s: line %q is not flush left", k.title, line)
				}
			}
		}
	}
}

// TestColorKeyExplanationsReadAsSentences: the notes are prose under a
// title, so they start with a capital. Easy to lose when one is reworded.
func TestColorKeyExplanationsReadAsSentences(t *testing.T) {
	loadKeyGlobals(t)

	for _, m := range allOrgColorModes {
		k := colorKeyFor(m, physiology.AbilityDigging)
		if k.empty() {
			continue
		}
		first := []rune(k.note)[0]
		if !unicode.IsUpper(first) {
			t.Errorf("%s: explanation %q does not start with a capital", k.title, k.note)
		}
	}
}

// TestAbilityKeyCoversTheWholeRange: the axis claims the bar runs 0 to
// the cap, so the bar's ends have to be the colours the grid gives a
// score of 0 and a score at the cap. The two were out of step while the
// ramp topped out at the specialist score.
func TestAbilityKeyCoversTheWholeRange(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorAbility, physiology.AbilityDigging)
	var lo, hi physiology.Scores
	lo[physiology.AbilityDigging] = 0
	hi[physiology.AbilityDigging] = physiology.MaxAbilityScore

	if got := k.bars[0].ramp(0); !sameColor(got, gh.AbilityColor(lo, physiology.AbilityDigging)) {
		t.Errorf("bar's left end is %v, a score of 0 is %v", got, gh.AbilityColor(lo, physiology.AbilityDigging))
	}
	if got := k.bars[0].ramp(1); !sameColor(got, gh.AbilityColor(hi, physiology.AbilityDigging)) {
		t.Errorf("bar's right end is %v, a score at the cap is %v", got, gh.AbilityColor(hi, physiology.AbilityDigging))
	}
	if want := fmt.Sprintf("%d", physiology.MaxAbilityScore); k.bars[0].axis[2] != want {
		t.Errorf("axis tops out at %q, want the cap %q", k.bars[0].axis[2], want)
	}
}

// TestEveryAbilityHasAKeyNote: the Ability key explains what the ability
// being coloured actually does, so a new ability arriving without an
// entry would show "Organism foo ability, 0-10." and then stop — the half
// of the sentence that matters missing, with nothing else to say so.
func TestEveryAbilityHasAKeyNote(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	for _, a := range physiology.AllAbilities {
		note, ok := abilityKeyNotes[a]
		if !ok || note == "" {
			t.Errorf("%s has no key note saying what the ability does", a.Name())
			continue
		}
		if !unicode.IsUpper([]rune(note)[0]) {
			t.Errorf("%s: note %q does not start with a capital", a.Name(), note)
		}
		// It goes on a plate the width of the minimap, so it has to wrap
		// into something that still fits.
		k := colorKeyFor(orgColorAbility, a)
		for _, line := range k.noteLines(testKeyWidth) {
			if w := boundString(r.FontSourceCodePro8, line).Dx(); w > keyInnerWidth(testKeyWidth) {
				t.Errorf("%s: note line %q is %dpx wide, plate holds %d",
					a.Name(), line, w, keyInnerWidth(testKeyWidth))
			}
		}
		// The ability being coloured has to be the one named and the one
		// described.
		if k.note != note {
			t.Errorf("%s: key note is %q, want its description", a.Name(), k.note)
		}
		if !strings.Contains(k.title, a.Name()) {
			t.Errorf("%s: key is titled %q and does not name the ability", a.Name(), k.title)
		}
		// The score range is on the axis (0 .. cap); the subtitle
		// carries the one thing the axis has no slot for, the score
		// the ramp saturates at.
		if !strings.Contains(k.bars[0].subtitle, fmt.Sprintf("%g", gh.AbilityFullGreenScore)) {
			t.Errorf("%s: subtitle %q does not say where the colour tops out", a.Name(), k.bars[0].subtitle)
		}
	}
}

// TestColorKeyHeightCoversWhatItDraws pins the plate against its
// contents. height() is computed from the same pieces drawColorKey lays
// out, but separately — the caller needs the height before drawing, to
// stack the key above the minimap — so the two can drift, and the way it
// shows is the last line of a note printed below the plate it belongs to.
func TestColorKeyHeightCoversWhatItDraws(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	for _, m := range allOrgColorModes {
		k := colorKeyFor(m, physiology.AbilityChemosynthesis)
		if k.empty() {
			continue
		}

		// Mirror drawColorKey's cursor: it starts at pad + one line and
		// advances by the same steps.
		ty := colorKeyPad + len(k.titleLines(testKeyWidth))*colorKeyLineH - 2
		for _, bar := range k.bars {
			ty += len(bar.subtitleLines(testKeyWidth))*colorKeyLineH +
				colorKeyGap + colorKeyBarH + colorKeyLineH - 2
		}
		ty += len(k.noteLines(testKeyWidth)) * colorKeyLineH

		if got := k.height(testKeyWidth); got < ty+colorKeyPad {
			t.Errorf("%s: plate is %dpx tall but its last line sits at %d", k.title, got, ty)
		}
	}
}

// TestToleranceKeyMidpointIsTheDocumentedOne: the grid tints by damage,
// which has no top end, so the key walks the tolerance *fraction*
// backwards instead. That only tells the truth if the middle of the bar
// is the colour an organism losing phToleranceMidpointDamage per cycle
// actually gets — the number the key prints under it.
func TestToleranceKeyMidpointIsTheDocumentedOne(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorTolerance, physiology.AbilityChemosynthesis)
	if len(k.bars) == 0 {
		t.Fatal("the tolerance key should paint a scale")
	}
	want := phToleranceColor(phToleranceMidpointDamage)
	if got := k.bars[0].ramp(0.5); !sameColor(got, want) {
		t.Errorf("mid of the bar is %v, an organism at the midpoint damage is %v", got, want)
	}

	// And the ends are the right way round: no loss at the left.
	if free := k.bars[0].ramp(0); !sameColor(free, phToleranceColor(0)) {
		t.Errorf("left end is %v, an environment costing nothing is %v", free, phToleranceColor(0))
	}
}

// TestToleranceAxisNamesTheDamageItShows: the axis is the whole point of
// the rework — "0 / 0.01 / 0.1+" is a claim about health per cycle, and
// it has to be the health per cycle those points on the bar are actually
// the colour of. The labels are formatted from the constants, so this
// checks the bar agrees with them rather than that two literals match.
func TestToleranceAxisNamesTheDamageItShows(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorTolerance, physiology.AbilityChemosynthesis)
	for _, tc := range []struct {
		at     float64
		damage float64
		label  string
	}{
		{0, 0, "0"},
		{0.5, phToleranceMidpointDamage, fmt.Sprintf("%g", phToleranceMidpointDamage)},
	} {
		if got := k.bars[0].ramp(tc.at); !sameColor(got, phToleranceColor(tc.damage)) {
			t.Errorf("bar at %v is %v, but its label %q means %v",
				tc.at, got, tc.label, phToleranceColor(tc.damage))
		}
	}

	// The right edge is past the top the label names, which is what the
	// "+" is promising: everything from there on is this colour.
	top := phToleranceColor(phToleranceKeyTop)
	atTop := gh.GreenRedColor(phToleranceFraction(phToleranceKeyTop))
	if !sameColor(top, atTop) {
		t.Fatalf("tolerance colour is not the fraction ramp any more")
	}
	if phToleranceFraction(phToleranceKeyTop) >= phToleranceFraction(phToleranceMidpointDamage) {
		t.Error("the bar's top should sit further into the red than its midpoint")
	}
}

// TestPhEffectKeyShowsWhatTheGridPaints: the key samples the same ramp
// function the organisms are tinted with, so the ends of the bar have to
// be the colours a fully acid and fully base organism get. A key with its
// own copy of the hue maths would pass every other test here and still be
// wrong the first time the colour scheme moved.
func TestPhEffectKeyShowsWhatTheGridPaints(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorPhEffect, physiology.AbilityChemosynthesis)
	// A lifetime that only ever pushed pH one way, past the ratio the
	// spectrum saturates at.
	acid := phEffectColor(0, 1)
	base := phEffectColor(1, 0)

	if got := k.bars[0].ramp(0); !sameColor(got, acid) {
		t.Errorf("left end is %v, a fully acid organism is %v", got, acid)
	}
	if got := k.bars[0].ramp(1); !sameColor(got, base) {
		t.Errorf("right end is %v, a fully base organism is %v", got, base)
	}
}

// TestFamilyKeyShowsBothRamps: the Family key carries two bars because
// its colours fan out from the selection along more than one axis, and a
// single bar can only follow one. Each has to be the ramp the grid
// actually paints for that kind of relative — the descendant bar's ends
// against a direct descendant, the cousin bar's against a real cousin,
// which is the part that would silently go wrong if the bar were built
// from its own idea of the colour scheme.
func TestFamilyKeyShowsBothRamps(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorFamily, physiology.AbilityDigging)
	if len(k.bars) != 2 {
		t.Fatalf("the family key has %d bars, want 2 (descendants and cousins)", len(k.bars))
	}

	desc, cousins := k.bars[0], k.bars[1]
	if got := desc.ramp(0); !sameColor(got, familyColor(kinship{related: true})) {
		t.Errorf("the descendant bar starts at %v, the selection is %v", got, familyColor(kinship{related: true}))
	}
	if got := desc.ramp(1); !sameColor(got, familyColor(kinship{down: familyFadeGenerations, related: true})) {
		t.Errorf("the descendant bar ends at %v, %d generations down is %v",
			got, familyFadeGenerations, familyColor(kinship{down: familyFadeGenerations, related: true}))
	}

	// Every sample of the cousin bar must be a genuine cousin, not a
	// point on the direct line the first bar already covers — a cousin
	// has someone in both directions.
	if got := cousins.ramp(0); !sameColor(got, familyColor(kinship{up: 1, down: 1, related: true})) {
		t.Errorf("the cousin bar starts at %v, a sibling is %v", got, familyColor(kinship{up: 1, down: 1, related: true}))
	}
	if got := cousins.ramp(1); !sameColor(got, familyUnrelatedColor()) {
		t.Errorf("the cousin bar should end at the unrelated gray %v, got %v", familyUnrelatedColor(), got)
	}

	// And the two bars must not be the same scale drawn twice.
	if sameColor(desc.ramp(1), cousins.ramp(1)) {
		t.Error("the two family bars end at the same colour, so one of them says nothing")
	}
}

// sameColor compares within a tolerance well under one 8-bit step, so the
// tests read as "the same colour" rather than pinning float bits.
func sameColor(a, b colorful.Color) bool {
	const eps = 1.0 / 512
	return math.Abs(a.R-b.R) < eps && math.Abs(a.G-b.G) < eps && math.Abs(a.B-b.B) < eps
}
