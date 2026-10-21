//go:build js

package config

import (
	"encoding/json"
	"testing"
)

// TestTheBrowserHookAppliesTheBrowserDefaults is all that is left for the js
// build to say: WHAT those defaults are is checked on every platform by
// browser_defaults_test.go, now that they live in an untagged file. This is
// the wiring — that the browser build's platformDefaults is that function and
// not a second set of numbers.
//
// Runs only in a js build: GOOS=js GOARCH=wasm go test ./config/
func TestTheBrowserHookAppliesTheBrowserDefaults(t *testing.T) {
	base := shippedDefaults(t)

	viaHook := base
	platformDefaults(&viaHook)

	direct := base
	BrowserDefaults(&direct)

	a, _ := json.Marshal(viaHook)
	b, _ := json.Marshal(direct)
	if string(a) != string(b) {
		t.Error("the browser build's platformDefaults is not BrowserDefaults")
	}
	if viaHook.GridUnitsWide == base.GridUnitsWide {
		t.Error("the hook did nothing at all, so this test would pass against an empty one")
	}
}
