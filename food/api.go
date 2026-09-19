package food

import "github.com/Zebbeni/protozoa/utils"

// API provides functions to look up or update information for the sim state
type API interface {
	AddFoodUpdate(p utils.Point)
	IsWallAtPoint(p utils.Point) bool
	// IsOrganismAtPoint reports whether a living organism is standing
	// on a cell. Food can't be placed on one: food blocks movement, so
	// an organism can never step onto food, and a cell holding both is
	// a state the rest of the simulation assumes cannot happen.
	IsOrganismAtPoint(p utils.Point) bool
}
