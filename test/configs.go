package test

import (
	c "../constants"
	m "../models"
	s "../simulation"
	w "../world"
)

// define configs to create a simple 5 x 5 simulation
const (
	gridWidth           = 5
	gridHeight          = 5
	numFood             = 10
	initialHealth       = 1
	numInitialOrganisms = 10
	maxOrganismsAllowed = 25
)

func testFoodConfig() m.FoodConfig {
	return m.FoodConfig{
		NumFood:    numFood,
		GridWidth:  gridWidth,
		GridHeight: gridHeight,
	}
}

func testOrganismConfig() m.OrganismConfig {
	return m.OrganismConfig{
		NumInitialOrganisms:         numInitialOrganisms,
		MaxOrganisms:                maxOrganismsAllowed,
		InitialHealth:               initialHealth,
		MaxHealth:                   c.MaxHealth,
		HealthChangePerTurn:         c.HealthChangePerTurn,
		HealthChangeFromMoving:      c.HealthChangeFromMoving,
		HealthChangeFromEating:      c.HealthChangeFromEating,
		HealthChangeFromReproducing: c.HealthChangeFromReproducing,
		HealthThresholdForEating:    c.HealthThresholdForEating,
		GridWidth:                   gridWidth,
		GridHeight:                  gridHeight,
	}
}

func testEnvironmentConfig() m.EnvironmentConfig {
	return m.EnvironmentConfig{
		FoodConfig: testFoodConfig(),
	}
}

func testWorldConfig() w.WorldConfig {
	return w.WorldConfig{
		EnvironmentConfig: testEnvironmentConfig(),
		OrganismConfig:    testOrganismConfig(),
	}
}

func testSimulationConfig() s.SimulationConfig {
	return s.SimulationConfig{
		WorldConfig: testWorldConfig(),
	}
}
