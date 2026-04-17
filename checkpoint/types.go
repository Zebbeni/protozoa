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
type SnapshotPayload struct {
	Cycle                 int
	RNGState              []byte
	TotalOrganismsCreated int
	Organisms             []OrganismRecord
	OrganismGrid          [][]int
	CurrentPhMap          [][]float64
	PreviousPhMap         [][]float64
	FoodItems             []FoodRecord
	Ancestors             []AncestorRecord
}

// OrganismRecord is the serializable form of a single organism.
type OrganismRecord struct {
	ID                     int
	Age                    int
	Health                 float64
	Size                   float64
	Children               int
	TraveledDist           int
	CyclesSinceLastSpawn   int
	LocationX, LocationY   int
	DirectionX, DirectionY int
	OriginalAncestorID     int

	// Traits
	ColorR, ColorG, ColorB     float64
	MaxSize                    float64
	SpawnHealth                float64
	MinHealthToSpawn           float64
	MinCyclesBetweenSpawns     int
	ChanceToMutateDecisionTree float64
	IdealPh                    float64
	PhTolerance                float64
	PhGrowthEffect             float64

	// Decision tree as serialized string
	DecisionTree  string
	CurrentAction int
}

// FoodRecord is the serializable form of a food item.
type FoodRecord struct {
	X, Y  int
	Value int
}

// AncestorRecord stores an ancestor's ID and color.
type AncestorRecord struct {
	ID             int
	ColorR, ColorG, ColorB float64
}

// DeltaPayload contains the changes for a single cycle.
type DeltaPayload struct {
	Cycle        int
	Births       []OrganismRecord
	Deaths       []int
	Moves        []MoveRecord
	StateChanges []StateChangeRecord
	FoodChanges  []FoodChangeRecord
}

// MoveRecord tracks an organism that changed position or direction.
type MoveRecord struct {
	ID                     int
	LocationX, LocationY   int
	DirectionX, DirectionY int
}

// StateChangeRecord tracks changes to an organism's health/size/action.
type StateChangeRecord struct {
	ID     int
	Health float64
	Size   float64
	Action int
}

// FoodChangeRecord tracks a food item change. Value=0 means removal.
type FoodChangeRecord struct {
	X, Y  int
	Value int
}

// DescendantTreesPayload contains the full descendant trees, serialized once
// at the end of a simulation run.
type DescendantTreesPayload struct {
	Trees []DescendantTreeRecord
}

// DescendantTreeRecord is a single ancestor's tree root.
type DescendantTreeRecord struct {
	AncestorID int
	Root       DescendantNodeRecord
}

// DescendantNodeRecord is the serializable form of a DescendantNode.
type DescendantNodeRecord struct {
	ID                   int
	ColorR, ColorG, ColorB float64
	PhEffectColorR, PhEffectColorG, PhEffectColorB float64
	StartCycle           int
	EndCycle             int
	AllBranchesDeadCycle int
	Children             []DescendantNodeRecord
}
