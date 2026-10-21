package ux

import (
	"testing"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// TestRulesAndAboutAreSeparateEntries: Rules is how the world works and About
// is who made it, and they were one entry labelled "About" while the credits
// did not exist. A label pointing at the wrong document is the failure here,
// since both open a screen and neither errors.
func TestRulesAndAboutAreSeparateEntries(t *testing.T) {
	for _, files := range []bool{true, false} {
		buttons := menuButtonsFor(files, true)
		byChoice := map[MenuChoice]string{}
		for _, b := range buttons {
			if first, dup := byChoice[b.choice]; dup {
				t.Errorf("choice %d is on two buttons, %q and %q", b.choice, first, b.label)
			}
			byChoice[b.choice] = b.label
		}
		if got := byChoice[MenuChoiceRules]; got != "Rules" {
			t.Errorf("files=%v: the rules entry is labelled %q", files, got)
		}
		if got := byChoice[MenuChoiceAbout]; got != "About" {
			t.Errorf("files=%v: the credits entry is labelled %q", files, got)
		}
	}
}

// TestEveryMenuButtonHasItsOwnLabel, in both builds: the browser build drops
// the entries that need a filesystem, so a label or choice that only appears
// in one of them is easy to get wrong in the other.
func TestEveryMenuButtonHasItsOwnLabel(t *testing.T) {
	for _, files := range []bool{true, false} {
		for _, canExit := range []bool{true, false} {
			seen := map[string]bool{}
			for _, b := range menuButtonsFor(files, canExit) {
				if b.label == "" {
					t.Errorf("files=%v exit=%v: choice %d has no label", files, canExit, b.choice)
				}
				if seen[b.label] {
					t.Errorf("files=%v exit=%v: %q is on two buttons", files, canExit, b.label)
				}
				seen[b.label] = true
				if b.choice == MenuChoiceNone {
					t.Errorf("files=%v exit=%v: %q does nothing", files, canExit, b.label)
				}
			}
		}
	}
}

// TestTheMenuStackFitsTheWindow: the stack is centred, so another entry does
// not push anything off a tall window — it runs off the top and bottom of a
// short one. Measured against the shipped window, which is the one a first
// run opens at.
func TestTheMenuStackFitsTheWindow(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	m := NewMainMenu()
	titleH := boundString(mainMenuTitleFace(), mainMenuTitle).Dy()
	top := m.titleY() - titleH
	_, lastY := m.buttonRect(len(mainMenuButtons) - 1)
	bottom := lastY + mainMenuButtonH

	if top < 0 {
		t.Errorf("the title starts %dpx above the window", -top)
	}
	if bottom > c.ScreenHeight() {
		t.Errorf("the last button ends %dpx below the %dpx window",
			bottom-c.ScreenHeight(), c.ScreenHeight())
	}
	t.Logf("%d entries, %dpx of stack in a %dpx window",
		len(mainMenuButtons), bottom-top, c.ScreenHeight())
}
