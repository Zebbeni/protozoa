package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/resources"
)

// TestThePreviewerShowsEveryAnimation: the matrix is what an artist checks
// their work in, so an animation missing from it is art nobody can look at
// without running a simulation and waiting for the action to happen.
func TestThePreviewerShowsEveryAnimation(t *testing.T) {
	shown := map[animation.Animation]int{}
	for _, cell := range demoAnimations {
		shown[cell.anim]++
		if cell.label == "" {
			t.Errorf("animation %d has no column label", cell.anim)
		}
	}
	for _, anim := range animation.AllAnimations {
		switch shown[anim] {
		case 1:
		case 0:
			t.Errorf("animation %d is not in the previewer", anim)
		default:
			t.Errorf("animation %d has %d columns", anim, shown[anim])
		}
	}
}

// TestThePreviewerSkipsTheUnusedSpriteSet: 8x8 is loaded and never drawn —
// Zoom8 renders the 16x16 art at half scale — so previewing it shows art the
// game never puts on screen.
func TestThePreviewerSkipsTheUnusedSpriteSet(t *testing.T) {
	if len(previewSpriteSets) == 0 {
		t.Fatal("the previewer shows no sprite sets at all")
	}
	seen := map[int]bool{}
	for _, zi := range previewSpriteSets {
		if zi < 0 || zi >= len(zoomSpriteSizes) {
			t.Fatalf("sprite set %d is not one of the %d loaded", zi, len(zoomSpriteSizes))
		}
		if seen[zi] {
			t.Errorf("sprite set %d is previewed twice", zi)
		}
		seen[zi] = true
	}
	for zi, size := range zoomSpriteSizes {
		if size == 8 && seen[zi] {
			t.Errorf("the 8x8 set is still previewed")
		}
	}
	if !seen[resources.ZoomHighRes] {
		t.Error("the 16x16 set is not previewed, and it is the only one with layered overlays")
	}
}
