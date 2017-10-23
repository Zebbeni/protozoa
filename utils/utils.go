package utils

import (
	"math"
)

// IsOnGrid returns whether a given x, y is on the simulation grid
func IsOnGrid(x, y, width, height int) bool {
	return !(x < 0 || y < 0 || x >= width || y >= height)
}

// CalcDirXForDirection returns the X vector given an angle
func CalcDirXForDirection(direction float64) int {
	// fmt.Printf("\nCalculating cos for %f...", direction)
	cos := math.Cos(direction)
	// fmt.Printf("done: %f\n", cos)
	return int(cos)
}

// CalcDirYForDirection returns the Y vector given an angle
func CalcDirYForDirection(direction float64) int {
	// fmt.Printf("\nCalculating sin for %f...", direction)
	sin := math.Sin(direction)
	// fmt.Printf("done: %f\n", sin)
	return int(sin)
}
