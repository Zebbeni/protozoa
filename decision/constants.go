package decision

type Action int

type Condition int

// Actions are listed first, then Conditions, in a single const block so every code maps to a unique int.
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
	IsHealthy
	IsHealthyPhHere
	IsHealthierPhAhead
	IsAgeMultipleOfTwo
	IsAgeMultipleOfTen
	IsWallAhead
	IsWallLeft
	IsWallRight
	CanChemosynthesizeHere
	IsRelativeAhead
	IsBiggerOrganismLeft
	IsBiggerOrganismRight
	IsRelativeLeft
	IsRelativeRight
	IsPhTooLowHere
	IsPhTooHighHere
	IsHealthAboveTwentyPercent
	// Size-RATIO reads, where IsBiggerOrganism only answers "larger at all".
	IsMuchBiggerOrganismAhead
	IsMuchBiggerOrganismLeft
	IsMuchBiggerOrganismRight
	IsMuchSmallerOrganismAhead
	IsMuchSmallerOrganismLeft
	IsMuchSmallerOrganismRight
	// Food is now eaten where an organism stands and dug up from under it.
	IsFoodHere
	IsFoodBuriedHere
	// How MUCH is here, against the asker's own size — the same idea as the organism size ratios, for food.
	IsMuchFoodHere
	IsMuchFoodBuriedHere
	// The health family, refining IsHealthy the way the pH family refines IsHealthyPhHere.
	IsVeryHealthy
	IsVeryUnhealthy
	// "Is anything at all in the cell to my left / right" — organism, wall or food.
	IsSomethingLeft
	IsSomethingRight
)

// Actions / Conditions list every code in declaration order.
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
		IsHealthy, IsHealthyPhHere, IsHealthierPhAhead,
		IsAgeMultipleOfTwo, IsAgeMultipleOfTen,
		IsWallAhead, IsWallLeft, IsWallRight,
		CanChemosynthesizeHere,
		IsRelativeAhead,
		IsBiggerOrganismLeft, IsBiggerOrganismRight,
		IsRelativeLeft, IsRelativeRight,
		IsPhTooLowHere, IsPhTooHighHere,
		IsHealthAboveTwentyPercent,
		IsMuchBiggerOrganismAhead, IsMuchBiggerOrganismLeft, IsMuchBiggerOrganismRight,
		IsMuchSmallerOrganismAhead, IsMuchSmallerOrganismLeft, IsMuchSmallerOrganismRight,
		IsFoodHere, IsFoodBuriedHere,
		IsMuchFoodHere, IsMuchFoodBuriedHere,
		IsVeryHealthy, IsVeryUnhealthy,
		IsSomethingLeft, IsSomethingRight,
	}
	// MutableActions / MutableConditions are the pools decision-tree mutation draws from.
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
		IsHealthy, IsHealthyPhHere, IsHealthierPhAhead,
		IsAgeMultipleOfTwo, IsAgeMultipleOfTen,
		IsWallAhead, IsWallLeft, IsWallRight,
		CanChemosynthesizeHere,
		IsRelativeAhead,
		IsBiggerOrganismLeft, IsBiggerOrganismRight,
		IsRelativeLeft, IsRelativeRight,
		IsPhTooLowHere, IsPhTooHighHere,
		IsMuchBiggerOrganismAhead, IsMuchBiggerOrganismLeft, IsMuchBiggerOrganismRight,
		IsMuchSmallerOrganismAhead, IsMuchSmallerOrganismLeft, IsMuchSmallerOrganismRight,
		IsFoodHere, IsFoodBuriedHere,
		IsMuchFoodHere, IsMuchFoodBuriedHere,
		IsVeryHealthy, IsVeryUnhealthy,
		IsSomethingLeft, IsSomethingRight,
	}

	Map = map[interface{}]string{
		ActAttack:              "Attack",
		ActEat:                 "Eat",
		ActChemosynthesis:      "Chemosynthesis",
		ActMove:                "Move Ahead",
		ActTurnLeft:            "Turn Left",
		ActTurnRight:           "Turn Right",
		ActSpawn:               "Spawn",
		ActIdle:                "Idle",
		ActDig:                 "Dig",
		CanMove:                "If Can Move Ahead",
		IsFoodAhead:            "If Food Ahead",
		IsFoodLeft:             "If Food Left",
		IsFoodRight:            "If Food Right",
		IsOrganismAhead:        "If Organism Ahead",
		IsBiggerOrganismAhead:  "If Bigger Organism Ahead",
		IsOrganismLeft:         "If Organism Left",
		IsOrganismRight:        "If Organism Right",
		IsHealthy:              "If Healthy",
		IsHealthyPhHere:        "If pH Safe Here",
		IsHealthierPhAhead:     "If Healthier pH Ahead",
		IsAgeMultipleOfTwo:     "If Age Divisible By 2",
		IsAgeMultipleOfTen:     "If Age Divisible By 10",
		IsWallAhead:            "If Wall Ahead",
		IsWallLeft:             "If Wall Left",
		IsWallRight:            "If Wall Right",
		CanChemosynthesizeHere: "If Can Chemosynthesize Here",
		IsRelativeAhead:        "If Relative Ahead",

		IsBiggerOrganismLeft:       "If Bigger Organism Left",
		IsBiggerOrganismRight:      "If Bigger Organism Right",
		IsRelativeLeft:             "If Relative Left",
		IsRelativeRight:            "If Relative Right",
		IsPhTooLowHere:             "If pH Too Low Here",
		IsPhTooHighHere:            "If pH Too High Here",
		IsHealthAboveTwentyPercent: "If Health Above 20%",
		IsVeryHealthy:              "If Very Healthy",
		IsVeryUnhealthy:            "If Very Unhealthy",
		IsSomethingLeft:            "If Something Left",
		IsSomethingRight:           "If Something Right",

		IsMuchBiggerOrganismAhead:  "If Much Bigger Organism Ahead",
		IsMuchBiggerOrganismLeft:   "If Much Bigger Organism Left",
		IsMuchBiggerOrganismRight:  "If Much Bigger Organism Right",
		IsMuchSmallerOrganismAhead: "If Much Smaller Organism Ahead",
		IsMuchSmallerOrganismLeft:  "If Much Smaller Organism Left",
		IsMuchSmallerOrganismRight: "If Much Smaller Organism Right",

		IsFoodHere:           "If Food Here",
		IsFoodBuriedHere:     "If Food Buried Here",
		IsMuchFoodHere:       "If Much Food Here",
		IsMuchFoodBuriedHere: "If Much Food Buried Here",
	}
)

// Names are the stable identifiers the settings file uses to enable and disable node types.
var Names = map[interface{}]string{
	ActAttack:         "ActAttack",
	ActEat:            "ActEat",
	ActChemosynthesis: "ActChemosynthesis",
	ActMove:           "ActMove",
	ActTurnLeft:       "ActTurnLeft",
	ActTurnRight:      "ActTurnRight",
	ActSpawn:          "ActSpawn",
	ActIdle:           "ActIdle",
	ActDig:            "ActDig",

	CanMove:                    "CanMove",
	IsFoodAhead:                "IsFoodAhead",
	IsFoodLeft:                 "IsFoodLeft",
	IsFoodRight:                "IsFoodRight",
	IsOrganismAhead:            "IsOrganismAhead",
	IsBiggerOrganismAhead:      "IsBiggerOrganismAhead",
	IsOrganismLeft:             "IsOrganismLeft",
	IsOrganismRight:            "IsOrganismRight",
	IsHealthy:                  "IsHealthy",
	IsHealthyPhHere:            "IsHealthyPhHere",
	IsHealthierPhAhead:         "IsHealthierPhAhead",
	IsAgeMultipleOfTwo:         "IsAgeMultipleOfTwo",
	IsAgeMultipleOfTen:         "IsAgeMultipleOfTen",
	IsWallAhead:                "IsWallAhead",
	IsWallLeft:                 "IsWallLeft",
	IsWallRight:                "IsWallRight",
	CanChemosynthesizeHere:     "CanChemosynthesizeHere",
	IsRelativeAhead:            "IsRelativeAhead",
	IsBiggerOrganismLeft:       "IsBiggerOrganismLeft",
	IsBiggerOrganismRight:      "IsBiggerOrganismRight",
	IsRelativeLeft:             "IsRelativeLeft",
	IsRelativeRight:            "IsRelativeRight",
	IsPhTooLowHere:             "IsPhTooLowHere",
	IsPhTooHighHere:            "IsPhTooHighHere",
	IsHealthAboveTwentyPercent: "IsHealthAboveTwentyPercent",
	IsVeryHealthy:              "IsVeryHealthy",
	IsVeryUnhealthy:            "IsVeryUnhealthy",
	IsSomethingLeft:            "IsSomethingLeft",
	IsSomethingRight:           "IsSomethingRight",
	IsMuchBiggerOrganismAhead:  "IsMuchBiggerOrganismAhead",
	IsMuchBiggerOrganismLeft:   "IsMuchBiggerOrganismLeft",
	IsMuchBiggerOrganismRight:  "IsMuchBiggerOrganismRight",
	IsMuchSmallerOrganismAhead: "IsMuchSmallerOrganismAhead",
	IsMuchSmallerOrganismLeft:  "IsMuchSmallerOrganismLeft",
	IsMuchSmallerOrganismRight: "IsMuchSmallerOrganismRight",
	IsFoodHere:                 "IsFoodHere",
	IsFoodBuriedHere:           "IsFoodBuriedHere",
	IsMuchFoodHere:             "IsMuchFoodHere",
	IsMuchFoodBuriedHere:       "IsMuchFoodBuriedHere",
}

func NodeName(v interface{}) string { return Names[v] }
