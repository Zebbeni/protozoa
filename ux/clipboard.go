//go:build !js

package ux

import "github.com/atotto/clipboard"

func copyToClipboard(s string) error {
	return clipboard.WriteAll(s)
}
