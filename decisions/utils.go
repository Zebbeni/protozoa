package decisions

import (
	"fmt"
	"math/rand"
)

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

// InitializeMetricsMap returns an initialize map of each Metric type to 0
func InitializeMetricsMap() map[Metric]float64 {
	return map[Metric]float64{
		MetricHealth: 0.0,
	}
}

// CopyTreeByValue recursively copies an existing tree by value given an
// existing one, initializing metrics to 0.
func CopyTreeByValue(source *Node) *Node {
	if source == nil {
		return nil
	}
	destination := Node{
		ID:          source.ID,
		NodeType:    source.NodeType,
		Metrics:     InitializeMetricsMap(),
		MetricsAvgs: InitializeMetricsMap(),
		Uses:        source.Uses,
		UsedYes:     source.UsedYes,
		UsedNo:      source.UsedNo,
	}
	destination.YesNode = CopyTreeByValue(source.YesNode)
	destination.NoNode = CopyTreeByValue(source.NoNode)
	return &destination
}

// MutateTree copies a root Node, makes changes to the full tree, and returns
func MutateTree(original *Node) *Node {
	mutated := CopyTreeByValue(original)
	MutateNode(mutated)
	mutated.UpdateNodeIDs()
	return mutated
}

// MutateNode randomly mutates nodes of a tree
func MutateNode(node *Node) {
	node.Uses = 0
	if isCondition(node.NodeType) {
		// If node is a condition and one of its paths has 0 uses, try
		// switching the condition type
		if !node.UsedYes || !node.UsedNo {
			node.NodeType = GetRandomCondition()
		} else {
			if rand.Float64() < 0.5 {
				MutateNode(node.YesNode)
			} else {
				MutateNode(node.NoNode)
			}
		}
	} else {
		if rand.Float64() < ChanceOfAddingNewSubTree {
			originalAction := node.NodeType.(Action)
			node.NodeType = GetRandomCondition()
			node.UsedYes = false
			node.UsedNo = false
			yesNode := Node{}
			noNode := Node{}
			if rand.Float64() < 0.5 {
				yesNode = TreeFromAction(GetRandomAction())
				noNode = TreeFromAction(originalAction)
			} else {
				yesNode = TreeFromAction(originalAction)
				noNode = TreeFromAction(GetRandomAction())
			}
			node.YesNode = &yesNode
			node.NoNode = &noNode
		} else {
			node.NodeType = GetRandomAction()
		}
	}
}

// Print pretty prints the node
func (node *Node) Print(indent string, last bool) string {
	toPrint := indent
	newIndent := indent
	if last {
		toPrint = fmt.Sprintf("%s└─", toPrint)
		newIndent = fmt.Sprintf("%s  ", newIndent)
	} else {
		toPrint = fmt.Sprintf("%s├─", toPrint)
		newIndent = fmt.Sprintf("%s│ ", newIndent)
	}
	toPrint = fmt.Sprintf("%s%s\n", toPrint, Map[node.NodeType])
	if !isAction(node.NodeType) {
		toPrint = fmt.Sprintf("%s%s", toPrint, node.YesNode.Print(newIndent, false))
		toPrint = fmt.Sprintf("%s%s", toPrint, node.NoNode.Print(newIndent, true))
	}
	return toPrint
}
