package utils

import (
	"image/color"
	"math/rand"
)

// GetRandomColor returns a random color
func GetRandomColor() color.Color {
	r := uint8(55 + rand.Intn(200))
	g := uint8(55 + rand.Intn(200))
	b := uint8(55 + rand.Intn(200))
	return color.RGBA{R: r, G: g, B: b, A: 255}
}
