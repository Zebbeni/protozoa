package decision

// Action is the custom type for all Organism actions
type Action int

// Condition is the custom type for all Organism conditions
type Condition int

// Define all possible actions for Organism
const (
	ActAttack Action = iota
	ActFeed
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
	IsRelatedOrganismAhead
	IsOrganismLeft
	IsRelatedOrganismLeft
	IsOrganismRight
	IsRelatedOrganismRight
	IsHealthAboveFiftyPercent
	IsHealthyPhHere
	//IsRandomFiftyPercent
)

// Define slices
var (
	Actions = [...]Action{
		ActAttack,
		ActFeed,
		ActEat,
		ActChemosynthesis,
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
		IsRelatedOrganismAhead,
		IsOrganismLeft,
		IsRelatedOrganismLeft,
		IsOrganismRight,
		IsRelatedOrganismRight,
		IsHealthAboveFiftyPercent,
		IsHealthyPhHere,
		//IsRandomFiftyPercent,
	}
	Map = map[interface{}]string{
		ActAttack:                 "Attack",
		ActFeed:                   "Feed",
		ActEat:                    "Eat",
		ActChemosynthesis:         "Chemosynthesis",
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
		IsRelatedOrganismAhead:    "If Related Organism Ahead",
		IsOrganismLeft:            "If Organism Left",
		IsRelatedOrganismLeft:     "If Related Organism Left",
		IsOrganismRight:           "If Organism Right",
		IsRelatedOrganismRight:    "If Related Organism Right",
		IsHealthAboveFiftyPercent: "IsHealthAboveFiftyPercent",
		IsHealthyPhHere:           "IsHealthyPhHere",
		//IsRandomFiftyPercent:      "IsRandomFiftyPercent",
	}
)
