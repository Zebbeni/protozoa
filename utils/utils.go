package utils

import (
	"image/color"
	"math"
	"math/rand"
)

// IsOnGrid returns whether a given x, y is on the simulation grid
func IsOnGrid(x, y, width, height int) bool {
	return !(x < 0 || y < 0 || x >= width || y >= height)
}

// CalcDirXForDirection returns the X vector given an angle
func CalcDirXForDirection(direction float64) int {
	cos := math.Cos(direction)
	return int(cos)
}

// CalcDirYForDirection returns the Y vector given an angle
func CalcDirYForDirection(direction float64) int {
	sin := math.Sin(direction)
	return int(sin)
}

// MutateColor returns a slight variation on a given color
func MutateColor(originalColor color.RGBA) color.RGBA {
	r := uint8(math.Max(50, math.Min(255, float64(int(originalColor.R)+rand.Intn(10)-5))))
	g := uint8(math.Max(50, math.Min(255, float64(int(originalColor.G)+rand.Intn(10)-5))))
	b := uint8(math.Max(50, math.Min(255, float64(int(originalColor.B)+rand.Intn(10)-5))))
	a := originalColor.A
	return color.RGBA{r, g, b, a}
}
