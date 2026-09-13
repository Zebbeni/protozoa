package ux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// flowColor reads the active theme, which lives in the process-wide
// config. Production wires that up from an embedded FS in main's init;
// tests can't reach it, so decode settings/default.json from disk.
func loadGlobalsForTest(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	config.SetGlobals(&g)
}

// TestFlowColorAlphaRamp pins the opacity ramp. The floor matters:
// scaling alpha straight from magnitude would fade the slowest
// currents to invisible, hiding exactly the moments worth watching —
// a current first forming, or one nearly decayed away.
func TestFlowColorAlphaRamp(t *testing.T) {
	loadGlobalsForTest(t)

	alphaAt := func(mag float64) uint8 {
		_, _, _, a := flowColor(mag).RGBA()
		return uint8(a >> 8)
	}

	if got := alphaAt(0); got != flowMinAlpha {
		t.Errorf("a standstill should still be visible: alpha %d, want %d", got, flowMinAlpha)
	}
	if got := alphaAt(1); got != 255 {
		t.Errorf("a full current should be opaque: alpha %d, want 255", got)
	}
	prev := alphaAt(0)
	for _, mag := range []float64{0.25, 0.5, 0.75, 1} {
		got := alphaAt(mag)
		if got <= prev {
			t.Errorf("alpha should rise with magnitude: %d -> %d at mag %g", prev, got, mag)
		}
		prev = got
	}
}
