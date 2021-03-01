package decision

import (
	"math"

	"github.com/Zebbeni/protozoa/config"
)

// Library is a Map containing all top-level decision trees for a given Organism
type Library map[string]*Tree

// NewLibrary creates a new Tree Library, initialized with a single random Action
func NewLibrary() Library {
	library := make(Library)
	tree := TreeFromAction(GetRandomAction())
	tree = library.RegisterAndReturnTree(tree)
	return library
}

// RegisterAndReturnTree checks if a Tree already exists in the library,
// adds it if not, and returns a pointer to the identical nodeLibrary Tree
func (l Library) RegisterAndReturnTree(node *Tree) *Tree {
	if matchingNode, doesExist := l[node.ID]; doesExist {
		return matchingNode
	}
	l[node.ID] = node
	return node
}

// GetRandomNode returns a random Tree from the TreeLibrary
func (l Library) GetRandomNode() *Tree {
	// This is not technically the best way to get a random element from
	// the map, but it doesn't really need to be perfectly random.
	for _, node := range l {
		return node
	}
	return nil
}

// GetBestDecisionTree returns the decision tree with the highest AvgHealthChange
func (l Library) GetBestDecisionTree() *Tree {
	var best *Tree
	bestHealth := -1 * math.MaxFloat64
	for _, node := range l {
		if node.AvgHealthChange > bestHealth {
			bestHealth = node.AvgHealthChange
			best = node
		}
	}
	return best
}

// GetTopLevelNodes returns a list of all decision tree nodes with top-level
// uses
func (l Library) GetTopLevelNodes() map[string]*Tree {
	topLevelNodes := make(map[string]*Tree)
	for key, node := range l {
		if node.Uses >= 0 {
			topLevelNodes[key] = node
		}
	}
	return topLevelNodes
}

// Prune deletes the worst-performing top-level decision tree if
// Map contains more than the max number of top-level nodes allowed
func (l Library) Prune() {
	topLevelNodes := l.GetTopLevelNodes()
	if len(topLevelNodes) <= config.MaxDecisionTrees() {
		return
	}

	worstTopLevelAvgHealth := math.MaxFloat64
	var worstTopLevelNodeID string

	for key, node := range topLevelNodes {
		// avoid pruning currently-used decision tree
		if node.InDecisionTree || node.UsedLastCycle {
			continue
		}
		if node.AvgHealthChange < worstTopLevelAvgHealth {
			worstTopLevelAvgHealth = node.AvgHealthChange
			worstTopLevelNodeID = key
		}
	}

	// If we didn't delete the number desired, delete the worst TopLevelNode
	delete(l, worstTopLevelNodeID)
}
