package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestIdealPhRangeCentresOnTheScale: one setting replaces the two ends,
// so the evolvable band is always centred on the middle of the pH scale
// — an off-centre band would just bias every lineage the same way.
func TestIdealPhRangeCentresOnTheScale(t *testing.T) {
	loadDefaults(t)
	g := config.GetCurrentGlobals()
	middle := (config.MinPh() + config.MaxPh()) / 2

	for _, width := range []float64{0, 4, 9, config.MaxPh() - config.MinPh()} {
		g.IdealPhRange = width
		lo, hi := config.MinIdealPh(), config.MaxIdealPh()
		if got := hi - lo; got != width {
			t.Errorf("range %v spans %v", width, got)
		}
		if got := (lo + hi) / 2; got != middle {
			t.Errorf("range %v centres on %v, want the scale's middle %v", width, got, middle)
		}
	}

	// The example from the design: 9 gives 0.5 to 9.5.
	g.IdealPhRange = 9
	if lo, hi := config.MinIdealPh(), config.MaxIdealPh(); lo != 0.5 || hi != 9.5 {
		t.Errorf("a range of 9 gives %v-%v, want 0.5-9.5", lo, hi)
	}

	// Every cell, and every founder, starts at that middle.
	if config.InitialPh() != middle {
		t.Errorf("worlds start at pH %v, want the scale's middle %v", config.InitialPh(), middle)
	}
}
