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

func TreeFromAction(action Action) *Tree {
	tree := &Tree{
		Node: NodeFromAction(action),
	}
	tree.ID = tree.Serialize()
	return tree
}

// TreeFromNode wraps a hand-built node as a Tree, recomputing the sizes along it and deriving the ID from its serialized form.
func TreeFromNode(node *Node) *Tree {
	if node == nil {
		return nil
	}
	node.CalcAndUpdateSize()
	tree := &Tree{Node: node}
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

func (t *Tree) CopyTree() *Tree {
	tree := &Tree{
		ID:   t.ID,
		Node: t.Node.CopyNode(),
	}
	return tree
}

// MutateTree copies a root Tree, makes changes to the full tree, and returns the mutated copy.
func MutateTree(rng *simrand.RNG, original *Tree) *Tree {
	tree := original.CopyTree()
	tree.mutate(rng)
	return tree
}

// mutate applies one mutation to the tree: pick a node at random, then pick what to do to it.
func (t *Tree) mutate(rng *simrand.RNG) {
	allSubNodes := t.getNodes()
	idx := rng.Intn(len(allSubNodes))
	node := allSubNodes[idx]

	maxTreeSize := config.MaxDecisionTreeSize()

	if node.IsAction() {
		// Growing adds two nodes, so it needs the room.
		canGrow := t.size <= maxTreeSize-2
		if canGrow && pickFirst(rng, config.MutationWeightGrowBranch(), config.MutationWeightSwapAction()) {
			originalAction := node.NodeType.(Action)
			node.NodeType = GetRandomCondition(rng, t.newConditionPool(node))
			if rng.Intn(2) == 0 {
				node.YesNode = NodeFromAction(GetRandomAction(rng, EnabledActions()))
				node.NoNode = NodeFromAction(originalAction)
			} else {
				node.YesNode = NodeFromAction(originalAction)
				node.NoNode = NodeFromAction(GetRandomAction(rng, EnabledActions()))
			}
		} else {
			node.NodeType = GetRandomAction(rng, EnabledActions())
		}
	} else {
		// Pruning is only offered where it is the exact inverse of growing.
		canPrune := node.YesNode != nil && node.NoNode != nil &&
			node.YesNode.IsAction() && node.NoNode.IsAction()
		if canPrune && pickFirst(rng, config.MutationWeightPruneBranch(), config.MutationWeightSwapCondition()) {
			survivor := node.YesNode
			if rng.Intn(2) == 0 {
				survivor = node.NoNode
			}
			node.NodeType = survivor.NodeType
			node.YesNode, node.NoNode = nil, nil
		} else {
			node.NodeType = GetRandomCondition(rng, t.swapConditionPool(node))
		}
	}

	t.size = t.CalcAndUpdateSize()
	t.ResetUsedLastCycle()
}

func pickFirst(rng *simrand.RNG, first, second float64) bool {
	roll := rng.Float64()
	total := first + second
	if total <= 0 {
		return true
	}
	return roll < first/total
}

func (t *Tree) newConditionPool(node *Node) []Condition {
	if !config.TieredConditionMutation() {
		return t.conditionPool(node)
	}
	return t.narrow(node, BasicConditions())
}

func (t *Tree) swapConditionPool(node *Node) []Condition {
	if !config.TieredConditionMutation() {
		return t.conditionPool(node)
	}
	current, ok := node.NodeType.(Condition)
	if !ok {
		// Only reached from the condition branch of mutate.
		return t.conditionPool(node)
	}
	return t.narrow(node, LadderConditions(current))
}

// narrow applies the smart-mutation filter to an already-restricted pool, falling back to the unfiltered one if nothing survives.
func (t *Tree) narrow(node *Node, pool []Condition) []Condition {
	if !config.SmartTreeMutation() {
		return pool
	}
	above, found := t.Node.factsAbove(node)
	if !found {
		return pool
	}
	if useful := usefulConditions(pool, above, node.downstreamConditions()); len(useful) > 0 {
		return useful
	}
	return pool
}

// conditionPool is the set of conditions mutation may put at node.
func (t *Tree) conditionPool(node *Node) []Condition {
	pool := EnabledConditions()
	if !config.SmartTreeMutation() {
		return pool
	}
	above, found := t.Node.factsAbove(node)
	if !found {
		return pool
	}
	return usefulConditions(pool, above, node.downstreamConditions())
}

// ConditionNodes returns the condition at every condition node in the tree, in traversal order.
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

// ActionNodes returns every action the tree can take, in node order and with duplicates kept.
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

func (t *Tree) Print() string {
	return t.print("", true, false)
}

// PrintLines returns structured line data for rendering with per-line styling.
func (t *Tree) PrintLines() []PrintLine {
	return t.printLines("", true, false)
}
