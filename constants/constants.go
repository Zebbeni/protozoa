package constants

// Constants
const (
	// Drawing constants
	GridWidth    = 350
	GridHeight   = 250
	ScreenWidth  = 1400
	ScreenHeight = 1000
	GridUnitSize = 4

	// Environment constants
	InitialFood     = 20000
	MinFood         = 2000
	MaxFood         = 20000
	MaxFoodLifespan = 100

	// Organism constants
	MinAgeToSpawn                 = 100
	MinHealthToSpawn              = .95
	NumInitialOrganisms           = 50
	MaxOrganismsAllowed           = 5000
	InitialHealth                 = 10.0
	MaxHealth                     = 100.0
	HealthChangePerCycle          = -0.5
	HealthChangeFromBeingIdle     = 0.5
	HealthChangeFromTurning       = -0.1
	HealthChangeFromMoving        = -2.0
	HealthChangeFromEatingAttempt = -0.1
	HealthChangeFromConsumingFood = 50.0
	HealthChangeFromAttacking     = -10.0
	HealthChangeFromBeingAttacked = -100.0
	HealthChangeFromReproducing   = -10.0

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
	PrintReportCycleInterval     = 100
)
