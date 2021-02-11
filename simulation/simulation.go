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

type size int

const (
	sizeTiny size = iota
	sizeSmall
	sizeMedium
	sizeLarge
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
	alpha := 60
	foodColor := color.RGBA{100, 255, 100, uint8(alpha)}

	value := float64(foodItem.Value())
	foodSize := sizeTiny
	if value < c.MaxFoodValue*0.1825 {
		foodSize = sizeTiny
	} else if value < c.MaxFoodValue*0.4375 {
		foodSize = sizeSmall
	} else if value < c.MaxFoodValue*0.8125 {
		foodSize = sizeMedium
	} else {
		foodSize = sizeLarge
	}

	drawSquare(screen, x, y, foodSize, foodColor)
}

// renderOrganism draws a food source to the screen
func renderOrganism(organism m.Organism, screen *ebiten.Image, mostReproductiveID int) {
	x := float64(organism.X) * c.GridUnitSize
	y := float64(organism.Y) * c.GridUnitSize

	organismSize := sizeTiny
	if organism.Size < c.MaximumMaxSize*0.1825 {
		organismSize = sizeTiny
	} else if organism.Size < c.MaximumMaxSize*0.4375 {
		organismSize = sizeSmall
	} else if organism.Size < c.MaximumMaxSize*0.8125 {
		organismSize = sizeMedium
	} else {
		organismSize = sizeLarge
	}

	organismColor := organism.Color()
	if organism.State == m.StateAttacking {
		organismColor = color.White
	}

	drawSquare(screen, x, y, organismSize, organismColor)

	if organism.ID == mostReproductiveID {
		ebitenutil.DrawLine(screen, x-2, y-2, x+c.GridUnitSize+3, y-2, organismColor)                               // top
		ebitenutil.DrawLine(screen, x-2, y-2, x-2, y+c.GridUnitSize+3, organismColor)                               // left
		ebitenutil.DrawLine(screen, x-2, y+c.GridUnitSize+3, x+c.GridUnitSize+3, y+c.GridUnitSize+3, organismColor) // bottom
		ebitenutil.DrawLine(screen, x+c.GridUnitSize+3, y-2, x+c.GridUnitSize+3, y+c.GridUnitSize+3, organismColor) // right
	}
}

func drawSquare(screen *ebiten.Image, x, y float64, sz size, col color.Color) {
	padding := 1.5
	switch sz {
	case sizeTiny:
		padding = 2.0
		break
	case sizeSmall:
		padding = 1.5
		break
	case sizeMedium:
		padding = 1.0
		break
	case sizeLarge:
		padding = 0.5
		break
	}

	ebitenutil.DrawRect(screen, x+padding, y+padding, c.GridUnitSize-(2*padding), c.GridUnitSize-(2*padding), col)
}
