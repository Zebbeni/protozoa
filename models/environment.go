package models

// Environment contains FoodManager
type Environment struct {
	FoodManager FoodManager
}

// NewEnvironment creates FoodManager
func NewEnvironment() Environment {
	foodManager := NewFoodManager()
	return Environment{FoodManager: foodManager}
}

// Update calls Update function for food manager
func (e *Environment) Update() {
	e.FoodManager.Update()
}

// GetFoodAtPoint returns the FoodItem value at a given point, and if it exists
func (e *Environment) GetFoodAtPoint(point Point) (int, bool) {
	return e.FoodManager.GetFoodAtPoint(point)
}

// GetFoodItems returns array of all Food Items from food manager
func (e *Environment) GetFoodItems() map[string]*FoodItem {
	return e.FoodManager.GetFoodItems()
}

// RemoveFood sets a food grid value to false
func (e *Environment) RemoveFood(point Point, value int) {
	e.FoodManager.RemoveFood(point, value)
}

// AddFoodAtPoint adds a FoodItem for a given value and (x, y) Point
func (e *Environment) AddFoodAtPoint(point Point, value int) {
	e.FoodManager.AddFoodItem(point, value)
}
