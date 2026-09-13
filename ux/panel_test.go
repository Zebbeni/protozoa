package ux

import "testing"

// TestDisplayTogglesRoundTrip guards the GRID DISPLAY row against the
// bug that shipped with FLOW: the button toggled its layer but always
// rendered inactive, because the read path and the write path were
// separate switch statements and only one had been updated.
//
// Asserting get/set agree for every entry means a new layer whose
// accessors disagree — or point at the wrong field — fails here rather
// than looking like a dead button in the UI.
func TestDisplayTogglesRoundTrip(t *testing.T) {
	for _, tc := range displayToggles {
		t.Run(tc.label, func(t *testing.T) {
			g := &Grid{}

			tc.set(g, true)
			if !tc.get(g) {
				t.Fatalf("set(true) then get() = false — accessors disagree")
			}
			tc.set(g, false)
			if tc.get(g) {
				t.Fatalf("set(false) then get() = true — accessors disagree")
			}
		})
	}
}

// TestDisplayTogglesAreDistinct catches copy-paste accessors: two
// entries wired to the same Grid field would make one button silently
// drive the other's layer.
func TestDisplayTogglesAreDistinct(t *testing.T) {
	for i, a := range displayToggles {
		for j, b := range displayToggles {
			if i >= j {
				continue
			}
			g := &Grid{}
			a.set(g, true)
			if b.get(g) {
				t.Errorf("%q and %q share a Grid field: setting %q lit %q",
					a.label, b.label, a.label, b.label)
			}
		}
	}
}
