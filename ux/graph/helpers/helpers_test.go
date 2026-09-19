package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

func loadGlobals(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	config.SetGlobals(&g)
}

// TestAbilityColorAnchors pins the ability colour scale shared by the
// grid, its key and the population graph: gray at zero, full green from
// AbilityFullGreenScore up, and still separating the scores below it.
//
// The ramp stops short of the 10-point cap on purpose — the top of the
// range is thinly populated, and running all the way there spent a
// quarter of the colour on scores almost nothing has.
func TestAbilityColorAnchors(t *testing.T) {
	loadGlobals(t)

	for _, a := range physiology.AllAbilities {
		var s physiology.Scores

		s[a] = 0
		if got, want := AbilityColor(s, a), GrayGreenColor(0); got != want {
			t.Errorf("%s at 0: got %v, want gray %v", a.Name(), got, want)
		}

		// Full green from the saturation point up, including the cap.
		for _, score := range []int{8, physiology.MaxAbilityScore} {
			s[a] = score
			if got, want := AbilityColor(s, a), GrayGreenColor(1); got != want {
				t.Errorf("%s at %d: got %v, want green %v", a.Name(), score, got, want)
			}
		}

		// And it still separates the scores below the saturation point:
		// the specialist score is where most organisms of interest sit,
		// and it must not already be maxed out.
		s[a] = physiology.SpecialistScore
		if got := AbilityColor(s, a); got == GrayGreenColor(1) {
			t.Errorf("%s at the specialist score is already full green, so nothing above it reads differently", a.Name())
		}
		// Score 7 is the last one below the saturation point, so it is
		// the tightest case for that.
		s[a] = 7
		if got := AbilityColor(s, a); got == GrayGreenColor(1) {
			t.Errorf("%s at 7 is already full green; the ramp saturates at %g", a.Name(), AbilityFullGreenScore)
		}
	}
}

// TestGrayGreenLowEndIsNeutral: a zero score must be a true gray, not a
// dim green or a leftover red. Equal channels is the check — any hue
// cast would reintroduce the "warning" reading the gray end replaced.
func TestGrayGreenLowEndIsNeutral(t *testing.T) {
	g := GrayGreenColor(0)
	const eps = 1e-6
	if d := g.R - g.G; d > eps || d < -eps {
		t.Errorf("zero end not gray: %+v", g)
	}
	if d := g.G - g.B; d > eps || d < -eps {
		t.Errorf("zero end not gray: %+v", g)
	}
}

// TestGrayGreenIsEasyToTellApart guards the reason the ramp changes
// brightness at all. Holding lightness fixed and varying only saturation
// made neighbouring scores nearly indistinguishable, so this checks that
// perceived lightness (CIE L*) rises steadily across the whole ramp and
// that the two ends are far apart.
func TestGrayGreenIsEasyToTellApart(t *testing.T) {
	const steps = 10
	prevL, _, _ := GrayGreenColor(0).Lab()
	lowL := prevL
	for i := 1; i <= steps; i++ {
		l, _, _ := GrayGreenColor(float64(i) / steps).Lab()
		if l <= prevL {
			t.Errorf("lightness should rise along the ramp: step %d went %.3f -> %.3f", i, prevL, l)
		}
		prevL = l
	}
	if spread := prevL - lowL; spread < 0.4 {
		t.Errorf("ends too close in lightness to tell apart: L* %.3f -> %.3f (spread %.3f)", lowL, prevL, spread)
	}
}

// TestPeakFractionScalesToTheRunSoFar: a graph drawn against the whole
// run's peak is cropped to the height the data has actually reached, so
// early cycles of a run that grows a hundredfold aren't a flat line.
func TestPeakFractionScalesToTheRunSoFar(t *testing.T) {
	if got := PeakFraction(3000, 3000); got != 1 {
		t.Errorf("at the run's peak the whole height is in use, got %v", got)
	}
	small, large := PeakFraction(30, 3000), PeakFraction(1500, 3000)
	if !(small > 0 && small < large && large < 1) {
		t.Errorf("fractions should rise with the peak so far: 30 → %v, 1500 → %v", small, large)
	}
	if got := PeakFraction(30, 0); got != 1 {
		t.Errorf("with no recorded peak the whole height is in use, got %v", got)
	}
	if got := PeakFraction(5000, 3000); got != 1 {
		t.Errorf("a peak past the ceiling clamps to the full height, got %v", got)
	}
}
