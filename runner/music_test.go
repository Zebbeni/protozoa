package runner

import (
	"testing"
	"time"

	"github.com/Zebbeni/protozoa/sound"
)

// TestEveryStateNamesAScene: a state missing from stateScenes takes the zero
// scene, which is SceneSilent, so the symptom of a forgotten entry is the
// music stopping on one screen with nothing in the build to say why.
func TestEveryStateNamesAScene(t *testing.T) {
	for _, state := range allRunnerStates {
		if _, ok := stateScenes[state]; !ok {
			t.Errorf("state %d names no scene", state)
		}
	}
	if len(stateScenes) != len(allRunnerStates) {
		t.Errorf("%d states name a scene against %d states", len(stateScenes), len(allRunnerStates))
	}
	// And the list itself has to BE the const block, or a state inserted into
	// the middle of it is missing from the list and the table alike and this
	// test passes while the new screen falls silent. Checking that the list
	// ends at stateReplay was not enough: it only caught a state appended
	// after the last one.
	if len(allRunnerStates) != int(stateCount) {
		t.Fatalf("%d states are listed against %d declared", len(allRunnerStates), int(stateCount))
	}
	for i, state := range allRunnerStates {
		if state != runnerState(i) {
			t.Errorf("the state list holds %d at index %d, so it is not the const block in order", state, i)
		}
	}
}

// TestTheReplayViewerIsTheOnlyStateOnItsOwnTrack is the rule asked for: the
// replay viewer's music is its own, and every menu, the designer and a
// running simulation's log are a separate piece.
func TestTheReplayViewerIsTheOnlyStateOnItsOwnTrack(t *testing.T) {
	if got := stateScenes[stateReplay]; got != sound.SceneReplay {
		t.Errorf("the replay viewer plays scene %d", got)
	}
	for _, state := range allRunnerStates {
		if state == stateReplay {
			continue
		}
		if stateScenes[state] == sound.SceneReplay {
			t.Errorf("state %d plays the replay viewer's music", state)
		}
	}
	// The running simulation's log shares the menus' music rather than having
	// none: it is one of the two places the second piece is for.
	for _, state := range []runnerState{stateMainMenu, stateMainMenuPopup} {
		if got := stateScenes[state]; got != sound.SceneMenu {
			t.Errorf("state %d plays scene %d, not the menu's", state, got)
		}
	}
}

// TestTheMusicClockIsBoundedAndMonotonic: the replay viewer's two pieces take
// turns on the frame clock, so a frame that took a long time — a dragged
// window, a machine resumed from sleep — must not hand a whole turn over in
// one tick, and a first frame has no previous one to measure from.
func TestTheMusicClockIsBoundedAndMonotonic(t *testing.T) {
	r := &Runner{}
	if first := r.sinceLastUpdate(); first != 0 {
		t.Errorf("the first frame advanced the music clock by %s", first)
	}
	// A frame that genuinely took no measurable time is 0, never negative.
	if dt := r.sinceLastUpdate(); dt < 0 {
		t.Errorf("a frame took %s", dt)
	}
	// Pretend the previous frame was an hour ago, which is what a sleeping
	// machine looks like.
	r.lastUpdate = time.Now().Add(-time.Hour)
	if dt := r.sinceLastUpdate(); dt != maxFrameStep {
		t.Errorf("an hour-long frame advanced the music by %s, want it capped at %s", dt, maxFrameStep)
	}
}
