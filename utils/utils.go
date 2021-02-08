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

// GetRandomDirection returns a random direction and its x, y components
func GetRandomDirection() (float64, int, int) {
	direction := math.Floor(rand.Float64()*4.0) * math.Pi / 2.0
	dirX := CalcDirXForDirection(direction)
	dirY := CalcDirYForDirection(direction)
	return direction, dirX, dirY
}

// GetRandomColor returns a random color
func GetRandomColor() color.Color {
	r := uint8(55 + rand.Intn(200))
	g := uint8(55 + rand.Intn(200))
	b := uint8(55 + rand.Intn(200))
	return color.RGBA{r, g, b, 255}
}

// MutateColor returns a slight variation on a given color
func MutateColor(originalColor color.Color) color.Color {
	r32, g32, b32, a32 := originalColor.RGBA()
	r := mutateColorValue(r32)
	g := mutateColorValue(g32)
	b := mutateColorValue(b32)
	a := uint8(a32)
	return color.RGBA{r, g, b, a}
}

func mutateColorValue(v uint32) uint8 {
	converted := int(uint8(v)) // cast to uint8 and back to int to avoid overflow
	mutated := math.Max(math.Min(float64(converted+rand.Intn(21)-10), 255), 50)
	return uint8(mutated)
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
