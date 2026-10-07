package checkpoint

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/gob"
	"io"
	"runtime"

	"os"
)

// writerBacking is the minimum API the Writer needs from its underlying store.
type writerBacking interface {
	io.Writer
	io.Seeker
	io.Closer
}

type Writer struct {
	file          writerBacking
	snapshotIndex []SnapshotEntry
	// bytesWritten is everything committed to the file so far.
	bytesWritten int64
}

func (w *Writer) BytesWritten() int64 {
	if w == nil {
		return 0
	}
	return w.bytesWritten
}

// NewWriter creates a new .pzr "file" and writes the header.
func NewWriter(path string, header FileHeader) (*Writer, error) {
	file, err := openWriterBacking(path)
	if err != nil {
		return nil, err
	}

	if _, err := file.Write(Magic[:]); err != nil {
		file.Close()
		return nil, err
	}
	if err := binary.Write(file, binary.LittleEndian, Version); err != nil {
		file.Close()
		return nil, err
	}

	// Write header with length prefix so reader knows exactly where sections start
	var headerBuf bytes.Buffer
	if err := gob.NewEncoder(&headerBuf).Encode(header); err != nil {
		file.Close()
		return nil, err
	}
	binary.Write(file, binary.LittleEndian, int32(headerBuf.Len()))
	if _, err := file.Write(headerBuf.Bytes()); err != nil {
		file.Close()
		return nil, err
	}

	// Magic + version + the length prefix + the header itself.
	written := int64(len(Magic)) + int64(binary.Size(Version)) + 4 + int64(headerBuf.Len())
	return &Writer{file: file, bytesWritten: written}, nil
}

func openWriterBacking(path string) (writerBacking, error) {
	if runtime.GOOS == "js" {
		// Treat NewWriter as truncate-on-create: register a fresh MemFile under the path, replacing any prior one.
		return registerMemFile(path).View(), nil
	}
	return os.Create(path)
}

// WriteSnapshot writes a full snapshot section and records its offset.
func (w *Writer) WriteSnapshot(payload *SnapshotPayload) error {
	offset, _ := w.file.Seek(0, io.SeekCurrent)
	w.snapshotIndex = append(w.snapshotIndex, SnapshotEntry{
		Cycle:      payload.Cycle,
		FileOffset: offset,
	})
	return w.writeSection(SectionSnapshot, payload.Cycle, payload)
}

func (w *Writer) WriteDelta(payload *DeltaPayload) error {
	return w.writeSection(SectionDelta, payload.Cycle, payload)
}

func (w *Writer) WriteDescendantTrees(payload *DescendantTreesPayload, finalCycle int) error {
	return w.writeSection(SectionDescendantTrees, finalCycle, payload)
}

// WriteHistory writes the full pH distribution and effect history as a final section.
func (w *Writer) WriteHistory(payload *HistoryPayload, finalCycle int) error {
	return w.writeSection(SectionHistory, finalCycle, payload)
}

// Close writes the snapshot index and footer, then closes the file.
func (w *Writer) Close() error {
	indexOffset, _ := w.file.Seek(0, io.SeekCurrent)

	if err := writeGob(w.file, w.snapshotIndex); err != nil {
		w.file.Close()
		return err
	}

	binary.Write(w.file, binary.LittleEndian, indexOffset)
	binary.Write(w.file, binary.LittleEndian, int32(len(w.snapshotIndex)))

	return w.file.Close()
}

func (w *Writer) writeSection(sectionType byte, cycle int, payload interface{}) error {
	var gobBuf bytes.Buffer
	if err := gob.NewEncoder(&gobBuf).Encode(payload); err != nil {
		return err
	}
	uncompressedLen := gobBuf.Len()

	var compBuf bytes.Buffer
	flateWriter, err := flate.NewWriter(&compBuf, flate.DefaultCompression)
	if err != nil {
		return err
	}
	if _, err := flateWriter.Write(gobBuf.Bytes()); err != nil {
		return err
	}
	if err := flateWriter.Close(); err != nil {
		return err
	}

	binary.Write(w.file, binary.LittleEndian, sectionType)
	binary.Write(w.file, binary.LittleEndian, int32(cycle))
	binary.Write(w.file, binary.LittleEndian, int32(compBuf.Len()))
	binary.Write(w.file, binary.LittleEndian, int32(uncompressedLen))

	n, err := w.file.Write(compBuf.Bytes())
	// The section header is one byte plus three int32s.
	w.bytesWritten += 1 + 4*3 + int64(n)
	return err
}

func writeGob(w io.Writer, v interface{}) error {
	return gob.NewEncoder(w).Encode(v)
}
