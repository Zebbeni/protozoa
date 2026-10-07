package checkpoint

import (
	"errors"
	"io"
)

type MemFile struct {
	data []byte
}

type memView struct {
	f   *MemFile
	pos int64
}

func NewMemFile() *MemFile { return &MemFile{} }

func (m *MemFile) View() *memView { return &memView{f: m} }

func (m *MemFile) Bytes() []byte { return m.data }

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

// memRegistry maps file paths to MemFile instances.
var memRegistry = map[string]*MemFile{}

func registerMemFile(path string) *MemFile {
	m := NewMemFile()
	memRegistry[path] = m
	return m
}

// lookupMemFile returns the MemFile registered at path (or nil).
func lookupMemFile(path string) *MemFile {
	return memRegistry[path]
}

// MemFileSize returns the size in bytes of the in-memory file registered at path, or (0, false) if no MemFile was registered for it.
func MemFileSize(path string) (int64, bool) {
	if m := memRegistry[path]; m != nil {
		return m.Size(), true
	}
	return 0, false
}
