package models

// Environment contains FoodManager
type Environment struct {
	FoodManager       FoodManager
	EnvironmentConfig EnvironmentConfig
}

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
	e.FoodManager.Update()
}

// IsFoodAtGridLocation returns current lifespan of food item at x, y
func (e *Environment) IsFoodAtGridLocation(x, y int) bool {
	return e.FoodManager.IsFoodAtLocation(x, y)
}

// GetFoodItems returns array of all Food Items from food manager
func (e *Environment) GetFoodItems() []FoodItem {
	return e.FoodManager.GetFoodItems()
}

// RemoveFood sets a food grid value to false
func (e *Environment) RemoveFood(x, y int) {
	e.FoodManager.RemoveFood(x, y)
}
