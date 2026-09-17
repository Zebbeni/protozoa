//go:build !js

package ux

import (
	"fmt"
	"os"
	"path/filepath"
)

// saveExport writes data to a new file called name (numbered if taken)
// in the settings folder when there is one, otherwise the working
// directory, and returns where it went. The browser build downloads the
// file instead; see export_file_js.go.
func saveExport(name string, data []byte) (string, error) {
	dir := "."
	if info, err := os.Stat("settings"); err == nil && info.IsDir() {
		dir = "settings"
	}
	ext := filepath.Ext(name)
	stem := name[:len(name)-len(ext)]
	path := filepath.Join(dir, name)
	for i := 2; ; i++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			path = filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := f.Write(data)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return "", werr
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		return path, nil
	}
}
