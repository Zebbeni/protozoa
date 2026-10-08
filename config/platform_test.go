package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func shippedDefaults(t *testing.T) Globals {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// TestPlatformDefaultsLeaveTheDesktopBaselineAlone: settings/default.json is
// the desktop baseline, so the hook must be a no-op here. A second set of
// numbers that drifted from the file would be invisible.
func TestPlatformDefaultsLeaveTheDesktopBaselineAlone(t *testing.T) {
	want := shippedDefaults(t)
	got := want
	platformDefaults(&got)
	// Globals holds slices, so compare the encoded form rather than the
	// struct: a field added later is then covered without touching this.
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Error("platformDefaults changed the desktop baseline")
	}
}

// TestAConfigFileStillWinsOverThePlatformBaseline: the hook runs on the
// BASELINE, which LoadGlobals then overlays a file onto. A replay relies on
// the same ordering, since it takes its grid from the recording's header.
func TestAConfigFileStillWinsOverThePlatformBaseline(t *testing.T) {
	// What LoadGlobals does, without the embedded FS a test has no handle on:
	// platform defaults land on the baseline, the file is overlaid onto it.
	base := shippedDefaults(t)
	platformDefaults(&base)
	named := `{"grid_units_wide": 123, "grid_units_high": 45}`
	g := applyGlobalsFromJson(readerFromBytes([]byte(named)), base)
	if g.GridUnitsWide != 123 || g.GridUnitsHigh != 45 {
		t.Errorf("a settings file naming 123x45 produced %dx%d",
			g.GridUnitsWide, g.GridUnitsHigh)
	}
}
