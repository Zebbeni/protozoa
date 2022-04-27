package simulation

import (
	"fmt"
	d "github.com/Zebbeni/protozoa/decision"
	"image/color"
	"time"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	options *config.Options

	cycle    int
	isPaused bool

	selectedID int

	organismManager    *manager.OrganismManager
	foodManager        *manager.FoodManager
	environmentManager *manager.EnvironmentManager

	// debug statistics
	UpdateTime, EnvironmentUpdateTime, FoodUpdateTime, OrganismUpdateTime time.Duration
}

// NewSimulation returns a simulation with generated world and organisms
// cycle increments at the beginning of Update() so start at -1 to ensure
// first actions are attributed to cycle 0
func NewSimulation(options *config.Options) *Simulation {
	sim := &Simulation{
		options:  options,
		cycle:    -1,
		isPaused: false,
	}
	sim.environmentManager = manager.NewEnvironmentManager(sim)
	sim.foodManager = manager.NewFoodManager()
	sim.organismManager = manager.NewOrganismManager(sim)

	return sim
}

// Update calls Update functions for controllers in simulation
func (s *Simulation) Update() {
	if s.isPaused {
		return
	}

	s.cycle++
	start := time.Now()

	s.updateEnvironment()
	s.updateFood()
	s.updateOrganisms()

	s.UpdateTime = time.Since(start)
}

func (s *Simulation) updateEnvironment() {
	start := time.Now()
	s.environmentManager.Update()
	s.EnvironmentUpdateTime = time.Since(start)
}

func (s *Simulation) updateFood() {
	start := time.Now()
	s.foodManager.Update()
	s.FoodUpdateTime = time.Since(start)
}

func (s *Simulation) updateOrganisms() {
	start := time.Now()
	s.organismManager.Update()
	s.OrganismUpdateTime = time.Since(start)
}

// IsDone returns true if end condition met
func (s *Simulation) IsDone() bool {
	if s.GetNumOrganisms() >= config.MaxOrganisms() {
		fmt.Printf("\nSimulation ended with %d organisms alive.", config.MaxOrganisms())
		return true
	}
	return false
}

// IsDebug returns true if debug flag set on run
func (s *Simulation) IsDebug() bool {
	return s.options.IsDebugging
}

// Cycle returns the current simulation cycle number
func (s *Simulation) Cycle() int {
	return s.cycle
}

// IsPaused returns whether the simulation is currently stopped
func (s *Simulation) IsPaused() bool {
	return s.isPaused
}

// Pause sets isPaused to the given boolean value
func (s *Simulation) Pause(pause bool) {
	s.isPaused = pause
}

// GetUpdatedFoodPoints returns a map of all points recently updated by the
// foodManager
func (s *Simulation) GetUpdatedFoodPoints() map[string]utils.Point {
	return s.foodManager.GetUpdatedPoints()
}

// GetUpdatedOrganismPoints returns a map of all points recently updated by the
// organismManager
func (s *Simulation) GetUpdatedOrganismPoints() map[string]utils.Point {
	return s.organismManager.GetUpdatedPoints()
}

// GetUpdatedPhPoints returns a map of all points recently updated by the
// environmentManager
func (s *Simulation) GetUpdatedPhPoints() map[string]utils.Point {
	return s.environmentManager.GetUpdatedPoints()
}

// clearUpdatedPoints clears all updated points for all content managers
func (s *Simulation) ClearUpdatedPoints() {
	s.environmentManager.ClearUpdatedPoints()
	s.foodManager.ClearUpdatedPoints()
	s.organismManager.ClearUpdatedPoints()
}

// GetAllOrganismInfo returns a map of Info on all living organisms
func (s *Simulation) GetAllOrganismInfo() map[int]*organism.Info {
	return s.organismManager.GetAllOrganismInfo()
}

// GetOrganismInfoAtPoint returns the Organism at a given point (nil if none found)
func (s *Simulation) GetOrganismInfoAtPoint(point utils.Point) *organism.Info {
	return s.organismManager.GetOrganismInfoAtPoint(point)
}

// GetOrganismInfoByID returns the Organism Info for a given ID (nil if none)
func (s *Simulation) GetOrganismInfoByID(id int) *organism.Info {
	return s.organismManager.GetOrganismInfoByID(id)
}

// GetOrganismTraitsByID returns the Organism Traits for a given ID and whether
func (s *Simulation) GetOrganismTraitsByID(id int) (organism.Traits, bool) {
	return s.organismManager.GetOrganismTraitsByID(id)
}

// GetOrganismDecisionTreeByID returns a copy of the currently-used decision tree of the
// given organism (nil if no organism found)
func (s *Simulation) GetOrganismDecisionTreeByID(id int) *d.Tree {
	return s.organismManager.GetOrganismDecisionTreeByID(id)
}

// GetAncestorColors returns a map of all ancestors with at least one descendant
// and the ancestor's color
func (s *Simulation) GetAncestorColors() map[int]color.Color {
	return s.organismManager.GetAncestorColors()
}

// GetAncestorsSorted returns a list of all original ancestor IDs in order
func (s *Simulation) GetAncestorsSorted() []int {
	return s.organismManager.GetAncestorsSorted()
}

// GetNumOrganisms returns the total number of all living organisms in the simulation.
func (s *Simulation) GetNumOrganisms() int {
	return s.organismManager.OrganismCount()
}

// GetFoodCount returns the total number of all food items in the simulation.
func (s *Simulation) GetFoodCount() int {
	return len(s.foodManager.GetFoodItems())
}

// GetDeadCount returns the total number of organisms that have died in the simulation.
func (s *Simulation) GetDeadCount() int {
	return s.organismManager.DeadCount()
}

// GetFoodItems returns an array of all food items in grid
func (s *Simulation) GetFoodItems() map[string]*food.Item {
	return s.foodManager.GetFoodItems()
}

// CheckOrganismAtPoint returns the result of running a check against any
// Organism object found at a given Point.
func (s *Simulation) CheckOrganismAtPoint(point utils.Point, checkFunc organism.OrgCheck) bool {
	return s.organismManager.CheckOrganismAtPoint(point, checkFunc)
}

// OrganismCount returns the current number of Organisms alive in the simulation
func (s *Simulation) OrganismCount() int {
	return s.organismManager.OrganismCount()
}

// GetFoodAtPoint returns the value of any food at a given point and whether
// a food item actually exists there.
func (s *Simulation) GetFoodAtPoint(point utils.Point) *food.Item {
	return s.foodManager.GetFoodAtPoint(point)
}

// CheckFoodAtPoint returns the result of running a check against any food Item
// object found at a given Point.
func (s *Simulation) CheckFoodAtPoint(point utils.Point, checkFunc organism.FoodCheck) bool {
	item := s.foodManager.GetFoodAtPoint(point)
	return checkFunc(item)
}

// AddFoodAtPoint attempts to add a food value to a given point and returns the actual
// amount of food added.
func (s *Simulation) AddFoodAtPoint(point utils.Point, value int) int {
	return s.foodManager.AddFoodAtPoint(point, value)
}

// RemoveFoodAtPoint attempts to add a food value to a given point and returns the actual
// amount of food added.
func (s *Simulation) RemoveFoodAtPoint(point utils.Point, value int) int {
	return s.foodManager.RemoveFoodAtPoint(point, value)
}

// Select sets the currently selected organism ID. -1 if none selected
func (s *Simulation) Select(id int) {
	s.selectedID = id
}

// GetSelected returns the currently selected organism ID. -1 if none selected
func (s *Simulation) GetSelected() int {
	return s.selectedID
}

// GetPhMap returns the full 2D map of all pH values in the environment
func (s *Simulation) GetPhMap() [][]float64 {
	return s.environmentManager.GetPhMap()
}

// GetPhAtPoint returns the current Ph of the environment at a given location
func (s *Simulation) GetPhAtPoint(point utils.Point) float64 {
	return s.environmentManager.GetPhAtPoint(point)
}

// AddPhChangeAtPoint adds a given value to the environment's Ph at a given location
func (s *Simulation) AddPhChangeAtPoint(point utils.Point, change float64) {
	s.environmentManager.AddPhChangeAtPoint(point, change)
}
