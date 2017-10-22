package world

import (
	m "../models"
)

// World contains all attributes defining the current simulation environment
type World struct {
	environment     *m.Environment
	organismManager *m.OrganismManager
}

type WorldConfig struct {
	EnvironmentConfig m.EnvironmentConfig
	OrganismConfig    m.OrganismConfig
}

// NewWorld constructs a new World objedt with newly initialized attributes
func NewWorld(config WorldConfig) World {
	environment := m.NewEnvironment(config.EnvironmentConfig)
	organismManager := m.NewOrganismManager(&environment, config.OrganismConfig)
	world := World{environment: &environment, organismManager: &organismManager}
	return world
}

// Update calls Update on environment and organism manager
func (w *World) Update() {
	w.environment.Update()
	w.organismManager.Update()
}

// GetFoodItems returns an array of all food items in grid
func (w *World) GetFoodItems() []m.FoodItem {
	return w.environment.GetFoodItems()
}

// GetOrganisms returns an array of all Organisms in grid
func (w *World) GetOrganisms() map[int]*m.Organism {
	return w.organismManager.GetOrganisms()
}

// GetBestOrganism returns the index of the most successful organism
func (w *World) GetBestOrganism() int {
	return w.organismManager.BestOrganismCurrent
}

// GetBestOrganismAge returns the index of the most successful organism
func (w *World) GetBestOrganismAge() int {
	return w.organismManager.BestAgeAllTime
}

// PrintStats shows various info about current simulation
func (w *World) PrintStats() {
	w.organismManager.PrintBest()
}
