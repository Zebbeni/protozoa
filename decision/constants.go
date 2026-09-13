package decision

// Action is the custom type for all Organism actions
type Action int

// Condition is the custom type for all Organism conditions
type Condition int

// Actions are listed first, then Conditions, in a single const block so
// every code maps to a unique int. The int values are the on-the-wire
// codes for serialized decision trees and so must stay stable across
// builds — adding a new entry means tail-appending it within its type
// section (a new Action after ActCirculate, or a new Condition after
// IsCurrentAligned) so the existing ints are unchanged.
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
	ActHunker
	ActFlare
	ActHide
	ActCirculate

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
	IsCurrentAligned
)

// Actions / Conditions list every code in declaration order. They feed
// codeToNodeType (see node.go) so the serialized int → typed value
// round-trip stays exact, and they double as the canonical iteration
// order for the labels Map below. Decision-tree mutation does NOT draw
// from these slices — it draws from the per-organism allowed pool the
// physiology layer hands MutateTree (see physiology.Set.AllowedActions /
// AllowedConditions). These are registration tables, not mutation
// pools.
var (
	Actions = [...]Action{
		ActAttack, ActEat, ActChemosynthesis,
		ActMove, ActTurnLeft, ActTurnRight,
		ActSpawn, ActIdle,
		ActDig, ActHunker, ActFlare, ActHide,
		ActCirculate,
	}
	Conditions = [...]Condition{
		CanMove,
		IsFoodAhead, IsFoodLeft, IsFoodRight,
		IsOrganismAhead, IsBiggerOrganismAhead,
		IsOrganismLeft, IsOrganismRight,
		IsHealthAboveFiftyPercent, IsHealthyPhHere, IsHealthierPhAhead,
		IsAgeMultipleOfTwo, IsAgeMultipleOfTen,
		IsWallAhead, IsWallLeft, IsWallRight,
		IsCurrentAligned,
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
		ActHunker:                 "Hunker",
		ActFlare:                  "Flare",
		ActHide:                   "Hide",
		ActCirculate:              "Circulate",
		CanMove:                   "If Can Move Ahead",
		IsFoodAhead:               "If Food Ahead",
		IsFoodLeft:                "If Food Left",
		IsFoodRight:               "If Food Right",
		IsOrganismAhead:           "If Organism Ahead",
		IsBiggerOrganismAhead:     "If Bigger Organism Ahead",
		IsOrganismLeft:            "If Organism Left",
		IsOrganismRight:           "If Organism Right",
		IsHealthAboveFiftyPercent: "IsHealthAboveFiftyPercent",
		IsHealthyPhHere:           "IsHealthyPhHere",
		IsHealthierPhAhead:        "IsHealthierPhAhead",
		IsAgeMultipleOfTwo:        "IsAgeMultipleOfTwo",
		IsAgeMultipleOfTen:        "IsAgeMultipleOfTen",
		IsWallAhead:               "If Wall Ahead",
		IsWallLeft:                "If Wall Left",
		IsWallRight:               "If Wall Right",
		IsCurrentAligned:          "If Current Aligned",
	}
)
