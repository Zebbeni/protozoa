package decision

import (
	"bytes"
	"fmt"
)

// Node contains an Action or Condition NodeType and (if a Condition), child
// references for its conditional branches
type Node struct {
	NodeType                      interface{}
	InDecisionTree, UsedLastCycle bool
	YesNode, NoNode               *Node
	size                          int
}

// NodeFromAction creates a simple Node object from an Action type
func NodeFromAction(action Action) *Node {
	return &Node{
		NodeType: action,
		size:     1,
	}
}

// IsAction returns true if Tree's type is Action (false if Condition)
func (n *Node) IsAction() bool {
	return isAction(n.NodeType)
}

// IsCondition returns true if Tree's type is Action (false if Condition)
func (n *Node) IsCondition() bool {
	return isCondition(n.NodeType)
}

// CopyNode returns a new Node with the same structure as the original
func (n Node) CopyNode() *Node {
	copy := &Node{
		NodeType:      n.NodeType,
		UsedLastCycle: n.UsedLastCycle,
		size:          n.size,
	}
	if n.IsAction() {
		return copy
	}
	copy.YesNode = n.YesNode.CopyNode()
	copy.NoNode = n.NoNode.CopyNode()
	return copy
}

// SetUsedInCurrentTree sets whether this Node is contained in a
// currently-used decision tree
func (n *Node) SetUsedInCurrentTree(isUsing bool) {
	n.InDecisionTree = isUsing
	if n.IsCondition() {
		n.YesNode.SetUsedInCurrentTree(isUsing)
		n.NoNode.SetUsedInCurrentTree(isUsing)
	}
}

// ResetUsedLastCycle triggers this Node (and any previously-used child Nodes)
// to set UsedLastCycle to false
func (n *Node) ResetUsedLastCycle() {
	n.UsedLastCycle = false
	if n.IsCondition() {
		if n.YesNode.UsedLastCycle {
			n.YesNode.ResetUsedLastCycle()
		} else {
			n.NoNode.ResetUsedLastCycle()
		}
	}
}

// Serialize generates and returns a string representing a Node's
// full Tree structure.
//
// Recursively walks through the Node tree to accumulate a string representing
// itself and all its children
func (n *Node) Serialize() string {
	var buffer bytes.Buffer
	nodeTypeString := fmt.Sprintf("%02d", n.NodeType)
	buffer.WriteString(nodeTypeString)
	if n.IsCondition() {
		buffer.WriteString(n.YesNode.Serialize())
		buffer.WriteString(n.NoNode.Serialize())
	}
	return buffer.String()
}

// getNodes returns a list of all nodes in a tree starting with the given root
func (n *Node) getNodes() (nodes []*Node) {
	nodes = make([]*Node, 0, n.size)
	nodes = append(nodes, n)
	if n.IsAction() {
		return
	}

	nodes = append(nodes, n.YesNode.getNodes()...)
	nodes = append(nodes, n.NoNode.getNodes()...)
	return
}

func (n *Node) print(indent string, first, last bool) string {
	toPrint := indent
	newIndent := indent
	if first {
		toPrint = fmt.Sprintf("%s", toPrint)
	} else if last {
		toPrint = fmt.Sprintf("%s└─", toPrint)
		newIndent = fmt.Sprintf("%s  ", newIndent)
	} else {
		toPrint = fmt.Sprintf("%s├─", toPrint)
		newIndent = fmt.Sprintf("%s│ ", newIndent)
	}
	if n.UsedLastCycle {
		toPrint = fmt.Sprintf("%s%s ◀◀\n", toPrint, Map[n.NodeType])
	} else {
		toPrint = fmt.Sprintf("%s%s\n", toPrint, Map[n.NodeType])
	}
	if n.IsCondition() {
		toPrint = fmt.Sprintf("%s%s", toPrint, n.YesNode.print(newIndent, false, false))
		toPrint = fmt.Sprintf("%s%s", toPrint, n.NoNode.print(newIndent, false, true))
	}
	return toPrint
}
