package simulation

import (
	"image/color"

	c "../constants"
	m "../models"
	w "../world"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

var foodColor = color.RGBA{100, 255, 100, 255}
var frames = 0

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
	for o, organism := range s.world.GetOrganisms() {
		isBest := s.world.GetBestOrganism() == o
		renderOrganism(organism, isBest, screen)
	}
	// if frames%100 == 0 {
	// 	fmt.Printf("\nFrame %d", frames)
	// 	s.world.PrintStats()
	// }
	frames++
}

// renderFood draws a food source to the screen
func renderFood(foodItem m.FoodItem, screen *ebiten.Image) {
	x := float64(foodItem.X) * c.GridUnitSize
	y := float64(foodItem.Y) * c.GridUnitSize
	ebitenutil.DrawRect(screen, x+2, y+2, c.GridUnitSize-4, c.GridUnitSize-4, foodColor)
}

// renderOrganism draws a food source to the screen
func renderOrganism(organism m.Organism, isBest bool, screen *ebiten.Image) {
	x := float64(organism.X) * c.GridUnitSize
	y := float64(organism.Y) * c.GridUnitSize
	var alpha uint8
	alpha = 100 + uint8(155.0*organism.Health/100.0)
	organismColor := color.RGBA{100, 100, 255, alpha}
	if isBest {
		ebitenutil.DrawRect(screen, x-2, y-2, c.GridUnitSize+4, c.GridUnitSize+4, organismColor)
	} else {
		ebitenutil.DrawRect(screen, x+1, y+1, c.GridUnitSize-2, c.GridUnitSize-2, organismColor)
	}
}
