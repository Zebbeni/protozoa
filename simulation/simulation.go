package simulation

import (
	"fmt"
	"image/color"

	c "github.com/Zebbeni/protozoa/constants"
	m "github.com/Zebbeni/protozoa/models"
	w "github.com/Zebbeni/protozoa/world"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	world     w.World
	numCycles int
}

// NewSimulation returns a simulation with generated world and organisms
func NewSimulation() Simulation {
	world := w.NewWorld()
	simulation := Simulation{
		world:     world,
		numCycles: 0,
	}
	return simulation
}

// Update calls Update functions for controllers in simulation
func (s *Simulation) Update() {
	s.world.Update()
	s.numCycles++
	if s.numCycles%c.PrintReportCycleInterval == 0 {
		fmt.Printf("\nCycle: %d\n", s.numCycles)
		s.world.PrintStats()
	}
}

// IsDone returns true if end condition met
func (s *Simulation) IsDone() bool {
	if s.GetNumOrganisms() >= c.MaxOrganismsAllowed {
		fmt.Printf("\nSimulation ended with %d organisms alive.", c.MaxOrganismsAllowed)
		return true
	}
	return false
}

// NumCycles returns the total number of simulated cycles
func (s *Simulation) NumCycles() int {
	return s.numCycles
}

// GetNumOrganisms returns the total number of all living organisms in the simulation.
func (s *Simulation) GetNumOrganisms() int {
	return s.world.GetNumOrganisms()
}

// GetFoodCount returns the total number of all food items in the simulation.
func (s *Simulation) GetFoodCount() int {
	return len(s.world.GetFoodItems())
}

// Render draws all particles and forces to the screen
func (s *Simulation) Render(screen *ebiten.Image) {
	for _, foodItem := range s.world.GetFoodItems() {
		renderFood(foodItem, screen)
	}
	organisms, mostReproductiveID := s.world.GetOrganisms()
	for _, organism := range organisms {
		renderOrganism(*organism, screen, mostReproductiveID)
	}
}

// renderFood draws a food source to the screen
func renderFood(foodItem *m.FoodItem, screen *ebiten.Image) {
	x := float64(foodItem.Point().X) * c.GridUnitSize
	y := float64(foodItem.Point().Y) * c.GridUnitSize
	alpha := 30.0 + ((foodItem.Value() / c.MaxFoodValue) * 30.0) // max opacity = 30 + 30
	foodColor := color.RGBA{100, 255, 100, uint8(alpha)}
	ebitenutil.DrawRect(screen, x+1, y+1, c.GridUnitSize-2, c.GridUnitSize-2, foodColor)
}

// renderOrganism draws a food source to the screen
func renderOrganism(organism m.Organism, screen *ebiten.Image, mostReproductiveID int) {
	x := float64(organism.X) * c.GridUnitSize
	y := float64(organism.Y) * c.GridUnitSize

	var padding float64
	if organism.Size < organism.MaxSize()*0.25 {
		padding = 1.25
	} else if organism.Size < organism.MaxSize()*0.5 {
		padding = 1.0
	} else if organism.Size < organism.MaxSize()*0.75 {
		padding = 0.75
	} else {
		padding = 0.5
	}

	organismColor := organism.Color()
	if organism.State == m.StateAttacking {
		organismColor = color.White
	}
	ebitenutil.DrawRect(screen, x+padding, y+padding, c.GridUnitSize-(2*padding), c.GridUnitSize-(2*padding), organismColor)

	if organism.ID == mostReproductiveID {
		ebitenutil.DrawLine(screen, x-4, y-4, x+c.GridUnitSize+5, y-4, organismColor)                               // top
		ebitenutil.DrawLine(screen, x-4, y-4, x-4, y+c.GridUnitSize+5, organismColor)                               // left
		ebitenutil.DrawLine(screen, x-4, y+c.GridUnitSize+5, x+c.GridUnitSize+5, y+c.GridUnitSize+5, organismColor) // bottom
		ebitenutil.DrawLine(screen, x+c.GridUnitSize+5, y-4, x+c.GridUnitSize+5, y+c.GridUnitSize+5, organismColor) // right
	}
}
