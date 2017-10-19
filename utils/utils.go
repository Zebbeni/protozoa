package utils

import (
	"math"

	c "../constants"
)

// IsOnGrid returns whether a given x, y is on the simulation grid
func IsOnGrid(x, y int) bool {
	return !(x < 0 || y < 0 || x >= c.GridWidth || y >= c.GridHeight)
}

// CalcDirXForDirection returns the X vector given an angle
func CalcDirXForDirection(direction float64) int {
	return int(math.Cos(direction))
}

// CalcDirYForDirection returns the Y vector given an angle
func CalcDirYForDirection(direction float64) int {
	return int(math.Sin(direction))
}
