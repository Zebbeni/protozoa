package models

// Environment contains FoodManager
type Environment struct {
	FoodManager       FoodManager
	EnvironmentConfig EnvironmentConfig
}

// EnvironmentConfig contains all config settings needed for Environment
type EnvironmentConfig struct {
	FoodConfig            FoodConfig
	GridWidth, GridHeight int
}

// NewEnvironment creates FoodManager
func NewEnvironment(config EnvironmentConfig) Environment {
	foodManager := NewFoodManager(config.FoodConfig)
	return Environment{FoodManager: foodManager, EnvironmentConfig: config}
}

// Update calls Update function for food manager
func (e *Environment) Update() {
	// TODO: make temperature or something change periodically
}

// IsFoodAtPoint returns current lifespan of food item at x, y
func (e *Environment) IsFoodAtPoint(point Point) bool {
	return e.FoodManager.IsFoodAtPoint(point)
}

// GetFoodItems returns array of all Food Items from food manager
func (e *Environment) GetFoodItems() map[string]Point {
	return e.FoodManager.GetFoodItems()
}

// RemoveFood sets a food grid value to false
func (e *Environment) RemoveFood(point Point) {
	e.FoodManager.RemoveFood(point)
}

// AddFoodAtPoint adds a food item on a given (x, y) Point
func (e *Environment) AddFoodAtPoint(point Point) {
	e.FoodManager.AddFoodAtPoint(point)
}
