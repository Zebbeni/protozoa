package manager

import (
	"fmt"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/checkpoint"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

func (m *OrganismManager) CaptureOrganismRecords() []checkpoint.OrganismRecord {
	records := make([]checkpoint.OrganismRecord, 0, len(m.organisms))
	for _, o := range m.organisms {
		records = append(records, organismToRecord(o))
	}
	return records
}

func (m *OrganismManager) CaptureOrganismGrid() [][]int {
	w := len(m.organismIDGrid)
	if w == 0 {
		return nil
	}
	h := len(m.organismIDGrid[0])
	grid := make([][]int, w)
	for x := 0; x < w; x++ {
		grid[x] = make([]int, h)
		copy(grid[x], m.organismIDGrid[x])
	}
	return grid
}

func (m *OrganismManager) CaptureAncestors() []checkpoint.AncestorRecord {
	records := make([]checkpoint.AncestorRecord, 0, len(m.originalAncestors))
	for _, id := range m.originalAncestors {
		col, _ := m.originalAncestorColors[id].(colorful.Color)
		records = append(records, checkpoint.AncestorRecord{
			ID:     uint32(id),
			ColorR: float32(col.R),
			ColorG: float32(col.G),
			ColorB: float32(col.B),
		})
	}
	return records
}

func (m *OrganismManager) TotalOrganismsCreated() int {
	return m.totalOrganismsCreated
}

func (m *FoodManager) CaptureFoodRecords() []checkpoint.FoodRecord {
	records := make([]checkpoint.FoodRecord, 0, len(m.Items))
	for _, item := range m.Items {
		records = append(records, checkpoint.FoodRecord{
			X:     uint16(item.Point.X),
			Y:     uint16(item.Point.Y),
			Value: uint16(item.Value),
		})
	}
	return records
}

func (m *FoodManager) CaptureBuriedFoodRecords() []checkpoint.FoodRecord {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	records := make([]checkpoint.FoodRecord, 0, len(m.Buried))
	for point, value := range m.Buried {
		records = append(records, checkpoint.FoodRecord{
			X:     uint16(point.X),
			Y:     uint16(point.Y),
			Value: uint16(value),
		})
	}
	return records
}

func (m *EnvironmentManager) CopyCurrentPhMap() [][]float64 {
	w := len(m.currentPhMap)
	if w == 0 {
		return nil
	}
	h := len(m.currentPhMap[0])
	out := make([][]float64, w)
	for x := 0; x < w; x++ {
		out[x] = make([]float64, h)
		copy(out[x], m.currentPhMap[x])
	}
	return out
}

func (m *EnvironmentManager) CurrentPhMap() [][]float64 {
	return m.currentPhMap
}

func (m *EnvironmentManager) CapturePhMaps() (current, previous [][]float64) {
	w := len(m.currentPhMap)
	if w == 0 {
		return nil, nil
	}
	h := len(m.currentPhMap[0])

	current = make([][]float64, w)
	previous = make([][]float64, w)
	for x := 0; x < w; x++ {
		current[x] = make([]float64, h)
		previous[x] = make([]float64, h)
		copy(current[x], m.currentPhMap[x])
		copy(previous[x], m.previousPhMap[x])
	}
	return
}

// CaptureHistory returns a deep copy of the history maps the graphs read (pH distribution, food count, wall count).
func (m *OrganismManager) CaptureHistory() *checkpoint.HistoryPayload {
	m.historyMutex.RLock()
	defer m.historyMutex.RUnlock()

	return &checkpoint.HistoryPayload{
		PhDistribution: copyHistoryMap(m.history[HistoryPhDistribution]),
		Food:           copyHistoryMap(m.history[HistoryFood]),
		BuriedFood:     copyHistoryMap(m.history[HistoryBuriedFood]),
		Walls:          copyHistoryMap(m.history[HistoryWalls]),
	}
}

func copyHistoryMap(src map[int]map[int]int32) map[int]map[int]int32 {
	dst := make(map[int]map[int]int32, len(src))
	for cycle, buckets := range src {
		dstBuckets := make(map[int]int32, len(buckets))
		for k, v := range buckets {
			dstBuckets[k] = v
		}
		dst[cycle] = dstBuckets
	}
	return dst
}

func (m *OrganismManager) CaptureDescendantTrees() *checkpoint.DescendantTreesPayload {
	trees := make([]checkpoint.DescendantTreeRecord, 0, len(m.originalAncestors))
	for _, id := range m.originalAncestors {
		root, ok := m.descendantTrees[id]
		if !ok || root == nil {
			continue
		}
		trees = append(trees, checkpoint.DescendantTreeRecord{
			AncestorID: uint32(id),
			Root:       nodeToRecord(root),
		})
	}
	return &checkpoint.DescendantTreesPayload{Trees: trees}
}

func nodeToRecord(n *organism.DescendantNode) checkpoint.DescendantNodeRecord {
	col, _ := n.Color.(colorful.Color)

	// Clamp StartCycle to 0 so the unsigned wire format doesn't round-trip negative values into huge positive ones.
	startCycle := n.StartCycle
	if startCycle < 0 {
		startCycle = 0
	}

	rec := checkpoint.DescendantNodeRecord{
		ID:                   uint32(n.ID),
		ColorR:               float32(col.R),
		ColorG:               float32(col.G),
		ColorB:               float32(col.B),
		StartCycle:           uint32(startCycle),
		EndCycle:             uint32(n.EndCycle),
		AllBranchesDeadCycle: uint32(n.AllBranchesDeadCycle),
		Abilities:            captureAbilities(n.Abilities),
	}

	n.ForEachChild(func(child *organism.DescendantNode) {
		rec.Children = append(rec.Children, nodeToRecord(child))
	})

	return rec
}

func organismToRecord(o *organism.Organism) checkpoint.OrganismRecord {
	traits := o.Traits()
	return checkpoint.OrganismRecord{
		ID:                     uint32(o.ID),
		Age:                    uint32(o.Age),
		Health:                 o.Health,
		Size:                   o.Size,
		Children:               uint16(o.Children),
		TraveledDist:           uint32(o.TraveledDist),
		CyclesSinceLastSpawn:   uint16(o.CyclesSinceLastSpawn),
		LocationX:              uint16(o.Location.X),
		LocationY:              uint16(o.Location.Y),
		DirectionX:             int8(o.Direction.X),
		DirectionY:             int8(o.Direction.Y),
		OriginalAncestorID:     uint32(o.OriginalAncestorID),
		ColorR:                 float32(traits.OrganismColor.R),
		ColorG:                 float32(traits.OrganismColor.G),
		ColorB:                 float32(traits.OrganismColor.B),
		MaxSize:                traits.MaxSize,
		SpawnHealth:            traits.SpawnHealth,
		MinHealthToSpawn:       traits.MinHealthToSpawn,
		MinCyclesBetweenSpawns: uint16(traits.MinCyclesBetweenSpawns),
		IdealPh:                traits.IdealPh,
		DecisionTree:           o.GetDecisionTreeCopy().Serialize(),
		CurrentAction:          uint8(o.Action()),
		Status:                 uint8(o.Status),
		AttackTotal:            uint32(o.AttackTotal),
		AttackHits:             uint32(o.AttackHits),
		KilledBy:               killedByRecord(o.KilledBy),
		PhPositive:             o.PhPositive,
		PhNegative:             o.PhNegative,
		Abilities:              captureAbilities(traits.Abilities),
	}
}

// The snapshot format pins the ability array to a literal length so the checkpoint package stays dependency-free.
func init() {
	var rec checkpoint.AbilityScores
	if len(rec) != len(physiology.AllAbilities) {
		panic(fmt.Sprintf(
			"manager: checkpoint.AbilityScores holds %d entries but physiology has %d abilities — "+
				"widen AbilityScores in checkpoint/types.go",
			len(rec), len(physiology.AllAbilities)))
	}
}

// AbilitiesFromRecord widens a snapshot's byte array back to live scores, reporting whether the stored distribution was valid.
func AbilitiesFromRecord(rec checkpoint.AbilityScores) (physiology.Scores, bool) {
	var out physiology.Scores
	for _, a := range physiology.AllAbilities {
		out[a] = int(rec[a])
	}
	if out.Validate() != nil {
		return physiology.BalancedScores(), false
	}
	return out, true
}

// captureAbilities narrows the live scores to the snapshot's byte array.
func captureAbilities(s physiology.Scores) checkpoint.AbilityScores {
	var out checkpoint.AbilityScores
	for _, a := range physiology.AllAbilities {
		out[a] = uint8(s[a])
	}
	return out
}

// killedByRecord stores a killer's ID plus one, so 0 means nobody and a
// snapshot written before the field existed decodes to that.
func killedByRecord(id int) uint32 {
	if id < 0 {
		return 0
	}
	return uint32(id + 1)
}
