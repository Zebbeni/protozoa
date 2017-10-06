package world

import (
	c "../constants"
)

// Controller provides functions for getting and updating a World object
type Controller struct {
	world World
}

// NewController returns a new world controller with an initialized world
func NewController() Controller {
	world := NewWorld()
	controller := Controller{world: world}
	return controller
}

// Update updates all attributes on world
func (wc *Controller) Update() {
	wc.world.Update()
}

// GetFoodItems returns a list of all food item positions in world's foodgrid
func (wc *Controller) GetFoodItems() [c.NumFood][2]int {
	return wc.world.foodGrid.FoodItems
}
