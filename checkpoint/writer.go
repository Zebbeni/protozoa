package checkpoint

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/gob"
	"io"
	"os"
)

// Writer writes checkpoint data to a .pzr file.
type Writer struct {
	file          *os.File
	snapshotIndex []SnapshotEntry
}

// NewWriter creates a new .pzr file and writes the header.
func NewWriter(path string, header FileHeader) (*Writer, error) {
	file, err := os.Create(path)
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

// WriteSnapshot writes a full snapshot section and records its offset.
func (w *Writer) WriteSnapshot(payload *SnapshotPayload) error {
	offset, _ := w.file.Seek(0, io.SeekCurrent)
	w.snapshotIndex = append(w.snapshotIndex, SnapshotEntry{
		Cycle:      payload.Cycle,
		FileOffset: offset,
	})
	return w.writeSection(SectionSnapshot, payload.Cycle, payload)
}

// WriteDelta writes a delta section.
func (w *Writer) WriteDelta(payload *DeltaPayload) error {
	return w.writeSection(SectionDelta, payload.Cycle, payload)
}

// WriteDescendantTrees writes the full descendant trees as a final section.
func (w *Writer) WriteDescendantTrees(payload *DescendantTreesPayload) error {
	return w.writeSection(SectionDescendantTrees, 0, payload)
}

// Close writes the snapshot index and footer, then closes the file.
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
