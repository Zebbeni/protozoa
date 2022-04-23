package simulation

import (
	"fmt"
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

	organismManager *manager.OrganismManager
	foodManager     *manager.FoodManager

	UpdatedPoints map[string]utils.Point

	// debug statistics
	UpdateTime, FoodUpdateTime, OrganismUpdateTime time.Duration
}

// NewSimulation returns a simulation with generated world and organisms
// cycle increments at the beginning of Update() so start at -1 to ensure
// first actions are attributed to cycle 0
func NewSimulation(options *config.Options) *Simulation {
	sim := &Simulation{
		options:       options,
		cycle:         -1,
		isPaused:      false,
		UpdatedPoints: make(map[string]utils.Point),
	}
	sim.foodManager = manager.NewFoodManager(sim)
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

	s.updateFood()
	s.updateOrganisms()

	s.UpdateTime = time.Since(start)
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

// AddUpdatedGridPoint adds a point to the grid locations that have been updated
func (s *Simulation) AddUpdatedGridPoint(point utils.Point) {
	s.UpdatedPoints[point.ToString()] = point
}

// ClearUpdatedGridPoints clears the current pointsToUpdate map
func (s *Simulation) ClearUpdatedGridPoints() {
	s.UpdatedPoints = make(map[string]utils.Point)
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

// GetHistory returns the full population history of all original ancestors as a
// map of cycles to maps of ancestorIDs to the living descendants at that time
func (s *Simulation) GetHistory() map[int]map[int]int16 {
	return s.organismManager.GetHistory()
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

// GetMostReproductiveID returns the ID of the living organism with the most children.
func (s *Simulation) GetMostReproductiveID() int {
	return s.organismManager.MostReproductiveCurrent.ID
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

// PrintStats shows various info about current simulation
func (s *Simulation) PrintStats() {
	s.organismManager.PrintBest()
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

// AddGridPointToUpdate indicates a point on the grid has been updated
// and needs to be re-rendered
func (s *Simulation) AddGridPointToUpdate(point utils.Point) {
	s.UpdatedPoints[point.ToString()] = point
}
