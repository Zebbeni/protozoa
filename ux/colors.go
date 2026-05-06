package ux

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"
	"golang.org/x/image/font"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/resources"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

// themeCycle is the rotation used by the T key: each press moves to the
// next entry, wrapping around.
var themeCycle = []string{"dark", "light"}

// themeBackgroundColor returns the window / panel fill as a color.Color,
// derived from config.ThemeBackgroundRGB. Used by fillThemeBackground and
// anywhere else that needs the bg directly.
func themeBackgroundColor() color.Color {
	r, g, b := config.ThemeBackgroundRGB()
	return color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: 255,
	}
}

// themedForeground returns the primary foreground colour for UI chrome —
// titles, borders, labels, stats. White on the dark theme; near-black on
// any light theme so everything stays legible against the window fill.
func themedForeground() color.Color {
	if config.IsLightTheme() {
		return color.RGBA{R: 30, G: 30, B: 35, A: 255}
	}
	return color.White
}

// themedForegroundDim returns a subdued foreground for secondary elements
// (hover state, thin borders). Mid-grey in both themes, biased dark or
// light so it sits between the background and primary foreground.
func themedForegroundDim() color.Color {
	if config.IsLightTheme() {
		return color.RGBA{R: 110, G: 110, B: 120, A: 255}
	}
	return color.RGBA{R: 180, G: 180, B: 180, A: 255}
}

// fadedForeground returns the primary foreground colour with its alpha
// replaced by the given value, so callers can paint accents that read
// as related-but-quieter than the main selection. Channels are
// pre-multiplied to match Go's standard alpha-premultiplied colour
// model used by ebiten.
func fadedForeground(alpha uint8) color.Color {
	var r, g, b uint8
	if config.IsLightTheme() {
		r, g, b = 30, 30, 35
	} else {
		r, g, b = 255, 255, 255
	}
	a := uint32(alpha)
	return color.RGBA{
		R: uint8(uint32(r) * a / 255),
		G: uint8(uint32(g) * a / 255),
		B: uint8(uint32(b) * a / 255),
		A: alpha,
	}
}

// fillThemeBackground paints the screen with the active theme's fill.
// Dark mode uses Clear (transparent → reads as black); the light themes
// fill with their specific background colour.
func fillThemeBackground(screen *ebiten.Image) {
	if config.Theme() == "dark" {
		screen.Clear()
		return
	}
	screen.Fill(themeBackgroundColor())
}

// cycleTheme advances the active theme to the next entry in themeCycle.
// ReloadImages keeps the hot-reload watermark in sync — sprite artwork
// itself is theme-agnostic now, but reloading also refreshes any edits
// the user made since the last cycle.
func cycleTheme() {
	current := config.Theme()
	next := themeCycle[0]
	for i, t := range themeCycle {
		if t == current {
			next = themeCycle[(i+1)%len(themeCycle)]
			break
		}
	}
	config.SetTheme(next)
	resources.ReloadImages()
}

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
