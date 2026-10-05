package ux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	c "github.com/Zebbeni/protozoa/config"
)

// writeSettingsFile puts a settings file on disk with one value changed, so
// a load can be told apart from the defaults.
func writeSettingsFile(t *testing.T, dir, name string, lifespan int) {
	t.Helper()
	var g c.Globals
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	g.MaxLifespan = lifespan
	g.ScreenWidth = 1
	g.ScreenHeight = 1
	g.Theme = "light"
	out, _ := json.MarshalIndent(&g, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, name), out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadSettingsReplacesTheEditedConfig is the feature: a file picked from
// the browser becomes the settings the New Simulation screen is editing.
func TestLoadSettingsReplacesTheEditedConfig(t *testing.T) {
	loadKeyGlobals(t)
	dir := t.TempDir()
	writeSettingsFile(t, dir, "tuned.json", 4321)

	cs, _ := abilityConfigScreen(t)
	before := *cs.globals
	cs.loadFiles = c.SavedSettings(dir)
	if len(cs.loadFiles) != 1 {
		t.Fatalf("expected 1 settings file, found %d", len(cs.loadFiles))
	}
	cs.loadSettingsFile(cs.loadFiles[0])

	if cs.globals.MaxLifespan != 4321 {
		t.Errorf("max_lifespan is %d, want the loaded 4321", cs.globals.MaxLifespan)
	}
	if cs.loadOpen {
		t.Error("the browser stayed open after a successful load")
	}
	// The three that describe the machine, not the simulation, are kept:
	// taking them from a file would resize the window under the user.
	if cs.globals.ScreenWidth != before.ScreenWidth || cs.globals.ScreenHeight != before.ScreenHeight {
		t.Errorf("window size changed to %dx%d, want %dx%d kept",
			cs.globals.ScreenWidth, cs.globals.ScreenHeight, before.ScreenWidth, before.ScreenHeight)
	}
	if cs.globals.Theme != before.Theme {
		t.Errorf("theme changed to %q, want %q kept", cs.globals.Theme, before.Theme)
	}
}

// TestLoadSettingsRepairsWhatTheFileOmits: a file written before a setting
// existed decodes it to zero, and the screen would then show and start a
// simulation with a value the repair pass exists to prevent.
func TestLoadSettingsRepairsWhatTheFileOmits(t *testing.T) {
	loadKeyGlobals(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "old.json"), []byte(`{"max_lifespan": 99}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cs, _ := abilityConfigScreen(t)
	cs.loadFiles = c.SavedSettings(dir)
	cs.loadSettingsFile(cs.loadFiles[0])

	if cs.globals.MaxLifespan != 99 {
		t.Errorf("max_lifespan is %d, want 99", cs.globals.MaxLifespan)
	}
	if cs.globals.MaxChemosynthesisPhWidth == 0 {
		t.Error("the chemosynthesis pH band was left at 0, so nothing could feed anywhere")
	}
	if cs.globals.MutationWeightSwapAction+cs.globals.MutationWeightGrowBranch == 0 {
		t.Error("the action mutation weights were left at zero, which divides by zero")
	}
}

// TestLoadSettingsRejectsAJunkFile: the browser lists anything ending .json,
// so a file that is not a config has to be reported rather than wiping the
// settings being edited.
func TestLoadSettingsRejectsAJunkFile(t *testing.T) {
	loadKeyGlobals(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "junk.json"), []byte("not json at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs, _ := abilityConfigScreen(t)
	before := cs.globals.MaxLifespan
	cs.loadFiles = c.SavedSettings(dir)
	cs.loadOpen = true
	cs.loadSettingsFile(cs.loadFiles[0])

	if cs.loadErr == "" {
		t.Error("loading a junk file reported nothing")
	}
	if !cs.loadOpen {
		t.Error("the browser closed on a failed load, hiding the error")
	}
	if cs.globals.MaxLifespan != before {
		t.Error("a failed load changed the settings being edited")
	}
}

// TestSavedSettingsListsNewestFirst: the one just saved is the one most
// likely to be wanted.
func TestSavedSettingsListsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	writeSettingsFile(t, dir, "older.json", 1)
	writeSettingsFile(t, dir, "newer.json", 2)
	now := c.SavedSettings(dir)
	if len(now) != 2 {
		t.Fatalf("found %d files, want 2", len(now))
	}
	if !now[0].ModTime.After(now[1].ModTime) && now[0].Name != "newer" {
		t.Errorf("listed %q first; want the newest", now[0].Name)
	}
	// Non-json is left out.
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)
	if len(c.SavedSettings(dir)) != 2 {
		t.Error("a non-json file was listed")
	}
}
