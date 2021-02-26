package config

import (
	"encoding/json"
	"io"
)

type Protozoa struct {
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
}

var defaultProtozoa Protozoa = Protozoa{
	GridUnitSize:                      5,
	GridWidth:                         1000,
	GridHeight:                        800,
	GridUnitsWide:                     200,
	GridUnitsHigh:                     160,
	ScreenWidth:                       1400,
	ScreenHeight:                      800,
	PopulationUpdateInterval:          100,
	ChanceToAddOrganism:               0.05,
	ChanceToAddFoodItem:               0.1,
	MaxFoodValue:                      100,
	MinFoodValue:                      2,
	MaxCyclesBetweenSpawns:            100,
	MinSpawnHealth:                    1.0,
	MaxSpawnHealthPercent:             0.5,
	MinChanceToMutateDecisionTree:     0.01,
	MaxChanceToMutateDecisionTree:     1.0,
	MaxCyclesToEvaluateDecisionTree:   100,
	MaxOrganisms:                      20000,
	GrowthFactor:                      0.5,
	MaximumMaxSize:                    100.0,
	MinimumMaxSize:                    10.0,
	HealthChangePerCycle:              -0.001,
	HealthChangeFromBeingIdle:         +0.003,
	HealthChangeFromTurning:           -0.001,
	HealthChangeFromMoving:            -0.03,
	HealthChangeFromEatingAttempt:     -0.01,
	HealthChangeFromAttacking:         -0.05,
	HealthChangeInflictedByAttack:     -0.5,
	HealthChangeFromFeeding:           -0.005,
	HealthPercentToChangeDecisionTree: 0.10,
}

func NewProtozoa() Protozoa {
	return defaultProtozoa
}

func LoadProtozoa(file io.Reader) *Protozoa {
	protozoa := NewProtozoa()
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&protozoa)
	if err != nil {
		panic("failed to read protozoa from file")
	}
	return &protozoa
}

func DumpProtozoa(protozoa *Protozoa, file io.Writer) {
	data, err := json.MarshalIndent(protozoa, "", "  ")
	if err != nil {
		panic("failed to convert protozoa to json")
	}

	_, err = file.Write(data)
	if err != nil {
		panic("failed to write protozoa to file")
	}
}
