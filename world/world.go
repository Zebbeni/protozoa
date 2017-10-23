package world

import (
	m "../models"
)

// World contains all attributes defining the current simulation environment
type World struct {
	Environment     *m.Environment
	OrganismManager *m.OrganismManager
}

type WorldConfig struct {
	EnvironmentConfig m.EnvironmentConfig
	OrganismConfig    m.OrganismConfig
}

// NewWorld constructs a new World objedt with newly initialized attributes
func NewWorld(config WorldConfig) World {
	environment := m.NewEnvironment(config.EnvironmentConfig)
	organismManager := m.NewOrganismManager(&environment, config.OrganismConfig)
	world := World{Environment: &environment, OrganismManager: &organismManager}
	return world
}

// Update calls Update on environment and organism manager
func (w *World) Update() {
	w.Environment.Update()
	w.OrganismManager.Update()
}

// GetFoodItems returns an array of all food items in grid
func (w *World) GetFoodItems() []m.FoodItem {
	return w.Environment.GetFoodItems()
}

// GetOrganisms returns an array of all Organisms in grid
func (w *World) GetOrganisms() map[int]*m.Organism {
	return w.OrganismManager.GetOrganisms()
}

// GetBestOrganism returns the index of the most successful organism
func (w *World) GetBestOrganism() int {
	return w.OrganismManager.BestOrganismCurrent
}

// GetBestOrganismAge returns the index of the most successful organism
func (w *World) GetBestOrganismAge() int {
	return w.OrganismManager.BestAgeAllTime
}

// PrintStats shows various info about current simulation
func (w *World) PrintStats() {
	w.OrganismManager.PrintBest()
}
