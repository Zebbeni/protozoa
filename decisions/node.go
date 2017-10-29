package decisions

import (
	"math/rand"

	c "../constants"
)

// Node includes an Action or Condition value
type Node struct {
	ID       string
	NodeType interface{}
	YesNode  *Node
	NoNode   *Node
	Metrics  map[Metric]int
	Uses     int
}

// IsAction returns true if Node's type is Action (false if Condition)
func (n *Node) IsAction() bool {
	return isAction(n.NodeType)
}

// MutateSequence mutates a given sequence by replacing a random sequence node
// with either a condition or an action
func MutateSequence(sequence Sequence) Sequence {
	mutatedSequence := make(Sequence, len(sequence))
	copy(mutatedSequence, sequence)
	// make several passes and mutate multiple nodes
	for n := 0; n < rand.Intn(c.MaxNodesToMutate); n++ {
		index := rand.Intn(len(mutatedSequence))
		if rand.Float32() < c.PercentActions {
			mutatedSequence[index] = GetRandomAction()
		} else {
			mutatedSequence[index] = GetRandomCondition()
		}
	}
	return mutatedSequence
}

// TreeFromAction creates a simple Node object from an Action type
func TreeFromAction(action Action) Node {
	node := Node{
		NodeType: action,
		Uses:     0,
		YesNode:  nil,
		NoNode:   nil,
	}
	node.Metrics = InitializeMetricsMap()
	node.UpdateNodeIDs()
	return node
}

// TreeFromSequence recursively calls itself to create a Node and its
// children from a sequence slice.
func TreeFromSequence(sequence Sequence) Node {
	if sequence == nil || len(sequence) == 0 {
		return Node{NodeType: ActIdle, Uses: 0}
	}
	nodeType := sequence[0]
	if isAction(nodeType) {
		return Node{NodeType: nodeType, Uses: 0}
	}
	index := 0
	numActionsMinusConditions := 0
	for numActionsMinusConditions < 1 && index < len(sequence) {
		sequenceItem := sequence[index]
		if isAction(sequenceItem) {
			numActionsMinusConditions++
		} else {
			numActionsMinusConditions--
		}
		index++
	}
	yesNode := TreeFromSequence(sequence[1:index])
	noNode := TreeFromSequence(sequence[index:])
	node := Node{
		NodeType: nodeType,
		Uses:     0,
		YesNode:  &yesNode,
		NoNode:   &noNode,
	}
	return node
}
