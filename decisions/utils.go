package decisions

import (
	"bytes"
	"math/rand"
	"strconv"

	c "../constants"
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

// InitializeMetricsMap returns an initialize map of each Metric type to 0
func InitializeMetricsMap() map[Metric]int {
	return map[Metric]int{
		MetricFood:   0,
		MetricHealth: 0,
	}
}

// CopyTreeByValue recursively copies an existing tree by value given an
// existing one, initializing uses and metrics to 0.
func CopyTreeByValue(source *Node) *Node {
	destination := Node{
		ID:       source.ID,
		NodeType: source.NodeType,
		Metrics:  InitializeMetricsMap(),
		Uses:     0,
		YesNode:  CopyTreeByValue(source.YesNode),
		NoNode:   CopyTreeByValue(source.NoNode),
	}
	return &destination
}

// UpdateNodeIDs sets a Node's ID to a hyphen-separated string listing its
// decision tree in serialized form.
//
// Recursively walks through Node tree updating ID for itself and all children.
func (node *Node) UpdateNodeIDs() string {
	var buffer bytes.Buffer
	nodeTypeString := strconv.Itoa(node.NodeType.(int))
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

// NewRandomSequence generates a new Sequence of random length
func NewRandomSequence() Sequence {
	numSequenceNodes := rand.Intn(c.MaxSequenceNodes)
	sequence := make(Sequence, numSequenceNodes)
	for n := 0; n < numSequenceNodes; n++ {
		if rand.Float32() < c.PercentActions {
			sequence[n] = GetRandomAction()
		} else {
			sequence[n] = GetRandomCondition()
		}
	}
	return sequence
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
