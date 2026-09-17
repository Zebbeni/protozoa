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
// returns the mutated copy.
//
// Every organism can express every node: what differs between them is
// how well each action works, which their ability scores decide. There
// is no per-organism pool to pass in, and therefore no way for an
// inherited tree to reference something its owner cannot perform.
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
			node.NodeType = GetRandomCondition(rng, MutableConditions)
			if rng.Intn(2) == 0 {
				node.YesNode = NodeFromAction(GetRandomAction(rng, MutableActions))
				node.NoNode = NodeFromAction(originalAction)
			} else {
				node.YesNode = NodeFromAction(originalAction)
				node.NoNode = NodeFromAction(GetRandomAction(rng, MutableActions))
			}
		} else {
			node.NodeType = GetRandomAction(rng, MutableActions)
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
			node.NodeType = GetRandomCondition(rng, MutableConditions)
			break
		}
	}

	t.size = t.CalcAndUpdateSize()
	t.ResetUsedLastCycle()
}

// ConditionNodes returns the condition at every condition node in the
// tree, in traversal order. Repeats are kept: a tree that tests for
// food three times is more of a food-sensing organism than one that
// tests once, and the appearance layer weighs that.
//
// Exported so the physiology layer can classify what an organism
// actually senses without reaching into node internals or parsing the
// printable form.
func (t *Tree) ConditionNodes() []Condition {
	nodes := t.getNodes()
	out := make([]Condition, 0, len(nodes))
	for _, n := range nodes {
		if c, ok := n.NodeType.(Condition); ok {
			out = append(out, c)
		}
	}
	return out
}

// ActionNodes returns every action the tree can take, in node order and
// with duplicates kept — the mirror of ConditionNodes. What a tree can
// *do* is as much a part of its behaviour as what it senses, and reading
// it needs the same access to node internals.
func (t *Tree) ActionNodes() []Action {
	nodes := t.getNodes()
	out := make([]Action, 0, len(nodes))
	for _, n := range nodes {
		if a, ok := n.NodeType.(Action); ok {
			out = append(out, a)
		}
	}
	return out
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
