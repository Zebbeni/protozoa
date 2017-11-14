package constants

// Constants
const (
	// Drawing constants
	GridWidth    = 180
	GridHeight   = 120
	GridUnitSize = 6.6667
	ScreenWidth  = 1200
	ScreenHeight = 800

	// Environment constants
	MinFood         = 500
	MaxFood         = 4000
	MaxFoodLifespan = 600

	// Organism constants
	NumInitialOrganisms           = 10
	MaxOrganismsAllowed           = 5000
	InitialHealth                 = 100.0
	MaxHealth                     = 100.0
	HealthChangeFromAttacking     = -2.0
	HealthChangeFromBeingAttacked = -25.0
	HealthChangePerTurn           = -1.0
	HealthChangeFromMoving        = -1.0
	HealthChangeFromEating        = 100.0
	HealthChangeFromReproducing   = -25.0

	// Decision Tree constants
	MaxNodes          = 30
	MaxNodesToMutate  = 4
	PercentActions    = 0.6
	PercentConditions = 0.4

	// Time trial constants
	OrganismAgeToEndSimulation = 1000
	MaxCyclesToRunHeadless     = 100000

	// Reporting constants
	PopulationDifferenceToReport = 5
)
