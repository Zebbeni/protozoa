package simulation

import (
	"fmt"
	"image/color"
	"time"

	c "github.com/Zebbeni/protozoa/constants"
	"github.com/Zebbeni/protozoa/decisions"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	w "github.com/Zebbeni/protozoa/world"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

type size int

const (
	sizeSmall size = iota
	sizeMedium
	sizeLarge
)

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	world     *w.World
	numCycles int

	totalUpdateDuration, totalRenderDuration time.Duration
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
	start := time.Now()
	s.world.Update()
	s.numCycles++
	if s.numCycles%c.PrintReportCycleInterval == 0 {
		fmt.Printf("\nCycle: %d\n", s.numCycles)
		s.world.PrintStats()
	}
	s.totalUpdateDuration = time.Since(start)
}

// IsDone returns true if end condition met
func (s *Simulation) IsDone() bool {
	if s.GetNumOrganisms() >= c.MaxOrganisms {
		fmt.Printf("\nSimulation ended with %d organisms alive.", c.MaxOrganisms)
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
	start := time.Now()
	for _, foodItem := range s.world.GetFoodItems() {
		renderFood(foodItem, screen)
	}
	organisms, mostReproductiveID := s.world.GetOrganisms()
	for _, o := range organisms {
		renderOrganism(*o, screen, mostReproductiveID)
	}
	s.totalRenderDuration = time.Since(start)
}

// TotalDuration returns the total duration to update and render a single cycle
func (s *Simulation) TotalDuration() time.Duration {
	return s.totalUpdateDuration + s.totalRenderDuration
}

// TotalUpdateDuration returns the total duration to render a single cycle
func (s *Simulation) TotalUpdateDuration() time.Duration {
	return s.totalUpdateDuration
}

// TotalRenderDuration returns the total duration to render a single cycle
func (s *Simulation) TotalRenderDuration() time.Duration {
	return s.totalRenderDuration
}

// OrganismUpdateDuration returns the total duration to update all organism actions for a single cycle
func (s *Simulation) OrganismUpdateDuration() time.Duration {
	return s.world.OrganismManager.UpdateDuration
}

// OrganismResolveDuration returns the total duration to resolve all organism actions for a single cycle
func (s *Simulation) OrganismResolveDuration() time.Duration {
	return s.world.OrganismManager.ResolveDuration
}

// renderFood draws a food source to the screen
func renderFood(item *food.Item, screen *ebiten.Image) {
	x := float64(item.Point.X) * c.GridUnitSize
	y := float64(item.Point.Y) * c.GridUnitSize
	alpha := 60
	foodColor := color.RGBA{100, 255, 100, uint8(alpha)}

	value := float64(item.Value)
	foodSize := sizeSmall
	if value < c.MaxFoodValue*0.4375 {
		foodSize = sizeSmall
	} else if value < c.MaxFoodValue*0.8125 {
		foodSize = sizeMedium
	} else {
		foodSize = sizeLarge
	}

	drawSquare(screen, x, y, foodSize, foodColor)
}

// renderOrganism draws a food source to the screen
func renderOrganism(o organism.Organism, screen *ebiten.Image, mostReproductiveID int) {
	point := o.Location.Times(c.GridUnitSize)
	x, y := float64(point.X), float64(point.Y)

	organismSize := sizeSmall
	if o.Size < c.MaximumMaxSize*0.4375 {
		organismSize = sizeSmall
	} else if o.Size < c.MaximumMaxSize*0.8125 {
		organismSize = sizeMedium
	} else {
		organismSize = sizeLarge
	}

	organismColor := o.Color()
	if o.Action() == decisions.ActAttack {
		organismColor = color.White
	}

	drawSquare(screen, x, y, organismSize, organismColor)

	if o.ID == mostReproductiveID {
		ebitenutil.DrawLine(screen, x-2, y-2, x+c.GridUnitSize+3, y-2, organismColor)                               // top
		ebitenutil.DrawLine(screen, x-2, y-2, x-2, y+c.GridUnitSize+3, organismColor)                               // left
		ebitenutil.DrawLine(screen, x-2, y+c.GridUnitSize+3, x+c.GridUnitSize+3, y+c.GridUnitSize+3, organismColor) // bottom
		ebitenutil.DrawLine(screen, x+c.GridUnitSize+3, y-2, x+c.GridUnitSize+3, y+c.GridUnitSize+3, organismColor) // right
	}
}

func drawSquare(screen *ebiten.Image, x, y float64, sz size, col color.Color) {
	padding := 1.5
	switch sz {
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
