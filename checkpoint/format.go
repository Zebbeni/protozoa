package checkpoint

var Magic = [4]byte{'P', 'Z', 'R', 0}

const (
	Version uint32 = 2

	SectionSnapshot        byte = 0x01
	SectionDelta           byte = 0x02
	SectionDescendantTrees byte = 0x03
	SectionHistory         byte = 0x04
)
