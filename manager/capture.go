package manager

import (
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/checkpoint"
	"github.com/Zebbeni/protozoa/organism"
)

// CaptureOrganismRecords returns all living organisms as checkpoint records.
func (m *OrganismManager) CaptureOrganismRecords() []checkpoint.OrganismRecord {
	records := make([]checkpoint.OrganismRecord, 0, len(m.organisms))
	for _, o := range m.organisms {
		records = append(records, organismToRecord(o))
	}
	return records
}

// CaptureOrganismGrid returns a copy of the organism ID grid.
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

// CaptureAncestors returns ancestor records for checkpointing.
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

// TotalOrganismsCreated returns the total count of organisms ever created.
func (m *OrganismManager) TotalOrganismsCreated() int {
	return m.totalOrganismsCreated
}

// CaptureFoodRecords returns all food items as checkpoint records.
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

// CopyCurrentPhMap returns a float64 deep copy of the current pH map.
// Used by the per-cycle reverse-delta path to capture the pre-Update
// pH state for later diffing — we keep the in-memory representation
// at float64, only narrowing to float32 at the on-disk boundary.
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

// CurrentPhMap returns a direct reference to the current pH map (no
// copy). Caller must not mutate. Used by the reverse-delta diff to
// compare against a previously copied pre-cycle state.
func (m *EnvironmentManager) CurrentPhMap() [][]float64 {
	return m.currentPhMap
}

// CapturePhMaps returns float64 copies of the current and previous pH
// maps. Stored at full simulation precision so save/restore is exactly
// lossless — replay from a snapshot reaches the same state at the
// same cycle as the recording. An older version narrowed to float32
// for ~halved snapshot size but the precision loss compounded across
// pH-diffusion cycles into observable replay drift.
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

// CaptureHistory returns a deep copy of the pH distribution and effect history maps.
func (m *OrganismManager) CaptureHistory() *checkpoint.HistoryPayload {
	m.historyMutex.RLock()
	defer m.historyMutex.RUnlock()

	return &checkpoint.HistoryPayload{
		PhDistribution: copyHistoryMap(m.history[HistoryPhDistribution]),
		PhEffect:       copyHistoryMap(m.history[HistoryPhEffect]),
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

// CaptureDescendantTrees serializes all descendant trees for the final checkpoint section.
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
	phCol, _ := n.PhEffectColor.(colorful.Color)

	// Clamp StartCycle to 0 so the unsigned wire format doesn't round-trip
	// negative values into huge positive ones. The very first ancestor
	// node is created at sim.cycle == -1 (during NewSimulation, before
	// the first Update); without this clamp, uint32(-1) decodes to
	// 4294967295 and the descendant-tree walk in countAlive bails out
	// at the root, making the population graph render as empty.
	startCycle := n.StartCycle
	if startCycle < 0 {
		startCycle = 0
	}

	rec := checkpoint.DescendantNodeRecord{
		ID:                   uint32(n.ID),
		ColorR:               float32(col.R),
		ColorG:               float32(col.G),
		ColorB:               float32(col.B),
		PhEffectColorR:       float32(phCol.R),
		PhEffectColorG:       float32(phCol.G),
		PhEffectColorB:       float32(phCol.B),
		PhGrowthEffect:       n.PhGrowthEffect,
		StartCycle:           uint32(startCycle),
		EndCycle:             uint32(n.EndCycle),
		AllBranchesDeadCycle: uint32(n.AllBranchesDeadCycle),
	}

	n.ForEachChild(func(child *organism.DescendantNode) {
		rec.Children = append(rec.Children, nodeToRecord(child))
	})

	return rec
}

func organismToRecord(o *organism.Organism) checkpoint.OrganismRecord {
	traits := o.Traits()
	return checkpoint.OrganismRecord{
		ID:                         uint32(o.ID),
		Age:                        uint32(o.Age),
		Health:                     o.Health,
		Size:                       o.Size,
		Children:                   uint16(o.Children),
		TraveledDist:               uint32(o.TraveledDist),
		CyclesSinceLastSpawn:       uint16(o.CyclesSinceLastSpawn),
		LocationX:                  uint16(o.Location.X),
		LocationY:                  uint16(o.Location.Y),
		DirectionX:                 int8(o.Direction.X),
		DirectionY:                 int8(o.Direction.Y),
		OriginalAncestorID:         uint32(o.OriginalAncestorID),
		ColorR:                 float32(traits.OrganismColor.R),
		ColorG:                 float32(traits.OrganismColor.G),
		ColorB:                 float32(traits.OrganismColor.B),
		MaxSize:                traits.MaxSize,
		SpawnHealth:            traits.SpawnHealth,
		MinHealthToSpawn:       traits.MinHealthToSpawn,
		MinCyclesBetweenSpawns: uint16(traits.MinCyclesBetweenSpawns),
		IdealPh:                traits.IdealPh,
		PhTolerance:            traits.PhTolerance,
		PhGrowthEffect:         traits.PhGrowthEffect,
		MaxLifespan:            uint16(traits.MaxLifespan),
		DecisionTree:               o.GetDecisionTreeCopy().Serialize(),
		CurrentAction:              uint8(o.Action()),
		AttackTotal:                uint32(o.AttackTotal),
		AttackHits:                 uint32(o.AttackHits),
	}
}
