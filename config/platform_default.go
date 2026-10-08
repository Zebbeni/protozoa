//go:build !js

package config

// platformDefaults adjusts the baseline settings for the host.
//
// Nothing to do away from the browser: settings/default.json IS the desktop
// baseline, and a second set of numbers held anywhere else would be one more
// thing to keep in step with it.
func platformDefaults(g *Globals) {}
