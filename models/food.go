package models

import (
	"math/rand"

	c "../constants"
)

// FoodItem contains x and y value for a given food item
type FoodItem struct {
	X, Y int
}

// NewFood creates a new Food object
func NewFood() FoodItem {
	return FoodItem{X: rand.Intn(c.GridWidth), Y: rand.Intn(c.GridHeight)}
}

// FoodManager contains 2D array of all food values
type FoodManager struct {
	FoodItems [c.NumFood]FoodItem
	Grid      [c.GridWidth][c.GridHeight]int
}

// NewFoodManager initializes a new food grid with random food
func NewFoodManager() FoodManager {
	var foodItems [c.NumFood]FoodItem
	foodManager := FoodManager{FoodItems: foodItems}
	return foodManager
}

// RelocateFood sets Food's grid location to new x, y
func (fm *FoodManager) RelocateFood(index, x, y int) {
	fm.FoodItems[index].X = x
	fm.FoodItems[index].Y = y
}

// Update decrements all food on grid, moves food to new x, y if food <= 0
func (fm *FoodManager) Update() {
	for i, food := range fm.FoodItems {
		value := fm.Grid[food.X][food.Y]
		if value > 0 {
			fm.Grid[food.X][food.Y]--
		} else {
			x := rand.Intn(c.GridWidth)
			y := rand.Intn(c.GridHeight)
			fm.RelocateFood(i, x, y)
			fm.Grid[x][y] += rand.Intn(c.MaxFoodLifespan)
		}
	}
}
