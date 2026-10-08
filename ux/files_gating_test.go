package ux

import "testing"

func labels[T any](items []T, label func(T) string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, label(it))
	}
	return out
}

func has(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestFileControlsAreLeftOutWithoutAFilesystem: in the browser there is no
// directory to write a settings file, a design or a saved recording into, so
// the controls are not drawn at all rather than offered and failing.
func TestFileControlsAreLeftOutWithoutAFilesystem(t *testing.T) {
	menu := labels(menuButtonsFor(false, false), func(b menuButton) string { return b.label })
	for _, gone := range []string{"Saved Recordings", "Organism Designer", "Exit"} {
		if has(menu, gone) {
			t.Errorf("the main menu still offers %q with no filesystem: %v", gone, menu)
		}
	}
	// Load Previous STAYS: checkpoint keeps the current run's .pzr in an
	// in-memory registry, so replaying the run just finished works.
	for _, kept := range []string{"New Simulation", "Load Previous", "About"} {
		if !has(menu, kept) {
			t.Errorf("the main menu dropped %q, which needs no filesystem: %v", kept, menu)
		}
	}

	replay := labels(replayMenuButtonsFor(false), func(b replayMenuButton) string { return b.label })
	if has(replay, "Save Recording As...") {
		t.Errorf("the replay menu still offers Save Recording As: %v", replay)
	}
	for _, kept := range []string{"Run Again (New Seed)", "View Settings", "Main Menu", "Close"} {
		if !has(replay, kept) {
			t.Errorf("the replay menu dropped %q: %v", kept, replay)
		}
	}

	settings := labels(settingsButtonsFor(false), func(b settingsButton) string { return b.label })
	if has(settings, "Save As...") {
		t.Errorf("the settings panel still offers Save As: %v", settings)
	}
	// Copy Seed goes to the clipboard, not to a file.
	for _, kept := range []string{"Copy Seed", "Close", "Edit Settings"} {
		if !has(settings, kept) {
			t.Errorf("the settings panel dropped %q: %v", kept, settings)
		}
	}
}

// TestEveryControlIsThereWithAFilesystem is the other half: the gating must
// not drop anything on a desktop build.
func TestEveryControlIsThereWithAFilesystem(t *testing.T) {
	menu := labels(menuButtonsFor(true, true), func(b menuButton) string { return b.label })
	for _, want := range []string{"New Simulation", "Load Previous", "Saved Recordings",
		"Organism Designer", "About", "Exit"} {
		if !has(menu, want) {
			t.Errorf("the main menu is missing %q: %v", want, menu)
		}
	}
	replay := labels(replayMenuButtonsFor(true), func(b replayMenuButton) string { return b.label })
	if !has(replay, "Save Recording As...") {
		t.Errorf("the replay menu is missing Save Recording As: %v", replay)
	}
	settings := labels(settingsButtonsFor(true), func(b settingsButton) string { return b.label })
	if !has(settings, "Save As...") {
		t.Errorf("the settings panel is missing Save As: %v", settings)
	}
}

// TestTheDesktopBuildHasAFilesystem pins the constant itself, so a build tag
// going wrong shows up here rather than as a missing menu.
func TestTheDesktopBuildHasAFilesystem(t *testing.T) {
	if !filesAvailable {
		t.Error("filesAvailable is false in a non-js build")
	}
}
