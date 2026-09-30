package checkpoint

type FileHeader struct {
	Seed               uint64
	CheckpointInterval int
	GridUnitsWide      int
	GridUnitsHigh      int
	// Config is the JSON-encoded config.Globals the simulation ran with (seed resolved).
	Config []byte
}

// SnapshotEntry maps a cycle to its file offset for fast seeking.
type SnapshotEntry struct {
	Cycle      int
	FileOffset int64
}

// SnapshotPayload contains the full simulation state at a checkpoint.
type SnapshotPayload struct {
	Cycle                 int
	RNGState              []byte
	TotalOrganismsCreated int
	// MinOrganismsArmed records whether the population has already grown to twice MinOrganisms.
	MinOrganismsArmed bool
	Organisms         []OrganismRecord
	OrganismGrid      [][]int
	// CurrentPhMap and PreviousPhMap are stored as float64 to preserve the simulation's internal precision exactly.
	CurrentPhMap  [][]float64
	PreviousPhMap [][]float64
	FoodItems     []FoodRecord
	// BuriedFood is the food that has settled out of reach, by cell.
	BuriedFood []FoodRecord
	Walls      []WallRecord
	Ancestors  []AncestorRecord
}

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

	ColorR, ColorG, ColorB float32 // render-only, precision loss is fine
	MaxSize                float64
	SpawnHealth            float64
	MinHealthToSpawn       float64
	MinCyclesBetweenSpawns uint16
	IdealPh                float64

	// PhPositive / PhNegative are lifetime cumulative magnitudes the organism has pushed pH up (eating) or down (chemosynthesis).
	PhPositive float64
	PhNegative float64

	DecisionTree  string
	CurrentAction uint8
	// Status is the resolved outcome of the organism's most recent cycle (see organism.Status).
	Status uint8

	// Lifetime attack counters used by the "MOST AGGRESSIVE" highlight and the "Attacks: hits/total" display.
	AttackTotal uint32
	AttackHits  uint32

	Abilities AbilityScores
}

// AbilityScores is the serializable form of one organism's ability distribution, in physiology.AllAbilities order.
type AbilityScores [7]uint8

type FoodRecord struct {
	X, Y  uint16
	Value uint16
}

// WallRecord is the serializable form of one wall cell — its location and strength.
type WallRecord struct {
	X, Y     uint16
	Strength uint8
}

type AncestorRecord struct {
	ID                     uint32
	ColorR, ColorG, ColorB float32
}

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

type FoodChangeRecord struct {
	X, Y  uint16
	Value uint16
}

type HistoryPayload struct {
	PhDistribution map[int]map[int]int32 // cycle -> bucket -> count
	Food           map[int]map[int]int32 // cycle -> 0 -> total food items
	Walls          map[int]map[int]int32 // cycle -> 0 -> total wall cells
	// BuriedFood was added after Food and Walls.
	BuriedFood map[int]map[int]int32 // cycle -> 0 -> total cells holding buried food
}

// DescendantTreesPayload contains the full descendant trees, serialized once at the end of a simulation run.
type DescendantTreesPayload struct {
	Trees []DescendantTreeRecord
}

type DescendantTreeRecord struct {
	AncestorID uint32
	Root       DescendantNodeRecord
}

type DescendantNodeRecord struct {
	ID                     uint32
	ColorR, ColorG, ColorB float32
	// Abilities lets the population graph colour dead organisms by ability after a load, the same as live ones.
	Abilities            AbilityScores
	StartCycle           uint32
	EndCycle             uint32
	AllBranchesDeadCycle uint32
	Children             []DescendantNodeRecord
}
