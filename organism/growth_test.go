package organism

import (
	"math"
	"testing"
)

// TestGrowthKeepsShareOfOverflow: health above size grows the organism by
// growthFactor of the overflow, up to its max size, and health is capped
// at the new size. Every case starts at size 20, health 20, max size 80.
func TestGrowthKeepsShareOfOverflow(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		factor, gain         float64
		wantSize, wantHealth float64
	}{
		{"half the overflow", 0.5, 30, 35, 35},
		{"all the overflow", 1, 30, 50, 50},
		{"capped at max size", 1, 500, 80, 80},
		{"no overflow, no growth", 1, -5, 20, 15},
	} {
		o := &Organism{Size: 20, Health: 20, traits: Traits{MaxSize: 80}}
		o.ApplyHealthChangeWithGrowth(tc.gain, tc.factor)
		if math.Abs(o.Size-tc.wantSize) > 1e-9 || math.Abs(o.Health-tc.wantHealth) > 1e-9 {
			t.Errorf("%s: size %.2f health %.2f, want size %.2f health %.2f",
				tc.name, o.Size, o.Health, tc.wantSize, tc.wantHealth)
		}
	}
}
