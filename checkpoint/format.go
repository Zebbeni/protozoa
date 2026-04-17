package checkpoint

// File format constants for .pzr (protozoa replay) files.
//
// Layout:
//   [Magic: 4 bytes] [Version: uint32]
//   [Header section]
//   [Section 0] [Section 1] ... [Section N]
//   [SnapshotIndex: gob-encoded []SnapshotEntry]
//   [IndexOffset: int64] [IndexCount: int32]

var Magic = [4]byte{'P', 'Z', 'R', 0}

const (
	Version uint32 = 1

	SectionSnapshot        byte = 0x01
	SectionDelta           byte = 0x02
	SectionDescendantTrees byte = 0x03
	SectionHistory         byte = 0x04
)
