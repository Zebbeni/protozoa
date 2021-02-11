package decisions

import (
	"bytes"
	"fmt"
	"math"
)

// Node includes an Action or Condition value
type Node struct {
	ID                               string
	NodeType                         interface{}
	YesNode                          *Node
	NoNode                           *Node
	AvgHealth, AvgHealthWhenTopLevel float64
	Uses, TopLevelUses               int
	Complexity                       int
	InDecisionTree, UsedLastCycle    bool
}

// IsAction returns true if Node's type is Action (false if Condition)
func (n *Node) IsAction() bool {
	return isAction(n.NodeType)
}

// IsCondition returns true if Node's type is Action (false if Condition)
func (n *Node) IsCondition() bool {
	return isCondition(n.NodeType)
}

// UpdateStats updates the toplevel Node's success rate on Health, traversing
// all nodes in the last used decision path.
// FUTURE: We may want more metrics besides health
func (n *Node) UpdateStats(health float64, topLevel bool, cyclesToConsider int) {
	n.UsedLastCycle = false

	n.Uses++
	uses := math.Min(float64(n.Uses), float64(cyclesToConsider))
	n.AvgHealth = (n.AvgHealth*(uses-1.0) + health) / uses

	if topLevel {
		n.TopLevelUses++
		uses = math.Min(float64(n.TopLevelUses), float64(cyclesToConsider))
		n.AvgHealthWhenTopLevel = (n.AvgHealthWhenTopLevel*(uses-1.0) + health) / uses
	}

	if n.IsCondition() {
		if n.YesNode.UsedLastCycle {
			n.YesNode.UpdateStats(health, false, cyclesToConsider)
		} else {
			n.NoNode.UpdateStats(health, false, cyclesToConsider)
		}
	}
}

// SetUsedInCurrentDecisionTree sets whether this Node is contained in a
// currently-used decision tree
func (n *Node) SetUsedInCurrentDecisionTree(isUsing bool) {
	n.InDecisionTree = isUsing
	if n.IsCondition() {
		n.YesNode.SetUsedInCurrentDecisionTree(isUsing)
		n.NoNode.SetUsedInCurrentDecisionTree(isUsing)
	}
}

// UpdateNodeIDs sets a Node's ID to a hyphen-separated string listing its
// decision tree in serialized form.
//
// Recursively walks through Node tree updating ID for itself and all children.
func (n *Node) UpdateNodeIDs() string {
	var buffer bytes.Buffer
	nodeTypeString := fmt.Sprintf("%v", n.NodeType)
	buffer.WriteString(nodeTypeString)
	if n.IsCondition() {
		buffer.WriteString("-")
		buffer.WriteString(n.YesNode.UpdateNodeIDs())
		buffer.WriteString("-")
		buffer.WriteString(n.NoNode.UpdateNodeIDs())
	}
	n.ID = buffer.String()
	return n.ID
}

// TreeFromAction creates a simple Node object from an Action type
func TreeFromAction(action Action) *Node {
	node := &Node{
		NodeType:      action,
		Uses:          0.0,
		YesNode:       nil,
		NoNode:        nil,
		UsedLastCycle: false,
		AvgHealth:     0.0,
	}
	node.UpdateNodeIDs()
	return node
}
