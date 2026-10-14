package resources

import (
	"embed"
	"io/fs"
	"os"
)

// assetsFS is the asset filesystem the resource loaders read from.
var assetsFS fs.FS

// UseEmbeddedAssets wires the embedded asset bundle into the resource loaders.
func UseEmbeddedAssets(efs embed.FS) {
	assetsFS = efs
}

// UseDirAssets repoints the asset loaders at a live directory on disk (rooted at `root`, typically the project working directory).
func UseDirAssets(root string) {
	assetsFS = os.DirFS(root)
}

// assetExists reports whether the given path resolves to a regular file in the asset FS.
func assetExists(path string) bool {
	if assetsFS == nil {
		return false
	}
	info, err := fs.Stat(assetsFS, path)
	return err == nil && !info.IsDir()
}

// ReadAsset reads one file out of the asset FS, reporting whether it was
// there. Unlike the font and image loaders this does not exit on a miss: the
// music track is optional, and a build without it should run in silence
// rather than refuse to start.
func ReadAsset(path string) ([]byte, bool) {
	if assetsFS == nil {
		return nil, false
	}
	data, err := fs.ReadFile(assetsFS, path)
	if err != nil {
		return nil, false
	}
	return data, true
}
