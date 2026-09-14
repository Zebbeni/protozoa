package manager

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

func globalsWithFalloff(t *testing.T, k float64) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	g.ChemoPhFalloff = k
	config.SetGlobals(&g)
}

// TestChemoFalloffDisabledIsFlat: k = 0 must reproduce the original flat
// window exactly. The naive formula would give (d/w)^0 = 1 and zero out
// every chemosynthesizer, so this pins the special case.
func TestChemoFalloffDisabledIsFlat(t *testing.T) {
	globalsWithFalloff(t, 0)
	for _, d := range []float64{0, 0.5, 1.9, 2.24} {
		if got := chemoPhEfficiency(d, 2.25); got != 1 {
			t.Errorf("falloff disabled: efficiency at distance %.2f = %v, want 1", d, got)
		}
	}
}

// TestChemoFalloffShape: full efficiency at ideal, none at the window
// edge, never rising with distance, and never outside [0, 1].
func TestChemoFalloffShape(t *testing.T) {
	const window = 2.25
	for _, k := range []float64{0.5, 1, 2} {
		globalsWithFalloff(t, k)
		if got := chemoPhEfficiency(0, window); got != 1 {
			t.Errorf("k=%v: efficiency at ideal = %v, want 1", k, got)
		}
		if got := chemoPhEfficiency(window, window); got != 0 {
			t.Errorf("k=%v: efficiency at window edge = %v, want 0", k, got)
		}
		if got := chemoPhEfficiency(window*1.5, window); got != 0 {
			t.Errorf("k=%v: efficiency past the edge = %v, want clamped to 0", k, got)
		}
		prev := 1.0
		for i := 1; i <= 20; i++ {
			got := chemoPhEfficiency(window*float64(i)/20, window)
			if got > prev+1e-12 {
				t.Errorf("k=%v: efficiency rose with distance at step %d: %v -> %v", k, i, prev, got)
			}
			prev = got
		}
	}

	// Linear falloff is exactly half at half the window.
	globalsWithFalloff(t, 1)
	if got := chemoPhEfficiency(window/2, window); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("k=1: efficiency at half window = %v, want 0.5", got)
	}
}
