package checkpoint

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"runtime"
)

// readerBacking is the minimum API the Reader needs: random-access
// reads, seeking, and close. Both *os.File and *memView satisfy it.
type readerBacking interface {
	io.Reader
	io.Seeker
	io.Closer
}

// Reader reads checkpoint data from a .pzr file.
type Reader struct {
	file          readerBacking
	path          string
	Header        FileHeader
	SnapshotIndex []SnapshotEntry
	sectionsStart int64 // file offset where sections begin (after header)
	indexOffset   int64 // file offset where snapshot index begins
}

// OpenReader opens a .pzr file and reads the header and snapshot
// index. On native, opens via os.Open. On WASM (runtime.GOOS == "js")
// looks up an in-memory MemFile registered by NewWriter under the
// same path; this lets a checkpoint written during the same browser
// session be read back without touching the (non-existent) browser
// filesystem.
func OpenReader(path string) (*Reader, error) {
	file, err := openReaderBacking(path)
	if err != nil {
		return nil, err
	}

	// Verify magic
	var magic [4]byte
	if _, err := io.ReadFull(file, magic[:]); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read magic: %w", err)
	}
	if magic != Magic {
		file.Close()
		return nil, fmt.Errorf("not a valid .pzr file")
	}

	// Verify version
	var version uint32
	if err := binary.Read(file, binary.LittleEndian, &version); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read version: %w", err)
	}
	if version != Version {
		file.Close()
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	// Read header (length-prefixed gob)
	var headerLen int32
	if err := binary.Read(file, binary.LittleEndian, &headerLen); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read header length: %w", err)
	}
	headerData := make([]byte, headerLen)
	if _, err := io.ReadFull(file, headerData); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read header data: %w", err)
	}
	var header FileHeader
	if err := gob.NewDecoder(bytes.NewReader(headerData)).Decode(&header); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	// Sections begin right after the header
	sectionsStart, _ := file.Seek(0, io.SeekCurrent)

	// Read snapshot index from footer.
	// Footer: [int64 indexOffset][int32 indexCount]
	footerSize := int64(8 + 4)
	if _, err := file.Seek(-footerSize, io.SeekEnd); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek to footer: %w", err)
	}

	var indexOffset int64
	var indexCount int32
	binary.Read(file, binary.LittleEndian, &indexOffset)
	binary.Read(file, binary.LittleEndian, &indexCount)

	// Read snapshot index
	if _, err := file.Seek(indexOffset, io.SeekStart); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek to index: %w", err)
	}

	var index []SnapshotEntry
	if err := gob.NewDecoder(file).Decode(&index); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read snapshot index: %w", err)
	}

	return &Reader{
		file:          file,
		path:          path,
		Header:        header,
		SnapshotIndex: index,
		sectionsStart: sectionsStart,
		indexOffset:   indexOffset,
	}, nil
}

func (r *Reader) SectionsStart() int64 { return r.sectionsStart }
func (r *Reader) IndexOffset() int64   { return r.indexOffset }
func (r *Reader) Path() string         { return r.path }

// SnapshotCount returns the number of snapshots in the file.
func (r *Reader) SnapshotCount() int {
	return len(r.SnapshotIndex)
}

// ReadSnapshot reads the snapshot at the given index.
func (r *Reader) ReadSnapshot(index int) (*SnapshotPayload, error) {
	if index < 0 || index >= len(r.SnapshotIndex) {
		return nil, fmt.Errorf("snapshot index %d out of range [0, %d)", index, len(r.SnapshotIndex))
	}

	entry := r.SnapshotIndex[index]
	if _, err := r.file.Seek(entry.FileOffset, io.SeekStart); err != nil {
		return nil, err
	}

	sectionType, _, payload, err := r.readSection()
	if err != nil {
		return nil, err
	}
	if sectionType != SectionSnapshot {
		return nil, fmt.Errorf("expected snapshot at offset %d, got section type %d", entry.FileOffset, sectionType)
	}

	snapshot, ok := payload.(*SnapshotPayload)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type at snapshot offset")
	}
	return snapshot, nil
}

// ReadNextSection reads the next section from the current file position.
// Returns the section type, cycle, and decoded payload (either *SnapshotPayload or *DeltaPayload).
func (r *Reader) ReadNextSection() (sectionType byte, cycle int, payload interface{}, err error) {
	// Stop before the snapshot index
	pos, _ := r.file.Seek(0, io.SeekCurrent)
	if pos >= r.indexOffset {
		err = io.EOF
		return
	}
	sectionType, cycle, payload, err = r.readSection()
	return
}

// SeekAfterHeader positions the file cursor right after the header,
// ready to read sections sequentially.
func (r *Reader) SeekAfterHeader() error {
	_, err := r.file.Seek(r.sectionsStart, io.SeekStart)
	return err
}

func (r *Reader) readSection() (sectionType byte, cycle int, payload interface{}, err error) {
	// Read section header
	if err = binary.Read(r.file, binary.LittleEndian, &sectionType); err != nil {
		return
	}
	var cycleI32 int32
	if err = binary.Read(r.file, binary.LittleEndian, &cycleI32); err != nil {
		return
	}
	cycle = int(cycleI32)

	var compressedLen, uncompressedLen int32
	binary.Read(r.file, binary.LittleEndian, &compressedLen)
	binary.Read(r.file, binary.LittleEndian, &uncompressedLen)

	// Read compressed data
	compData := make([]byte, compressedLen)
	if _, err = io.ReadFull(r.file, compData); err != nil {
		return
	}

	// Decompress
	flateReader := flate.NewReader(bytes.NewReader(compData))
	decompData := make([]byte, 0, uncompressedLen)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := flateReader.Read(buf)
		if n > 0 {
			decompData = append(decompData, buf[:n]...)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			err = readErr
			return
		}
	}
	flateReader.Close()

	// Gob-decode
	switch sectionType {
	case SectionSnapshot:
		var snap SnapshotPayload
		if err = gob.NewDecoder(bytes.NewReader(decompData)).Decode(&snap); err != nil {
			return
		}
		payload = &snap
	case SectionDelta:
		var delta DeltaPayload
		if err = gob.NewDecoder(bytes.NewReader(decompData)).Decode(&delta); err != nil {
			return
		}
		payload = &delta
	case SectionDescendantTrees:
		var trees DescendantTreesPayload
		if err = gob.NewDecoder(bytes.NewReader(decompData)).Decode(&trees); err != nil {
			return
		}
		payload = &trees
	case SectionHistory:
		var hist HistoryPayload
		if err = gob.NewDecoder(bytes.NewReader(decompData)).Decode(&hist); err != nil {
			return
		}
		payload = &hist
	default:
		err = fmt.Errorf("unknown section type: %d", sectionType)
	}
	return
}

// Close closes the underlying file.
func (r *Reader) Close() error {
	return r.file.Close()
}

// openReaderBacking returns a readable backing for the given path —
// real os.File on native, fresh *memView (from the registered
// MemFile) on WASM. WASM reads return io.EOF beyond the data the
// writer wrote.
func openReaderBacking(path string) (readerBacking, error) {
	if runtime.GOOS == "js" {
		mf := lookupMemFile(path)
		if mf == nil {
			return nil, fmt.Errorf("checkpoint: no in-memory file registered for %q", path)
		}
		return mf.View(), nil
	}
	return os.Open(path)
}
