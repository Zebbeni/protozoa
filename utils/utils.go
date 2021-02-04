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
	r, g, b, a := uint8(r32), uint8(g32), uint8(b32), uint8(a32)
	r += uint8(rand.Intn(20) - 10)
	g += uint8(rand.Intn(20) - 10)
	b += uint8(rand.Intn(20) - 10)
	r = uint8(math.Max(50, math.Min(255, float64(r))))
	g = uint8(math.Max(50, math.Min(255, float64(g))))
	b = uint8(math.Max(50, math.Min(255, float64(b))))
	return color.RGBA{r, g, b, a}
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
