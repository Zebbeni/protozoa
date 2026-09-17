package ux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	c "github.com/Zebbeni/protozoa/config"
)

func buttonFor(t *testing.T, label string) replayMenuButton {
	t.Helper()
	for _, b := range replayMenuButtons {
		if b.label == label {
			return b
		}
	}
	t.Fatalf("no replay menu button %q", label)
	return replayMenuButton{}
}

func TestReplayMenuChoicesCloseAndReachTheRunner(t *testing.T) {
	for label, want := range map[string]ReplayMenuChoice{
		"Run Again (New Seed)": ReplayMenuRunAgain,
		"Edit Settings":        ReplayMenuEditSettings,
		"Main Menu":            ReplayMenuMainMenu,
	} {
		m := NewReplayMenu(c.Globals{})
		m.Open()
		m.activate(buttonFor(t, label))
		if m.IsOpen() {
			t.Errorf("%s left the menu open", label)
		}
		if got := m.Take(); got != want {
			t.Errorf("%s gave choice %d, want %d", label, got, want)
		}
		if m.Take() != ReplayMenuNone {
			t.Errorf("%s choice was delivered twice", label)
		}
	}
}

func TestViewSettingsIsReadOnlyCopy(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	g := *cs.globals
	m := NewReplayMenu(g)
	m.Open()
	m.activate(buttonFor(t, "View Settings"))
	if m.settings == nil || !m.settings.readOnly || !m.IsOpen() {
		t.Fatal("View Settings should open a read-only settings viewer")
	}
	if m.Take() != ReplayMenuNone {
		t.Error("viewing settings shouldn't leave the replay")
	}
	m.settings.globals.MaxLifespan++
	m.settings.globals.InitialAbilityScores[0]++
	if m.globals.MaxLifespan != g.MaxLifespan || m.globals.InitialAbilityScores[0] != g.InitialAbilityScores[0] {
		t.Error("the settings viewer shares state with the replay's settings")
	}
	m.Close()
	if m.IsOpen() {
		t.Error("Close left the settings viewer open")
	}
}

// TestExportSettingsWritesLoadableConfig: Export saves the replay's
// settings as a config file -config can load, into the settings folder,
// without overwriting an earlier export.
func TestExportSettingsWritesLoadableConfig(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	g := *cs.globals
	g.Seed = 4242
	g.MaxLifespan = 777

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "settings"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	m := NewReplayMenu(g)
	m.exportSettings()
	m.exportSettings()
	if m.noticeErr {
		t.Fatalf("export failed: %s", m.notice)
	}
	for _, name := range []string{"settings_seed_4242.json", "settings_seed_4242-2.json"} {
		data, err := os.ReadFile(filepath.Join(dir, "settings", name))
		if err != nil {
			t.Fatalf("expected export %s: %v", name, err)
		}
		var loaded c.Globals
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("%s isn't a config file: %v", name, err)
		}
		if loaded.Seed != 4242 || loaded.MaxLifespan != 777 {
			t.Errorf("%s loads seed %d / max_lifespan %d, want 4242 / 777", name, loaded.Seed, loaded.MaxLifespan)
		}
	}
}
