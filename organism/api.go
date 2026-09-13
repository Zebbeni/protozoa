package organism

import (
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/utils"
)

// FoodCheck is a true/false test to run on a given food Item
type FoodCheck func(item *food.Item, exists bool) bool

// OrgCheck is a true/false test to run on a given food Organism
type OrgCheck func(item *Organism) bool

// LookupAPI provides functions to look up items and organisms
type LookupAPI interface {
	CheckFoodAtPoint(point utils.Point, checkFunc FoodCheck) bool
	CheckOrganismAtPoint(point utils.Point, checkFunc OrgCheck) bool
	GetFoodAtPoint(point utils.Point) (*food.Item, bool)
	GetPhAtPoint(point utils.Point) float64
	GetPhMap() [][]float64
	// GetFlowAtPoint returns the environment's current ("flow")
	// vector at a point. The zero vector means still water.
	GetFlowAtPoint(point utils.Point) utils.Vector
	IsWallAtPoint(point utils.Point) bool
	OrganismCount() int
	FoodCount() int
	Cycle() int
	GetSelected() int
}

// ChangeAPI provides callback functions to make changes to the simulation
type ChangeAPI interface {
	// AddFoodAtPoint requests adding some amount of food at a Point
	AddFoodAtPoint(point utils.Point, value int)
	// RemoveFoodAtPoint requests removing some amount of food at a Point
	RemoveFoodAtPoint(point utils.Point, value int)
	// AddPhChangeAtPoint adds a positive or negative value to the environment
	// pH at a given point, bounded by the min / max pH allowed by the config
	AddPhChangeAtPoint(point utils.Point, change float64)
	// AddOrganismUpdate adds a point to the update map of noteworthy locations
	// affected by organism activity
	AddOrganismUpdate(point utils.Point)
	// AddWallStrength adjusts the wall strength at a point. Positive delta
	// reinforces, negative digs. Returns the resulting strength clamped to
	// [0, MaxWallStrength]. Reaching 0 removes the wall from the grid.
	AddWallStrength(point utils.Point, delta int) int
	// AddWallUpdate flags a wall cell as dirty so the renderer repaints
	// it on the next incremental refresh.
	AddWallUpdate(point utils.Point)
	// CirculateFlowAtPoint nudges the environment's flow vector at a
	// point toward dir by strength, clamped to unit magnitude. Pushes
	// from multiple organisms accumulate, so a colony facing the same
	// way builds a stronger current than any one of them could.
	CirculateFlowAtPoint(point utils.Point, dir utils.Point, strength float64)
}

// API provides functions needed to lookup and make changes to world objects
type API interface {
	LookupAPI
	ChangeAPI
}
