package ux

import (
	"encoding/json"
	"fmt"
	d "github.com/Zebbeni/protozoa/decision"
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

const testKeyWidth = minimapMaxW

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

// TestTrueColorShowsNoKey: True's colours are a lineage's inherited identity, not a measurement.
func TestTrueColorShowsNoKey(t *testing.T) {
	loadKeyGlobals(t)

	if k := colorKeyFor(orgColorTrue, physiology.AbilityDigging, d.ActEat); !k.empty() {
		t.Errorf("True should show no key, got %q", k.title)
	}
}

func TestEveryColorModeHasAKey(t *testing.T) {
	loadKeyGlobals(t)

	for _, m := range allOrgColorModes {
		if m == orgColorTrue {
			continue
		}
		k := colorKeyFor(m, physiology.AbilityChemosynthesis, d.ActEat)
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

// TestColorKeyTextFitsItsPlate measures every key's wrapped note against the width it is drawn in.
func TestColorKeyTextFitsItsPlate(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	maxW := keyInnerWidth(testKeyWidth)
	for _, m := range allOrgColorModes {
		k := colorKeyFor(m, physiology.AbilityChemosynthesis, d.ActEat)
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
			// The three axis labels share one line.
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

func TestColorKeyTextIsFlushLeft(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	for _, m := range allOrgColorModes {
		k := colorKeyFor(m, physiology.AbilityDigging, d.ActEat)
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

func TestColorKeyExplanationsReadAsSentences(t *testing.T) {
	loadKeyGlobals(t)

	for _, m := range allOrgColorModes {
		k := colorKeyFor(m, physiology.AbilityDigging, d.ActEat)
		if k.empty() {
			continue
		}
		first := []rune(k.note)[0]
		if !unicode.IsUpper(first) {
			t.Errorf("%s: explanation %q does not start with a capital", k.title, k.note)
		}
	}
}

func TestAbilityKeyCoversTheWholeRange(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorAbility, physiology.AbilityDigging, d.ActEat)
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
		// It goes on a plate the width of the minimap, so it has to wrap into something that still fits.
		k := colorKeyFor(orgColorAbility, a, d.ActEat)
		for _, line := range k.noteLines(testKeyWidth) {
			if w := boundString(r.FontSourceCodePro8, line).Dx(); w > keyInnerWidth(testKeyWidth) {
				t.Errorf("%s: note line %q is %dpx wide, plate holds %d",
					a.Name(), line, w, keyInnerWidth(testKeyWidth))
			}
		}
		// The ability being coloured has to be the one named and the one described.
		if k.note != note {
			t.Errorf("%s: key note is %q, want its description", a.Name(), k.note)
		}
		if !strings.Contains(k.title, a.Name()) {
			t.Errorf("%s: key is titled %q and does not name the ability", a.Name(), k.title)
		}
		// The score range is on the axis (0 ..
		if !strings.Contains(k.bars[0].subtitle, fmt.Sprintf("%g", gh.AbilityFullGreenScore)) {
			t.Errorf("%s: subtitle %q does not say where the colour tops out", a.Name(), k.bars[0].subtitle)
		}
	}
}

func TestColorKeyHeightCoversWhatItDraws(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	for _, m := range allOrgColorModes {
		k := colorKeyFor(m, physiology.AbilityChemosynthesis, d.ActEat)
		if k.empty() {
			continue
		}

		// Mirror drawColorKey's cursor: it starts at pad + one line and advances by the same steps.
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

func TestToleranceKeyMidpointIsTheDocumentedOne(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorTolerance, physiology.AbilityChemosynthesis, d.ActEat)
	if len(k.bars) == 0 {
		t.Fatal("the tolerance key should paint a scale")
	}
	want := phToleranceColor(phToleranceMidpointDamage)
	if got := k.bars[0].ramp(0.5); !sameColor(got, want) {
		t.Errorf("mid of the bar is %v, an organism at the midpoint damage is %v", got, want)
	}

	if free := k.bars[0].ramp(0); !sameColor(free, phToleranceColor(0)) {
		t.Errorf("left end is %v, an environment costing nothing is %v", free, phToleranceColor(0))
	}
}

func TestToleranceAxisNamesTheDamageItShows(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorTolerance, physiology.AbilityChemosynthesis, d.ActEat)
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

	// The right edge is past the top the label names.
	top := phToleranceColor(phToleranceKeyTop)
	atTop := gh.GreenRedColor(phToleranceFraction(phToleranceKeyTop))
	if !sameColor(top, atTop) {
		t.Fatalf("tolerance colour is not the fraction ramp any more")
	}
	if phToleranceFraction(phToleranceKeyTop) >= phToleranceFraction(phToleranceMidpointDamage) {
		t.Error("the bar's top should sit further into the red than its midpoint")
	}
}

func TestPhEffectKeyShowsWhatTheGridPaints(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorPhEffect, physiology.AbilityChemosynthesis, d.ActEat)
	// A lifetime that only ever pushed pH one way, past the ratio the spectrum saturates at.
	acid := phEffectColor(0, 1)
	base := phEffectColor(1, 0)

	if got := k.bars[0].ramp(0); !sameColor(got, acid) {
		t.Errorf("left end is %v, a fully acid organism is %v", got, acid)
	}
	if got := k.bars[0].ramp(1); !sameColor(got, base) {
		t.Errorf("right end is %v, a fully base organism is %v", got, base)
	}
}

func TestFamilyKeyShowsBothRamps(t *testing.T) {
	loadKeyGlobals(t)

	k := colorKeyFor(orgColorFamily, physiology.AbilityDigging, d.ActEat)
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

	// Every sample of the cousin bar must be a genuine cousin, not a point on the direct line the first bar already covers.
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

// sameColor compares within a tolerance well under one 8-bit step.
func sameColor(a, b colorful.Color) bool {
	const eps = 1.0 / 512
	return math.Abs(a.R-b.R) < eps && math.Abs(a.G-b.G) < eps && math.Abs(a.B-b.B) < eps
}
