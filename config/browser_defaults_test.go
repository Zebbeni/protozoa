package config

import (
	"encoding/json"
	"testing"
)

// TestBrowserDefaultsShrinkTheWorldAndKeepItsDensity runs on EVERY platform,
// which is the point of BrowserDefaults living in an untagged file: what a
// browser visitor gets used to be checkable only from a wasm build.
func TestBrowserDefaultsShrinkTheWorldAndKeepItsDensity(t *testing.T) {
	before := shippedDefaults(t)
	g := before
	BrowserDefaults(&g)

	cells := func(x Globals) int { return x.GridUnitsWide * x.GridUnitsHigh }
	if cells(g) >= cells(before) {
		t.Errorf("the browser world is %d cells against the desktop's %d", cells(g), cells(before))
	}
	if g.GridUnitsWide != wasmGridWide || g.GridUnitsHigh != wasmGridHigh {
		t.Errorf("grid is %dx%d, want %dx%d", g.GridUnitsWide, g.GridUnitsHigh, wasmGridWide, wasmGridHigh)
	}

	// Item COUNTS, not densities: they have to come down with the area, or
	// the smaller world starts several times denser than the numbers were
	// chosen for. Allowed a unit of rounding either way.
	share := float64(cells(g)) / float64(cells(before))
	for _, c := range []struct {
		name          string
		before, after int
	}{
		{"initial_food", before.InitialFood, g.InitialFood},
		{"initial_buried_food", before.InitialBuriedFood, g.InitialBuriedFood},
		{"initial_walls", before.InitialWalls, g.InitialWalls},
	} {
		want := int(float64(c.before) * share)
		if c.after != want {
			t.Errorf("%s is %d in the browser world, want %d (%d scaled by %.3f)",
				c.name, c.after, want, c.before, share)
		}
		if c.before > 0 && c.after >= c.before {
			t.Errorf("%s did not come down with the area: %d -> %d", c.name, c.before, c.after)
		}
	}
	// No browser-only balance change any more: the chemo push it used to
	// raise is the baseline's, measured at nine tenths of the population when
	// it was not.
	if g.ChemoPhEffect != before.ChemoPhEffect {
		t.Errorf("chemo_ph_effect is %v in the browser world, want the baseline's %v",
			g.ChemoPhEffect, before.ChemoPhEffect)
	}
}

// TestBrowserDefaultsChangeNothingElse: every other setting is the desktop
// baseline, so a browser-only balance change has to be added here deliberately
// rather than drifting in.
func TestBrowserDefaultsChangeNothingElse(t *testing.T) {
	before := shippedDefaults(t)
	g := before
	BrowserDefaults(&g)

	// Put back exactly what BrowserDefaults is allowed to touch — the grid
	// and the per-world counts, nothing else. Anything left over is a
	// browser-only balance change that drifted in rather than being decided.
	g.GridUnitsWide, g.GridUnitsHigh = before.GridUnitsWide, before.GridUnitsHigh
	g.InitialFood, g.InitialBuriedFood, g.InitialWalls = before.InitialFood, before.InitialBuriedFood, before.InitialWalls

	a, _ := json.Marshal(before)
	b, _ := json.Marshal(g)
	if string(a) != string(b) {
		t.Error("BrowserDefaults changed a setting beyond the grid, the per-world counts and the chemo push")
	}
}
