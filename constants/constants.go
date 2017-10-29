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
	InitialHealth                 = 100
	MaxHealth                     = 100
	HealthChangeFromAttacking     = -2
	HealthChangeFromBeingAttacked = -25
	HealthChangePerTurn           = -1
	HealthChangeFromMoving        = -1
	HealthChangeFromEating        = 100
	HealthChangeFromReproducing   = -50

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
