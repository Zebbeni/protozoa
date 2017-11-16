package decisions

import (
	"image/color"
	"math/rand"
)

// NodeLibrary contains a map of Node pointers to aggregated data for each node
type NodeLibrary struct {
	Map map[string]*Node
}

// NewNodeLibrary creates a new *NodeLibrary, initialized with single Actions
func NewNodeLibrary() NodeLibrary {
	nodeLibrary := NodeLibrary{
		Map: make(map[string]*Node),
	}
	for _, a := range Actions {
		node := TreeFromAction(a)
		nodeLibrary.RegisterAndReturnNewNode(&node)
	}
	return nodeLibrary
}

// RegisterAndReturnNewNode checks if a Node already exists in the library,
// adds it if not, and returns a pointer to the nodeLibrary Node
//
// Recursively walks through Node tree and registers all children. If a node
// ID already exists in the node library, returns a pointer to this node and
// does not recreate. Otherwise, adds the new node to the library and returns
// a pointer to that one.
func (nl *NodeLibrary) RegisterAndReturnNewNode(node *Node) *Node {
	// FUTURE: This operation will need to be locked if we do multiple routines
	if matchingNode, doesExist := nl.Map[node.ID]; doesExist {
		return matchingNode
	}
	if !node.IsAction() {
		node.YesNode = nl.RegisterAndReturnNewNode(node.YesNode)
		node.NoNode = nl.RegisterAndReturnNewNode(node.NoNode)
		node.Complexity = node.YesNode.Complexity + node.NoNode.Complexity
	} else {
		node.Complexity = 1
	}
	r := uint8(55 + rand.Intn(200))
	g := uint8(55 + rand.Intn(200))
	b := uint8(55 + rand.Intn(200))
	node.Color = color.RGBA{r, g, b, 255}
	nl.Map[node.ID] = node
	return node
}

// GetRandomNode returns a random Node from the NodeLibrary
func (nl *NodeLibrary) GetRandomNode() *Node {
	// This is not technically be the best way to get a random element from
	// the map, but it doesn't really need to be perfectly random.
	for _, node := range nl.Map {
		return node
	}
	return nil
}

// GetBetterNodeForMetric returns the node with the best average increase for a
// given metrics
//
func (nl *NodeLibrary) GetBetterNodeForMetric(metric Metric, metricAvg float32, uses int) *Node {
	bestNode := &Node{}
	bestAvg := float32(-999999.0)
	isEnoughUses := false
	for _, node := range nl.Map {
		// only accept a better average if it has been used at least as many
		// times as the sqrt of the current algorithm's uses
		isEnoughUses = node.Uses > MinUsesToConsiderChanging
		if node.MetricsAvgs[metric] > bestAvg && isEnoughUses {
			bestAvg = node.MetricsAvgs[metric]
			bestNode = node
		}
	}
	if bestNode.NodeType != nil {
		return bestNode
	}
	return nil
}

// PruneUnusedNodes removes any unused nodes from the node library to improve
// performance.
func (nl *NodeLibrary) PruneUnusedNodes() {
	if len(nl.Map) > MaxNodesAllowed {
		nodesToRemove := len(nl.Map) - MaxNodesAllowed
		nodesRemoved := 0
		for key, node := range nl.Map {
			if node.NumOrganismsUsing <= 0 {
				delete(nl.Map, key)
				nodesRemoved++
				if nodesRemoved >= nodesToRemove {
					return
				}
			}
		}
	}
}
