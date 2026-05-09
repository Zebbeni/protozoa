package organism

import (
	"image/color"
	"sync"
)

// DescendantNode represents a single organism in the ancestor family tree.
type DescendantNode struct {
	ID         int
	Color      color.Color
	StartCycle int
	EndCycle   int // 0 means still alive
	Parent     *DescendantNode
	Children   []*DescendantNode
	childMu    sync.Mutex

	// Dead-branch pruning: skip entire sub-trees during tree walks
	deadBranchesCount    int // how many direct children have all branches dead
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
