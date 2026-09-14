package decision

// Action is the custom type for all Organism actions
type Action int

// Condition is the custom type for all Organism conditions
type Condition int

// Actions are listed first, then Conditions, in a single const block so
// every code maps to a unique int. The int values are the on-the-wire
// codes for serialized decision trees and so must stay stable across
// builds — adding a new entry means tail-appending it within its type
// section (a new Action after ActDig, or a new Condition after
// IsRelativeAhead) so the existing ints are unchanged.
const (
	ActAttack Action = iota
	ActEat
	ActChemosynthesis
	ActMove
	ActTurnLeft
	ActTurnRight
	ActSpawn
	ActIdle
	ActDig

	CanMove Condition = iota
	IsFoodAhead
	IsFoodLeft
	IsFoodRight
	IsOrganismAhead
	IsBiggerOrganismAhead
	IsOrganismLeft
	IsOrganismRight
	IsHealthAboveFiftyPercent
	IsHealthyPhHere
	IsHealthierPhAhead
	IsAgeMultipleOfTwo
	IsAgeMultipleOfTen
	IsWallAhead
	IsWallLeft
	IsWallRight
	CanChemosynthesizeHere
	IsRelativeAhead
)

// Actions / Conditions list every code in declaration order. They feed
// codeToNodeType (see node.go) so the serialized int → typed value
// round-trip stays exact, and they double as the canonical iteration
// order for the labels Map below.
//
// These are registration tables, not mutation pools: mutation draws
// from MutableActions / MutableConditions below, which deliberately
// exclude ActSpawn.
var (
	Actions = [...]Action{
		ActAttack, ActEat, ActChemosynthesis,
		ActMove, ActTurnLeft, ActTurnRight,
		ActSpawn, ActIdle,
		ActDig,
	}
	Conditions = [...]Condition{
		CanMove,
		IsFoodAhead, IsFoodLeft, IsFoodRight,
		IsOrganismAhead, IsBiggerOrganismAhead,
		IsOrganismLeft, IsOrganismRight,
		IsHealthAboveFiftyPercent, IsHealthyPhHere, IsHealthierPhAhead,
		IsAgeMultipleOfTwo, IsAgeMultipleOfTen,
		IsWallAhead, IsWallLeft, IsWallRight,
		CanChemosynthesizeHere,
		IsRelativeAhead,
	}
	// MutableActions / MutableConditions are the pools decision-tree
	// mutation draws from. Distinct from the Actions / Conditions
	// registration tables above, which must list every code so
	// serialization can round-trip.
	//
	// ActSpawn is deliberately absent: spawning is driven by the
	// organism's own health and cycle thresholds, not chosen from the
	// tree. Letting mutation pick it would hand every lineage
	// voluntary reproduction, which is a different simulation.
	MutableActions = []Action{
		ActAttack, ActEat, ActChemosynthesis,
		ActMove, ActTurnLeft, ActTurnRight,
		ActIdle,
		ActDig,
	}
	MutableConditions = []Condition{
		CanMove,
		IsFoodAhead, IsFoodLeft, IsFoodRight,
		IsOrganismAhead, IsBiggerOrganismAhead,
		IsOrganismLeft, IsOrganismRight,
		IsHealthAboveFiftyPercent, IsHealthyPhHere, IsHealthierPhAhead,
		IsAgeMultipleOfTwo, IsAgeMultipleOfTen,
		IsWallAhead, IsWallLeft, IsWallRight,
		CanChemosynthesizeHere,
		IsRelativeAhead,
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
		ActDig:                    "Dig",
		CanMove:                   "If Can Move Ahead",
		IsFoodAhead:               "If Food Ahead",
		IsFoodLeft:                "If Food Left",
		IsFoodRight:               "If Food Right",
		IsOrganismAhead:           "If Organism Ahead",
		IsBiggerOrganismAhead:     "If Bigger Organism Ahead",
		IsOrganismLeft:            "If Organism Left",
		IsOrganismRight:           "If Organism Right",
		IsHealthAboveFiftyPercent: "IsHealthAboveFiftyPercent",
		IsHealthyPhHere:           "If pH Safe Here",
		IsHealthierPhAhead:        "IsHealthierPhAhead",
		IsAgeMultipleOfTwo:        "IsAgeMultipleOfTwo",
		IsAgeMultipleOfTen:        "IsAgeMultipleOfTen",
		IsWallAhead:               "If Wall Ahead",
		IsWallLeft:                "If Wall Left",
		IsWallRight:               "If Wall Right",
		CanChemosynthesizeHere:    "If Can Chemosynthesize Here",
		IsRelativeAhead:           "If Relative Ahead",
	}
)
