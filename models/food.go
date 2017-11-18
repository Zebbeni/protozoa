package models

import (
	"math/rand"
)

// FoodConfig contains all attributes needed to set up FoodManager
type FoodConfig struct {
	MinFood, MaxFood, GridWidth, GridHeight int
}

// FoodManager contains 2D array of all food values
type FoodManager struct {
	config    FoodConfig
	FoodItems map[string]Point
}

// NewFoodManager initializes a new foodItem map of MinFood
func NewFoodManager(config FoodConfig) FoodManager {
	foodManager := FoodManager{config: config}
	foodManager.FoodItems = make(map[string]Point)
	for f := 0; f < config.MinFood; f++ {
		foodManager.AddFoodItemAtRandom()
	}
	return foodManager
}

// AddFoodItemAtRandom creates a new FoodItem object at a location whre one does not
// already exist and adds it to the foodItems map
func (fm *FoodManager) AddFoodItemAtRandom() {
	for true {
		x := rand.Intn(fm.config.GridWidth)
		y := rand.Intn(fm.config.GridHeight)
		point := Point{X: x, Y: y}
		pointString := point.toString()
		if _, ok := fm.FoodItems[pointString]; !ok {
			fm.FoodItems[pointString] = point
			return
		}
	}
}

// AddFoodAtPoint adds food to a given x, y location if not already occupied
func (fm *FoodManager) AddFoodAtPoint(point Point) {
	if _, exists := fm.FoodItems[point.toString()]; !exists {
		fm.FoodItems[point.toString()] = point
	}
}

// RemoveFood for given location
func (fm *FoodManager) RemoveFood(point Point) {
	if _, exists := fm.FoodItems[point.toString()]; exists {
		delete(fm.FoodItems, point.toString())
		// replace with a new food immediately if under minimum
		if len(fm.FoodItems) < fm.config.MinFood {
			fm.AddFoodItemAtRandom()
		}
	}
}

// IsFoodAtPoint returns true if given Point(x, y) exists in FoodItems
func (fm *FoodManager) IsFoodAtPoint(point Point) bool {
	_, exists := fm.FoodItems[point.toString()]
	return exists
}

// GetFoodItems returns the current list of food items
func (fm *FoodManager) GetFoodItems() map[string]Point {
	return fm.FoodItems
}
