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
	InitialFood         = 5000
	ChanceToAddOrganism = 0.01
	ChanceToAddFoodItem = 0.1
	MaxFoodValue        = 300
	MinFoodValue        = 2
	MaxFoodLifespan     = 100

	// Organism constants
	MaxCyclesBetweenSpawns          = 100
	MinSpawnHealth                  = 1.0
	MaxSpawnHealthPercent           = 0.5
	MinChanceToMutateDecisionTree   = 0.01
	MaxChanceToMutateDecisionTree   = 1.00
	MaxCyclesToEvaluateDecisionTree = 50
	MaxOrganismsAllowed             = 15000
	GrowthFactor                    = 0.5
	MaximumMaxSize                  = 100.0
	MinimumMaxSize                  = 10.0

	// Health change constants (as a percent of an organism's size)
	HealthChangePerCycle          = -0.0001
	HealthChangeFromBeingIdle     = +0.001
	HealthChangeFromTurning       = -0.001
	HealthChangeFromMoving        = -0.005
	HealthChangeFromEatingAttempt = -0.001
	HealthChangeFromAttacking     = -0.05
	HealthChangeInflictedByAttack = -0.10

	// Decision Tree constants
	HealthPercentToChangeDecisionTree = 0.10
	MaxNodes                          = 30
	MaxNodesToMutate                  = 4
	PercentActions                    = 0.6
	PercentConditions                 = 0.4

	// Time trial constants
	OrganismAgeToEndSimulation = 1000
	MaxCyclesToRunHeadless     = 100000

	// Reporting constants
	PopulationDifferenceToReport = 5
	PrintReportCycleInterval     = 100
)
