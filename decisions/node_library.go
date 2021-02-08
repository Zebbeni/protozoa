package decisions

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
		nodeLibrary.RegisterAndReturnNewNode(node)
	}
	return &nodeLibrary
}

// Clone returns a new NodeLibrary with all decision trees from original
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
	if node.IsCondition() {
		node.YesNode = nl.RegisterAndReturnNewNode(node.YesNode)
		node.NoNode = nl.RegisterAndReturnNewNode(node.NoNode)
		node.Complexity = node.YesNode.Complexity + node.NoNode.Complexity
	} else {
		node.Complexity = 1
	}
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

// GetBestNodesForHealth returns decision nodes to maintain good health
// Returns both the most successful top-level node and the most-successful
// node overall
func (nl *NodeLibrary) GetBestNodesForHealth() (best, bestTopLevel *Node) {
	best, bestTopLevel = nil, nil
	bestHealth, bestHealthWhenTopLevel := -999999.9, -999999.9
	isEnoughUses := false
	for _, node := range nl.Map {
		// only accept a better average if it has been used enough times to
		// get a fair evaluation
		isEnoughUses = node.Uses >= node.Complexity*node.Complexity
		if isEnoughUses {
			if node.AvgHealth > bestHealth {
				bestHealth = node.AvgHealth
				best = node
			}
		}
		isEnoughUses = node.TopLevelUses >= node.Complexity*node.Complexity
		if isEnoughUses {
			if node.AvgHealthWhenTopLevel > bestHealthWhenTopLevel {
				bestHealthWhenTopLevel = node.AvgHealthWhenTopLevel
				bestTopLevel = node
			}
		}
	}
	return
}

// PruneUnusedNodes removes any unused nodes from the node library to improve
// performance.
func (nl *NodeLibrary) PruneUnusedNodes() {
	if len(nl.Map) > MaxNodesAllowed {
		nodesToRemove := len(nl.Map) - MaxNodesAllowed
		nodesRemoved := 0
		for key, node := range nl.Map {
			if node.TopLevelUses <= 0 {
				delete(nl.Map, key)
				nodesRemoved++
				if nodesRemoved >= nodesToRemove {
					return
				}
			}
		}
	}
}
