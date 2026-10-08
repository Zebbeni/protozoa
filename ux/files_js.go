//go:build js

package ux

// filesAvailable is false in the browser: there is no directory to write a
// settings file, a design or a saved recording into.
//
// A replay of the CURRENT run still works, because checkpoint keeps its
// .pzr in an in-memory registry keyed by path (see checkpoint/writer.go),
// so Load Previous is deliberately not gated on this.
const filesAvailable = false
