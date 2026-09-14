package resources

import (
	"image"
	"image/color"
	"testing"
)

func grey(v uint8, a uint8) color.NRGBA { return color.NRGBA{R: v, G: v, B: v, A: a} }

// TestToneImageCurve: black maps to the shadow point, white stays white,
// greys keep their order, and alpha is untouched.
func TestToneImageCurve(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 5, 1))
	src.SetNRGBA(0, 0, grey(0, 255))
	src.SetNRGBA(1, 0, grey(56, 255))
	src.SetNRGBA(2, 0, grey(111, 255))
	src.SetNRGBA(3, 0, grey(255, 128))
	src.SetNRGBA(4, 0, color.NRGBA{A: 0})

	out := toneImage(src, 0.25, 0.65, 0)

	if got := out.NRGBAAt(0, 0); got.R != 64 || got.A != 255 {
		t.Errorf("black should map to the 0.25 shadow point (64): %v", got)
	}
	if got := out.NRGBAAt(3, 0); got.R != 255 || got.A != 128 {
		t.Errorf("white should stay white and keep its alpha: %v", got)
	}
	if got := out.NRGBAAt(4, 0); got.A != 0 {
		t.Errorf("transparent pixel became visible: %v", got)
	}
	for x := 0; x < 3; x++ {
		if out.NRGBAAt(x, 0).R >= out.NRGBAAt(x+1, 0).R {
			t.Errorf("greys should keep their order: %d -> %d", out.NRGBAAt(x, 0).R, out.NRGBAAt(x+1, 0).R)
		}
	}
}

// TestToneImageGammaBrightensMidtones: below 1, gamma lifts a midtone more
// than a plain shadow lift of the same black point would.
func TestToneImageGammaBrightensMidtones(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, grey(111, 255))
	linear := toneImage(src, 0.25, 1, 0).NRGBAAt(0, 0).R
	curved := toneImage(src, 0.25, 0.65, 0).NRGBAAt(0, 0).R
	if curved <= linear {
		t.Errorf("gamma 0.65 midtone %d should be brighter than linear %d", curved, linear)
	}
}

func TestToneImageIdentity(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 24, G: 56, B: 111, A: 200})
	if got := toneImage(src, 0, 1, 0).NRGBAAt(0, 0); got != src.NRGBAAt(0, 0) {
		t.Errorf("shadow 0, gamma 1, darken 0 changed the pixel: %v", got)
	}
}

// TestToneImageDarkenDeepensDarksOnly: darken pulls dark greys well down
// while light greys and white barely move, and greys never swap order, even
// at the strongest darken.
func TestToneImageDarkenDeepensDarksOnly(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 6, 1))
	for x, v := range []uint8{0, 24, 56, 111, 165, 255} {
		src.SetNRGBA(x, 0, grey(v, 255))
	}
	base := toneImage(src, 0.25, 0.65, 0)
	dark := toneImage(src, 0.25, 0.65, 0.3)

	if drop := int(base.NRGBAAt(1, 0).R) - int(dark.NRGBAAt(1, 0).R); drop < 20 {
		t.Errorf("dark grey 24 should darken noticeably, dropped only %d", drop)
	}
	if drop := int(base.NRGBAAt(4, 0).R) - int(dark.NRGBAAt(4, 0).R); drop > 5 {
		t.Errorf("light grey 165 should barely change, dropped %d", drop)
	}
	if dark.NRGBAAt(5, 0).R != 255 {
		t.Errorf("white should stay white, got %d", dark.NRGBAAt(5, 0).R)
	}
	strongest := toneImage(src, 0.25, 0.65, 0.5)
	for x := 0; x < 5; x++ {
		if strongest.NRGBAAt(x, 0).R > strongest.NRGBAAt(x+1, 0).R {
			t.Errorf("darken 0.5 swapped greys at %d: %d > %d", x, strongest.NRGBAAt(x, 0).R, strongest.NRGBAAt(x+1, 0).R)
		}
	}
}
