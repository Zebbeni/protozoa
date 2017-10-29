package decisions

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
	}
	nl.Map[node.ID] = node
	return node
}

// GetRandomNode returns a random Node from the NodeLibrary
func (nl *NodeLibrary) GetRandomNode() *Node {
	// This is not technically be the best way to get a random element from
	// the map, but it doesn't really need to be perfectly random.
	for _, v := range nl.Map {
		return v
	}
	return nil
}
