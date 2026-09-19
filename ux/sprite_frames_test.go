package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/animation"
)

// TestSmallestSpritesAnimate: 4x4 used to hold a single frame, which
// made it the one sprite set that could only ever hold a pose. It now
// gets two, like 8x8.
func TestSmallestSpritesAnimate(t *testing.T) {
	for set, size := range zoomSpriteSizes {
		frames := zoomSpriteFrameCounts[set]
		if frames < 2 {
			t.Errorf("the %dx%d set holds %d frame(s); every set should animate", size, size, frames)
		}
		if frames > animation.BaseFramesPerCycle {
			t.Errorf("the %dx%d set holds %d frames, past the %d-frame ceiling",
				size, size, frames, animation.BaseFramesPerCycle)
		}
	}
}

// TestFrameCountsRiseWithResolution: more pixels can carry more motion,
// and the top set is the only one that earns the full four — higher zoom
// levels upscale it rather than asking for more steps.
func TestFrameCountsRiseWithResolution(t *testing.T) {
	prev := 0
	for set := range zoomSpriteSizes {
		got := zoomSpriteFrameCounts[set]
		if got < prev {
			t.Errorf("the %dx%d set holds %d frames, fewer than the smaller set's %d",
				zoomSpriteSizes[set], zoomSpriteSizes[set], got, prev)
		}
		prev = got
	}
	if last := zoomSpriteFrameCounts[len(zoomSpriteFrameCounts)-1]; last != animation.BaseFramesPerCycle {
		t.Errorf("the largest set holds %d frames, want the full %d", last, animation.BaseFramesPerCycle)
	}
}
