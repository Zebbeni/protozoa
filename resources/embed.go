package resources

import (
	"embed"
	"io/fs"
)

// assetsFS is the asset filesystem the resource loaders read from.
// Set once at startup by main.go via UseEmbeddedAssets; before that
// any load attempt panics. Paths are project-relative
// (e.g. "resources/images/play_button.png").
var assetsFS fs.FS

// UseEmbeddedAssets wires the embedded asset bundle into the resource
// loaders. Must be called before Init / SelectZoom / any sprite or font
// access.
func UseEmbeddedAssets(efs embed.FS) {
	assetsFS = efs
}

// assetExists reports whether the given path resolves to a regular file
// in the asset FS. Used for the loadOrGenerate* helpers' "fall back to
// generated art" decision.
func assetExists(path string) bool {
	if assetsFS == nil {
		return false
	}
	info, err := fs.Stat(assetsFS, path)
	return err == nil && !info.IsDir()
}
