package ux

import (
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/organism"
)

const phMaxHue = 100.0

// PhEffectColor maps a normalized spectrum value [0, 1] to a color.
// 0 = most negative effect (green/acid), 0.5 = neutral, 1 = most positive (pink/base)
func PhEffectColor(spectrumValue float64) colorful.Color {
	return organism.ComputePhEffectColor(spectrumValue)
}
