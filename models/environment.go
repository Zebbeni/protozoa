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

// For drawing

// GetFoodItems returns array of all Food Items from food manager
func (e *Environment) GetFoodItems() [c.NumFood]FoodItem {
	return e.foodManager.FoodItems
}
