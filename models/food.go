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
	Grid      [c.GridWidth][c.GridHeight]bool
}

// NewFoodManager initializes a new food grid with random food
func NewFoodManager() FoodManager {
	var foodItems [c.NumFood]FoodItem
	var grid [c.GridWidth][c.GridHeight]bool
	for i := 0; i < c.GridWidth; i++ {
		for j := 0; j < c.GridHeight; j++ {
			grid[i][j] = false
		}
	}
	foodManager := FoodManager{FoodItems: foodItems, Grid: grid}
	return foodManager
}

// Update checks for empty food locations. If found, creates food at new x, y
func (fm *FoodManager) Update() {
	for i, food := range fm.FoodItems {
		if !fm.Grid[food.X][food.Y] {
			x := rand.Intn(c.GridWidth)
			y := rand.Intn(c.GridHeight)
			fm.FoodItems[i].X = x
			fm.FoodItems[i].Y = y
			fm.Grid[x][y] = true
		}
	}
}
