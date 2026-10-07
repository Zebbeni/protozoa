package food

import "github.com/Zebbeni/protozoa/utils"

// API provides functions to look up or update information for the sim state
type API interface {
	AddFoodUpdate(p utils.Point)
	IsWallAtPoint(p utils.Point) bool
	// IsOrganismAtPoint reports whether a living organism is standing on a cell.
	IsOrganismAtPoint(p utils.Point) bool
}
