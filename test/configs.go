package test

import (
	c "github.com/Zebbeni/protozoa/constants"
	m "github.com/Zebbeni/protozoa/models"
	s "github.com/Zebbeni/protozoa/simulation"
	w "github.com/Zebbeni/protozoa/world"
)

// define configs to create a simple 5 x 5 simulation
const (
	gridWidth           = 5
	gridHeight          = 5
	numFood             = 10
	minFood             = 10
	maxFood             = 1000
	initialHealth       = 1
	numInitialOrganisms = 50
	maxOrganismsAllowed = 1000
)

func testFoodConfig() m.FoodConfig {
	return m.FoodConfig{
		MinFood:    minFood,
		MaxFood:    maxFood,
		GridWidth:  gridWidth,
		GridHeight: gridHeight,
	}
}

func testOrganismConfig() m.OrganismConfig {
	return m.OrganismConfig{
		NumInitialOrganisms:       numInitialOrganisms,
		MaxOrganisms:              maxOrganismsAllowed,
		InitialHealth:             initialHealth,
		MaxHealth:                 c.MaxHealth,
		HealthChangePerCycle:      c.HealthChangePerCycle,
		HealthChangeFromAttacking: c.HealthChangeFromAttacking,
		HealthChangeFromMoving:    c.HealthChangeFromMoving,
		HealthChangeFromEating:    c.HealthChangeFromEating,
		HealthChangeFromTurning:   c.HealthChangeFromTurning,
		HealthChangeFromBeingIdle: c.HealthChangeFromBeingIdle,
		GridWidth:                 gridWidth,
		GridHeight:                gridHeight,
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

func testSimulationConfig() s.Config {
	return s.Config{
		WorldConfig: testWorldConfig(),
	}
}
