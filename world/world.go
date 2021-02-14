package world

import (
	"github.com/Zebbeni/protozoa/environment"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// World contains all attributes defining the current simulation environment
type World struct {
	organism.WorldAPI

	EnvironmentManager *environment.Manager
	OrganismManager    *organism.Manager
}

// NewWorld constructs a new World objedt with newly initialized attributes
func NewWorld() *World {
	world := World{}
	world.EnvironmentManager = environment.NewManager()
	world.OrganismManager = organism.NewManager(&world)
	return &world
}

// Update calls Update on environment and organism manager
func (w *World) Update() {
	w.EnvironmentManager.Update()
	w.OrganismManager.Update()
}

// GetFoodItems returns an array of all food items in grid
func (w *World) GetFoodItems() map[string]*food.Item {
	return w.EnvironmentManager.GetFoodItems()
}

// GetOrganisms returns an array of all Organisms in grid as well as the ID of the most
// reproductive organism currently alive.
func (w *World) GetOrganisms() (map[int]*organism.Organism, int) {
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

// CheckOrganismAtPoint returns the result of running a check against any
// Organism object found at a given Point.
func (w *World) CheckOrganismAtPoint(point utils.Point, checkFunc organism.OrgCheck) bool {
	return w.OrganismManager.CheckOrganismAtPoint(point, checkFunc)
}

// CheckFoodAtPoint returns the result of running a check against any food Item
// object found at a given Point.
func (w *World) CheckFoodAtPoint(point utils.Point, checkFunc organism.FoodCheck) bool {
	item := w.EnvironmentManager.GetFoodAtPoint(point)
	return checkFunc(item)
}

// AddFoodAtPoint attempts to add a food value to a given point and returns the actual
// amount of food added.
func (w *World) AddFoodAtPoint(point utils.Point, value int) int {
	return w.EnvironmentManager.AddFoodAtPoint(point, value)
}

// RemoveFoodAtPoint attempts to add a food value to a given point and returns the actual
// amount of food added.
func (w *World) RemoveFoodAtPoint(point utils.Point, value int) int {
	return w.EnvironmentManager.RemoveFoodAtPoint(point, value)
}
