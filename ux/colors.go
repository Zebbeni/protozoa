package ux

import (
	"math"

	"github.com/lucasb-eyer/go-colorful"
)

const phMaxHue = 100.0

// PhEffectColor maps a normalized spectrum value [0, 1] to a color.
// 0 = most negative effect (green/acid), 0.5 = neutral, 1 = most positive (pink/base)
func PhEffectColor(spectrumValue float64) colorful.Color {
	hue := phMaxHue - (phMaxHue * spectrumValue)
	sat := 0.5 + math.Abs(spectrumValue-0.5)
	light := math.Abs(spectrumValue - 0.5)
	return colorful.HSLuv(hue, sat, light)
}
