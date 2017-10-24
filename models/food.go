package models

import (
	"math/rand"

	u "../utils"
)

// FoodItem contains x and y value for a given food item
type FoodItem struct {
	X, Y int
}

// FoodConfig contains all attributes needed to set up FoodManager
type FoodConfig struct {
	MinFood, MaxFood, GridWidth, GridHeight int
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
	foodManager := FoodManager{config: config}
	foodManager.Grid = make([][]bool, config.GridWidth)
	for r := 0; r < config.GridWidth; r++ {
		foodManager.Grid[r] = make([]bool, config.GridHeight)
	}
	for i := 0; i < config.GridWidth; i++ {
		for j := 0; j < config.GridHeight; j++ {
			foodManager.Grid[i][j] = false
		}
	}
	foodManager.FoodItems = make([]FoodItem, config.MinFood)
	for f := 0; f < config.MinFood; f++ {
		foodItem := NewFood(config.GridWidth, config.GridHeight)
		foodManager.Grid[foodItem.X][foodItem.Y] = true
		foodManager.FoodItems[f] = foodItem
	}
	return foodManager
}

// Update checks for empty food locations. If found, creates food at new x, y
func (fm *FoodManager) Update() {
	for i, food := range fm.FoodItems {
		if !fm.Grid[food.X][food.Y] {
			if len(fm.FoodItems) < fm.config.MinFood {
				x := rand.Intn(fm.config.GridWidth)
				y := rand.Intn(fm.config.GridHeight)
				fm.FoodItems[i].X = x
				fm.FoodItems[i].Y = y
				fm.Grid[x][y] = true
			} else {
				// remove food item from list if no new foods needed
				if i < len(fm.FoodItems)-1 {
					fm.FoodItems = append(fm.FoodItems[:i], fm.FoodItems[i+1:]...)
				} else {
					fm.FoodItems = fm.FoodItems[:i]
				}
			}
		}
	}
}

// IsFoodAtLocation returns true if given (x, y) on food grid is true
func (fm *FoodManager) IsFoodAtLocation(x, y int) bool {
	width := fm.config.GridWidth
	height := fm.config.GridHeight
	return u.IsOnGrid(x, y, width, height) && fm.Grid[x][y]
}

// AddFood to a given x, y location
func (fm *FoodManager) AddFood(x, y int) {
	if len(fm.FoodItems) < fm.config.MaxFood {
		fm.FoodItems = append(fm.FoodItems, FoodItem{X: x, Y: y})
		fm.Grid[x][y] = true
	}
}

// RemoveFood for given location
func (fm *FoodManager) RemoveFood(x, y int) {
	width := fm.config.GridWidth
	height := fm.config.GridHeight
	if u.IsOnGrid(x, y, width, height) {
		fm.Grid[x][y] = false
	}
}

// GetFoodItems returns the current list of food items
func (fm *FoodManager) GetFoodItems() []FoodItem {
	return fm.FoodItems
}
