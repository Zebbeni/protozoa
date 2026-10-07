package ux

import (
	"errors"
	"syscall/js"
)

// copyToClipboard puts s on the clipboard through the browser's async Clipboard API.
func copyToClipboard(s string) error {
	clip := js.Global().Get("navigator").Get("clipboard")
	if clip.IsUndefined() {
		return errors.New("clipboard unavailable")
	}
	clip.Call("writeText", s)
	return nil
}
