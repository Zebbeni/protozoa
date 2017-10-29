package decisions

import (
	"bytes"
	"fmt"
)

// Node includes an Action or Condition value
type Node struct {
	ID       string
	NodeType interface{}
	YesNode  *Node
	NoNode   *Node
	Metrics  map[Metric]float32
	Uses     int
}

// IsAction returns true if Node's type is Action (false if Condition)
func (n *Node) IsAction() bool {
	return isAction(n.NodeType)
}

// UpdateStats updates all Node Metrics according to a map of changes and
// increments number of Uses
func (n *Node) UpdateStats(metricsChange map[Metric]float32) {
	n.Uses++
	for key, change := range metricsChange {
		n.Metrics[key] += change
	}
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

// UpdateNodeIDs sets a Node's ID to a hyphen-separated string listing its
// decision tree in serialized form.
//
// Recursively walks through Node tree updating ID for itself and all children.
func (node *Node) UpdateNodeIDs() string {
	var buffer bytes.Buffer
	nodeTypeString := fmt.Sprintf("%v", node.NodeType)
	buffer.WriteString(nodeTypeString)
	if !isAction(node.NodeType) {
		buffer.WriteString("-")
		buffer.WriteString(node.YesNode.UpdateNodeIDs())
		buffer.WriteString("-")
		buffer.WriteString(node.NoNode.UpdateNodeIDs())
	}
	node.ID = buffer.String()
	return node.ID
}
