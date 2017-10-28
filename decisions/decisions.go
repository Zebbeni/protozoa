package decisions

import (
	"bytes"
	"math/rand"

	c "../constants"
)

// Action is the custom type for all Organism actions
type Action int

// Condition is the custom type for all Organism conditions
type Condition int

// Sequence is a slice representing a serialized tree of NodeType values
type Sequence []interface{}

// Define all possible actions for Organism
const (
	ActAttack Action = iota
	ActEat
	ActIdle
	ActMove
	ActReproduce
	ActTurnLeft
	ActTurnRight
	CanMove Condition = iota
	CanReproduce
	IsFoodAhead
	IsFoodLeft
	IsFoodRight
	IsOrganismAhead
	IsOrganismLeft
	IsOrganismRight
)

// Define slices
var (
	Actions    = [...]Action{ActEat, ActIdle, ActMove, ActReproduce, ActTurnLeft, ActTurnRight}
	Conditions = [...]Condition{CanMove, CanReproduce, IsFoodAhead, IsFoodLeft, IsFoodRight, IsOrganismAhead, IsOrganismLeft, IsOrganismRight}
	Map        = map[interface{}]string{
		ActAttack:       "Attack",
		ActEat:          "Eat",
		ActIdle:         "Be Idle",
		ActMove:         "Move Ahead",
		ActReproduce:    "Reproduce",
		ActTurnLeft:     "Turn Left",
		ActTurnRight:    "Turn Right",
		CanMove:         "If Can Move Ahead",
		CanReproduce:    "If Can Reproduce",
		IsFoodAhead:     "If Food Ahead",
		IsFoodLeft:      "If Food Left",
		IsFoodRight:     "If Food Right",
		IsOrganismAhead: "If Organism Ahead",
		IsOrganismLeft:  "If Organism Left",
		IsOrganismRight: "If Organism Right",
	}
)

// Node includes an Action or Condition value
type Node struct {
	NodeType interface{}
	UseCount int
	YesNode  *Node
	NoNode   *Node
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

// TreeFromSequence recursively calls itself to create a Node and its
// children from a sequence slice.
func TreeFromSequence(sequence Sequence) Node {
	if sequence == nil || len(sequence) == 0 {
		return Node{NodeType: ActIdle, UseCount: 0}
	}
	nodeType := sequence[0]
	if isAction(nodeType) {
		return Node{NodeType: nodeType, UseCount: 0}
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
		UseCount: 0,
		YesNode:  &yesNode,
		NoNode:   &noNode,
	}
	return node
}

// PrintSequence prints sequence chronologically
func PrintSequence(sequence Sequence) string {
	var buffer bytes.Buffer
	for i, s := range sequence {
		if i > 0 {
			buffer.WriteString(" | ")
		}
		buffer.WriteString(Map[s])
	}
	return buffer.String()
}

// PrintNode prints node and all children showing hierarchy
func PrintNode(node Node, spaces int) string {
	var buffer bytes.Buffer
	buffer.WriteString(Map[node.NodeType])
	buffer.WriteString("\n")
	if !isAction(node.NodeType) {
		for i := 0; i < spaces; i++ {
			buffer.WriteString("  ")
		}
		buffer.WriteString("Then: ")
		buffer.WriteString(PrintNode(*node.YesNode, spaces+1))
		for i := 0; i < spaces; i++ {
			buffer.WriteString("  ")
		}
		buffer.WriteString("Otherwise: ")
		buffer.WriteString(PrintNode(*node.NoNode, spaces+1))
	}
	return buffer.String()
}
