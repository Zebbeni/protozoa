package constants

// Constants
const (
	// Drawing constants

	// GridWidth    = 350
	// GridHeight   = 250
	// ScreenWidth  = 1400
	// ScreenHeight = 1000
	// GridUnitSize = 4

	GridWidth    = 280
	GridHeight   = 200
	ScreenWidth  = 1400
	ScreenHeight = 1000
	GridUnitSize = 5

	// Environment constants
	ChanceToAddOrganism = 0.05
	ChanceToAddFoodItem = 0.5
	MaxFoodValue        = 100
	MinFoodValue        = 2
	MaxFoodLifespan     = 100

	// Organism constants
	MaxCyclesBetweenSpawns          = 100
	MinSpawnHealth                  = 1.0
	MaxSpawnHealthPercent           = 0.5
	MinChanceToMutateDecisionTree   = 0.01
	MaxChanceToMutateDecisionTree   = 1.00
	MaxCyclesToEvaluateDecisionTree = 100
	MaxOrganisms                    = 20000
	GrowthFactor                    = 0.5
	MaximumMaxSize                  = 100.0
	MinimumMaxSize                  = 10.0

	// Health change constants (as a percent of an organism's size)
	HealthChangePerCycle          = -0.001
	HealthChangeFromBeingIdle     = +0.003
	HealthChangeFromTurning       = -0.001
	HealthChangeFromMoving        = -0.03
	HealthChangeFromEatingAttempt = -0.01
	HealthChangeFromAttacking     = -0.05
	HealthChangeInflictedByAttack = -0.5
	HealthChangeFromFeeding       = -0.005

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
