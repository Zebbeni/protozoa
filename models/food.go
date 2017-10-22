package models

import (
	"math/rand"
)

// FoodItem contains x and y value for a given food item
type FoodItem struct {
	X, Y int
}

// FoodConfig contains all attributes needed to set up FoodManager
type FoodConfig struct {
	NumFood, GridWidth, GridHeight int
}

// NewFood creates a new Food object
func NewFood(gridWidth, gridHeight int) FoodItem {
	return FoodItem{X: rand.Intn(gridWidth), Y: rand.Intn(gridHeight)}
}

// FoodManager contains 2D array of all food values
type FoodManager struct {
	config    FoodConfig
	FoodItems []FoodItem
	Grid      [][]bool
}

// NewFoodManager initializes a new food grid with random food
func NewFoodManager(config FoodConfig) FoodManager {
	var foodItems = make([]FoodItem, config.NumFood)
	for f := 0; f < config.NumFood; f++ {
		foodItems[f] = NewFood(config.GridWidth, config.GridHeight)
	}
	grid := make([][]bool, config.GridWidth)
	for r := 0; r < config.GridWidth; r++ {
		grid[r] = make([]bool, config.GridHeight)
	}
	for i := 0; i < config.GridWidth; i++ {
		for j := 0; j < config.GridHeight; j++ {
			grid[i][j] = false
		}
	}
	foodManager := FoodManager{config: config, FoodItems: foodItems, Grid: grid}
	return foodManager
}

// Update checks for empty food locations. If found, creates food at new x, y
func (fm *FoodManager) Update() {
	for i, food := range fm.FoodItems {
		if !fm.Grid[food.X][food.Y] {
			x := rand.Intn(fm.config.GridWidth)
			y := rand.Intn(fm.config.GridHeight)
			fm.FoodItems[i].X = x
			fm.FoodItems[i].Y = y
			fm.Grid[x][y] = true
		}
	}
}
