package physiology

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

func globalsWithChemoExponent(t *testing.T, exp float64) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	g.ChemoCurveExponent = exp
	config.SetGlobals(&g)
}

// TestChemoCurveExponentKeepsEndpoints: shaping the chemo curve must not
// move where it pivots (1.0 at genesis) or where it tops out (atMax a
// span above genesis) — otherwise changing the shape would silently
// rebaseline every genesis organism, which is the failure the genesis
// pivot exists to prevent.
func TestChemoCurveExponentKeepsEndpoints(t *testing.T) {
	for _, exp := range []float64{0.5, 1, 2} {
		globalsWithChemoExponent(t, exp)
		neutral := neutralScore(AbilityChemosynthesis)
		full := neutral + float64(config.AbilitySpecializationSpan())
		_, atMax := curveEndpoints(AbilityChemosynthesis)

		if got := multiplierFor(AbilityChemosynthesis, neutral); math.Abs(got-1) > 1e-9 {
			t.Errorf("exp=%v: multiplier at genesis = %v, want 1", exp, got)
		}
		if got := multiplierFor(AbilityChemosynthesis, full); math.Abs(got-atMax) > 1e-9 {
			t.Errorf("exp=%v: multiplier at specialist score = %v, want %v", exp, got, atMax)
		}
	}
}

// TestChemoCurveExponentGivesDiminishingReturns: below 1 the curve is
// concave, so the midpoint of the climb pays more than linear would, and
// the second half of the climb pays less than the first.
func TestChemoCurveExponentGivesDiminishingReturns(t *testing.T) {
	globalsWithChemoExponent(t, 0.5)
	neutral := neutralScore(AbilityChemosynthesis)
	span := float64(config.AbilitySpecializationSpan())
	_, atMax := curveEndpoints(AbilityChemosynthesis)

	mid := multiplierFor(AbilityChemosynthesis, neutral+span/2)
	linearMid := 1 + (atMax-1)*0.5
	if !(mid > linearMid) {
		t.Errorf("concave curve should beat linear at the midpoint: %v vs %v", mid, linearMid)
	}
	firstHalf := mid - 1
	secondHalf := atMax - mid
	if !(secondHalf < firstHalf) {
		t.Errorf("second half of the climb should pay less than the first: %v vs %v", secondHalf, firstHalf)
	}

	// Other abilities are unaffected by the chemo exponent.
	for _, a := range AllAbilities {
		if a == AbilityChemosynthesis {
			continue
		}
		n := neutralScore(a)
		_, m := curveEndpoints(a)
		want := 1 + (m-1)*0.5
		if got := multiplierFor(a, n+span/2); math.Abs(got-want) > 1e-9 {
			t.Errorf("%s changed with the chemo exponent: %v, want linear %v", a.Name(), got, want)
		}
	}
}
