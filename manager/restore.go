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

// RestoreEnvironmentManager creates an EnvironmentManager with pre-populated
// pH maps. Snapshot pH maps are float64 to preserve simulation precision
// exactly, so we deep-copy here without any narrow/widen conversion.
func RestoreEnvironmentManager(api environment.API, currentPh, previousPh [][]float64) *EnvironmentManager {
	dup := func(src [][]float64) [][]float64 {
		if len(src) == 0 {
			return nil
		}
		dst := make([][]float64, len(src))
		for x, col := range src {
			dst[x] = make([]float64, len(col))
			copy(dst[x], col)
		}
		return dst
	}
	return &EnvironmentManager{
		api:           api,
		currentPhMap:  dup(currentPh),
		previousPhMap: dup(previousPh),
	}
}

// RestoreFoodManager creates a FoodManager with pre-populated food items.
func RestoreFoodManager(api food.API, rng *simrand.RNG, items []checkpoint.FoodRecord) *FoodManager {
	foodItems := make(map[utils.Point]*food.Item)
	for _, rec := range items {
		p := utils.Point{X: int(rec.X), Y: int(rec.Y)}
		foodItems[p] = food.NewItem(p, int(rec.Value))
	}
	return &FoodManager{
		api:           api,
		rng:           rng,
		Items:         foodItems,
		isInitialized: true,
	}
}

// RestoreHistory injects pre-built pH distribution history into the OrganismManager.
func (m *OrganismManager) RestoreHistory(payload *checkpoint.HistoryPayload) {
	m.historyMutex.Lock()
	defer m.historyMutex.Unlock()
	m.history[HistoryPhDistribution] = payload.PhDistribution
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
		id := int(treeRec.AncestorID)
		trees[id] = root
		ancestorIDs = append(ancestorIDs, id)
		ancestorColors[id] = root.Color
	}

	m.descendantTrees = trees
	m.originalAncestors = ancestorIDs
	m.originalAncestorColors = ancestorColors

	// Persist the by-ID index so GetTreeNodeByID is O(1) for both alive
	// and dead organisms — used per render frame by the descendant
	// highlight code.
	m.descendantNodeIndex = make(map[int]*organism.DescendantNode)
	for _, root := range trees {
		indexTreeNodes(root, m.descendantNodeIndex)
	}
	for _, o := range m.organisms {
		if node, ok := m.descendantNodeIndex[o.ID]; ok {
			o.TreeNode = node
		}
	}

	// Precompute the "most successful" set so the Most Successful
	// select mode can do an O(1) per-organism lookup each frame.
	m.mostSuccessful = computeMostSuccessfulSet(trees)
}

// computeMostSuccessfulSet builds the set of "most successful" tree
// node IDs: organisms still alive at the end of the recorded run plus
// every ancestor of a survivor. If the recording ends with no
// survivors, falls back to the chain ending in the latest-dying leaf
// plus its ancestors. The returned set is keyed by organism ID for
// O(1) membership checks.
//
// Reads only EndCycle (set authoritatively by MarkDead, never modified
// post-load), avoiding any dependence on AllBranchesDeadCycle which
// can shift during forward-play in replay mode.
func computeMostSuccessfulSet(trees map[int]*organism.DescendantNode) map[int]struct{} {
	set := make(map[int]struct{})

	// Walk post-order. Returns true if any node in this subtree has
	// EndCycle == 0 (alive at recording end). Every node on a path to
	// such a survivor is added to the set.
	var walkSurvivor func(*organism.DescendantNode) bool
	walkSurvivor = func(n *organism.DescendantNode) bool {
		hasSurvivor := n.EndCycle == 0
		n.ForEachChild(func(c *organism.DescendantNode) {
			if walkSurvivor(c) {
				hasSurvivor = true
			}
		})
		if hasSurvivor {
			set[n.ID] = struct{}{}
		}
		return hasSurvivor
	}
	for _, root := range trees {
		if root != nil {
			walkSurvivor(root)
		}
	}
	if len(set) > 0 {
		return set
	}

	// No survivors. Find the latest death cycle and collect every node
	// on a path to a leaf with that EndCycle.
	maxCycle := 0
	var walkMax func(*organism.DescendantNode)
	walkMax = func(n *organism.DescendantNode) {
		if n.EndCycle > maxCycle {
			maxCycle = n.EndCycle
		}
		n.ForEachChild(walkMax)
	}
	for _, root := range trees {
		if root != nil {
			walkMax(root)
		}
	}
	var walkLatest func(*organism.DescendantNode) bool
	walkLatest = func(n *organism.DescendantNode) bool {
		onLatest := n.EndCycle == maxCycle
		n.ForEachChild(func(c *organism.DescendantNode) {
			if walkLatest(c) {
				onLatest = true
			}
		})
		if onLatest {
			set[n.ID] = struct{}{}
		}
		return onLatest
	}
	for _, root := range trees {
		if root != nil {
			walkLatest(root)
		}
	}
	return set
}

// indexTreeNodes recursively collects all tree nodes into a map keyed by ID.
func indexTreeNodes(node *organism.DescendantNode, index map[int]*organism.DescendantNode) {
	index[node.ID] = node
	node.ForEachChild(func(child *organism.DescendantNode) {
		indexTreeNodes(child, index)
	})
}

func recordToNode(rec checkpoint.DescendantNodeRecord, parent *organism.DescendantNode) *organism.DescendantNode {
	// Recover from older files that round-tripped a negative
	// StartCycle (e.g. -1 for the very first ancestor) through uint32.
	// 4294967295 et al. show up as implausibly-huge ints; treat any
	// value beyond the practical sim cycle range as 0 so countAlive
	// doesn't short-circuit at the root and skip the entire tree.
	startCycle := int(rec.StartCycle)
	if startCycle > 1<<30 {
		startCycle = 0
	}
	node := &organism.DescendantNode{
		ID:                   int(rec.ID),
		Color:                colorful.Color{R: float64(rec.ColorR), G: float64(rec.ColorG), B: float64(rec.ColorB)},
		StartCycle:           startCycle,
		EndCycle:             int(rec.EndCycle),
		AllBranchesDeadCycle: int(rec.AllBranchesDeadCycle),
		Parent:               parent,
	}
	for _, childRec := range rec.Children {
		child := recordToNode(childRec, node)
		node.Children = append(node.Children, child)
	}
	return node
}

// RestoreOrganismManager creates an OrganismManager with pre-populated state.
// The grid is deep-copied so subsequent simulation Updates don't mutate
// the caller's array — when restoring from an in-memory ring snapshot,
// the snap's OrganismGrid is the buffer's stored copy, and aliasing
// would let forward-play write back into the snapshot, corrupting it
// for future step-back lookups.
func RestoreOrganismManager(
	api organism.API, rng *simrand.RNG,
	organisms map[int]*organism.Organism,
	grid [][]int, totalCreated int,
	ancestorIDs []int, ancestorColors map[int]color.Color,
) *OrganismManager {
	gridCopy := make([][]int, len(grid))
	for x, col := range grid {
		gridCopy[x] = make([]int, len(col))
		copy(gridCopy[x], col)
	}
	return &OrganismManager{
		api:                    api,
		rng:                    rng,
		requestManager:         RequestManager{},
		organisms:              organisms,
		organismIDGrid:         gridCopy,
		totalOrganismsCreated:  totalCreated,
		organismIds:            make([]int, 0, c.MaxOrganisms()),
		originalAncestors:      ancestorIDs,
		originalAncestorColors: ancestorColors,
		descendantTrees:        make(map[int]*organism.DescendantNode),
		descendantNodeIndex:    make(map[int]*organism.DescendantNode),
		history: map[HistoryType]map[int]map[int]int32{
			HistoryPopulation:     make(map[int]map[int]int32),
			HistoryPhDistribution: make(map[int]map[int]int32),
		},
	}
}
