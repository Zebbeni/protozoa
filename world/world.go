package world

import (
	m "../models"
)

// World contains all attributes defining the current simulation environment
type World struct {
	foodGrid m.FoodGrid
}

// NewWorld constructs a new World objedt with newly initialized attributes
func NewWorld() World {
	foodGrid := m.NewFoodGrid()
	world := World{foodGrid: foodGrid}
	return world
}

// Update updates all attributes on world
func (world *World) Update() {
	world.foodGrid.Update()
}
