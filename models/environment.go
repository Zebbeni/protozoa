package models

import c "../constants"

// Environment contains FoodManager
type Environment struct {
	foodManager FoodManager
}

// NewEnvironment creates FoodManager
func NewEnvironment() Environment {
	foodManager := NewFoodManager()
	return Environment{foodManager: foodManager}
}

// Update calls Update function for food manager
func (e *Environment) Update() {
	e.foodManager.Update()
}

// GetFoodAtGridLocation returns current lifespan of food item at x, y
func (e *Environment) GetFoodAtGridLocation(x, y int) int {
	value := e.foodManager.Grid[x][y]
	return value
}

// GetFoodItems returns array of all Food Items from food manager
func (e *Environment) GetFoodItems() [c.NumFood]FoodItem {
	return e.foodManager.FoodItems
}
