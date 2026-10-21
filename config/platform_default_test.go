//go:build !js

package config

import (
	"encoding/json"
	"testing"
)

// TestPlatformDefaultsLeaveTheDesktopBaselineAlone: settings/default.json is
// the desktop baseline, so the hook must be a no-op here. A second set of
// numbers that drifted from the file would be invisible.
//
// Build-tagged !js, because on the browser build the hook is deliberately NOT
// a no-op — that is what BrowserDefaults is. Untagged, this compiled into the
// wasm test binary and failed there, which nothing surfaced: running the js
// package's tests needs a wasm runner, so `go test ./config/` on a desktop
// never built this for js at all. The js half is
// TestTheBrowserHookAppliesTheBrowserDefaults in platform_js_values_test.go.
//
//	GOROOT="$(go env GOROOT)"
//	GOOS=js GOARCH=wasm go test -exec=<node wasm_exec_node.js wrapper> ./config/
func TestPlatformDefaultsLeaveTheDesktopBaselineAlone(t *testing.T) {
	want := shippedDefaults(t)
	got := want
	platformDefaults(&got)
	// Globals holds slices, so compare the encoded form rather than the
	// struct: a field added later is then covered without touching this.
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Error("platformDefaults changed the desktop baseline")
	}
}
