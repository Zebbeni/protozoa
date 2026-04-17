package manager

import (
	"image/color"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/checkpoint"
	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/environment"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// RestoreEnvironmentManager creates an EnvironmentManager with pre-populated pH maps.
func RestoreEnvironmentManager(api environment.API, currentPh, previousPh [][]float64) *EnvironmentManager {
	return &EnvironmentManager{
		api:           api,
		currentPhMap:  currentPh,
		previousPhMap: previousPh,
	}
}

// RestoreFoodManager creates a FoodManager with pre-populated food items.
func RestoreFoodManager(api food.API, rng *simrand.RNG, items []checkpoint.FoodRecord) *FoodManager {
	foodItems := make(map[string]*food.Item)
	for _, rec := range items {
		p := utils.Point{X: rec.X, Y: rec.Y}
		foodItems[p.ToString()] = food.NewItem(p, rec.Value)
	}
	return &FoodManager{
		api:           api,
		rng:           rng,
		Items:         foodItems,
		isInitialized: true,
	}
}

// RestoreHistory injects pre-built pH distribution and effect history into the OrganismManager.
func (m *OrganismManager) RestoreHistory(payload *checkpoint.HistoryPayload) {
	m.historyMutex.Lock()
	defer m.historyMutex.Unlock()
	m.history[HistoryPhDistribution] = payload.PhDistribution
	m.history[HistoryPhEffect] = payload.PhEffect
}

// RestoreDescendantTrees rebuilds the descendant trees from a serialized payload
// and injects them into the OrganismManager. Also rebuilds the ancestor ID list
// and color map from the tree roots so all ancestors are available for graphing.
func (m *OrganismManager) RestoreDescendantTrees(payload *checkpoint.DescendantTreesPayload) {
	trees := make(map[int]*organism.DescendantNode)
	ancestorIDs := make([]int, 0, len(payload.Trees))
	ancestorColors := make(map[int]color.Color)
	for _, treeRec := range payload.Trees {
		root := recordToNode(treeRec.Root, nil)
		trees[treeRec.AncestorID] = root
		ancestorIDs = append(ancestorIDs, treeRec.AncestorID)
		ancestorColors[treeRec.AncestorID] = root.Color
	}

	m.descendantTrees = trees
	m.originalAncestors = ancestorIDs
	m.originalAncestorColors = ancestorColors

	// Link living organisms to their tree nodes
	nodeIndex := make(map[int]*organism.DescendantNode)
	for _, root := range trees {
		indexTreeNodes(root, nodeIndex)
	}
	for _, o := range m.organisms {
		if node, ok := nodeIndex[o.ID]; ok {
			o.TreeNode = node
		}
	}
}

// indexTreeNodes recursively collects all tree nodes into a map keyed by ID.
func indexTreeNodes(node *organism.DescendantNode, index map[int]*organism.DescendantNode) {
	index[node.ID] = node
	node.ForEachChild(func(child *organism.DescendantNode) {
		indexTreeNodes(child, index)
	})
}

func recordToNode(rec checkpoint.DescendantNodeRecord, parent *organism.DescendantNode) *organism.DescendantNode {
	node := &organism.DescendantNode{
		ID:                   rec.ID,
		Color:                colorful.Color{R: rec.ColorR, G: rec.ColorG, B: rec.ColorB},
		PhEffectColor:        colorful.Color{R: rec.PhEffectColorR, G: rec.PhEffectColorG, B: rec.PhEffectColorB},
		StartCycle:           rec.StartCycle,
		EndCycle:             rec.EndCycle,
		AllBranchesDeadCycle: rec.AllBranchesDeadCycle,
		Parent:               parent,
	}
	for _, childRec := range rec.Children {
		child := recordToNode(childRec, node)
		node.Children = append(node.Children, child)
	}
	return node
}

// RestoreOrganismManager creates an OrganismManager with pre-populated state.
func RestoreOrganismManager(
	api organism.API, rng *simrand.RNG,
	organisms map[int]*organism.Organism,
	grid [][]int, totalCreated int,
	ancestorIDs []int, ancestorColors map[int]color.Color,
) *OrganismManager {
	return &OrganismManager{
		api:                    api,
		rng:                    rng,
		requestManager:         RequestManager{},
		organisms:              organisms,
		organismIDGrid:         grid,
		totalOrganismsCreated:  totalCreated,
		organismIds:            make([]int, 0, c.MaxOrganisms()),
		originalAncestors:      ancestorIDs,
		originalAncestorColors: ancestorColors,
		descendantTrees:        make(map[int]*organism.DescendantNode),
		history: map[HistoryType]map[int]map[int]int32{
			HistoryPopulation:     make(map[int]map[int]int32),
			HistoryPhEffect:       make(map[int]map[int]int32),
			HistoryPhDistribution: make(map[int]map[int]int32),
		},
	}
}
