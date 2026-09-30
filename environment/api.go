package environment

import "github.com/Zebbeni/protozoa/utils"

// API provides functions to look up information about the sim state
type API interface {
	Cycle() int
	AddPhUpdate(p utils.Point)
	IsWallAtPoint(p utils.Point) bool
	// GetWallStrengthAtPoint is the wall's strength at p, 0 where there is none.
	GetWallStrengthAtPoint(p utils.Point) int
}
