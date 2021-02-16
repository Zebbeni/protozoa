package world

import (
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// World contains all attributes defining the current simulation environment
type World struct {
	organism.WorldAPI

	FoodManager     *manager.FoodManager
	OrganismManager *manager.OrganismManager
}

// NewWorld constructs a new World objedt with newly initialized attributes
func NewWorld() *World {
	world := World{}
	world.FoodManager = manager.NewFoodManager()
	world.OrganismManager = manager.NewOrganismManager(&world)
	return &world
}

// Update calls Update on environment and organism manager
func (w *World) Update() {
	w.FoodManager.Update()
	w.OrganismManager.Update()
}

// GetFoodItems returns an array of all food items in grid
func (w *World) GetFoodItems() map[string]*food.Item {
	return w.FoodManager.GetFoodItems()
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

// OrganismCount returns the current number of Organisms alive in the simulation
func (w *World) OrganismCount() int {
	return w.OrganismManager.OrganismCount()
}

// GetFoodAtPoint returns the value of any food at a given point and whether
// a food item actually exists there.
func (w *World) GetFoodAtPoint(point utils.Point) (int, bool) {
	if item := w.FoodManager.GetFoodAtPoint(point); item != nil {
		return item.Value, true
	}
	return 0, false
}

// CheckFoodAtPoint returns the result of running a check against any food Item
// object found at a given Point.
func (w *World) CheckFoodAtPoint(point utils.Point, checkFunc organism.FoodCheck) bool {
	item := w.FoodManager.GetFoodAtPoint(point)
	return checkFunc(item)
}

// AddFoodAtPoint attempts to add a food value to a given point and returns the actual
// amount of food added.
func (w *World) AddFoodAtPoint(point utils.Point, value int) int {
	return w.FoodManager.AddFoodAtPoint(point, value)
}

// RemoveFoodAtPoint attempts to add a food value to a given point and returns the actual
// amount of food added.
func (w *World) RemoveFoodAtPoint(point utils.Point, value int) int {
	return w.FoodManager.RemoveFoodAtPoint(point, value)
}
