package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SettingsDir is where settings files are read from and written to, beside
// the designs and recordings directories.
const SettingsDir = "settings"

// SlugName turns a user-typed name into a file-name stem: lower case, with
// spaces, dashes and underscores becoming dashes and everything else
// dropped. fallback is used when nothing survives.
//
// Lives here because it is shared by settings, recordings and designs, and
// config is the one package all three already import — utils cannot hold it
// because utils imports config.
func SlugName(name, fallback string) string {
	clean := strings.ToLower(strings.TrimSpace(name))
	clean = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r == ' ', r == '-', r == '_':
			return '-'
		default:
			return -1
		}
	}, clean)
	if clean == "" {
		return fallback
	}
	return clean
}

// SettingsFileName is the file a typed settings name is saved as.
func SettingsFileName(name string) string {
	return SlugName(name, "settings") + ".json"
}

// SaveSettingsAs writes data to dir under a name derived from the typed one.
//
// Refuses to overwrite, like saving a recording does: the caller keeps the
// prompt up with the typed name intact so a collision is corrected rather
// than retyped, and a settings file someone has tuned is not something to
// replace silently.
func SaveSettingsAs(dir, name string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("nothing to save")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, SettingsFileName(name))
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%s already exists", filepath.Base(path))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// SettingsFile is one settings file, as a browser lists it.
type SettingsFile struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
}

// SavedSettings lists the .json files in dir, newest first.
//
// A file it cannot stat is skipped rather than listed with zero values, so a
// half-written one does not appear as an empty entry.
func SavedSettings(dir string) []SettingsFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []SettingsFile
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, SettingsFile{
			Name:    strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())),
			Path:    filepath.Join(dir, e.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out
}
