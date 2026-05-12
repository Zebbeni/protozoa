package resources

import (
	"embed"
	"io/fs"
	"os"
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

// UseDirAssets repoints the asset loaders at a live directory on disk
// (rooted at `root`, typically the project working directory). Used by
// the animation-test mode so editing a sprite PNG and pressing the
// reload hotkey (or letting the mtime poller fire) actually picks up
// the new bytes instead of re-reading the embedded copy baked into the
// binary at build time. Must be called after UseEmbeddedAssets to take
// effect — and before any ReloadImages call that should hit disk.
func UseDirAssets(root string) {
	assetsFS = os.DirFS(root)
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
