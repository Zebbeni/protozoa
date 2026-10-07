package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/animation"
)

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
