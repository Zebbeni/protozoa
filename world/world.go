package world

import (
	m "github.com/Zebbeni/protozoa/models"
)

// World contains all attributes defining the current simulation environment
type World struct {
	Environment     *m.Environment
	OrganismManager *m.OrganismManager
}

// NewWorld constructs a new World objedt with newly initialized attributes
func NewWorld() World {
	environment := m.NewEnvironment()
	organismManager := m.NewOrganismManager(&environment)
	world := World{Environment: &environment, OrganismManager: &organismManager}
	return world
}

// Update calls Update on environment and organism manager
func (w *World) Update() {
	w.Environment.Update()
	w.OrganismManager.Update()
}

// GetFoodItems returns an array of all food items in grid
func (w *World) GetFoodItems() map[string]m.Point {
	return w.Environment.GetFoodItems()
}

// GetOrganisms returns an array of all Organisms in grid as well as the ID of the most
// reproductive organism currently alive.
func (w *World) GetOrganisms() (map[int]*m.Organism, int) {
	return w.OrganismManager.GetOrganisms(), w.OrganismManager.MostReproductiveCurrent.ID()
}

// GetNumOrganisms returns the current count of all organisms in the grid.
func (w *World) GetNumOrganisms() int {
	return len(w.OrganismManager.GetOrganisms())
}

// PrintStats shows various info about current simulation
func (w *World) PrintStats() {
	w.OrganismManager.PrintBest()
}
