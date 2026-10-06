package ux

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestOnlyADownscaleIsFiltered: linear on pixel art blurs it, so an upscale
// has to stay nearest. Only a draw smaller than its source is filtered, and
// at the shipped zoom table that is Zoom8 alone.
func TestOnlyADownscaleIsFiltered(t *testing.T) {
	for _, scale := range []float64{0.25, 0.5, 0.99} {
		if got := spriteFilter(scale); got != spriteDownscaleFilter {
			t.Errorf("scale %v uses filter %v, want the downscale filter %v",
				scale, got, spriteDownscaleFilter)
		}
	}
	for _, scale := range []float64{1, 2, 4} {
		if got := spriteFilter(scale); got != ebiten.FilterNearest {
			t.Errorf("scale %v uses filter %v, want nearest", scale, got)
		}
	}
}

// TestZoom8IsTheOnlyDownscale documents which zoom levels the filter reaches,
// so changing the zoom table shows up here rather than silently blurring a
// level that used to be crisp.
func TestZoom8IsTheOnlyDownscale(t *testing.T) {
	loadKeyGlobals(t)
	cam := &Camera{}
	for z := ZoomMin; z <= ZoomMax; z++ {
		cam.Zoom = z
		scale := cam.SpriteScale()
		down := scale < 1
		if (z == Zoom8) != down {
			t.Errorf("zoom %d has sprite scale %v (downscale %v); only Zoom8 should be below 1",
				z, scale, down)
		}
	}
}
