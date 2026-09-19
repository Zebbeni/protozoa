package environment

import "github.com/Zebbeni/protozoa/utils"

// API provides functions to look up information about the sim state
type API interface {
	Cycle() int
	AddPhUpdate(p utils.Point)
	IsWallAtPoint(p utils.Point) bool
	// GetWallStrengthAtPoint is the wall's strength at p, 0 where there
	// is none. pH diffusion is slowed in proportion to it rather than
	// stopped outright, so a flimsy wall is nearly water and only a
	// strong one seals.
	GetWallStrengthAtPoint(p utils.Point) int
}
