package organism

import (
	"image/color"
	"math"
	"sync"

	"github.com/lucasb-eyer/go-colorful"
)

// DescendantNode represents a single organism in the ancestor family tree.
type DescendantNode struct {
	ID            int
	Color         color.Color
	PhEffectColor color.Color
	StartCycle    int
	EndCycle      int // 0 means still alive
	Children      []*DescendantNode
	childMu       sync.Mutex
}

// PhEffectSpectrumValue computes a normalized [0,1] spectrum value for an
// organism's pH effect, matching the grid's phEffect organism coloring.
// The size cancels out, so this depends only on PhGrowthEffect.
func PhEffectSpectrumValue(phGrowthEffect, maxEffect float64) float64 {
	if maxEffect == 0 {
		return 0.5
	}
	v := (phGrowthEffect + maxEffect) / (2.0 * maxEffect)
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ComputePhEffectColor returns a color for the given spectrum value [0,1]
// using the same formula as ux.PhEffectColor.
func ComputePhEffectColor(spectrumValue float64) colorful.Color {
	const phMaxHue = 100.0
	hue := phMaxHue - (phMaxHue * spectrumValue)
	sat := 0.5 + math.Abs(spectrumValue-0.5)
	light := math.Abs(spectrumValue - 0.5)
	return colorful.HSLuv(hue, sat, light)
}

// AddChild safely appends a child node.
func (n *DescendantNode) AddChild(child *DescendantNode) {
	n.childMu.Lock()
	n.Children = append(n.Children, child)
	n.childMu.Unlock()
}

// ForEachChild calls fn for each child while holding the lock.
func (n *DescendantNode) ForEachChild(fn func(*DescendantNode)) {
	n.childMu.Lock()
	for _, child := range n.Children {
		fn(child)
	}
	n.childMu.Unlock()
}
