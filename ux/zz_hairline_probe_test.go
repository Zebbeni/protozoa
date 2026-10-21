package ux

import (
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/Zebbeni/protozoa/config"
)

// TestHairlineProbe counts the pixels a minimum-width connection actually
// writes, at the two angles the diagram draws them at and with antialiasing on
// and off. Asserts nothing. It runs inside a game loop because reading pixels
// back needs one.
//
//	HAIRLINE_PROBE=1 go test ./ux/ -run TestHairlineProbe -v
func TestHairlineProbe(t *testing.T) {
	if os.Getenv("HAIRLINE_PROBE") == "" {
		t.Skip("set HAIRLINE_PROBE=1 to count the pixels a thin line writes")
	}
	loadKeyGlobals(t)
	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err := ebiten.RunGame(&hairlineProbe{t: t})
	if err != nil && err != ebiten.Termination {
		t.Logf("window teardown: %v", err)
	}
}

type hairlineProbe struct {
	t    *testing.T
	done bool
}

func (p *hairlineProbe) Layout(w, h int) (int, int) {
	return config.ScreenWidth(), config.ScreenHeight()
}

func (p *hairlineProbe) Update() error {
	if p.done {
		return ebiten.Termination
	}
	return nil
}

func (p *hairlineProbe) Draw(screen *ebiten.Image) {
	if p.done {
		return
	}
	drawn := func(width float32, y0, y1 float32, aa bool) int {
		img := ebiten.NewImage(64, 64)
		vector.StrokeLine(img, 4, y0, 60, y1, width, color.RGBA{R: 255, G: 255, B: 255, A: 255}, aa)
		count := 0
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
					count++
				}
			}
		}
		return count
	}

	for _, w := range []float32{brainMinEdgeW, 0.75, 1, brainMaxEdgeW} {
		// Horizontal is what a 1-unit layer feeding another 1-unit layer
		// produces: both columns centre on the same row.
		p.t.Logf("width %.2f horizontal at y=32.0: %3d px without AA, %3d with",
			w, drawn(w, 32, 32, false), drawn(w, 32, 32, true))
		p.t.Logf("width %.2f horizontal at y=32.5: %3d px without AA, %3d with",
			w, drawn(w, 32.5, 32.5, false), drawn(w, 32.5, 32.5, true))
		p.t.Logf("width %.2f diagonal:            %3d px without AA, %3d with",
			w, drawn(w, 10, 54, false), drawn(w, 10, 54, true))
	}
	p.done = true
}
