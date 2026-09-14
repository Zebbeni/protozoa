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
// grid and the population graph: gray at zero, green at the ability's
// specialist score, and no further change above it. Anchoring on the
// 100-point budget instead would leave nearly every organism near gray,
// since real scores cluster far below 100.
func TestAbilityColorAnchors(t *testing.T) {
	loadGlobals(t)

	for _, a := range physiology.AllAbilities {
		var s physiology.Scores

		s[a] = 0
		if got, want := AbilityColor(s, a), GrayGreenColor(0); got != want {
			t.Errorf("%s at 0: got %v, want gray %v", a.Name(), got, want)
		}

		s[a] = physiology.SpecialistScore(a)
		if got, want := AbilityColor(s, a), GrayGreenColor(1); got != want {
			t.Errorf("%s at specialist score %d: got %v, want green %v", a.Name(), s[a], got, want)
		}

		s[a] = physiology.PointTotal
		if got, want := AbilityColor(s, a), GrayGreenColor(1); got != want {
			t.Errorf("%s above specialist score should clamp to green: got %v", a.Name(), got)
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
