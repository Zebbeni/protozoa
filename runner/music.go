package runner

import "github.com/Zebbeni/protozoa/sound"

// allRunnerStates is every state the runner can be in, so the music table
// below can be walked against it. A new state missing from that table would
// otherwise take the zero scene and fall silent with nothing to say why.
var allRunnerStates = []runnerState{
	stateSplash,
	stateMainMenu,
	stateRules,
	stateAbout,
	stateRecordings,
	stateDesigner,
	stateMainMenuPopup,
	stateLoadingReplay,
	stateReplay,
}

// stateScenes is which piece of music each state plays. Derived from the
// state in one place rather than set at each transition: there are a dozen
// ways into the replay viewer and out of it, and a music call at each is one
// a new path would be written without.
//
// stateLoadingReplay keeps the menu's music because it keeps the menu's
// picture — the replay's track starts when its viewer does.
var stateScenes = map[runnerState]sound.Scene{
	stateSplash:        sound.SceneMenu,
	stateMainMenu:      sound.SceneMenu,
	stateRules:         sound.SceneMenu,
	stateAbout:         sound.SceneMenu,
	stateRecordings:    sound.SceneMenu,
	stateDesigner:      sound.SceneMenu,
	stateMainMenuPopup: sound.SceneMenu,
	stateLoadingReplay: sound.SceneMenu,
	stateReplay:        sound.SceneReplay,
}
