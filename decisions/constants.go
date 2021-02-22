package decisions

// Action is the custom type for all Organism actions
type Action int

// Condition is the custom type for all Organism conditions
type Condition int

// Define all possible actions for Organism
const (
	ActAttack Action = iota
	ActFeed
	ActEat
	ActIdle
	ActMove
	ActTurnLeft
	ActTurnRight
	ActSpawn
	CanMove Condition = iota
	IsFoodAhead
	IsFoodLeft
	IsFoodRight
	IsOrganismAhead
	IsBiggerOrganismAhead
	IsSmallerOrganismAhead
	IsRelatedOrganismAhead
	IsOrganismLeft
	IsBiggerOrganismLeft
	IsSmallerOrganismLeft
	IsRelatedOrganismLeft
	IsOrganismRight
	IsBiggerOrganismRight
	IsSmallerOrganismRight
	IsRelatedOrganismRight
	IsRandomFiftyPercent
	IsHealthAboveFiftyPercent
)

// Define slices
var (
	Actions = [...]Action{
		ActAttack,
		ActFeed,
		ActEat,
		ActIdle,
		ActMove,
		ActTurnLeft,
		ActTurnRight,
		// ActSpawn <-- Leave this out since it's not something we want organisms to 'choose' to do
	}
	Conditions = [...]Condition{
		CanMove,
		IsFoodAhead,
		IsFoodLeft,
		IsFoodRight,
		IsOrganismAhead,
		IsBiggerOrganismAhead,
		IsSmallerOrganismAhead,
		IsRelatedOrganismAhead,
		IsOrganismLeft,
		IsBiggerOrganismLeft,
		IsSmallerOrganismLeft,
		IsRelatedOrganismLeft,
		IsOrganismRight,
		IsBiggerOrganismRight,
		IsSmallerOrganismRight,
		IsRelatedOrganismRight,
		IsRandomFiftyPercent,
		IsHealthAboveFiftyPercent,
	}
	Map = map[interface{}]string{
		ActAttack:                 "Attack",
		ActFeed:                   "Feed",
		ActEat:                    "Eat",
		ActIdle:                   "Be Idle",
		ActMove:                   "Move Ahead",
		ActTurnLeft:               "Turn Left",
		ActTurnRight:              "Turn Right",
		ActSpawn:                  "Spawn",
		CanMove:                   "If Can Move Ahead",
		IsFoodAhead:               "If Food Ahead",
		IsFoodLeft:                "If Food Left",
		IsFoodRight:               "If Food Right",
		IsOrganismAhead:           "If Organism Ahead",
		IsBiggerOrganismAhead:     "If Bigger Organism Ahead",
		IsSmallerOrganismAhead:    "If Smaller Organism Ahead",
		IsRelatedOrganismAhead:    "If Related Organism Ahead",
		IsOrganismLeft:            "If Organism Left",
		IsBiggerOrganismLeft:      "If Bigger Organism Left",
		IsSmallerOrganismLeft:     "If Smaller Organism Left",
		IsRelatedOrganismLeft:     "If Related Organism Left",
		IsOrganismRight:           "If Organism Right",
		IsBiggerOrganismRight:     "If Bigger Organism Right",
		IsSmallerOrganismRight:    "If Smaller Organism Right",
		IsRelatedOrganismRight:    "If Related Organism Right",
		IsRandomFiftyPercent:      "IsRandomFiftyPercent",
		IsHealthAboveFiftyPercent: "IsHealthAboveFiftyPercent",
	}
	ChanceOfAddingNewSubTree  = 0.5
	MinUsesToConsiderChanging = 100
	// NodeLibrary constants
	UsesToConsiderPruningMultiplier = 200
	MaxTopLevelNodes                = 50
	MinTopLevelUsesToConsiderPrune  = 25
)
