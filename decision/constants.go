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
	// IsRelatedOrganismAhead / Left / Right: removed but kept as numeric
	// placeholders so later Condition values stay at their original ints
	// and old .pzr checkpoint files still decode correctly.
	_ Condition = iota
	IsOrganismLeft
	_
	IsOrganismRight
	_
	IsHealthAboveFiftyPercent
	IsHealthyPhHere
	IsHealthierPhAhead
	IsAgeMultipleOfTwo
	IsAgeMultipleOfTen
	ActIdle Action = iota
	// Trait-tree placeholder actions. Their semantics are wired up in
	// a later slice — until then the action handler treats them as
	// no-ops. They are intentionally NOT added to the legacy Actions
	// slice; the allowed-action pool is computed per-organism from its
	// feature set (see physiology package). These constants exist so
	// the physiology Specs registry can reference them.
	ActSting
	ActDig
	ActHunker
	ActFlare
	ActHide
	ActBurrow
	// Wall-perception conditions unlocked by FeatFeelers. The
	// `Condition = iota` reassignment retypes subsequent untyped
	// entries from Action (inherited from ActIdle) back to Condition.
	// Tail-appended so all pre-existing serialized Action/Condition
	// values stay stable.
	IsWallAhead Condition = iota
	IsWallLeft
	IsWallRight
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
		IsOrganismRight,
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
		IsOrganismRight:           "If Organism Right",
		IsHealthAboveFiftyPercent: "IsHealthAboveFiftyPercent",
		IsHealthyPhHere:           "IsHealthyPhHere",
		IsHealthierPhAhead:        "IsHealthierPhAhead",
		IsAgeMultipleOfTwo:        "IsAgeMultipleOfTwo",
		IsAgeMultipleOfTen:        "IsAgeMultipleOfTen",
		ActSting:                  "Sting",
		ActDig:                    "Dig",
		ActHunker:                 "Hunker",
		ActFlare:                  "Flare",
		ActHide:                   "Hide",
		ActBurrow:                 "Burrow",
		IsWallAhead:               "If Wall Ahead",
		IsWallLeft:                "If Wall Left",
		IsWallRight:               "If Wall Right",
	}
)
