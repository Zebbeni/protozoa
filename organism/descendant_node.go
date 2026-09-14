package organism

import (
	"image/color"
	"sync"

	"github.com/Zebbeni/protozoa/physiology"
)

// DescendantNode represents a single organism in the ancestor family tree.
type DescendantNode struct {
	ID    int
	Color color.Color
	// Abilities is the organism's ability distribution, recorded on the
	// node so views over the whole family history — the population
	// graph's ability colouring — can read it for organisms that are
	// long dead. Scores are fixed for an organism's lifetime, so a copy
	// taken at birth never goes stale.
	Abilities  physiology.Scores
	StartCycle int
	EndCycle   int // 0 means still alive
	// LineageEndCycle is the latest death in this organism's line of
	// descent (itself and every descendant), or 0 if any of them is
	// still alive at the end of the recorded run. Precomputed once when
	// a recording's trees load (see ComputeLineageEnds); 0 on every node
	// of a live run, where the end isn't known yet.
	LineageEndCycle int
	Parent          *DescendantNode
	Children        []*DescendantNode
	childMu         sync.Mutex

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

// ComputeLineageEnds fills LineageEndCycle on every node of the given trees
// in one post-order pass, and returns the end of the recorded run.
//
// The end of the run is taken as the latest birth or death anywhere in the
// trees: the recorded trees don't carry the final cycle, and in a populated
// run something is born or dies within a cycle or two of the end. Reads
// EndCycle only — set authoritatively on death and never changed after a
// load — rather than AllBranchesDeadCycle, which replay forward play can
// shift.
func ComputeLineageEnds(trees map[int]*DescendantNode) (runEnd int) {
	var span func(*DescendantNode)
	span = func(n *DescendantNode) {
		runEnd = max(runEnd, n.StartCycle, n.EndCycle)
		n.ForEachChild(span)
	}
	for _, root := range trees {
		if root != nil {
			span(root)
		}
	}

	// lineageEnd returns the latest EndCycle in n's subtree, or 0 if
	// anything in it is still alive at the end.
	var lineageEnd func(*DescendantNode) int
	lineageEnd = func(n *DescendantNode) int {
		latest := n.EndCycle
		survivor := n.EndCycle == 0
		n.ForEachChild(func(c *DescendantNode) {
			d := lineageEnd(c)
			if d == 0 {
				survivor = true
			}
			latest = max(latest, d)
		})
		if survivor {
			latest = 0
		}
		n.LineageEndCycle = latest
		return latest
	}
	for _, root := range trees {
		if root != nil {
			lineageEnd(root)
		}
	}
	return runEnd
}

// LineageSuccess is how much of the run still to come an organism's line of
// descent survives, viewed from cycle now: (lineageEnd - now) / (runEnd -
// now), clamped to [0, 1]. Both sides subtract the cycles already elapsed,
// so the measure is always relative to the time left rather than the whole
// run.
//
// It is 1 when a descendant survives to the end (lineageEnd 0), and also
// when the end isn't known (runEnd 0, a live run) or has already been
// reached.
func LineageSuccess(lineageEnd, now, runEnd int) float64 {
	if lineageEnd == 0 || runEnd <= now {
		return 1
	}
	return min(1, max(0, float64(lineageEnd-now)/float64(runEnd-now)))
}
