package simulation

import (
	"fmt"
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

// SimulationConfig contains all attributes needed to create a Simulation
type SimulationConfig struct {
	WorldConfig w.WorldConfig
}

func DefaultSimulationConfig() SimulationConfig {
	foodConfig := m.FoodConfig{
		NumFood:    c.NumFood,
		GridWidth:  c.GridWidth,
		GridHeight: c.GridHeight,
	}
	organismConfig := m.OrganismConfig{
		NumInitialOrganisms:         c.NumInitialOrganisms,
		MaxOrganisms:                c.MaxOrganismsAllowed,
		InitialHealth:               c.InitialHealth,
		MaxHealth:                   c.MaxHealth,
		HealthChangePerTurn:         c.HealthChangePerTurn,
		HealthChangeFromMoving:      c.HealthChangeFromMoving,
		HealthChangeFromEating:      c.HealthChangeFromEating,
		HealthChangeFromReproducing: c.HealthChangeFromReproducing,
		HealthThresholdForEating:    c.HealthThresholdForEating,
		GridWidth:                   c.GridWidth,
		GridHeight:                  c.GridHeight,
	}
	environmentConfig := m.EnvironmentConfig{
		FoodConfig: foodConfig,
	}
	worldConfig := w.WorldConfig{
		EnvironmentConfig: environmentConfig,
		OrganismConfig:    organismConfig,
	}
	config := SimulationConfig{
		WorldConfig: worldConfig,
	}
	return config
}

// NewSimulation returns a simulation with generated world and organisms
func NewSimulation(config SimulationConfig) Simulation {
	world := w.NewWorld(config.WorldConfig)
	simulation := Simulation{world: world}
	return simulation
}

// Update calls Update functions for controllers in simulation
func (s *Simulation) Update() {
	s.world.Update()
}

// IsDone returns true if end condition met
func (s *Simulation) IsDone() bool {
	if s.GetNumOrganisms() >= c.MaxOrganismsAllowed {
		fmt.Printf("\nSimulation ended with %d organisms alive.", c.MaxOrganismsAllowed)
		return true
	}
	if frames >= c.MaxCyclesToRunHeadless {
		fmt.Printf("\nSimulation ended at maximum (%d) cycles", c.MaxCyclesToRunHeadless)
		return true
	}
	if len(s.world.GetOrganisms()) <= 0 {
		fmt.Printf("\nSimulation ended. All organisms dead.", frames)
		return true
	}
	return false
}

func (s *Simulation) GetNumOrganisms() int {
	return len(s.world.GetOrganisms())
}

// Render draws all particles and forces to the screen
func (s *Simulation) Render(screen *ebiten.Image) {
	for _, food := range s.world.GetFoodItems() {
		renderFood(food, screen)
	}
	for o, organism := range s.world.GetOrganisms() {
		isBest := s.world.GetBestOrganism() == o
		renderOrganism(*organism, isBest, screen)
	}
}

// renderFood draws a food source to the screen
func renderFood(foodItem m.FoodItem, screen *ebiten.Image) {
	x := float64(foodItem.X) * c.GridUnitSize
	y := float64(foodItem.Y) * c.GridUnitSize
	ebitenutil.DrawRect(screen, x+1, y+1, c.GridUnitSize-2, c.GridUnitSize-2, foodColor)
}

// renderOrganism draws a food source to the screen
func renderOrganism(organism m.Organism, isBest bool, screen *ebiten.Image) {
	x := float64(organism.X) * c.GridUnitSize
	y := float64(organism.Y) * c.GridUnitSize
	organismColor := organism.Color
	ebitenutil.DrawRect(screen, x+0.5, y+0.5, c.GridUnitSize-1, c.GridUnitSize-1, organismColor)
}
