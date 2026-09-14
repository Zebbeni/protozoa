package ux

import "testing"

// TestSliderRangesCentreOnDefaults: every scaled slider puts its default
// exactly in the middle, for signed health changes and plain quantities.
func TestSliderRangesCentreOnDefaults(t *testing.T) {
	for _, tc := range []struct {
		tag     string
		initial float64
	}{
		{"health_change_inflicted_by_attack", -100},
		{"health_change_from_chemosynthesis", 0.01},
		{"chemosynthesis_ph_effect_per_size", 0.01},
		{"grid_units_wide", 100},
	} {
		lo, hi := centeredSliderRange(tc.tag, tc.initial)
		if mid := (lo + hi) / 2; !approx(mid, tc.initial) {
			t.Errorf("%s: range [%g, %g] centres on %g, want the default %g", tc.tag, lo, hi, mid, tc.initial)
		}
		if lo >= hi {
			t.Errorf("%s: empty range [%g, %g]", tc.tag, lo, hi)
		}
	}
}

// TestNonNegativeSlidersStayNonNegative: quantities other than health
// changes never offer a negative value.
func TestNonNegativeSlidersStayNonNegative(t *testing.T) {
	for _, initial := range []float64{0, 0.01, 5, 50000} {
		if lo, _ := centeredSliderRange("max_organisms", initial); lo < 0 {
			t.Errorf("default %g: slider starts at %g, below zero", initial, lo)
		}
	}
}

// TestNonPositiveSlidersStopAtZero: damage-only fields never offer a
// positive value, and still centre on their default.
func TestNonPositiveSlidersStopAtZero(t *testing.T) {
	lo, hi := centeredSliderRange("health_change_per_unhealthy_ph", -0.5)
	if hi != 0 {
		t.Errorf("unhealthy pH slider max = %g, want 0", hi)
	}
	if !approx((lo+hi)/2, -0.5) {
		t.Errorf("unhealthy pH slider [%g, %g] doesn't centre on -0.5", lo, hi)
	}
	if _, hi := centeredSliderRange("health_change_per_unhealthy_ph", 0); hi != 0 {
		t.Errorf("zero default should still cap at 0, got max %g", hi)
	}
}
