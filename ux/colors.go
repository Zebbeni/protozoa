package ux

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"
	"golang.org/x/image/font"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/resources"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

// themeBackgroundColor returns the window / panel fill as a color.Color, derived from config.ThemeBackgroundRGB.
func themeBackgroundColor() color.Color {
	r, g, b := config.ThemeBackgroundRGB()
	return color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: 255,
	}
}

// themedForeground returns the primary foreground colour for UI chrome — titles, borders, labels, stats.
func themedForeground() color.Color {
	if config.IsLightTheme() {
		return color.RGBA{R: 30, G: 30, B: 35, A: 255}
	}
	return color.White
}

// themedForegroundDim returns a subdued foreground for secondary elements (hover state, thin borders).
func themedForegroundDim() color.Color {
	if config.IsLightTheme() {
		return color.RGBA{R: 110, G: 110, B: 120, A: 255}
	}
	return color.RGBA{R: 180, G: 180, B: 180, A: 255}
}

// fadedForeground returns the primary foreground colour with its alpha replaced by the given value.
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

// chrome picks one of two RGBA values based on the active theme.
func chrome(dark, light color.RGBA) color.RGBA {
	if config.IsLightTheme() {
		return light
	}
	return dark
}

func fillThemeBackground(screen *ebiten.Image) {
	if config.Theme() == "dark" {
		screen.Clear()
		return
	}
	screen.Fill(themeBackgroundColor())
}

// fillPhExtremeBackground paints the screen with the colour the grid env layer would draw at the given pH.
func fillPhExtremeBackground(screen *ebiten.Image, ph float64) {
	r, g, b := config.PhTargetColorRGB(ph)
	screen.Fill(color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: 255,
	})
}

// setTheme switches to the named theme and reloads images so any theme-keyed assets (e.g. the light theme's lifted sprites) pick up the change.
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

// phEffectMaxRatio defines where the pH-effect tint hits the saturated acid/base extreme.
const phEffectMaxRatio = 10.0

// phEffectSpectrum maps an organism's lifetime cumulative positive and negative pH contributions onto a [0, 1] spectrum where 0 is full acid (negative-dominant), 0.5 is neutral.
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

// phEffectColor returns the grid-tint colour for an organism with the given cumulative pH contributions.
func phEffectColor(positive, negative float64) colorful.Color {
	return phEffectSpectrumColor(phEffectSpectrum(positive, negative))
}

// phEffectSpectrumColor is the colour for a point on the spectrum, split out so the on-screen key can walk the ramp end to end and be sure it is showing the colours the grid actually paints.
func phEffectSpectrumColor(spec float64) colorful.Color {
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

// healthColor maps an organism's health/size ratio onto the green→red spectrum.
func healthColor(health, size float64) colorful.Color {
	if size <= 0 {
		return colorful.Color{R: 1, G: 0, B: 0}
	}
	return gh.GreenRedColor(health / size)
}

// phToleranceMidpointDamage is the health an organism loses per cycle to the water it is sitting in at the middle of the TOLERANCE view's green→red ramp.
const phToleranceMidpointDamage = 0.01

// phToleranceColor tints an organism green→red by the health per cycle the water is costing it.
func phToleranceColor(damage float64) colorful.Color {
	return gh.GreenRedColor(phToleranceFraction(damage))
}

// phToleranceFraction maps a per-cycle health loss to 1 (green) at zero and 0.5 at phToleranceMidpointDamage, approaching 0 without reaching it.
func phToleranceFraction(damage float64) float64 {
	return phToleranceMidpointDamage / (phToleranceMidpointDamage + math.Abs(damage))
}

// phEffectTextColor returns a text colour for the PH EFFECT stat based on an organism's cumulative positive/negative pH contributions.
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

func boundString(face font.Face, s string) image.Rectangle {
	bounds, _ := font.BoundString(face, s)
	return image.Rect(
		bounds.Min.X.Round(),
		bounds.Min.Y.Round(),
		bounds.Max.X.Round(),
		bounds.Max.Y.Round(),
	)
}

// textAdvance returns the horizontal advance (cursor move) for a string in the given face, including trailing whitespace.
func drawTextBottomUp(dst *ebiten.Image, s string, face font.Face, x, y int, col color.Color) {
	img := verticalTextImage(s, face)
	op := &ebiten.DrawImageOptions{}
	// A -90° turn maps (x, y) to (y, -x).
	op.GeoM.Rotate(-math.Pi / 2)
	op.GeoM.Translate(float64(x), float64(y+img.Bounds().Dx()))
	op.ColorScale.ScaleWithColor(col)
	dst.DrawImage(img, op)
}

// verticalTextImage renders s white-on-transparent, ready to be tinted and rotated by drawTextBottomUp.
func verticalTextImage(s string, face font.Face) *ebiten.Image {
	if img, ok := verticalTextCache[s]; ok {
		return img
	}
	w := textAdvance(face, s)
	h := face.Metrics().Height.Round()
	img := ebiten.NewImage(max(w, 1), max(h, 1))
	text.Draw(img, s, face, 0, face.Metrics().Ascent.Round(), color.White)
	verticalTextCache[s] = img
	return img
}

var verticalTextCache = map[string]*ebiten.Image{}

func textAdvance(face font.Face, s string) int {
	return font.MeasureString(face, s).Round()
}

// shiftRGB returns c with each colour channel shifted by delta and clamped to [0, 255].
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

// themedLabel is the ink for a secondary label — the name of a value, beside the value itself.
func themedLabel() color.RGBA {
	return chrome(
		color.RGBA{R: 180, G: 180, B: 180, A: 255},
		color.RGBA{R: 95, G: 95, B: 105, A: 255},
	)
}

func themedValue() color.RGBA {
	return chrome(
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
		color.RGBA{R: 25, G: 25, B: 32, A: 255},
	)
}

func themedMuted() color.RGBA {
	return chrome(
		color.RGBA{R: 110, G: 110, B: 110, A: 255},
		color.RGBA{R: 138, G: 138, B: 146, A: 255},
	)
}

func themedSectionTitle() color.RGBA {
	return chrome(
		color.RGBA{R: 180, G: 180, B: 255, A: 255},
		color.RGBA{R: 70, G: 70, B: 155, A: 255},
	)
}

func themedChanged() color.RGBA {
	return chrome(
		color.RGBA{R: 240, G: 190, B: 90, A: 255},
		color.RGBA{R: 155, G: 105, B: 10, A: 255},
	)
}

// themedSelectedRow is the band behind the row the keyboard is on, and themedSelectedInk the value drawn on it.
func themedSelectedRow() color.RGBA {
	return chrome(
		color.RGBA{R: 40, G: 40, B: 60, A: 255},
		color.RGBA{R: 206, G: 212, B: 238, A: 255},
	)
}

func themedSelectedInk() color.RGBA {
	return chrome(
		color.RGBA{R: 100, G: 255, B: 100, A: 255},
		color.RGBA{R: 20, G: 95, B: 20, A: 255},
	)
}

// themedOK and themedBad are the two verdicts a total can carry: the ability budget adding up, or not.
func themedOK() color.RGBA {
	return chrome(
		color.RGBA{R: 100, G: 220, B: 100, A: 255},
		color.RGBA{R: 30, G: 120, B: 30, A: 255},
	)
}

func themedBad() color.RGBA {
	return chrome(
		color.RGBA{R: 235, G: 90, B: 90, A: 255},
		color.RGBA{R: 180, G: 35, B: 35, A: 255},
	)
}

// themedControlFill is the fill of a small piece of chrome drawn on the window background.
func themedControlFill() color.RGBA {
	return chrome(
		color.RGBA{R: 70, G: 70, B: 95, A: 255},
		color.RGBA{R: 202, G: 202, B: 214, A: 255},
	)
}

func themedControlDim() color.RGBA {
	return chrome(
		color.RGBA{R: 45, G: 45, B: 50, A: 255},
		color.RGBA{R: 216, G: 216, B: 222, A: 255},
	)
}

// themedTrack, themedTrackFill and themedTrackHandle are a slider's three parts.
func themedTrack() color.RGBA {
	return chrome(
		color.RGBA{R: 50, G: 50, B: 60, A: 255},
		color.RGBA{R: 208, G: 208, B: 214, A: 255},
	)
}

func themedTrackFill() color.RGBA {
	return chrome(
		color.RGBA{R: 80, G: 80, B: 120, A: 255},
		color.RGBA{R: 150, G: 150, B: 195, A: 255},
	)
}

func themedTrackHandle() color.RGBA {
	return chrome(
		color.RGBA{R: 150, G: 150, B: 200, A: 255},
		color.RGBA{R: 80, G: 80, B: 140, A: 255},
	)
}
