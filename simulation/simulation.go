package simulation

import (
	"image/color"

	c "../constants"
	m "../models"
	w "../world"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

var organismColor = color.RGBA{150, 150, 255, 255}
var foodColor = color.RGBA{100, 255, 100, 255}

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	world w.World
}

// NewSimulation returns a simulation with generated world and organisms
func NewSimulation() Simulation {
	world := w.NewWorld()
	simulation := Simulation{world: world}
	return simulation
}

// Update calls Update functions for controllers in simulation
func (s *Simulation) Update() {
	s.world.Update()
}

// Render draws all particles and forces to the screen
func (s *Simulation) Render(screen *ebiten.Image) {
	for _, food := range s.world.GetFoodItems() {
		renderFood(food, screen)
	}
	for _, organism := range s.world.GetOrganisms() {
		renderOrganism(organism, screen)
	}
	s.world.PrintStats()
}

// renderFood draws a food source to the screen
func renderFood(foodItem m.FoodItem, screen *ebiten.Image) {
	x := float64(foodItem.X) * c.GridUnitSize
	y := float64(foodItem.Y) * c.GridUnitSize
	ebitenutil.DrawRect(screen, x, y, c.GridUnitSize, c.GridUnitSize, foodColor)
}

// renderOrganism draws a food source to the screen
func renderOrganism(organism m.Organism, screen *ebiten.Image) {
	x := float64(organism.X) * c.GridUnitSize
	y := float64(organism.Y) * c.GridUnitSize
	ebitenutil.DrawRect(screen, x, y, c.GridUnitSize, c.GridUnitSize, organismColor)
}
