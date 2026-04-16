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
	idx := rand.Intn(len(allSubNodes))
	isRoot := idx == 0
	node := allSubNodes[idx]

	maxTreeSize := config.MaxDecisionTreeSize()

	if node.IsAction() {
		if isRoot || (rand.Intn(2) == 0 && t.size <= maxTreeSize-2) {
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
		randInt := rand.Intn(3)
		switch randInt {
		case 0:
			// option 1: replace condition with its yes node
			node = node.YesNode
			break
		case 1:
			// option 2: replace condition with its no node
			node = node.NoNode
			break
		default:
			// option 3: change condition type
			node.NodeType = GetRandomCondition()
			break
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

// PrintLines returns structured line data for rendering with per-line styling.
func (t *Tree) PrintLines() []PrintLine {
	return t.printLines("", true, false)
}

// ActionWeights returns the weighted probability distribution over actions.
// At each condition node, weight is split 50/50 to yes/no branches.
// The result maps each reachable action to its total weight (summing to 1.0).
func (t *Tree) ActionWeights() map[Action]float64 {
	weights := make(map[Action]float64)
	t.Node.accumulateActionWeights(1.0, weights)
	return weights
}

// ConditionWeights returns the weighted distribution over conditions.
// Each condition node receives the full weight flowing through it, and splits
// 50/50 to its children. Conditions closer to the root have more weight.
func (t *Tree) ConditionWeights() map[Condition]float64 {
	weights := make(map[Condition]float64)
	t.Node.accumulateConditionWeights(1.0, weights)
	return weights
}
