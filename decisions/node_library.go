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
func NewNodeLibrary() *NodeLibrary {
	nodeLibrary := NodeLibrary{
		Map: make(map[string]*Node),
	}
	for _, a := range Actions {
		node := TreeFromAction(a)
		nodeLibrary.RegisterAndReturnNewNode(&node)
	}
	return &nodeLibrary
}

// Clone returns a new NodeLibrary with all decision trees from original
// with metrics set to 0
func (nl *NodeLibrary) Clone() *NodeLibrary {
	newLibrary := &NodeLibrary{
		Map: make(map[string]*Node),
	}
	for _, node := range nl.Map {
		newNode := CopyTreeByValue(node)
		newLibrary.RegisterAndReturnNewNode(newNode)
	}
	return newLibrary
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
	// This is not technically the best way to get a random element from
	// the map, but it doesn't really need to be perfectly random.
	for _, node := range nl.Map {
		return node
	}
	return nil
}

// GetBestNodesForMetrics returns the node with the best average increase for a
// given metrics
//
func (nl *NodeLibrary) GetBestNodesForMetrics() map[Metric]*Node {
	bestNodes := make(map[Metric]*Node)
	bestAvgs := make(map[Metric]float64)
	for _, metric := range Metrics {
		bestNodes[metric] = nil
		bestAvgs[metric] = -999999.9
	}
	isEnoughUses := false
	for _, node := range nl.Map {
		for _, metric := range Metrics {
			// only accept a better average if it has been used
			isEnoughUses = node.Uses >= float64(10*node.Complexity)
			if node.MetricsAvgs[metric] > bestAvgs[metric] && isEnoughUses {
				bestAvgs[metric] = node.MetricsAvgs[metric]
				bestNodes[metric] = node
			}
		}
	}
	return bestNodes
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
