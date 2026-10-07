package ux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font"

	"github.com/Zebbeni/protozoa/checkpoint"
	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
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
		m := NewReplayMenu(c.Globals{}, "")
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
	m := NewReplayMenu(g, "")
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

	m := NewReplayMenu(g, "")
	// Save As opens the name prompt rather than writing a file of its own
	// choosing, so the user picks what it is called.
	m.exportSettings()
	if !m.naming || m.namingWhat != namingSettings {
		t.Fatalf("Save As did not open the settings name prompt (naming %v, what %v)",
			m.naming, m.namingWhat)
	}
	m.name = "My Tuned Run"
	m.commitNaming()
	if m.noticeErr {
		t.Fatalf("save failed: %s", m.notice)
	}
	if m.naming {
		t.Error("the prompt stayed open after a successful save")
	}

	data, err := os.ReadFile(filepath.Join(dir, "settings", "my-tuned-run.json"))
	if err != nil {
		t.Fatalf("expected the typed name to be slugged to my-tuned-run.json: %v", err)
	}
	var loaded c.Globals
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("the saved file isn't a config file: %v", err)
	}
	if loaded.Seed != 4242 || loaded.MaxLifespan != 777 {
		t.Errorf("loads seed %d / max_lifespan %d, want 4242 / 777", loaded.Seed, loaded.MaxLifespan)
	}

	// Saving the same name again refuses rather than replacing a file
	// someone has tuned, and leaves the prompt up with the name intact.
	m.exportSettings()
	m.name = "My Tuned Run"
	m.commitNaming()
	if !m.noticeErr {
		t.Error("saving over an existing settings file was allowed")
	}
	if !m.naming || m.name != "My Tuned Run" {
		t.Error("after a collision the prompt should stay open with the typed name")
	}
}

func TestSavePromptTextFitsItsBox(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	inner := nameBoxW - 2*replayMenuPad
	lines := []struct {
		text string
		face font.Face
	}{
		{"SAVE RECORDING", r.FontSourceCodePro12},
		{"Saved into " + checkpoint.RecordingsDir + "/ — the next run overwrites the original.", r.FontSourceCodePro8},
		// The widest name the field accepts, plus the caret it grows while typing.
		{strings.Repeat("W", nameMaxRunes) + "_", r.FontSourceCodePro12},
		{"saves as " + checkpoint.RecordingFileName(strings.Repeat("W", nameMaxRunes)), r.FontSourceCodePro8},
	}
	for _, l := range lines {
		if w := boundString(l.face, l.text).Dx(); w > inner {
			t.Errorf("%q is %dpx, the box holds %d", l.text, w, inner)
		}
	}

	// The two footer buttons share one row and must not overlap.
	if nameCancelRect().Max.X >= nameSaveRect().Min.X {
		t.Errorf("Cancel ends at %d, Save starts at %d", nameCancelRect().Max.X, nameSaveRect().Min.X)
	}
	box := nameBoxRect()
	if nameCancelRect().Min.X < box.Min.X || nameSaveRect().Max.X > box.Max.X {
		t.Error("a footer button hangs outside the prompt")
	}
	if nameFieldRect().Max.Y >= nameSaveRect().Min.Y {
		t.Error("the name field overlaps the buttons")
	}
}

func TestSavePromptPrefillsAndSlugs(t *testing.T) {
	g := c.Globals{Seed: 4242}
	m := NewReplayMenu(g, "somewhere/protozoa_last.pzr")
	m.Open()
	m.openNaming(namingRecording)

	if !m.IsOpen() {
		t.Error("the prompt doesn't count as open, so the viewer would take clicks behind it")
	}
	if m.name != "seed-4242" {
		t.Errorf("prefilled %q, want the seed so Enter alone works", m.name)
	}
	if got := checkpoint.RecordingFileName(m.name); got != "seed-4242.pzr" {
		t.Errorf("the prefill saves as %q", got)
	}

	// Escape backs all the way out rather than leaving the menu up behind a dismissed prompt.
	m.Close()
	if m.IsOpen() {
		t.Error("closing the prompt left something showing")
	}
}

func TestSaveWithNoSourceSaysSo(t *testing.T) {
	m := NewReplayMenu(c.Globals{}, "")
	m.Open()
	m.openNaming(namingRecording)
	m.saveRecording()

	if !m.noticeErr {
		t.Error("saving with no source reported success")
	}
	if !m.naming {
		t.Error("the prompt closed on a failed save; the user can't correct it")
	}
}
