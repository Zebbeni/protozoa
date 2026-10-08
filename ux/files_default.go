//go:build !js

package ux

// filesAvailable reports whether the host has a filesystem the player can
// save to and load from. Controls that write or browse real files are left
// out of the UI entirely when it does not, rather than offered and failing.
const filesAvailable = true
