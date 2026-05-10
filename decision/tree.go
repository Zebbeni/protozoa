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

// MutateTree copies a root Tree, makes changes to the full tree, and
// returns the mutated copy. The allowed pools constrain mutation to
// actions/conditions the organism's physiology supports — typically
// derived from the spawning organism's feature set via
// physiology.Set.AllowedActions / AllowedConditions.
func MutateTree(rng *simrand.RNG, original *Tree, allowedActions []Action, allowedConditions []Condition) *Tree {
	tree := original.CopyTree()
	tree.mutate(rng, allowedActions, allowedConditions)
	return tree
}

func (t *Tree) mutate(rng *simrand.RNG, allowedActions []Action, allowedConditions []Condition) {
	allSubNodes := t.getNodes()
	idx := rng.Intn(len(allSubNodes))
	isRoot := idx == 0
	node := allSubNodes[idx]

	maxTreeSize := config.MaxDecisionTreeSize()

	if node.IsAction() {
		if isRoot || (rng.Intn(2) == 0 && t.size <= maxTreeSize-2) {
			originalAction := node.NodeType.(Action)
			node.NodeType = GetRandomCondition(rng, allowedConditions)
			if rng.Intn(2) == 0 {
				node.YesNode = NodeFromAction(GetRandomAction(rng, allowedActions))
				node.NoNode = NodeFromAction(originalAction)
			} else {
				node.YesNode = NodeFromAction(originalAction)
				node.NoNode = NodeFromAction(GetRandomAction(rng, allowedActions))
			}
		} else {
			node.NodeType = GetRandomAction(rng, allowedActions)
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
			node.NodeType = GetRandomCondition(rng, allowedConditions)
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
