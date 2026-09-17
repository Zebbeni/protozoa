package ux

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/physiology"
)

func loadDefaults(t *testing.T) {
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

// TestPhToleranceFraction: the TOLERANCE view is green where the water
// costs an organism nothing, half way when it is costing
// phToleranceMidpointDamage health per cycle, and redder from there.
func TestPhToleranceFraction(t *testing.T) {
	loadDefaults(t)

	if got := phToleranceFraction(0); got != 1 {
		t.Errorf("water at the ideal pH scores %v, want 1", got)
	}
	if got := phToleranceFraction(-phToleranceMidpointDamage); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("losing %v health per cycle scores %v, want the midpoint 0.5",
			phToleranceMidpointDamage, got)
	}
	// Sign doesn't matter: the simulation states pH damage as a negative
	// health change and the view asks only how fast it costs.
	if a, b := phToleranceFraction(0.004), phToleranceFraction(-0.004); a != b {
		t.Errorf("sign changed the colour: %v vs %v", a, b)
	}

	// Redder all the way down, and never quite red: an organism being
	// killed has to look worse than one that is merely uncomfortable.
	prev := 1.0
	for _, damage := range []float64{0.001, 0.005, 0.01, 0.02, 0.1, 1} {
		got := phToleranceFraction(damage)
		if !(got < prev) {
			t.Errorf("losing %v scores %v, not below the %v above it", damage, got, prev)
		}
		if got <= 0 {
			t.Errorf("losing %v scores %v; the ramp should approach 0 without reaching it", damage, got)
		}
		prev = got
	}

	// A bigger organism pays more for the same water, so it reads redder.
	g := config.GetCurrentGlobals()
	const tolerance, distance = 3, 2.0
	small := effects.PhDamage(g, tolerance, 1, distance)
	big := effects.PhDamage(g, tolerance, 20, distance)
	if !(phToleranceFraction(big) < phToleranceFraction(small)) {
		t.Errorf("size 20 scores %v and size 1 scores %v; the bigger one should read redder",
			phToleranceFraction(big), phToleranceFraction(small))
	}

	// A tolerant organism in the same water reads greener than one
	// without, since it is taking less damage. Tolerance has to be worth
	// something for the view to distinguish them at all, so this asks for
	// a width the shipped settings may not have.
	wide := *g
	wide.MaxPhToleranceWidth = 4
	tolerant := effects.PhDamage(&wide, physiology.MaxAbilityScore, 1, 2)
	fragile := effects.PhDamage(&wide, 0, 1, 2)
	if !(phToleranceFraction(tolerant) > phToleranceFraction(fragile)) {
		t.Errorf("tolerant %v should read greener than fragile %v",
			phToleranceFraction(tolerant), phToleranceFraction(fragile))
	}
}
