package checkpoint

// FileHeader is written once at the start of the file.
type FileHeader struct {
	Seed               uint64
	CheckpointInterval int
	GridUnitsWide      int
	GridUnitsHigh      int
}

// SnapshotEntry maps a cycle to its file offset for fast seeking.
type SnapshotEntry struct {
	Cycle      int
	FileOffset int64
}

// SnapshotPayload contains the full simulation state at a checkpoint.
//
// Field types are intentionally narrowed (uint16 / float32 / uint32) at
// the serialization boundary even though the in-memory simulation keeps
// using int / float64. The conversion happens in capture.go / restore.go.
// The narrowed encoding cuts ~40% off snapshot size on typical runs and
// keeps file size manageable when CheckpointInterval is small.
type SnapshotPayload struct {
	Cycle                 int
	RNGState              []byte
	TotalOrganismsCreated int
	Organisms             []OrganismRecord
	OrganismGrid          [][]int
	// CurrentPhMap and PreviousPhMap are stored as float64 to preserve
	// the simulation's internal precision exactly. An earlier version
	// of this format used float32 to halve snapshot size, but the
	// resulting precision loss compounded across cycles of pH diffusion
	// and made replay drift from recording — replays from a snapshot
	// would not reach the same state at the same cycle as the original
	// recording, breaking step-back/forward repeatability and the
	// "most successful" highlighting.
	CurrentPhMap  [][]float64
	PreviousPhMap [][]float64
	FoodItems     []FoodRecord
	Ancestors     []AncestorRecord
}

// OrganismRecord is the serializable form of a single organism.
//
// Sizing rationale (per field):
//   - IDs, Age, TraveledDist, OriginalAncestorID: uint32 — sims don't
//     produce more than ~4 billion organisms, and gob encodes uint32
//     in 1–5 bytes via varint.
//   - Children, MinCyclesBetweenSpawns, CyclesSinceLastSpawn:
//     uint16 — bounded by config & biology.
//   - Locations: uint16 — supports grid sizes up to 65535×65535.
//   - Direction: int8 — only ever -1, 0, +1.
//   - CurrentAction: uint8 — Action enum has well under 256 values.
//   - Health, Size, and all simulation-affecting trait floats: float64.
//     Earlier this format used float32 to halve per-float storage,
//     but the precision loss compounded across many cycles into
//     observable replay drift — replay from a snapshot diverged from
//     the recording's state. Color is render-only and stays float32.
type OrganismRecord struct {
	ID                   uint32
	Age                  uint32
	Health               float64
	Size                 float64
	Children             uint16
	TraveledDist         uint32
	CyclesSinceLastSpawn uint16
	LocationX, LocationY uint16
	DirectionX           int8
	DirectionY           int8
	OriginalAncestorID   uint32

	// Traits
	ColorR, ColorG, ColorB float32 // render-only, precision loss is fine
	MaxSize                float64
	SpawnHealth            float64
	MinHealthToSpawn       float64
	MinCyclesBetweenSpawns uint16
	IdealPh                float64
	// Features is the bitmask of evolved physiological features
	// (physiology.Set is a uint64). Stored verbatim — no migration
	// from pre-physiology snapshots; old .pzr files won't decode.
	Features uint64

	// PhPositive / PhNegative are lifetime cumulative magnitudes the
	// organism has pushed pH up (eating) or down (chemosynthesis).
	PhPositive float64
	PhNegative float64

	// Decision tree as serialized string
	DecisionTree  string
	CurrentAction uint8

	// Lifetime attack counters used by the "MOST AGGRESSIVE"
	// highlight and the "Attacks: hits/total" display.
	AttackTotal uint32
	AttackHits  uint32
}

// FoodRecord is the serializable form of a food item.
type FoodRecord struct {
	X, Y  uint16
	Value uint16
}

// AncestorRecord stores an ancestor's ID and color.
type AncestorRecord struct {
	ID                     uint32
	ColorR, ColorG, ColorB float32
}

// DeltaPayload contains the changes for a single cycle. Reserved for a
// future per-cycle forward-delta section type; not currently written.
// (See ReverseDeltaPayload below for the actively-used structure.)
type DeltaPayload struct {
	Cycle        int
	Births       []OrganismRecord
	Deaths       []uint32
	Moves        []MoveRecord
	StateChanges []StateChangeRecord
	FoodChanges  []FoodChangeRecord
}

// MoveRecord tracks an organism that changed position or direction.
type MoveRecord struct {
	ID                     uint32
	LocationX, LocationY   uint16
	DirectionX, DirectionY int8
}

// StateChangeRecord tracks changes to an organism's health/size/action.
type StateChangeRecord struct {
	ID     uint32
	Health float32
	Size   float32
	Action uint8
}

// FoodChangeRecord tracks a food item change. Value=0 means removal.
type FoodChangeRecord struct {
	X, Y  uint16
	Value uint16
}

// HistoryPayload stores the complete pH distribution history,
// serialized once at the end of a simulation run.
type HistoryPayload struct {
	PhDistribution map[int]map[int]int32 // cycle -> bucket -> count
}

// DescendantTreesPayload contains the full descendant trees, serialized once
// at the end of a simulation run.
type DescendantTreesPayload struct {
	Trees []DescendantTreeRecord
}

// DescendantTreeRecord is a single ancestor's tree root.
type DescendantTreeRecord struct {
	AncestorID uint32
	Root       DescendantNodeRecord
}

// DescendantNodeRecord is the serializable form of a DescendantNode.
type DescendantNodeRecord struct {
	ID                     uint32
	ColorR, ColorG, ColorB float32
	StartCycle             uint32
	EndCycle               uint32
	AllBranchesDeadCycle   uint32
	Children               []DescendantNodeRecord
}
