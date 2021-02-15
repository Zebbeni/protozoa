package utils

import (
	"image/color"
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
