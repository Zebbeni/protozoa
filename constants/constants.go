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
	NumInitialOrganisms         = 100
	MaxOrganismsAllowed         = 1000
	InitialHealth               = 100
	MaxHealth                   = 200
	HealthChangePerTurn         = -1
	HealthChangeFromMoving      = -1
	HealthChangeFromEating      = 100
	HealthChangeFromReproducing = -50
	HealthThresholdForEating    = 0

	// Sequence constants
	MaxSequenceNodes  = 30
	MaxNodesToMutate  = 3
	PercentActions    = 0.5
	PercentConditions = 0.5

	// Time trial constants
	OrganismAgeToEndSimulation = 1000
	MaxCyclesToRunHeadless     = 10000

	// Reporting constants
	PopulationDifferenceToReport = 5
)
