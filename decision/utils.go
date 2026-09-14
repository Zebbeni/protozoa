package decision

import (
	"github.com/Zebbeni/protozoa/simrand"
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

// GetRandomCondition returns a random Condition drawn from the supplied
// pool — in practice MutableConditions. Every organism can express every
// node under ability scores, so the pool is global rather than
// per-organism; what differs between organisms is how well each action
// works, not which ones they can reference.
func GetRandomCondition(rng *simrand.RNG, allowed []Condition) Condition {
	return allowed[rng.Intn(len(allowed))]
}

// GetRandomAction returns a random Action drawn from the supplied
// allowed pool. See GetRandomCondition for how the pool is built.
func GetRandomAction(rng *simrand.RNG, allowed []Action) Action {
	return allowed[rng.Intn(len(allowed))]
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
