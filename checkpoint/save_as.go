package checkpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RecordingsDir is where saved recordings go, beside the designs and settings directories so the three kinds of saved thing sit together.
const RecordingsDir = "recordings"

const recordingExt = ".pzr"

// RecordingFileName turns a user-typed name into a file name, the same way designs do.
func RecordingFileName(name string) string {
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
		clean = "recording"
	}
	return clean + recordingExt
}

// SaveRecordingAs copies the recording at srcPath into dir under a file name derived from name.
func SaveRecordingAs(srcPath, dir, name string) (string, error) {
	data, err := readRecording(srcPath)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("recording is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, RecordingFileName(name))
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%s already exists", filepath.Base(path))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// readRecording reads a .pzr's bytes, from the in-memory registry when the path names a MemFile (the wasm build) and from disk otherwise.
func readRecording(path string) ([]byte, error) {
	if m := lookupMemFile(path); m != nil {
		// Copy rather than hand back the live slice: the writer may still be appending to it.
		b := m.Bytes()
		out := make([]byte, len(b))
		copy(out, b)
		return out, nil
	}
	return os.ReadFile(path)
}

// SavedRecording is one file in the recordings directory, as the browser shows it.
type SavedRecording struct {
	Name string
	Path string
	Size int64
	Mod  time.Time
}

func SavedRecordings(dir string) []SavedRecording {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []SavedRecording
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), recordingExt) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, SavedRecording{
			Name: strings.TrimSuffix(e.Name(), recordingExt),
			Path: filepath.Join(dir, e.Name()),
			Size: info.Size(),
			Mod:  info.ModTime(),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Mod.After(out[j].Mod) })
	return out
}

func DeleteRecording(dir, name string) error {
	err := os.Remove(filepath.Join(dir, RecordingFileName(name)))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
