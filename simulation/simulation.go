package simulation

import (
	"fmt"
	"image/color"
	"time"

	c "github.com/Zebbeni/protozoa/constants"
	w "github.com/Zebbeni/protozoa/world"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

type size int

var (
	backgroundColor = color.RGBA{15, 5, 15, 255}
)

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	world     *w.World
	numCycles int

	totalUpdateDuration, totalRenderDuration time.Duration
	previousRenderFrame                      *ebiten.Image
	previousPanelFrame                       *ebiten.Image
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
	screen.Clear()
	ebitenutil.DrawRect(screen, 0, 0, float64(c.ScreenWidth), float64(c.ScreenWidth), backgroundColor)

	s.renderGrid(screen)
	s.renderPanel(screen)
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
