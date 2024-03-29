package decision

import (
	"math/rand"
)

// CalcAndUpdateSize returns the total number of nodes descending from this root node (including itself)
// Update each node's size value to avoid calculating this multiple times
func (n *Node) CalcAndUpdateSize() int {
	if n.IsAction() {
		n.size = 1
		return 1
	}

	n.size = 1 + n.YesNode.CalcAndUpdateSize() + n.NoNode.CalcAndUpdateSize()
	return n.size
}

// GetRandomCondition returns a random Condition from the Conditions array
func GetRandomCondition() Condition {
	return Conditions[rand.Intn(len(Conditions))]
}

// GetRandomAction returns a random Action from the Actions array
func GetRandomAction() Action {
	return Actions[rand.Intn(len(Actions))]
}

// isAction returns true if the object passed in is an Action
func isAction(v interface{}) bool {
	switch v.(type) {
	case Action:
		return true
	}
	return false
}

// isCondition returns true if the object passed in is a Condition
func isCondition(v interface{}) bool {
	switch v.(type) {
	case Condition:
		return true
	}
	return false
}
