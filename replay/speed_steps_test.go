package replay

import (
	"math"
	"testing"

	"github.com/Zebbeni/protozoa/animation"
)

func isPowerOfTwo(v float64) bool {
	if v <= 0 {
		return false
	}
	l := math.Log2(v)
	return math.Abs(l-math.Round(l)) < 1e-9
}

func TestAutoSpeedsArePowersOfTwo(t *testing.T) {
	for _, unitSize := range []int{64, 32, 16, 8, 4, 2, 1} {
		if got := SpeedForUnitSize(unitSize); !isPowerOfTwo(got) {
			t.Errorf("unit size %d auto-speeds to %v, not a power of two", unitSize, got)
		}
	}
}

func TestSnapToPowerOfTwoPicksByRatio(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{0.25, 0.25}, {0.5, 0.5}, {1, 1}, {2, 2}, {4, 4}, {64, 64},
		{3, 4},    // log2 3 ≈ 1.58 → 2
		{1.5, 2},  // log2 1.5 ≈ 0.58 → 1
		{6, 8},    // the value the auto table used to return
		{0.75, 1}, // log2 0.75 ≈ -0.41 → 0
	} {
		if got := SnapToPowerOfTwo(tc.in); got != tc.want {
			t.Errorf("SnapToPowerOfTwo(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestEverySpeedTheButtonsReachIsAPowerOfTwo walks the halve/double controls across their whole range from every auto-speed start.
func TestEverySpeedTheButtonsReachIsAPowerOfTwo(t *testing.T) {
	for _, start := range []int{64, 32, 16, 8, 4, 2, 1} {
		// setSpeedInternal writes through to the animation clock, so the controller needs one.
		c := &Controller{Speed: 1, AnimState: animation.NewState()}
		c.setSpeedInternal(SpeedForUnitSize(start))

		// All the way up, then all the way down, then up again.
		for i := 0; i < 12; i++ {
			c.setSpeedInternal(c.Speed * 2)
			if !isPowerOfTwo(c.Speed) {
				t.Fatalf("from unit %d, doubling reached %v", start, c.Speed)
			}
		}
		for i := 0; i < 24; i++ {
			c.setSpeedInternal(c.Speed / 2)
			if !isPowerOfTwo(c.Speed) {
				t.Fatalf("from unit %d, halving reached %v", start, c.Speed)
			}
		}
		if c.Speed != MinReplaySpeed {
			t.Errorf("halving bottomed out at %v, want the minimum %v", c.Speed, MinReplaySpeed)
		}
	}
}

func TestSpeedBoundsAreThemselvesPowersOfTwo(t *testing.T) {
	if !isPowerOfTwo(MinReplaySpeed) {
		t.Errorf("MinReplaySpeed %v is not a power of two", MinReplaySpeed)
	}
	if !isPowerOfTwo(MaxReplaySpeed) {
		t.Errorf("MaxReplaySpeed %v is not a power of two", MaxReplaySpeed)
	}
}
