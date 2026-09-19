package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestAgeFractionUsesMaxLifespanWhenSet: the point of the AGE view is
// "how close is this to dying of old age", which only means anything
// against the span it is measured by.
func TestAgeFractionUsesMaxLifespanWhenSet(t *testing.T) {
	loadKeyGlobals(t)
	config.GetCurrentGlobals().MaxLifespan = 1000

	for _, tc := range []struct {
		age, oldest int
		want        float64
	}{
		{0, 5000, 0},
		{500, 5000, 0.5},
		{1000, 5000, 1},
		// Past the span (an organism dies at it, but a restored one can
		// be beyond it) clamps rather than running off the ramp.
		{2000, 5000, 1},
	} {
		if got := ageFraction(tc.age, tc.oldest); got != tc.want {
			t.Errorf("age %d against a span of 1000 = %v, want %v", tc.age, got, tc.want)
		}
	}
}

// TestAgeFractionFallsBackToTheOldestAlive: with no fixed span every age
// is just a number of cycles with nothing to measure it against, so the
// view measures against whoever is oldest instead. The scale moving as
// that organism dies is the cost of having one at all.
func TestAgeFractionFallsBackToTheOldestAlive(t *testing.T) {
	loadKeyGlobals(t)
	config.GetCurrentGlobals().MaxLifespan = 0

	if got := ageFraction(50, 100); got != 0.5 {
		t.Errorf("age 50 against an oldest of 100 = %v, want 0.5", got)
	}
	if got := ageFraction(100, 100); got != 1 {
		t.Errorf("the oldest organism should be at the top of the ramp, got %v", got)
	}
	// Nothing alive, or everything newborn: no scale, so no colour.
	if got := ageFraction(0, 0); got != 0 {
		t.Errorf("with no span and no oldest, got %v, want 0", got)
	}
}

// TestSizeFractionIsAFixedScale: size is the one quantity worth
// comparing between frames, so it is measured against the configured
// cap rather than against the largest organism currently alive — a
// scale that rescaled itself every frame would make a growing
// population look static.
func TestSizeFractionIsAFixedScale(t *testing.T) {
	loadKeyGlobals(t)
	maxSize := config.MaximumMaxSize()

	if got := sizeFraction(0); got != 0 {
		t.Errorf("size 0 = %v, want 0", got)
	}
	if got := sizeFraction(maxSize); got != 1 {
		t.Errorf("size at the cap = %v, want 1", got)
	}
	if got := sizeFraction(maxSize / 2); got != 0.5 {
		t.Errorf("half the cap = %v, want 0.5", got)
	}
	// Clamped, since an organism restored from another configuration can
	// be bigger than this one allows.
	if got := sizeFraction(maxSize * 10); got != 1 {
		t.Errorf("past the cap = %v, want 1", got)
	}
}

// TestAgeKeyNamesItsDenominator: the two fallbacks mean quite different
// things — a fixed span the whole world shares, or a moving one set by
// whoever happens to be oldest — so the key has to say which is in
// force rather than leaving "100%" to mean either.
func TestAgeKeyNamesItsDenominator(t *testing.T) {
	loadKeyGlobals(t)

	config.GetCurrentGlobals().MaxLifespan = 5000
	fixed := colorKeyFor(orgColorAge, 0).bars[0].subtitle

	config.GetCurrentGlobals().MaxLifespan = 0
	moving := colorKeyFor(orgColorAge, 0).bars[0].subtitle

	if fixed == moving {
		t.Errorf("the age key reads %q either way; it should say which span it means", fixed)
	}
}
