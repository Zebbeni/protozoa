package decision

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/Zebbeni/protozoa/config"
)

// Tree is a Node with info to track its success as a top-level decision tree
type Tree struct {
	ID              string
	AvgHealthChange float64
	Uses            int
	*Node
}

// TreeFromAction returns a simple decision Tree from an Action type
func TreeFromAction(action Action) *Tree {
	tree := &Tree{
		Node: NodeFromAction(action),
	}
	tree.ID = tree.GenerateID()
	return tree
}

// UpdateStats updates a Tree's Uses and average health change history
// cyclesToConsider provides a maximum number of cycles to factor
func (t *Tree) UpdateStats(health float64) {
	t.Uses++
	uses := math.Min(float64(t.Uses), float64(config.MaxCyclesToCalculateStatsAverage()))
	t.AvgHealthChange = (t.AvgHealthChange*(uses-1.0) + health) / uses

	// set UsedLastCycle to false for this and all child Nodes
	t.ResetUsedLastCycle()
}

// CopyTree returns a new, identical decision tree
// Includes current stats as well if copyHistory=true
func (t *Tree) CopyTree(copyHistory bool) *Tree {
	tree := &Tree{
		ID:   t.ID,
		Node: t.Node.CopyNode(),
	}
	if copyHistory {
		tree.AvgHealthChange = t.AvgHealthChange
		tree.Uses = t.Uses
	}
	return tree
}

// MutateTree copies a root Tree, makes changes to the full tree, and returns
func MutateTree(original *Tree) *Tree {
	tree := original.CopyTree(false)
	tree.mutate()
	return tree
}

// mutate randomly mutates a single tree of a tree. This function
// should only be called on root tree nodes because it uses the tree size.
func (t *Tree) mutate() {
	// pick a random t anywhere in the decision tree
	allSubNodes := t.getNodes()
	node := allSubNodes[rand.Intn(len(allSubNodes))]

	treeSize := t.Size()
	maxTreeSize := config.MaxDecisionTreeSize()

	if node.IsAction() {
		if rand.Intn(2) == 0 && treeSize < maxTreeSize-1 {
			// convert action to condition + 2 actions
			originalAction := node.NodeType.(Action)
			node.NodeType = GetRandomCondition()
			if rand.Intn(2) == 0 {
				node.YesNode = NodeFromAction(GetRandomAction())
				node.NoNode = NodeFromAction(originalAction)
			} else {
				node.YesNode = NodeFromAction(originalAction)
				node.NoNode = NodeFromAction(GetRandomAction())
			}
		} else {
			// change action type
			node.NodeType = GetRandomAction()
		}
	} else {
		if rand.Intn(2) == 0 {
			// convert condition to action (simplify)
			node.NodeType = GetRandomAction()
			node.YesNode = nil
			node.NoNode = nil
		} else {
			// change condition type
			node.NodeType = GetRandomCondition()
		}
	}
	node.UsedLastCycle = false
	t.UsedLastCycle = false
	t.ID = t.GenerateID()
}

// PrintStats prints the tree uses and average health change when used
func (t *Tree) PrintStats() string {
	return fmt.Sprintf(
		"Uses:%d\nΔHealth: %.2f\n",
		t.Uses,
		t.AvgHealthChange,
	)
}

// Print prints the full tree structure
func (t *Tree) Print() string {
	return t.print("", true, false)
}
