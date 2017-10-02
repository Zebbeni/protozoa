package simulation

import (
	"image/color"

	c "../constants"
	m "../models"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

var foodColor = color.RGBA{255, 100, 100, 255}

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	foodGrid m.FoodGrid
}

// NewSimulation returns a simulation with generated particles and forces
func NewSimulation() Simulation {
	foodGrid := m.NewFoodGrid()
	simulation := Simulation{foodGrid: foodGrid}
	return simulation
}

// Update calls Update functions for all particles and forces in simulation
func (s *Simulation) Update() {
	s.foodGrid.Update()
}

// Render draws all particles and forces to the screen
func (s *Simulation) Render(screen *ebiten.Image) {
	for _, food := range s.foodGrid.FoodItems {
		renderFood(food, screen)
	}
}

// renderFood draws a food source to the screen
func renderFood(food [2]int, screen *ebiten.Image) {
	x := float64(food[0]) * c.GridUnitSize
	y := float64(food[1]) * c.GridUnitSize
	ebitenutil.DrawRect(screen, x, y, c.GridUnitSize, c.GridUnitSize, foodColor)
}
