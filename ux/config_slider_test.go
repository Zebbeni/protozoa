package ux

import (
	"reflect"
	"strconv"
	"testing"
)

func TestSliderRangesCentreOnDefaults(t *testing.T) {
	for _, tc := range []struct {
		tag     string
		initial float64
	}{
		{"health_change_from_something_unclassified", -100},
		{"health_change_from_something_unclassified", 0.01},
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

// TestNonNegativeSlidersStayNonNegative: quantities other than health changes never offer a negative value.
func TestNonNegativeSlidersStayNonNegative(t *testing.T) {
	for _, initial := range []float64{0, 0.01, 5, 50000} {
		if lo, _ := centeredSliderRange("max_organisms", initial); lo < 0 {
			t.Errorf("default %g: slider starts at %g, below zero", initial, lo)
		}
	}
}

func TestSignBoundSlidersCantCrossZero(t *testing.T) {
	for _, tag := range []string{"health_change_from_moving", "health_change_from_spawning"} {
		for _, initial := range []float64{-0.5, 0} {
			lo, hi := centeredSliderRange(tag, initial)
			if hi != 0 || lo >= 0 {
				t.Errorf("%s (default %g): range [%g, %g] should run up to zero from below", tag, initial, lo, hi)
			}
		}
	}
	for _, tag := range []string{"health_change_inflicted_by_attack", "health_change_inflicted_by_thorns", "unhealthy_ph_damage"} {
		for _, initial := range []float64{650, 0} {
			lo, hi := centeredSliderRange(tag, initial)
			if lo != 0 || hi <= 0 {
				t.Errorf("%s (default %g): range [%g, %g] should run up from zero", tag, initial, lo, hi)
			}
		}
	}

	cs, _ := abilityConfigScreen(t)
	f := findField(t, cs, "health_change_inflicted_by_attack")
	cs.selectedRow, cs.editingValue = cs.rowIndexOfTag(f.jsonTag), "-400"
	cs.commitEdit()
	if got := cs.globals.AttackDamageAtFullAttack; got != 400 {
		t.Errorf("typing -400 into a damage field gave %v, want 400 damage", got)
	}

	f = findField(t, cs, "health_change_from_moving")
	cs.selectedRow, cs.editingValue = cs.rowIndexOfTag(f.jsonTag), "0.5"
	cs.commitEdit()
	if got := cs.globals.HealthChangeFromMoving; got != -0.5 {
		t.Errorf("typing 0.5 into a cost field gave %v, want -0.5", got)
	}
}

func TestTreeSizeCappedAt32(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	f := findField(t, cs, "max_decision_tree_size")
	for _, start := range []int{16, 64} {
		cs.initValues.MaxDecisionTreeSize = start
		if lo, hi := cs.getSliderRange(f); lo != 1 || hi != 32 {
			t.Errorf("starting at %d: range [%g, %g], want [1, 32]", start, lo, hi)
		}
	}
	for typed, want := range map[string]int{"100": 32, "0": 1, "20": 20} {
		cs.selectedRow = rowIndexOf(t, cs, f)
		cs.editingValue = typed
		cs.commitEdit()
		if got := cs.globals.MaxDecisionTreeSize; got != want {
			t.Errorf("typed %s: tree size %d, want %d", typed, got, want)
		}
	}
}

func rowIndexOf(t *testing.T, cs *ConfigScreen, want configField) int {
	t.Helper()
	idx := 0
	for _, section := range cs.sections {
		for _, field := range section.fields {
			if field.jsonTag == want.jsonTag && field.row == want.row {
				return idx
			}
			idx++
		}
	}
	t.Fatalf("no row for %q", want.jsonTag)
	return -1
}

func TestEditedValuesStopAtFiveDecimals(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"0.15346258503401359", "0.15346"},
		{"0.30000000000000004", "0.3"},
		{"-0.03125", "-0.03125"},
		{"-650", "-650"},
		{"0", "0"},
		{"0.000001", "1e-06"}, // too small to show: an exponent, not a flat 0
	} {
		in, err := strconv.ParseFloat(tc.in, 64)
		if err != nil {
			t.Fatal(err)
		}
		if got := formatConfigValue(in); got != tc.want {
			t.Errorf("formatConfigValue(%s) = %q, want %q", tc.in, got, tc.want)
		}
	}

	cs, _ := abilityConfigScreen(t)
	field, ok := cs.fieldByTag("max_chemosynthesis_gain")
	if !ok {
		t.Fatal("no field to drag")
	}
	for _, ratio := range []float64{0.137, 0.611, 0.929} {
		cs.setValueFromSliderRatio(ratio, field)
		got := reflect.ValueOf(cs.globals).Elem().Field(field.fieldIdx).Float()
		if got != roundConfigValue(got) {
			t.Errorf("a drag to %v stored %v, which isn't what the screen shows (%s)",
				ratio, got, formatConfigValue(got))
		}
	}
}

func TestSizeRatioBoundsKeepTheirNamesTrue(t *testing.T) {
	big, ok := fixedBounds["much_bigger_size_ratio"]
	if !ok {
		t.Fatal("much_bigger_size_ratio has no fixed bounds, so its slider is derived from the default and can cross 1")
	}
	if big[0] < 1 {
		t.Errorf("much_bigger_size_ratio can go down to %g; below 1 it names a smaller neighbour", big[0])
	}
	small, ok := fixedBounds["much_smaller_size_ratio"]
	if !ok {
		t.Fatal("much_smaller_size_ratio has no fixed bounds")
	}
	if small[1] > 1 {
		t.Errorf("much_smaller_size_ratio can go up to %g; above 1 it names a larger neighbour", small[1])
	}
}
