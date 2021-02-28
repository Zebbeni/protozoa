package decision

// Size returns the total size of the tree, including this tree.
func (n *Node) Size() int {
	if n.IsAction() {
		return 1
	}

	return 1 + n.YesNode.Size() + n.NoNode.Size()
}
