package ux

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"
	"golang.org/x/image/font"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/resources"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

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

// chrome picks one of two RGBA values based on the active theme. Used
// by panel chrome (buttons, timeline scrubber, graph backgrounds) so a
// single call site declares both palettes side by side.
func chrome(dark, light color.RGBA) color.RGBA {
	if config.IsLightTheme() {
		return light
	}
	return dark
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

// fillPhExtremeBackground paints the screen with the colour the grid
// env layer would draw at the given pH — the fully-saturated target
// from config.PhTargetColorRGB, since the blend weight at MinPh / MaxPh
// is 1.0 (no theme background mixed in). Used by the animation-test
// background-toggle hotkey so sprites can be previewed against the
// extreme env-tints.
func fillPhExtremeBackground(screen *ebiten.Image, ph float64) {
	r, g, b := config.PhTargetColorRGB(ph)
	screen.Fill(color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: 255,
	})
}

// setTheme switches to the named theme and reloads images so any
// theme-keyed assets (e.g. grid_light vs grid_dark sprite directories)
// pick up the change. No-op when the theme is already active.
func setTheme(name string) {
	if config.Theme() == name {
		return
	}
	config.SetTheme(name)
	resources.ReloadImages()
}

const phMaxHue = gh.PhMaxHue

// PhValueColor maps a pH value to RGBA floats using the grid's pH color spectrum.
func PhValueColor(ph float64) (float32, float32, float32, float32) {
	return gh.PhValueColor(ph)
}

// phEffectMaxRatio defines where the pH-effect tint hits the saturated
// acid/base extreme: a 10×-or-greater imbalance between an organism's
// positive and negative cumulative pH contributions.
const phEffectMaxRatio = 10.0

// phEffectSpectrum maps an organism's lifetime cumulative positive and
// negative pH contributions onto a [0, 1] spectrum where 0 is full
// acid (negative-dominant), 0.5 is neutral, and 1 is full base
// (positive-dominant). The intensity from neutral grows as the larger
// magnitude approaches phEffectMaxRatio× the smaller; equal magnitudes
// or both-zero collapse to 0.5.
func phEffectSpectrum(positive, negative float64) float64 {
	switch {
	case positive == 0 && negative == 0:
		return 0.5
	case positive == 0:
		return 0
	case negative == 0:
		return 1
	}
	if positive > negative {
		ratio := positive / negative
		intensity := (ratio - 1) / (phEffectMaxRatio - 1)
		if intensity > 1 {
			intensity = 1
		}
		return 0.5 + 0.5*intensity
	}
	ratio := negative / positive
	intensity := (ratio - 1) / (phEffectMaxRatio - 1)
	if intensity > 1 {
		intensity = 1
	}
	return 0.5 - 0.5*intensity
}

// phEffectColor returns the grid-tint colour for an organism with the
// given cumulative pH contributions. Mirrors ComputePhEffectColor's
// old behaviour: blends to background at neutral, toward the active
// scheme's acid/base hue at the extremes.
func phEffectColor(positive, negative float64) colorful.Color {
	spec := phEffectSpectrum(positive, negative)
	acidHue, baseHue := config.PhEffectHueRange()
	hue := acidHue + (baseHue-acidHue)*spec
	dist := spec - 0.5
	if dist < 0 {
		dist = -dist
	}
	sat := 0.5 + dist
	light := dist
	if config.IsLightTheme() {
		light = 1 - dist
	}
	col := colorful.HSLuv(hue, sat, light)
	bgR, bgG, bgB := config.ThemeBackgroundRGB()
	weight := dist * 2
	return colorful.Color{
		R: weight*col.R + (1-weight)*bgR,
		G: weight*col.G + (1-weight)*bgG,
		B: weight*col.B + (1-weight)*bgB,
	}
}

// greenRedColor maps t in [0, 1] onto the green→red HSLuv spectrum:
// 1 = green, 0 = red, 0.5 = yellow. Hue 0° = red, 120° = green; HSLuv
// keeps the transitions perceptually uniform.
func greenRedColor(t float64) colorful.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return colorful.HSLuv(120.0*t, 0.9, 0.5)
}

// healthColor maps an organism's health/size ratio onto the green→red
// spectrum. Ratio 1 (full health) is green; 0 (about to die) is red.
// Used by the HEALTH grid render mode and the panel's HEALTH stat.
func healthColor(health, size float64) colorful.Color {
	if size <= 0 {
		return colorful.Color{R: 1, G: 0, B: 0}
	}
	return greenRedColor(health / size)
}

// phIdealTextColor returns a text colour for the PH TOL stat, based
// on the organism's ideal pH (the centre of its tolerance range).
// Mirrors how the grid renders pH cells — acid extremes pull toward
// the scheme's acid hue, base extremes toward the base hue — but
// blends toward the themed foreground at neutral instead of the
// background, so a neutral-pH organism's value still reads as plain
// white-on-dark or black-on-light text. The blend ramps via sqrt and
// uses the world's MinIdealPh/MaxIdealPh range (not the full pH map)
// so realistic ideal-pH values reach the saturated extremes — using
// the full pH map would clip the weight at ~0.78 and wash everything
// out.
func phIdealTextColor(idealPh float64) color.Color {
	minIdeal := config.MinIdealPh()
	maxIdeal := config.MaxIdealPh()
	mid := (minIdeal + maxIdeal) / 2.0
	half := (maxIdeal - minIdeal) / 2.0
	weight := 0.0
	if half > 0 {
		diff := idealPh - mid
		if diff < 0 {
			diff = -diff
		}
		weight = diff / half
		if weight > 1 {
			weight = 1
		}
	}
	weight = math.Sqrt(weight)

	acidHue, baseHue := config.PhEffectHueRange()
	hue := acidHue
	if idealPh >= mid {
		hue = baseHue
	}
	sat := 1.0
	light := 0.65
	if config.IsLightTheme() {
		light = 0.4
	}
	accent := colorful.HSLuv(hue, sat, light)

	fgR, fgG, fgB := 1.0, 1.0, 1.0
	if config.IsLightTheme() {
		fgR, fgG, fgB = 30.0/255.0, 30.0/255.0, 35.0/255.0
	}
	return colorful.Color{
		R: weight*accent.R + (1-weight)*fgR,
		G: weight*accent.G + (1-weight)*fgG,
		B: weight*accent.B + (1-weight)*fgB,
	}
}

// phEffectTextColor returns a text colour for the PH EFFECT stat
// based on an organism's cumulative positive/negative pH
// contributions. At balanced (or both zero) it returns the themed
// foreground; an imbalance shifts the colour toward the active pH
// colour scheme's acid (negative-dominant) or base (positive-
// dominant) extreme. The blend ramps via sqrt so a mid-range
// imbalance shows a clear tint.
func phEffectTextColor(positive, negative float64) color.Color {
	spec := phEffectSpectrum(positive, negative)
	dist := spec - 0.5
	if dist < 0 {
		dist = -dist
	}
	weight := math.Sqrt(dist * 2)

	acidHue, baseHue := config.PhEffectHueRange()
	hue := acidHue + (baseHue-acidHue)*spec
	sat := 1.0
	light := 0.65
	if config.IsLightTheme() {
		light = 0.4
	}
	accent := colorful.HSLuv(hue, sat, light)

	fgR, fgG, fgB := 1.0, 1.0, 1.0
	if config.IsLightTheme() {
		fgR, fgG, fgB = 30.0/255.0, 30.0/255.0, 35.0/255.0
	}
	return colorful.Color{
		R: weight*accent.R + (1-weight)*fgR,
		G: weight*accent.G + (1-weight)*fgG,
		B: weight*accent.B + (1-weight)*fgB,
	}
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

// textAdvance returns the horizontal advance (cursor move) for a
// string in the given face, including trailing whitespace. Use this
// when positioning text relative to a prefix; boundString returns the
// visual bounding box and ignores trailing spaces, so it's not safe
// for layout offsets.
func textAdvance(face font.Face, s string) int {
	return font.MeasureString(face, s).Round()
}

// shiftRGB returns c with each colour channel shifted by delta and
// clamped to [0, 255]. Positive delta lightens, negative darkens.
// Alpha is preserved. Used by buttons and other chrome to derive
// hover/pressed tints from a single base colour.
func shiftRGB(c color.RGBA, delta int) color.RGBA {
	clamp := func(v int) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	return color.RGBA{
		R: clamp(int(c.R) + delta),
		G: clamp(int(c.G) + delta),
		B: clamp(int(c.B) + delta),
		A: c.A,
	}
}
