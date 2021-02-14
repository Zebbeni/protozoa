package organism

import (
	"math"

	"github.com/Zebbeni/protozoa/decisions"
)

// State defines the type of action Organism is doing
type State int

// Define Organism States
const (
	StateAttacking State = iota
	StateFeeding
	StateIdle
	StateMoving
	StateTurning
	StateEating
	StateReproducing

	LeftTurnAngle  = math.Pi / 2.0
	RightTurnAngle = -1.0 * (math.Pi / 2.0)
)

var (
	ActionStateMap = map[decisions.Action]State{}
)
