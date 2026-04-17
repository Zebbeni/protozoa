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
	Parent        *DescendantNode
	Children      []*DescendantNode
	childMu       sync.Mutex

	// Dead-branch pruning: skip entire sub-trees during tree walks
	deadBranchesCount   int // how many direct children have all branches dead
	AllBranchesDeadCycle int // cycle when this node AND all descendants are dead (0 = still has living)
}

// AncestorAtGeneration walks up the tree n generations and returns that ancestor.
// Returns the root if fewer than n generations exist above this node.
func (n *DescendantNode) AncestorAtGeneration(generations int) *DescendantNode {
	node := n
	for i := 0; i < generations && node.Parent != nil; i++ {
		node = node.Parent
	}
	return node
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

// MarkDead sets the EndCycle and propagates dead-branch counts up the tree.
// Call this when an organism dies instead of setting EndCycle directly.
func (n *DescendantNode) MarkDead(cycle int) {
	n.EndCycle = cycle
	// Check if all children are also fully dead
	n.childMu.Lock()
	allChildrenDead := n.deadBranchesCount == len(n.Children)
	n.childMu.Unlock()

	if allChildrenDead {
		n.AllBranchesDeadCycle = cycle
		n.propagateDeadToParent(cycle)
	}
}

// propagateDeadToParent increments the parent's dead branch count and
// recurses upward if the parent is also fully dead.
func (n *DescendantNode) propagateDeadToParent(cycle int) {
	parent := n.Parent
	if parent == nil {
		return
	}

	parent.childMu.Lock()
	parent.deadBranchesCount++
	allDead := parent.EndCycle != 0 && parent.deadBranchesCount == len(parent.Children)
	parent.childMu.Unlock()

	if allDead {
		parent.AllBranchesDeadCycle = cycle
		parent.propagateDeadToParent(cycle)
	}
}

// AddChild safely appends a child node and sets its parent pointer.
func (n *DescendantNode) AddChild(child *DescendantNode) {
	child.Parent = n
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
