//go:build js

package config

// platformDefaults adjusts the baseline settings for the host. The browser's
// adjustment is BrowserDefaults, in an untagged file so a native test can
// measure the world it produces.
func platformDefaults(g *Globals) { BrowserDefaults(g) }
