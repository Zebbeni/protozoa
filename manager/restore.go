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

// RestoreWallManager creates a WallManager with a pre-populated wall map, bypassing the initial-walls seeding in NewWallManager.
func RestoreWallManager(rng *simrand.RNG, walls map[utils.Point]int) *WallManager {
	return &WallManager{
		rng:   rng,
		walls: walls,
	}
}

// RestoreFoodManager creates a FoodManager with pre-populated food items and buried store.
func RestoreFoodManager(api food.API, rng *simrand.RNG, items, buried []checkpoint.FoodRecord) *FoodManager {
	foodItems := make(map[utils.Point]*food.Item)
	for _, rec := range items {
		p := utils.Point{X: int(rec.X), Y: int(rec.Y)}
		foodItems[p] = food.NewItem(p, int(rec.Value))
	}
	buriedFood := make(map[utils.Point]int, len(buried))
	for _, rec := range buried {
		buriedFood[utils.Point{X: int(rec.X), Y: int(rec.Y)}] = int(rec.Value)
	}
	return &FoodManager{
		api:           api,
		rng:           rng,
		Items:         foodItems,
		Buried:        buriedFood,
		isInitialized: true,
	}
}

// RestoreHistory injects pre-built graph history (pH distribution, food count, wall count) into the OrganismManager.
func (m *OrganismManager) RestoreHistory(payload *checkpoint.HistoryPayload) {
	m.historyMutex.Lock()
	defer m.historyMutex.Unlock()
	m.history[HistoryPhDistribution] = copyHistoryMap(payload.PhDistribution)
	m.history[HistoryFood] = copyHistoryMap(payload.Food)
	m.history[HistoryBuriedFood] = copyHistoryMap(payload.BuriedFood)
	m.history[HistoryWalls] = copyHistoryMap(payload.Walls)
}

// DescendantTrees is a recording's family history, decoded from its payload and pre-indexed.
type DescendantTrees struct {
	trees          map[int]*organism.DescendantNode
	ancestorIDs    []int
	ancestorColors map[int]color.Color
	nodeIndex      map[int]*organism.DescendantNode
	mostSuccessful map[int]struct{}
	endCycle       int
	// generation distinguishes one decoded set of trees from another.
	generation int
}

// treeGenerations numbers decoded tree sets; see DescendantTrees.
var treeGenerations int

// BuildDescendantTrees decodes a serialized payload into the trees, the ancestor list and colour map, the by-ID node index, the "most successful" set.
func BuildDescendantTrees(payload *checkpoint.DescendantTreesPayload) *DescendantTrees {
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

	nodeIndex := make(map[int]*organism.DescendantNode)
	for _, root := range trees {
		indexTreeNodes(root, nodeIndex)
	}

	treeGenerations++
	return &DescendantTrees{
		trees:          trees,
		ancestorIDs:    ancestorIDs,
		ancestorColors: ancestorColors,
		nodeIndex:      nodeIndex,
		mostSuccessful: computeMostSuccessfulSet(trees),
		endCycle:       organism.ComputeLineageEnds(trees),
		generation:     treeGenerations,
	}
}

func (d *DescendantTrees) Generation() int { return d.generation }

func (m *OrganismManager) RestoreDescendantTrees(payload *checkpoint.DescendantTreesPayload) {
	m.InstallDescendantTrees(BuildDescendantTrees(payload))
}

// InstallDescendantTrees points the manager at already-decoded trees and links its organisms to their nodes.
func (m *OrganismManager) InstallDescendantTrees(d *DescendantTrees) {
	m.descendantTrees = d.trees
	m.originalAncestors = d.ancestorIDs
	m.originalAncestorColors = d.ancestorColors
	m.descendantNodeIndex = d.nodeIndex
	m.mostSuccessful = d.mostSuccessful
	m.recordedEndCycle = d.endCycle
	m.treesGeneration = d.generation

	for _, o := range m.organisms {
		if node, ok := m.descendantNodeIndex[o.ID]; ok {
			o.TreeNode = node
		}
	}
}

func (m *OrganismManager) TreesGeneration() int { return m.treesGeneration }

// RecordedEndCycle returns the end of the recorded run whose descendant trees were restored, or 0 in a live run.
func (m *OrganismManager) RecordedEndCycle() int {
	return m.recordedEndCycle
}

func computeMostSuccessfulSet(trees map[int]*organism.DescendantNode) map[int]struct{} {
	set := make(map[int]struct{})

	// Walk post-order.
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
	// Recover from older files that round-tripped a negative StartCycle (e.g. -1 for the very first ancestor) through uint32.
	startCycle := int(rec.StartCycle)
	if startCycle > 1<<30 {
		startCycle = 0
	}
	// A pre-abilities file yields balanced scores here.
	abilities, _ := AbilitiesFromRecord(rec.Abilities)
	node := &organism.DescendantNode{
		ID:                   int(rec.ID),
		Color:                colorful.Color{R: float64(rec.ColorR), G: float64(rec.ColorG), B: float64(rec.ColorB)},
		Abilities:            abilities,
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
		history:                newHistoryMaps(),
	}
}
