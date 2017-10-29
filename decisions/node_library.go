package decisions

// NodeLibrary contains a map of Node pointers to aggregated data for each node
type NodeLibrary struct {
	Map map[string]*Node
}

// RegisterAndReturnNewNode checks if a Node already exists in the library and
// adds it if not.
//
// Recursively walks through Node tree and registers all children. If a node
// already exists in the node library, returns a pointer to this node.
// Otherwise, adds the new node to the library and returns pointer to that one.
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

// NewNodeLibrary creates a new *NodeLibrary, initialized with single Actions
func NewNodeLibrary() NodeLibrary {
	nodeLibrary := NodeLibrary{}
	for _, a := range Actions {
		node := TreeFromAction(a)
		nodeLibrary.RegisterAndReturnNewNode(&node)
	}
	return nodeLibrary
}
