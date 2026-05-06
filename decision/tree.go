package decision

import (
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
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

// DeserializeTree reconstructs a Tree from a serialized string produced by Serialize().
func DeserializeTree(s string) *Tree {
	node, _ := Deserialize(s)
	if node == nil {
		return nil
	}
	return &Tree{
		ID:   s,
		Node: node,
	}
}

// CopyTree returns a new, identical decision tree
func (t *Tree) CopyTree() *Tree {
	tree := &Tree{
		ID:   t.ID,
		Node: t.Node.CopyNode(),
	}
	return tree
}

// MutateTree copies a root Tree, makes changes to the full tree, and returns
func MutateTree(rng *simrand.RNG, original *Tree) *Tree {
	tree := original.CopyTree()
	tree.mutate(rng)
	return tree
}

func (t *Tree) mutate(rng *simrand.RNG) {
	allSubNodes := t.getNodes()
	idx := rng.Intn(len(allSubNodes))
	isRoot := idx == 0
	node := allSubNodes[idx]

	maxTreeSize := config.MaxDecisionTreeSize()

	if node.IsAction() {
		if isRoot || (rng.Intn(2) == 0 && t.size <= maxTreeSize-2) {
			originalAction := node.NodeType.(Action)
			node.NodeType = GetRandomCondition(rng)
			if rng.Intn(2) == 0 {
				node.YesNode = NodeFromAction(GetRandomAction(rng))
				node.NoNode = NodeFromAction(originalAction)
			} else {
				node.YesNode = NodeFromAction(originalAction)
				node.NoNode = NodeFromAction(GetRandomAction(rng))
			}
		} else {
			node.NodeType = GetRandomAction(rng)
		}
	} else {
		randInt := rng.Intn(3)
		switch randInt {
		case 0:
			node = node.YesNode
			break
		case 1:
			node = node.NoNode
			break
		default:
			node.NodeType = GetRandomCondition(rng)
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
func (t *Tree) ActionWeights() map[Action]float64 {
	weights := make(map[Action]float64)
	t.Node.accumulateActionWeights(1.0, weights)
	return weights
}

// ConditionWeights returns the weighted distribution over conditions.
func (t *Tree) ConditionWeights() map[Condition]float64 {
	weights := make(map[Condition]float64)
	t.Node.accumulateConditionWeights(1.0, weights)
	return weights
}
