package world

import (
	c "../constants"
	m "../models"
)

// World contains all attributes defining the current simulation environment
type World struct {
	environment     *m.Environment
	organismManager *m.OrganismManager
}

// NewWorld constructs a new World objedt with newly initialized attributes
func NewWorld() World {
	environment := m.NewEnvironment()
	organismManager := m.NewOrganismManager(&environment)
	world := World{environment: &environment, organismManager: &organismManager}
	return world
}

// Update calls Update on environment and organism manager
func (w *World) Update() {
	w.environment.Update()
	w.organismManager.Update()
}

// GetFoodItems returns an array of all food items in grid
func (w *World) GetFoodItems() [c.NumFood]m.FoodItem {
	return w.environment.GetFoodItems()
}

// GetOrganisms returns an array of all Organisms in grid
func (w *World) GetOrganisms() [c.NumOrganisms]m.Organism {
	return w.organismManager.GetOrganisms()
}

// PrintStats shows various info about current simulation
func (w *World) PrintStats() {
	w.organismManager.PrintMaxScore()
}
