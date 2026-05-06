package organism

import (
	"image/color"
	"math"
	"sync"

	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
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
//
// Neutral pH effect (spectrum value 0.5) blends into the active theme's
// background so it visually disappears against the window fill. The
// extremes stay high-contrast. Colours stored on DescendantNode at
// organism birth are baked in at that moment — toggling themes mid-run
// won't re-theme existing organisms, only those born afterwards — but the
// dynamic pH-effect distribution graph recomputes each render and picks
// up theme changes immediately.
func ComputePhEffectColor(spectrumValue float64) colorful.Color {
	const phMaxHue = 100.0
	hue := phMaxHue - (phMaxHue * spectrumValue)
	dist := math.Abs(spectrumValue - 0.5)
	sat := 0.5 + dist
	light := dist
	if c.IsLightTheme() {
		light = 1 - dist
	}
	col := colorful.HSLuv(hue, sat, light)
	// Blend towards the theme background by distance from neutral, so
	// dist=0 hands back the pure background colour and dist=0.5 hands
	// back the pure HSLuv curve value.
	bgR, bgG, bgB := c.ThemeBackgroundRGB()
	weight := dist * 2 // [0, 1]
	return colorful.Color{
		R: weight*col.R + (1-weight)*bgR,
		G: weight*col.G + (1-weight)*bgG,
		B: weight*col.B + (1-weight)*bgB,
	}
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
	// In replay mode, the tree is restored from a snapshot with EndCycle
	// already populated on every node from its original death. For an
	// ancestor whose organism hasn't yet died in the current replay
	// timeline, EndCycle is the *future* recorded death cycle, not 0.
	// Without the EndCycle <= cycle check, a single dying leaf could
	// trigger ABDC propagation up through still-alive ancestors and
	// blank out the population graph from then on.
	parentDead := parent.EndCycle != 0 && parent.EndCycle <= cycle
	allDead := parentDead && parent.deadBranchesCount == len(parent.Children)
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
