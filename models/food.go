package models

import (
	"math/rand"

	c "github.com/Zebbeni/protozoa/constants"
)

// FoodManager contains 2D array of all food values
type FoodManager struct {
	FoodItems map[string]Point
}

// NewFoodManager initializes a new foodItem map of MinFood
func NewFoodManager() FoodManager {
	foodManager := FoodManager{}
	foodManager.FoodItems = make(map[string]Point)
	for foodManager.FoodCount() < c.InitialFood {
		foodManager.AddFoodItemAtRandom()
	}
	return foodManager
}

// FoodCount returns a count of all food items in the FoodManager map
func (fm *FoodManager) FoodCount() int {
	return len(fm.FoodItems)
}

// AddFoodItemAtRandom attempts to add a FoodItem object to a random location
// Gives up if first attempt to place food fails.
func (fm *FoodManager) AddFoodItemAtRandom() {
	x := rand.Intn(c.GridWidth)
	y := rand.Intn(c.GridHeight)
	point := Point{X: x, Y: y}
	fm.AddFoodAtPoint(point)
}

// AddFoodAtPoint adds food to a given x, y location if not already occupied
func (fm *FoodManager) AddFoodAtPoint(point Point) {
	if fm.FoodCount() >= c.MaxFood {
		return
	}
	if _, exists := fm.FoodItems[point.toString()]; !exists {
		fm.FoodItems[point.toString()] = point
	}
}

// RemoveFood for given location
func (fm *FoodManager) RemoveFood(point Point) {
	if _, exists := fm.FoodItems[point.toString()]; exists {
		delete(fm.FoodItems, point.toString())
		// replace with a new food immediately if under minimum
		if fm.FoodCount() < c.MinFood {
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
