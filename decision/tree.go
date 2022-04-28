package decision

import (
	"math/rand"

	"github.com/Zebbeni/protozoa/config"
)

// Tree is a Node with info to track its success as a top-level decision tree
type Tree struct {
	ID string
	*Node
}

// TreeFromAction returns a simple decision Tree from an Action type
func TreeFromAction(action Action) *Tree {
	tree := &Tree{
		Node: NodeFromAction(action),
	}
	tree.ID = tree.Serialize()
	return tree
}

// CopyTree returns a new, identical decision tree
// Includes current stats as well if copyHistory=true
func (t *Tree) CopyTree() *Tree {
	tree := &Tree{
		ID:   t.ID,
		Node: t.Node.CopyNode(),
	}
	return tree
}

// MutateTree copies a root Tree, makes changes to the full tree, and returns
func MutateTree(original *Tree) *Tree {
	tree := original.CopyTree()
	tree.mutate()
	return tree
}

// mutate randomly mutates a single node of a tree. This function
// should only be called on root tree nodes because it uses the tree size.
func (t *Tree) mutate() {
	// pick a random t anywhere in the decision tree
	allSubNodes := t.getNodes()
	node := allSubNodes[rand.Intn(len(allSubNodes))]

	maxTreeSize := config.MaxDecisionTreeSize()

	if node.IsAction() {
		if rand.Intn(2) == 0 && t.size < maxTreeSize-1 {
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

	t.size = t.CalcAndUpdateSize()
	t.ResetUsedLastCycle()
}

func (t *Tree) Size() int {
	return t.size
}

// Print prints the full tree structure
func (t *Tree) Print() string {
	return t.print("", true, false)
}
