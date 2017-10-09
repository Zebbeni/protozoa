package decisions

import (
	"math/rand"
)

// Action is the custom type for all Organism actions
type Action int

// Condition is the custom type for all Organism conditions
type Condition int

// Sequence is a slice representing a serialized tree of NodeType values
type Sequence []interface{}

// Define all possible actions for Organism
const (
	ActEat Action = iota
	ActMove
	ActTurnLeft
	ActTurnRight
	CanMove Condition = iota
	IsOnFood
)

// Define slices
var (
	Actions    = [...]Action{ActEat, ActMove, ActTurnLeft, ActTurnRight}
	Conditions = [...]Condition{CanMove, IsOnFood}
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

// NewRandomSequence generates a new Sequence of random length
func NewRandomSequence() Sequence {
	sequence := MutateSequence(newRandomSubSequence())
	return sequence
}

// MutateSequence mutates a given sequence by replacing a random number of
// actions with condition - action - action blocks
func MutateSequence(sequence Sequence) Sequence {
	mutatedSequence := sequence
	index := rand.Intn(len(sequence))
	if isAction(sequence[index]) {
		// in 25 % of cases where the node is an Action, replace with C-A-A
		if rand.Float32() < 0.25 {
			// insert random subsquence in place of action index to be replaced
			subSequence := newRandomSubSequence()
			subSequence = append(subSequence, mutatedSequence[index+1:]...)
			mutatedSequence = append(mutatedSequence[:index], subSequence...)
		} else {
			mutatedSequence[index] = GetRandomAction()
		}
	} else {
		mutatedSequence[index] = GetRandomCondition()
	}
	return mutatedSequence
}

// TreeFromSequence recursively calls itself to create a Node and its
// children from a sequence slice.
func TreeFromSequence(sequence Sequence) Node {
	nodeType := sequence[0]
	if isAction(nodeType) {
		return Node{NodeType: nodeType, UseCount: 0}
	}
	index := 1
	numActionsMinusConditions := 0
	for numActionsMinusConditions < 1 {
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
	return Node{
		NodeType: nodeType,
		UseCount: 0,
		YesNode:  &yesNode,
		NoNode:   &noNode,
	}
}
