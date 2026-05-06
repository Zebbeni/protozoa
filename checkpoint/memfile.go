package checkpoint

import (
	"errors"
	"io"
)

// MemFile is an in-memory backing store for a .pzr file. Used by the
// WASM build, where the browser sandbox doesn't expose a real
// filesystem — Go's os.Open returns "not implemented on js" — but a
// single browser session can still produce a checkpoint, finish the
// simulation, and then read back the same bytes when the user clicks
// Explore.
//
// MemFile holds a single byte slice; callers obtain independent
// io.ReadWriteSeeker views (memView) so the writer and reader can
// each hold their own position cursor without stepping on each other.
type MemFile struct {
	data []byte
}

// memView is a read/write/seek view over a MemFile. Each View has
// its own position; Read / Write / Seek mutate the View's pos and
// (for Write) the underlying MemFile's data buffer.
type memView struct {
	f   *MemFile
	pos int64
}

// NewMemFile returns a fresh empty in-memory file.
func NewMemFile() *MemFile { return &MemFile{} }

// View returns a fresh read/write/seek view over this file. Writers
// and readers should each hold their own view; Close on a view does
// not destroy the underlying data, so the same MemFile can be handed
// to a writer first and a reader after.
func (m *MemFile) View() *memView { return &memView{f: m} }

// Bytes returns the accumulated bytes (no copy). For diagnostics —
// reading a MemFile in production should go through View().
func (m *MemFile) Bytes() []byte { return m.data }

// Size reports the current logical length of the file in bytes.
func (m *MemFile) Size() int64 { return int64(len(m.data)) }

func (v *memView) Read(p []byte) (int, error) {
	if v.pos >= int64(len(v.f.data)) {
		return 0, io.EOF
	}
	n := copy(p, v.f.data[v.pos:])
	v.pos += int64(n)
	return n, nil
}

func (v *memView) Write(p []byte) (int, error) {
	end := v.pos + int64(len(p))
	if end > int64(len(v.f.data)) {
		if end > int64(cap(v.f.data)) {
			grown := make([]byte, end, end*2+1)
			copy(grown, v.f.data)
			v.f.data = grown
		} else {
			v.f.data = v.f.data[:end]
		}
	}
	n := copy(v.f.data[v.pos:], p)
	v.pos += int64(n)
	return n, nil
}

// Close on a memView is a no-op — the underlying MemFile lives in the
// package registry so the same data can be opened again later by
// OpenReader. Satisfies io.Closer.
func (v *memView) Close() error { return nil }

func (v *memView) Seek(offset int64, whence int) (int64, error) {
	var p int64
	switch whence {
	case io.SeekStart:
		p = offset
	case io.SeekCurrent:
		p = v.pos + offset
	case io.SeekEnd:
		p = int64(len(v.f.data)) + offset
	default:
		return 0, errors.New("memview: invalid whence")
	}
	if p < 0 {
		return 0, errors.New("memview: negative position")
	}
	v.pos = p
	return p, nil
}

// memRegistry maps file paths to MemFile instances. Used as the
// single source of truth for the WASM build: NewWriter creates &
// registers a MemFile under the path; OpenReader looks the path up
// and returns a fresh view over the same data. On native builds the
// registry stays empty — file I/O goes straight to the OS.
var memRegistry = map[string]*MemFile{}

// registerMemFile inserts or replaces the in-memory file at the given
// path and returns the MemFile so the caller can hand its View() to a
// Writer.
func registerMemFile(path string) *MemFile {
	m := NewMemFile()
	memRegistry[path] = m
	return m
}

// lookupMemFile returns the MemFile registered at path (or nil).
func lookupMemFile(path string) *MemFile {
	return memRegistry[path]
}

// MemFileSize returns the size in bytes of the in-memory file
// registered at path, or (0, false) if no MemFile was registered for
// it. Used by callers that want to size-report the simulated
// replay file without touching the (non-existent) browser FS.
func MemFileSize(path string) (int64, bool) {
	if m := memRegistry[path]; m != nil {
		return m.Size(), true
	}
	return 0, false
}
