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

// writerBacking is the minimum API the Writer needs from its underlying
// store: write bytes, seek (to record current offsets and rewrite the
// footer), and close. Both *os.File and *memView satisfy this.
type writerBacking interface {
	io.Writer
	io.Seeker
	io.Closer
}

// Writer writes checkpoint data to a .pzr file.
type Writer struct {
	file          writerBacking
	snapshotIndex []SnapshotEntry
}

// NewWriter creates a new .pzr "file" and writes the header.
//
// On native builds the path resolves to a real file via os.Create.
// On WASM (runtime.GOOS == "js") the path becomes a key in an
// in-memory registry; OpenReader will find the same MemFile by the
// same path. This means a checkpoint written during a sim is
// readable later in the same browser session, even though the
// browser has no real filesystem to back it.
func NewWriter(path string, header FileHeader) (*Writer, error) {
	file, err := openWriterBacking(path)
	if err != nil {
		return nil, err
	}

	// Write magic and version
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

	return &Writer{file: file}, nil
}

// openWriterBacking returns a writable backing for the given path —
// real os.File on native, *memView (registered in memRegistry under
// the path) on WASM. The MemFile entry persists in the package-level
// registry so a subsequent OpenReader call with the same path finds
// the data.
func openWriterBacking(path string) (writerBacking, error) {
	if runtime.GOOS == "js" {
		// Treat NewWriter as truncate-on-create: register a fresh
		// MemFile under the path, replacing any prior one.
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

// WriteDelta writes a forward-delta section. (Reserved; not currently
// emitted by the simulation.)
func (w *Writer) WriteDelta(payload *DeltaPayload) error {
	return w.writeSection(SectionDelta, payload.Cycle, payload)
}

// WriteDescendantTrees writes the full descendant trees as a final section.
// finalCycle is the cycle the simulation ended on; the replay reader uses
// it to extend FinalCycle past the last snapshot (snapshots fire at fixed
// intervals, so the true end cycle is almost always between them).
func (w *Writer) WriteDescendantTrees(payload *DescendantTreesPayload, finalCycle int) error {
	return w.writeSection(SectionDescendantTrees, finalCycle, payload)
}

// WriteHistory writes the full pH distribution and effect history as a
// final section. finalCycle has the same meaning as in
// WriteDescendantTrees.
func (w *Writer) WriteHistory(payload *HistoryPayload, finalCycle int) error {
	return w.writeSection(SectionHistory, finalCycle, payload)
}

// Close writes the snapshot index and footer, then closes the file.
//
// Footer layout (read in reverse from end of file):
//   [int64 indexOffset][int32 indexCount]
func (w *Writer) Close() error {
	// Record where the index starts
	indexOffset, _ := w.file.Seek(0, io.SeekCurrent)

	// Write snapshot index
	if err := writeGob(w.file, w.snapshotIndex); err != nil {
		w.file.Close()
		return err
	}

	// Write footer: index offset + count
	binary.Write(w.file, binary.LittleEndian, indexOffset)
	binary.Write(w.file, binary.LittleEndian, int32(len(w.snapshotIndex)))

	return w.file.Close()
}

func (w *Writer) writeSection(sectionType byte, cycle int, payload interface{}) error {
	// Gob-encode the payload
	var gobBuf bytes.Buffer
	if err := gob.NewEncoder(&gobBuf).Encode(payload); err != nil {
		return err
	}
	uncompressedLen := gobBuf.Len()

	// Flate-compress
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

	// Write section header: [type][cycle][compressedLen][uncompressedLen]
	binary.Write(w.file, binary.LittleEndian, sectionType)
	binary.Write(w.file, binary.LittleEndian, int32(cycle))
	binary.Write(w.file, binary.LittleEndian, int32(compBuf.Len()))
	binary.Write(w.file, binary.LittleEndian, int32(uncompressedLen))

	// Write compressed data
	_, err = w.file.Write(compBuf.Bytes())
	return err
}

func writeGob(w io.Writer, v interface{}) error {
	return gob.NewEncoder(w).Encode(v)
}
