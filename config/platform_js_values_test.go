//go:build js

package config

import "testing"

// These run only in a js build: GOOS=js GOARCH=wasm go test ./config/
func TestBrowserDefaultsShrinkTheWorldAndScaleItsCounts(t *testing.T) {
	g := shippedDefaults(t)
	before := g
	platformDefaults(&g)

	if g.GridUnitsWide != wasmGridWide || g.GridUnitsHigh != wasmGridHigh {
		t.Errorf("grid is %dx%d, want %dx%d",
			g.GridUnitsWide, g.GridUnitsHigh, wasmGridWide, wasmGridHigh)
	}
	if g.GridUnitsWide*g.GridUnitsHigh >= before.GridUnitsWide*before.GridUnitsHigh {
		t.Error("the browser world is not smaller than the desktop one")
	}
	// Item counts, not densities: they have to come down with the area or
	// the smaller world starts several times denser.
	if g.InitialFood >= before.InitialFood {
		t.Errorf("initial_food stayed at %d in a world %d times the area",
			g.InitialFood, before.GridUnitsWide*before.GridUnitsHigh)
	}
	if g.ChemoPhEffect != wasmChemoPhEffect {
		t.Errorf("chemo_ph_effect is %v, want %v", g.ChemoPhEffect, wasmChemoPhEffect)
	}
}
