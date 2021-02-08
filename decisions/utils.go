package decisions

import (
	"fmt"
	"math/rand"
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

// isCondition returns true if the object passed in is a Condition
func isCondition(v interface{}) bool {
	switch v.(type) {
	case Condition:
		return true
	}
	return false
}

// CopyTreeByValue recursively copies an existing tree by value
func CopyTreeByValue(source *Node) *Node {
	if source == nil {
		return nil
	}
	destination := Node{
		ID:                    source.ID,
		NodeType:              source.NodeType,
		AvgHealthWhenTopLevel: source.AvgHealthWhenTopLevel,
		TopLevelUses:          0,
		AvgHealth:             source.AvgHealth,
		Uses:                  0,
		UsedLastCycle:         false,
	}
	destination.YesNode = CopyTreeByValue(source.YesNode)
	destination.NoNode = CopyTreeByValue(source.NoNode)
	return &destination
}

// MutateTree copies a root Node, makes changes to the full tree, and returns
func MutateTree(original *Node) *Node {
	mutated := CopyTreeByValue(original)
	MutateNode(mutated)
	mutated.UpdateNodeIDs()
	return mutated
}

func (node *Node) getAllSubNodes(includeActions, includeConditions bool) []*Node {
	nodes := make([]*Node, 0, node.Complexity)

	if node.IsAction() {
		if includeActions {
			nodes = append(nodes, node)
		}
	}

	if node.IsCondition() {
		if includeConditions {
			nodes = append(nodes, node)
		}
		nodes = append(nodes, node.YesNode.getAllSubNodes(includeActions, includeConditions)...)
		nodes = append(nodes, node.NoNode.getAllSubNodes(includeActions, includeConditions)...)
	}

	return nodes
}

// MutateNode randomly mutates a single node of a tree
func MutateNode(node *Node) {
	// pick a random node anywhere in the decision tree
	allSubNodes := node.getAllSubNodes(true, true)
	toMutate := allSubNodes[rand.Intn(len(allSubNodes))]

	if toMutate.IsAction() {
		if rand.Intn(2) == 0 {
			// convert action to condition
			originalAction := toMutate.NodeType.(Action)
			toMutate.NodeType = GetRandomCondition()
			if rand.Float64() < 0.5 {
				toMutate.YesNode = TreeFromAction(GetRandomAction())
				toMutate.NoNode = TreeFromAction(originalAction)
			} else {
				toMutate.YesNode = TreeFromAction(originalAction)
				toMutate.NoNode = TreeFromAction(GetRandomAction())
			}
		} else {
			// change action type
			toMutate.NodeType = GetRandomAction()
		}
	} else {
		if rand.Intn(2) == 0 {
			// convert condition to action (simplify)
			toMutate.NodeType = GetRandomAction()
			toMutate.YesNode = nil
			toMutate.NoNode = nil
		} else {
			// change condition type
			toMutate.NodeType = GetRandomCondition()
		}
	}
	toMutate.TopLevelUses = 0
	toMutate.Uses = 0
	toMutate.UsedLastCycle = false
	node.Uses = 0
	node.UsedLastCycle = false
}

// Print pretty prints the node
func (node *Node) Print(indent string, first, last bool) string {
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
	toPrint = fmt.Sprintf("%s%s (%d uses)\n", toPrint, Map[node.NodeType], node.Uses)
	if node.IsCondition() {
		toPrint = fmt.Sprintf("%s%s", toPrint, node.YesNode.Print(newIndent, false, false))
		toPrint = fmt.Sprintf("%s%s", toPrint, node.NoNode.Print(newIndent, false, true))
	}

	if first {
		toPrint = fmt.Sprintf(
			"%s\nUses: %d\nAvgHealth: %.2f\nTopLevelUses:%d\nAvgHealthWhenTopLevel: %.2f\n",
			toPrint,
			node.Uses,
			node.AvgHealth,
			node.TopLevelUses,
			node.AvgHealthWhenTopLevel,
		)
	}

	return toPrint
}
