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

// readerBacking is the minimum API the Reader needs: random-access reads, seeking, and close.
type readerBacking interface {
	io.Reader
	io.Seeker
	io.Closer
}

type Reader struct {
	file          readerBacking
	path          string
	Header        FileHeader
	SnapshotIndex []SnapshotEntry
	sectionsStart int64 // file offset where sections begin (after header)
	indexOffset   int64 // file offset where snapshot index begins
}

// OpenReader opens a .pzr file and reads the header and snapshot index.
func OpenReader(path string) (*Reader, error) {
	file, err := openReaderBacking(path)
	if err != nil {
		return nil, err
	}

	var magic [4]byte
	if _, err := io.ReadFull(file, magic[:]); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read magic: %w", err)
	}
	if magic != Magic {
		file.Close()
		return nil, fmt.Errorf("not a valid .pzr file")
	}

	var version uint32
	if err := binary.Read(file, binary.LittleEndian, &version); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read version: %w", err)
	}
	if version != Version {
		file.Close()
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

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

	sectionsStart, _ := file.Seek(0, io.SeekCurrent)

	// Look up the file's total size; needed for both the fast-path footer validation and the fallback section scan.
	fileSize, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek to end of file: %w", err)
	}

	// Try the fast path first: read the footer at the end of the file and seek to the snapshot index.
	index, indexOffset, ok := tryReadFooterIndex(file, fileSize)
	if !ok {
		index, indexOffset = scanSectionsForIndex(file, sectionsStart, fileSize)
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

func tryReadFooterIndex(file readerBacking, fileSize int64) ([]SnapshotEntry, int64, bool) {
	const footerSize = int64(8 + 4)
	if fileSize < footerSize {
		return nil, 0, false
	}
	if _, err := file.Seek(-footerSize, io.SeekEnd); err != nil {
		return nil, 0, false
	}
	var indexOffset int64
	var indexCount int32
	if err := binary.Read(file, binary.LittleEndian, &indexOffset); err != nil {
		return nil, 0, false
	}
	if err := binary.Read(file, binary.LittleEndian, &indexCount); err != nil {
		return nil, 0, false
	}
	if indexOffset <= 0 || indexOffset >= fileSize-footerSize || indexCount < 0 {
		return nil, 0, false
	}
	if _, err := file.Seek(indexOffset, io.SeekStart); err != nil {
		return nil, 0, false
	}
	var index []SnapshotEntry
	if err := gob.NewDecoder(file).Decode(&index); err != nil {
		return nil, 0, false
	}
	return index, indexOffset, true
}

// scanSectionsForIndex walks sections from sectionsStart forward, rebuilding the snapshot index from whatever sections are present and valid.
func scanSectionsForIndex(file readerBacking, sectionsStart, fileSize int64) ([]SnapshotEntry, int64) {
	var index []SnapshotEntry
	if _, err := file.Seek(sectionsStart, io.SeekStart); err != nil {
		return index, fileSize
	}
	for {
		offset, err := file.Seek(0, io.SeekCurrent)
		if err != nil || offset >= fileSize {
			break
		}
		const headerSize = int64(1 + 4 + 4 + 4)
		if offset+headerSize > fileSize {
			break
		}
		var sectionType byte
		var cycle, compLen, uncompLen int32
		if err := binary.Read(file, binary.LittleEndian, &sectionType); err != nil {
			break
		}
		if err := binary.Read(file, binary.LittleEndian, &cycle); err != nil {
			break
		}
		if err := binary.Read(file, binary.LittleEndian, &compLen); err != nil {
			break
		}
		if err := binary.Read(file, binary.LittleEndian, &uncompLen); err != nil {
			break
		}
		// Validate the section header before trusting compLen.
		if compLen < 0 || uncompLen < 0 || offset+headerSize+int64(compLen) > fileSize {
			break
		}
		if sectionType == SectionSnapshot {
			index = append(index, SnapshotEntry{
				Cycle:      int(cycle),
				FileOffset: offset,
			})
		}
		// Skip past the compressed payload to the next section.
		if _, err := file.Seek(int64(compLen), io.SeekCurrent); err != nil {
			break
		}
	}
	return index, fileSize
}

func (r *Reader) SectionsStart() int64 { return r.sectionsStart }
func (r *Reader) IndexOffset() int64   { return r.indexOffset }
func (r *Reader) Path() string         { return r.path }

func (r *Reader) SnapshotCount() int {
	return len(r.SnapshotIndex)
}

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

func (r *Reader) ReadNextSection() (sectionType byte, cycle int, payload interface{}, err error) {
	pos, _ := r.file.Seek(0, io.SeekCurrent)
	if pos >= r.indexOffset {
		err = io.EOF
		return
	}
	sectionType, cycle, payload, err = r.readSection()
	return
}

// SeekAfterHeader positions the file cursor right after the header, ready to read sections sequentially.
func (r *Reader) SeekAfterHeader() error {
	_, err := r.file.Seek(r.sectionsStart, io.SeekStart)
	return err
}

func (r *Reader) readSection() (sectionType byte, cycle int, payload interface{}, err error) {
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

	compData := make([]byte, compressedLen)
	if _, err = io.ReadFull(r.file, compData); err != nil {
		return
	}

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

func (r *Reader) Close() error {
	return r.file.Close()
}

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
