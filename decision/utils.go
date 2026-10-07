package decision

import (
	"github.com/Zebbeni/protozoa/simrand"
)

// CalcAndUpdateSize returns the total number of nodes descending from this root node (including itself) Update each node's size value to avoid calculating this multiple times
func (n *Node) CalcAndUpdateSize() int {
	if n.IsAction() {
		n.size = 1
		return 1
	}

	n.size = 1 + n.YesNode.CalcAndUpdateSize() + n.NoNode.CalcAndUpdateSize()
	return n.size
}

// GetRandomCondition returns a random Condition drawn from the supplied pool — in practice MutableConditions.
func GetRandomCondition(rng *simrand.RNG, allowed []Condition) Condition {
	return allowed[rng.Intn(len(allowed))]
}

func GetRandomAction(rng *simrand.RNG, allowed []Action) Action {
	return allowed[rng.Intn(len(allowed))]
}

func isAction(v interface{}) bool {
	switch v.(type) {
	case Action:
		return true
	}
	return false
}

func isCondition(v interface{}) bool {
	switch v.(type) {
	case Condition:
		return true
	}
	return false
}
