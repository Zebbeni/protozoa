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
	NumOrganisms           = 200
	MaxHealth              = 200
	HealthChangePerTurn    = -1
	HealthChangeFromMoving = -1
	HealthChangeFromEating = 100
	MaxSequenceConditions  = 5

	// Time trial constants
	OrganismAgeToEndSimulation = 10000
)
