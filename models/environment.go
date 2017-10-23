package models

import (
	u "../utils"
)

// Environment contains FoodManager
type Environment struct {
	foodManager       FoodManager
	EnvironmentConfig EnvironmentConfig
}

type EnvironmentConfig struct {
	FoodConfig            FoodConfig
	GridWidth, GridHeight int
}

// NewEnvironment creates FoodManager
func NewEnvironment(config EnvironmentConfig) Environment {
	foodManager := NewFoodManager(config.FoodConfig)
	return Environment{foodManager: foodManager, EnvironmentConfig: config}
}

// Update calls Update function for food manager
func (e *Environment) Update() {
	e.foodManager.Update()
}

// IsFoodAtGridLocation returns current lifespan of food item at x, y
func (e *Environment) IsFoodAtGridLocation(x, y int) bool {
	width := e.EnvironmentConfig.GridWidth
	height := e.EnvironmentConfig.GridHeight
	return u.IsOnGrid(x, y, width, height) && e.foodManager.Grid[x][y]
}

// GetFoodItems returns array of all Food Items from food manager
func (e *Environment) GetFoodItems() []FoodItem {
	return e.foodManager.FoodItems
}

// RemoveFood sets a food grid value to false
func (e *Environment) RemoveFood(x, y int) {
	width := e.EnvironmentConfig.GridWidth
	height := e.EnvironmentConfig.GridHeight
	if u.IsOnGrid(x, y, width, height) {
		e.foodManager.Grid[x][y] = false
	}
}
