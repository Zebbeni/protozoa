package ux

import (
	"image"

	"github.com/lucasb-eyer/go-colorful"
	"golang.org/x/image/font"

	"github.com/Zebbeni/protozoa/organism"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

const phMaxHue = gh.PhMaxHue

// PhValueColor maps a pH value to RGBA floats using the grid's pH color spectrum.
func PhValueColor(ph float64) (float32, float32, float32, float32) {
	return gh.PhValueColor(ph)
}

// PhEffectColor maps a normalized spectrum value [0, 1] to a color.
// 0 = most negative effect (green/acid), 0.5 = neutral, 1 = most positive (pink/base)
func PhEffectColor(spectrumValue float64) colorful.Color {
	return organism.ComputePhEffectColor(spectrumValue)
}

// boundString returns the bounding rectangle of the given text rendered with
// the given font face. Replaces the deprecated ebiten text.BoundString.
func boundString(face font.Face, s string) image.Rectangle {
	bounds, _ := font.BoundString(face, s)
	return image.Rect(
		bounds.Min.X.Round(),
		bounds.Min.Y.Round(),
		bounds.Max.X.Round(),
		bounds.Max.Y.Round(),
	)
}
