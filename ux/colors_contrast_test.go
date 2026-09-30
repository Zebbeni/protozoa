package ux

import (
	"encoding/json"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// loadContrastGlobals installs the shipped defaults so config.Theme and config.ThemeBackgroundRGB have something to read.
func loadContrastGlobals(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	config.SetGlobals(&g)
	restore := config.Theme()
	t.Cleanup(func() { config.SetTheme(restore) })
}

func relativeLuminance(c color.RGBA) float64 {
	lin := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// contrastRatio is WCAG's, running from 1 (identical) to 21 (black on white).
func contrastRatio(a, b color.RGBA) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// minInkContrast is the ratio every ink has to clear against whatever it is drawn on.
const minInkContrast = 3.0

// TestThemedInksStayLegible is what catches an ink that was picked for one theme and left to fend for itself in the other.
func TestThemedInksStayLegible(t *testing.T) {
	loadContrastGlobals(t)

	inks := []struct {
		name string
		ink  func() color.RGBA
		// on is what the ink is drawn on.
		on func() color.RGBA
	}{
		{"themedLabel", themedLabel, nil},
		{"themedValue", themedValue, nil},
		{"themedMuted", themedMuted, nil},
		{"themedSectionTitle", themedSectionTitle, nil},
		{"themedChanged", themedChanged, nil},
		{"themedOK", themedOK, nil},
		{"themedBad", themedBad, nil},
		{"themedSelectedInk", themedSelectedInk, themedSelectedRow},
	}

	for _, theme := range []string{"dark", "light"} {
		config.SetTheme(theme)
		bg := backgroundRGBA()
		for _, ink := range inks {
			on := bg
			if ink.on != nil {
				on = ink.on()
			}
			if got := contrastRatio(ink.ink(), on); got < minInkContrast {
				t.Errorf("%s theme: %s has contrast %.2f against what it is drawn on, want >= %.1f",
					theme, ink.name, got, minInkContrast)
			}
		}
	}
}

// minSurfaceContrast is what a *surface* has to clear — a slider track, a selection band, a small button's fill.
const minSurfaceContrast = 1.15

func TestThemedSurfacesAreDistinguishable(t *testing.T) {
	loadContrastGlobals(t)

	surfaces := []struct {
		name    string
		surface func() color.RGBA
		// against is what it has to be told apart from.
		against func() color.RGBA
	}{
		{"themedSelectedRow", themedSelectedRow, nil},
		{"themedControlFill", themedControlFill, nil},
		{"themedControlDim", themedControlDim, nil},
		{"themedTrack", themedTrack, nil},
		{"themedTrackFill", themedTrackFill, themedTrack},
		{"themedTrackHandle", themedTrackHandle, themedTrackFill},
	}

	for _, theme := range []string{"dark", "light"} {
		config.SetTheme(theme)
		bg := backgroundRGBA()
		for _, s := range surfaces {
			against := bg
			if s.against != nil {
				against = s.against()
			}
			if got := contrastRatio(s.surface(), against); got < minSurfaceContrast {
				t.Errorf("%s theme: %s is %.2f against what it sits on, too close to make out",
					theme, s.name, got)
			}
		}
	}
}

// backgroundRGBA is the active theme's window fill as an RGBA.
func backgroundRGBA() color.RGBA {
	r, g, b := config.ThemeBackgroundRGB()
	return color.RGBA{R: uint8(r * 255), G: uint8(g * 255), B: uint8(b * 255), A: 255}
}
