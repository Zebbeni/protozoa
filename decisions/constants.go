package decisions

// Action is the custom type for all Organism actions
type Action int

// Condition is the custom type for all Organism conditions
type Condition int

// Define all possible actions for Organism
const (
	ActAttack Action = iota
	ActEat
	ActIdle
	ActMove
	ActTurnLeft
	ActTurnRight
	CanMove Condition = iota
	IsFoodAhead
	IsFoodLeft
	IsFoodRight
	IsOrganismAhead
	IsOrganismLeft
	IsOrganismRight
	IsRandomFiftyPercent
	IsHealthAboveNinetyPercent
	IsHealthAboveFiftyPercent
	IsHealthAboveTenPercent
)

// Define slices
var (
	Actions = [...]Action{
		ActAttack,
		ActEat,
		ActIdle,
		ActMove,
		ActTurnLeft,
		ActTurnRight,
	}
	Conditions = [...]Condition{
		CanMove,
		IsFoodAhead,
		IsFoodLeft,
		IsFoodRight,
		IsOrganismAhead,
		IsOrganismLeft,
		IsOrganismRight,
		IsRandomFiftyPercent,
		IsHealthAboveNinetyPercent,
		IsHealthAboveFiftyPercent,
		IsHealthAboveTenPercent,
	}
	Map = map[interface{}]string{
		ActAttack:                  "Attack",
		ActEat:                     "Eat",
		ActIdle:                    "Be Idle",
		ActMove:                    "Move Ahead",
		ActTurnLeft:                "Turn Left",
		ActTurnRight:               "Turn Right",
		CanMove:                    "If Can Move Ahead",
		IsFoodAhead:                "If Food Ahead",
		IsFoodLeft:                 "If Food Left",
		IsFoodRight:                "If Food Right",
		IsOrganismAhead:            "If Organism Ahead",
		IsOrganismLeft:             "If Organism Left",
		IsOrganismRight:            "If Organism Right",
		IsRandomFiftyPercent:       "IsRandomFiftyPercent",
		IsHealthAboveNinetyPercent: "IsHealthAboveNinetyPercent",
		IsHealthAboveFiftyPercent:  "IsHealthAboveFiftyPercent",
		IsHealthAboveTenPercent:    "IsHealthAboveTenPercent",
	}
	ChanceOfAddingNewSubTree  = 0.5
	MinUsesToConsiderChanging = 100
	// NodeLibrary constants
	UsesToConsiderPruningMultiplier = 100
	MaxNodesAllowed                 = 200
)
