package constants

// Constants
const (
	// Drawing constants
	GridWidth    = 280
	GridHeight   = 200
	ScreenWidth  = 1400
	ScreenHeight = 1000
	GridUnitSize = 5

	// Environment constants
	InitialFood     = 10000
	MinFood         = 1000
	MaxFood         = 20000
	MaxFoodLifespan = 100

	// Organism constants
	MaxCyclesBetweenSpawns          = 1000
	MaxInitialHealthPercent         = 0.50
	MaxHealthToSpawnPercent         = 0.98
	MaxChanceToMutateDecisionTree   = 1.00
	MaxCyclesToEvaluateDecisionTree = 200
	NumInitialOrganisms             = 20
	MaxOrganismsAllowed             = 15000
	MaxHealth                       = 100.0
	HealthChangePerCycle            = -0.1
	HealthChangeFromBeingIdle       = 0.05
	HealthChangeFromTurning         = -0.1
	HealthChangeFromMoving          = -1.0
	HealthChangeFromEatingAttempt   = -0.5
	HealthChangeFromConsumingFood   = 10.0
	HealthChangeFromAttacking       = -5.0
	HealthChangeFromBeingAttacked   = -10.0
	HealthChangeFromReproducing     = -50.0

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
