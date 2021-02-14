package utils

import (
	"image/color"
	"math"
	"math/rand"

	c "github.com/Zebbeni/protozoa/constants"
)

// IsOnGrid returns whether a given x, y is on the simulation grid
func IsOnGrid(p Point) bool {
	return !(p.X < 0 || p.Y < 0 || p.X >= c.GridWidth || p.Y >= c.GridHeight)
}

// GetRandomColor returns a random color
func GetRandomColor() color.Color {
	r := uint8(55 + rand.Intn(200))
	g := uint8(55 + rand.Intn(200))
	b := uint8(55 + rand.Intn(200))
	return color.RGBA{r, g, b, 255}
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
