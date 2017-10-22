package models

import (
	u "../utils"
)

// Environment contains FoodManager
type Environment struct {
	foodManager FoodManager
}

type EnvironmentConfig struct {
	FoodConfig FoodConfig
}

// NewEnvironment creates FoodManager
func NewEnvironment(config EnvironmentConfig) Environment {
	foodManager := NewFoodManager(config.FoodConfig)
	return Environment{foodManager: foodManager}
}

// Update calls Update function for food manager
func (e *Environment) Update() {
	e.foodManager.Update()
}

// IsFoodAtGridLocation returns current lifespan of food item at x, y
func (e *Environment) IsFoodAtGridLocation(x, y int) bool {
	return u.IsOnGrid(x, y) && e.foodManager.Grid[x][y]
}

// GetFoodItems returns array of all Food Items from food manager
func (e *Environment) GetFoodItems() []FoodItem {
	return e.foodManager.FoodItems
}

// RemoveFood sets a food grid value to false
func (e *Environment) RemoveFood(x, y int) {
	if u.IsOnGrid(x, y) {
		e.foodManager.Grid[x][y] = false
	}
}
