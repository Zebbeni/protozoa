//go:build !js

package ux

import "github.com/atotto/clipboard"

// copyToClipboard puts s on the system clipboard. Ebiten has no
// clipboard API; the browser build has its own version in
// clipboard_js.go.
func copyToClipboard(s string) error {
	return clipboard.WriteAll(s)
}
