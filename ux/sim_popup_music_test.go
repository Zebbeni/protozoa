package ux

import (
	"testing"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/sound"
)

// TestANewSimulationKeepsTheMusicChoice: the shipped defaults say music is on
// and so does every settings file written with it on, so a run started after
// the user switched music off used to switch it back on. It is a choice about
// the machine looking at the simulation, like the theme and the window size,
// not a property of the run.
func TestANewSimulationKeepsTheMusicChoice(t *testing.T) {
	loadKeyGlobals(t)
	// Installs the config-defaults stub the screen inside the popup reads.
	abilityConfigScreen(t)
	restore := sound.Preferred()
	defer sound.SetEnabled(restore)

	for _, choice := range []bool{false, true} {
		sound.SetEnabled(choice)

		// The form a user would be starting from, carrying the music setting
		// the shipped defaults and every settings file written with music on
		// both have.
		globals := *c.GetCurrentGlobals()
		globals.Music = true
		NewSimPopup(&globals)
		if globals.Music != choice {
			t.Errorf("with music set to %v, a new simulation starts with music %v", choice, globals.Music)
		}
	}
}

// TestRestoringDefaultsKeepsTheMusicChoice: restoring the defaults is about
// the balance of the simulation, so it leaves the three settings that describe
// the machine alone — and music is the fourth.
func TestRestoringDefaultsKeepsTheMusicChoice(t *testing.T) {
	loadKeyGlobals(t)
	cs, _ := abilityConfigScreen(t)

	cs.globals.Music = !cs.defaults.Music
	want := cs.globals.Music
	cs.globals.Theme = "light"
	cs.restoreAllDefaults()

	if cs.globals.Music != want {
		t.Errorf("restoring defaults set music to %v, want the %v the user chose", cs.globals.Music, want)
	}
	if cs.globals.Theme != "light" {
		t.Errorf("restoring defaults set the theme to %q", cs.globals.Theme)
	}
}
