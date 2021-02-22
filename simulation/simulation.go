package simulation

import (
	"fmt"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"

	c "github.com/Zebbeni/protozoa/constants"
)

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	organismManager *manager.OrganismManager
	foodManager     *manager.FoodManager

	cycle int

	UpdatedPoints map[string]utils.Point
}

// NewSimulation returns a simulation with generated world and organisms
func NewSimulation() *Simulation {
	simulation := &Simulation{
		cycle:         0,
		UpdatedPoints: make(map[string]utils.Point),
	}
	simulation.foodManager = manager.NewFoodManager(simulation)
	simulation.organismManager = manager.NewOrganismManager(simulation)
	return simulation
}

// Update calls Update functions for controllers in simulation
func (s *Simulation) Update() {
	s.foodManager.Update()
	s.organismManager.Update()

	s.cycle++
}

// IsDone returns true if end condition met
func (s *Simulation) IsDone() bool {
	if s.GetNumOrganisms() >= c.MaxOrganisms {
		fmt.Printf("\nSimulation ended with %d organisms alive.", c.MaxOrganisms)
		return true
	}
	return false
}

// Cycle returns the current simulation cycle number
func (s *Simulation) Cycle() int {
	return s.cycle
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
