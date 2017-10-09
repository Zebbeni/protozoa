package constants

// Constants
const (
	// Drawing constants
	GridWidth    = 120
	GridHeight   = 80
	GridUnitSize = 10.0
	ScreenWidth  = 1200
	ScreenHeight = 800

	// Environment constants
	NumFood         = 500
	MaxFoodLifespan = 600

	// Organism constants
	NumOrganisms           = 50
	MaxHealth              = 100
	HealthChangePerTurn    = -0.5
	HealthChangeFromMoving = -1.0
	HealthChangeFromEating = 10
	MaxSequenceConditions  = 5
)
