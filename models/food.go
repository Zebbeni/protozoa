package models

import (
	"math/rand"

	c "github.com/Zebbeni/protozoa/constants"
)

// FoodItem contains an x, y coordinate and a food value
type FoodItem struct {
	point Point
	value int
}

// Point returns FoodItem's Point value
func (f *FoodItem) Point() Point {
	return f.point
}

// Value returns FoodItem's current value
func (f *FoodItem) Value() int {
	return f.value
}

// FoodManager contains 2D array of all food values
type FoodManager struct {
	FoodItems map[string]*FoodItem
}

// NewFoodManager initializes a new foodItem map of MinFood
func NewFoodManager() FoodManager {
	foodManager := FoodManager{}
	foodManager.FoodItems = make(map[string]*FoodItem)
	for foodManager.FoodCount() < c.InitialFood {
		foodManager.AddRandomFoodItem()
	}
	return foodManager
}

// Update is called on every cycle and adds new FoodItems at a constant rate
func (fm *FoodManager) Update() {
	if rand.Float64() < c.ChanceToAddFoodItem {
		fm.AddRandomFoodItem()
	}
}

// FoodCount returns a count of all food items in the FoodManager map
func (fm *FoodManager) FoodCount() int {
	return len(fm.FoodItems)
}

// AddRandomFoodItem attempts to add a FoodItem object to a random location
// Gives up if first attempt to place food fails.
func (fm *FoodManager) AddRandomFoodItem() {
	x := rand.Intn(c.GridWidth)
	y := rand.Intn(c.GridHeight)
	value := rand.Intn(c.MaxFoodValue)
	point := Point{X: x, Y: y}
	fm.AddFoodItem(point, value)
}

// AddFoodItem adds foodItem for a given value and x, y location and if not already occupied
func (fm *FoodManager) AddFoodItem(point Point, value int) {
	if value < c.MinFoodValue {
		return
	}

	locationString := point.toString()
	if _, exists := fm.FoodItems[locationString]; exists {
		return
	}

	fm.FoodItems[locationString] = &FoodItem{
		point: point,
		value: value,
	}
}

// RemoveFood subtracts a given value from the FoodItem at a given location.
// If value is more than the current food value, remove foodItem from the map
func (fm *FoodManager) RemoveFood(point Point, value int) {
	foodItem, exists := fm.FoodItems[point.toString()]
	if !exists {
		return
	}

	foodItem.value -= value
	if foodItem.value < c.MinFoodValue {
		delete(fm.FoodItems, point.toString())
	}
}

// GetFoodAtPoint returns the FoodItem value at a given point, and if it exists
func (fm *FoodManager) GetFoodAtPoint(point Point) (int, bool) {
	if foodItem, ok := fm.FoodItems[point.toString()]; ok {
		return foodItem.value, true
	}
	return 0, false
}

// GetFoodItems returns the current list of food items
func (fm *FoodManager) GetFoodItems() map[string]*FoodItem {
	return fm.FoodItems
}
