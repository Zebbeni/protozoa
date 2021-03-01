package config

import (
	"encoding/json"
	"io"
)

var constants *Globals

// SetGlobals allows a one-time initialization of all globally-referenced constants
func SetGlobals(g *Globals) {
	if constants != nil {
		return
	}
	constants = g
}

func GridUnitSize() int                          { return constants.GridUnitSize }
func GridWidth() int                             { return constants.GridWidth }
func GridHeight() int                            { return constants.GridHeight }
func GridUnitsWide() int                         { return constants.GridUnitsWide }
func GridUnitsHigh() int                         { return constants.GridUnitsHigh }
func ScreenWidth() int                           { return constants.ScreenWidth }
func ScreenHeight() int                          { return constants.ScreenHeight }
func PopulationUpdateInterval() int              { return constants.PopulationUpdateInterval }
func ChanceToAddOrganism() float64               { return constants.ChanceToAddOrganism }
func ChanceToAddFoodItem() float64               { return constants.ChanceToAddFoodItem }
func MaxFoodValue() int                          { return constants.MaxFoodValue }
func MinFoodValue() int                          { return constants.MinFoodValue }
func MaxCyclesBetweenSpawns() int                { return constants.MaxCyclesBetweenSpawns }
func MinSpawnHealth() float64                    { return constants.MinSpawnHealth }
func MaxSpawnHealthPercent() float64             { return constants.MaxSpawnHealthPercent }
func MinChanceToMutateDecisionTree() float64     { return constants.MinChanceToMutateDecisionTree }
func MaxChanceToMutateDecisionTree() float64     { return constants.MaxChanceToMutateDecisionTree }
func MinCyclesToEvaluateDecisionTree() int       { return constants.MinCyclesToEvaluateDecisionTree }
func MaxCyclesToEvaluateDecisionTree() int       { return constants.MaxCyclesToEvaluateDecisionTree }
func MaxOrganisms() int                          { return constants.MaxOrganisms }
func GrowthFactor() float64                      { return constants.GrowthFactor }
func MaximumMaxSize() float64                    { return constants.MaximumMaxSize }
func MinimumMaxSize() float64                    { return constants.MinimumMaxSize }
func HealthChangePerCycle() float64              { return constants.HealthChangePerCycle }
func HealthChangeFromBeingIdle() float64         { return constants.HealthChangeFromBeingIdle }
func HealthChangeFromTurning() float64           { return constants.HealthChangeFromTurning }
func HealthChangeFromMoving() float64            { return constants.HealthChangeFromMoving }
func HealthChangeFromEatingAttempt() float64     { return constants.HealthChangeFromEatingAttempt }
func HealthChangeFromAttacking() float64         { return constants.HealthChangeFromAttacking }
func HealthChangeInflictedByAttack() float64     { return constants.HealthChangeInflictedByAttack }
func HealthChangeFromFeeding() float64           { return constants.HealthChangeFromFeeding }
func HealthPercentToChangeDecisionTree() float64 { return constants.HealthPercentToChangeDecisionTree }
func MaxDecisionTreeSize() int                   { return constants.MaxDecisionTreeSize }
func MaxDecisionTrees() int                      { return constants.MaxDecisionTrees }

type Globals struct {
	// Drawing parameters
	GridUnitSize  int `json:"grid_unit_size"`
	GridWidth     int `json:"grid_width"`
	GridHeight    int `json:"grid_height"`
	GridUnitsWide int `json:"grid_units_wide"`
	GridUnitsHigh int `json:"grid_units_high"`
	ScreenWidth   int `json:"screen_width"`
	ScreenHeight  int `json:"screen_height"`

	// Statistics parameters
	PopulationUpdateInterval int `json:"population_update_interval"`

	// Environment parameters
	ChanceToAddOrganism float64 `json:"chance_to_add_organism"`
	ChanceToAddFoodItem float64 `json:"chance_to_add_food_item"`
	MaxFoodValue        int     `json:"max_food_value"`
	MinFoodValue        int     `json:"min_food_value"`

	// Organism parameters
	MaxCyclesBetweenSpawns          int     `json:"max_cycles_between_spawns"`
	MinSpawnHealth                  float64 `json:"min_spawn_health"`
	MaxSpawnHealthPercent           float64 `json:"max_spawn_health_percent"`
	MinChanceToMutateDecisionTree   float64 `json:"min_chance_to_mutate_decision_tree"`
	MaxChanceToMutateDecisionTree   float64 `json:"max_chance_to_mutate_decision_tree"`
	MinCyclesToEvaluateDecisionTree int     `json:"min_cycles_to_evaluate_decision_tree"`
	MaxCyclesToEvaluateDecisionTree int     `json:"max_cycles_to_evaluate_decision_tree"`
	MaxOrganisms                    int     `json:"max_organisms"`
	GrowthFactor                    float64 `json:"growth_factor"`
	MaximumMaxSize                  float64 `json:"maximum_max_size"`
	MinimumMaxSize                  float64 `json:"minimum_max_size"`

	// Health parameters (percent of organism size)
	HealthChangePerCycle          float64 `json:"health_change_per_cycle"`
	HealthChangeFromBeingIdle     float64 `json:"health_change_from_being_idle"`
	HealthChangeFromTurning       float64 `json:"health_change_from_turning"`
	HealthChangeFromMoving        float64 `json:"health_change_from_moving"`
	HealthChangeFromEatingAttempt float64 `json:"health_change_from_eating_attempt"`
	HealthChangeFromAttacking     float64 `json:"health_change_from_attacking"`
	HealthChangeInflictedByAttack float64 `json:"health_change_inflicted_by_attack"`
	HealthChangeFromFeeding       float64 `json:"health_change_from_feeding"`

	// Decision tree parameters
	HealthPercentToChangeDecisionTree float64 `json:"health_percent_to_change_decision_tree"`
	MaxDecisionTreeSize               int     `json:"max_decision_tree_size"`
	MaxDecisionTrees                  int     `json:"max_decision_trees"`
}

var defaultGlobals = Globals{
	GridUnitSize:                      5,
	GridWidth:                         1000,
	GridHeight:                        800,
	GridUnitsWide:                     200,
	GridUnitsHigh:                     160,
	ScreenWidth:                       1400,
	ScreenHeight:                      800,
	PopulationUpdateInterval:          1000,
	ChanceToAddOrganism:               0.01,
	ChanceToAddFoodItem:               0.01,
	MaxFoodValue:                      100,
	MinFoodValue:                      2,
	MaxCyclesBetweenSpawns:            100,
	MinSpawnHealth:                    1.0,
	MaxSpawnHealthPercent:             0.5,
	MinChanceToMutateDecisionTree:     0.01,
	MaxChanceToMutateDecisionTree:     1.0,
	MinCyclesToEvaluateDecisionTree:   5,
	MaxCyclesToEvaluateDecisionTree:   100,
	MaxOrganisms:                      20000,
	GrowthFactor:                      0.5,
	MaximumMaxSize:                    100.0,
	MinimumMaxSize:                    10.0,
	HealthChangePerCycle:              -0.001,
	HealthChangeFromBeingIdle:         +0.002,
	HealthChangeFromTurning:           -0.001,
	HealthChangeFromMoving:            -0.02,
	HealthChangeFromEatingAttempt:     -0.01,
	HealthChangeFromAttacking:         -0.05,
	HealthChangeInflictedByAttack:     -0.5,
	HealthChangeFromFeeding:           -0.005,
	HealthPercentToChangeDecisionTree: 0.10,
	MaxDecisionTreeSize:               32,
	MaxDecisionTrees:                  5,
}

func NewGlobals() Globals {
	return defaultGlobals
}

func LoadGlobals(file io.Reader) *Globals {
	g := NewGlobals()
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&g)
	if err != nil {
		panic("failed to read globals from file")
	}
	return &g
}

func DumpGlobals(g *Globals, file io.Writer) {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		panic("failed to convert globals to json")
	}

	_, err = file.Write(data)
	if err != nil {
		panic("failed to write globals to file")
	}
}
