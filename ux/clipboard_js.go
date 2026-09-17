package ux

import (
	"errors"
	"syscall/js"
)

// copyToClipboard puts s on the clipboard through the browser's async
// Clipboard API. The write finishes after this returns; a click
// handler counts as the user gesture browsers require.
func copyToClipboard(s string) error {
	clip := js.Global().Get("navigator").Get("clipboard")
	if clip.IsUndefined() {
		return errors.New("clipboard unavailable")
	}
	clip.Call("writeText", s)
	return nil
}
