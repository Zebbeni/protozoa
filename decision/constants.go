package decision

// Action is the custom type for all Organism actions
type Action int

// Condition is the custom type for all Organism conditions
type Condition int

// Define all possible actions for Organism.
//
// ActIdle is appended at the end of the const block so its numeric value
// sits after every Condition — this keeps previously-serialized Action /
// Condition integer values stable, so old .pzr checkpoint files still
// decode correctly.
const (
	ActAttack Action = iota
	ActEat
	ActChemosynthesis
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
	// IsRelatedOrganismAhead: removed but kept as a numeric placeholder so
	// later Condition values stay at their original ints and old .pzr
	// checkpoint files still decode correctly.
	_ Condition = iota
	IsOrganismLeft
	IsRelatedOrganismLeft
	IsOrganismRight
	IsRelatedOrganismRight
	IsHealthAboveFiftyPercent
	IsHealthyPhHere
	IsHealthierPhAhead
	IsAgeMultipleOfTwo
	IsAgeMultipleOfTen
	ActIdle Action = iota
)

// Define slices
var (
	Actions = [...]Action{
		ActAttack,
		ActEat,
		ActChemosynthesis,
		ActMove,
		ActTurnLeft,
		ActTurnRight,
		ActIdle,
		// ActSpawn <-- Leave this out since it's not something we want organisms to 'choose' to do
	}
	Conditions = [...]Condition{
		CanMove,
		IsFoodAhead,
		IsFoodLeft,
		IsFoodRight,
		IsOrganismAhead,
		IsBiggerOrganismAhead,
		IsOrganismLeft,
		IsRelatedOrganismLeft,
		IsOrganismRight,
		IsRelatedOrganismRight,
		IsHealthAboveFiftyPercent,
		IsHealthyPhHere,
		IsHealthierPhAhead,
		IsAgeMultipleOfTwo,
		IsAgeMultipleOfTen,
	}
	Map = map[interface{}]string{
		ActAttack:                 "Attack",
		ActEat:                    "Eat",
		ActChemosynthesis:         "Chemosynthesis",
		ActMove:                   "Move Ahead",
		ActTurnLeft:               "Turn Left",
		ActTurnRight:              "Turn Right",
		ActSpawn:                  "Spawn",
		ActIdle:                   "Idle",
		CanMove:                   "If Can Move Ahead",
		IsFoodAhead:               "If Food Ahead",
		IsFoodLeft:                "If Food Left",
		IsFoodRight:               "If Food Right",
		IsOrganismAhead:           "If Organism Ahead",
		IsBiggerOrganismAhead:     "If Bigger Organism Ahead",
		IsOrganismLeft:            "If Organism Left",
		IsRelatedOrganismLeft:     "If Related Organism Left",
		IsOrganismRight:           "If Organism Right",
		IsRelatedOrganismRight:    "If Related Organism Right",
		IsHealthAboveFiftyPercent: "IsHealthAboveFiftyPercent",
		IsHealthyPhHere:           "IsHealthyPhHere",
		IsHealthierPhAhead:        "IsHealthierPhAhead",
		IsAgeMultipleOfTwo:        "IsAgeMultipleOfTwo",
		IsAgeMultipleOfTen:        "IsAgeMultipleOfTen",
	}
)
