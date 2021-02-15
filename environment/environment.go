package environment

import (
	"github.com/Zebbeni/protozoa/food"
	u "github.com/Zebbeni/protozoa/utils"
)

// Manager updates and looks up all food in simulation
type Manager struct {
	foodManager *food.Manager
}

// NewManager creates and returns a new environment manager
func NewManager() *Manager {
	return &Manager{
		foodManager: food.NewManager(),
	}
}

// Update calls Update function for food manager
func (e *Manager) Update() {
	e.foodManager.Update()
}

// GetFoodAtPoint returns the FoodItem value at a given point, and if it exists
func (e *Manager) GetFoodAtPoint(point u.Point) *food.Item {
	return e.foodManager.GetFoodAtPoint(point)
}

// GetFoodItems returns array of all Food Items from food manager
func (e *Manager) GetFoodItems() map[string]*food.Item {
	return e.foodManager.GetFoodItems()
}

// RemoveFoodAtPoint removes an amount of food from a given point
// Returns the actual amount of food removed (0 if no food at location)
func (e *Manager) RemoveFoodAtPoint(point u.Point, value int) int {
	return e.foodManager.RemoveFoodAtPoint(point, value)
}

// AddFoodAtPoint adds a food value at a given point Point
// Returns the actual amount of food added (A food Item cannot have more than
// the max amount of food at any time, so the foodManager adds up to this value)
func (e *Manager) AddFoodAtPoint(point u.Point, value int) int {
	return e.foodManager.AddFoodAtPoint(point, value)
}
