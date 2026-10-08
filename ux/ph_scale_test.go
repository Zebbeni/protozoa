package ux

import "testing"

// TestThePhFieldIsNeverSmoothedPastTheDisplay: it is smoothed up from one
// pixel a cell, so building it finer than the display shows is detail the
// next draw throws away. At Zoom8, which draws the 16x16 art at half size,
// that was 1632x1312 scaled straight back down to 816x656 every frame.
func TestThePhFieldIsNeverSmoothedPastTheDisplay(t *testing.T) {
	loadKeyGlobals(t)
	cam := &Camera{}
	for z := ZoomMin; z <= ZoomMax; z++ {
		cam.Zoom = z
		if cell, unit := cam.PhCellSize(), cam.GridUnitSize(); cell > unit {
			t.Errorf("zoom %d smooths to %dpx a cell for a %dpx display cell", z, cell, unit)
		}
	}
}

// TestThePhFieldIsOnlyEverEnlarged: a nearest-neighbour DOWNSCALE drops
// every other row, which is what capping the smoothing also avoids.
func TestThePhFieldIsOnlyEverEnlarged(t *testing.T) {
	loadKeyGlobals(t)
	cam := &Camera{}
	for z := ZoomMin; z <= ZoomMax; z++ {
		cam.Zoom = z
		if s := cam.PhScale(); s < 1 {
			t.Errorf("zoom %d enlarges the pH field by %v, which is a downscale", z, s)
		}
	}
}

// TestThePhFieldStillLandsAtWorldPixelSize is the property the cap must not
// break: smoothed size times the scale is the display cell size.
func TestThePhFieldStillLandsAtWorldPixelSize(t *testing.T) {
	loadKeyGlobals(t)
	cam := &Camera{}
	for z := ZoomMin; z <= ZoomMax; z++ {
		cam.Zoom = z
		if got, want := float64(cam.PhCellSize())*cam.PhScale(), float64(cam.GridUnitSize()); got != want {
			t.Errorf("zoom %d lands the pH field at %v px a cell, want %v", z, got, want)
		}
	}
}

// TestOnlyZoom8ChangedPixelCount documents what this cost nothing elsewhere:
// every other level already smoothed to its own display resolution or below.
func TestOnlyZoom8ChangedPixelCount(t *testing.T) {
	loadKeyGlobals(t)
	cam := &Camera{}
	for z := ZoomMin; z <= ZoomMax; z++ {
		cam.Zoom = z
		capped, uncapped := cam.PhCellSize(), cam.SpriteSize()
		if (z == Zoom8) != (capped != uncapped) {
			t.Errorf("zoom %d smooths to %d where the sprite size is %d; only Zoom8 should differ",
				z, capped, uncapped)
		}
	}
}
