package config

import (
	"embed"
	"io"
	"io/fs"
)

// assetsFS is the asset filesystem the config loader reads default settings from.
var assetsFS fs.FS

// UseEmbeddedAssets wires the embedded asset bundle into the config loader.
func UseEmbeddedAssets(efs embed.FS) {
	assetsFS = efs
}

func loadEmbeddedDefault() io.Reader {
	if assetsFS == nil {
		panic("config: asset FS not initialised; UseEmbeddedAssets must be called first")
	}
	data, err := fs.ReadFile(assetsFS, "settings/default.json")
	if err != nil {
		panic("config: failed to read embedded default settings: " + err.Error())
	}
	return readerFromBytes(data)
}

type byteReader struct{ data []byte }

func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func readerFromBytes(b []byte) io.Reader { return &byteReader{data: b} }
