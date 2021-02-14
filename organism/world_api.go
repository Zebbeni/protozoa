package organism

import (
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/utils"
)

// FoodCheck is a true/false test to run on a given food Item
type FoodCheck func(item *food.Item) bool

// OrgCheck is a true/false test to run on a given food Organism
type OrgCheck func(item *Organism) bool

// WorldLookupAPI provides functions to look up items and organisms
type WorldLookupAPI interface {
	CheckFoodAtPoint(point utils.Point, checkFunc FoodCheck) bool
	CheckOrganismAtPoint(point utils.Point, checkFunc OrgCheck) bool
}

// WorldChangeAPI provides functions to make changes
type WorldChangeAPI interface {
	// AddFoodAtPoint requests adding some amount of food at a Point
	// returns how much food was actually added
	AddFoodAtPoint(point utils.Point, value int) int
	// RemoveFoodAtPoint requests removing some amount of food at a Point
	// returns how much food was actually removed
	RemoveFoodAtPoint(point utils.Point, value int) int
}

// WorldAPI provides functions needed to lookup and make changes to world objects
type WorldAPI interface {
	WorldLookupAPI
	WorldChangeAPI
}
