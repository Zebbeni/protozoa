package models

import (
	"math/rand"

	c "../constants"
)

// FoodGrid contains 2D array of all food values
type FoodGrid struct {
	FoodItems [c.NumFood][2]int
	Grid      [c.GridWidth][c.GridHeight]int
}

// NewFoodGrid initializes a new food grid with random food
func NewFoodGrid() FoodGrid {
	var foodItems [c.NumFood][2]int
	foodGrid := FoodGrid{FoodItems: foodItems}
	return foodGrid
}

// Update decrements all food on grid. Plants food at new x, y if food <= 0
func (foodGrid *FoodGrid) Update() {
	for f, food := range foodGrid.FoodItems {
		x := food[0]
		y := food[1]
		value := foodGrid.Grid[x][y]
		if value > 0 {
			// decrement food grid item
			foodGrid.Grid[x][y]--
		} else {
			// move food to new location and reset lifespan
			x = rand.Intn(c.GridWidth)
			y = rand.Intn(c.GridHeight)
			timeToLive := rand.Intn(c.MaxFoodLifespan)
			foodGrid.Grid[x][y] += timeToLive
			foodGrid.FoodItems[f][0] = x
			foodGrid.FoodItems[f][1] = y
		}
	}
}
