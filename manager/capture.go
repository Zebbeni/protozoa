package manager

import (
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
		col := m.originalAncestorColors[id]
		r, g, b, _ := col.RGBA()
		records = append(records, checkpoint.AncestorRecord{
			ID:     id,
			ColorR: float64(r) / 65535.0,
			ColorG: float64(g) / 65535.0,
			ColorB: float64(b) / 65535.0,
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
			X:     item.Point.X,
			Y:     item.Point.Y,
			Value: item.Value,
		})
	}
	return records
}

// CapturePhMaps returns copies of the current and previous pH maps.
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
		copy(current[x], m.currentPhMap[x])
		previous[x] = make([]float64, h)
		copy(previous[x], m.previousPhMap[x])
	}
	return
}

func organismToRecord(o *organism.Organism) checkpoint.OrganismRecord {
	traits := o.Traits()
	cr, cg, cb, _ := traits.OrganismColor.RGBA()
	return checkpoint.OrganismRecord{
		ID:                     o.ID,
		Age:                    o.Age,
		Health:                 o.Health,
		Size:                   o.Size,
		Children:               o.Children,
		TraveledDist:           o.TraveledDist,
		CyclesSinceLastSpawn:   o.CyclesSinceLastSpawn,
		LocationX:              o.Location.X,
		LocationY:              o.Location.Y,
		DirectionX:             o.Direction.X,
		DirectionY:             o.Direction.Y,
		OriginalAncestorID:     o.OriginalAncestorID,
		ColorR:                 float64(cr) / 65535.0,
		ColorG:                 float64(cg) / 65535.0,
		ColorB:                 float64(cb) / 65535.0,
		MaxSize:                traits.MaxSize,
		SpawnHealth:            traits.SpawnHealth,
		MinHealthToSpawn:       traits.MinHealthToSpawn,
		MinCyclesBetweenSpawns: traits.MinCyclesBetweenSpawns,
		ChanceToMutateDecisionTree: traits.ChanceToMutateDecisionTree,
		IdealPh:                traits.IdealPh,
		PhTolerance:            traits.PhTolerance,
		PhGrowthEffect:         traits.PhGrowthEffect,
		DecisionTree:           o.GetDecisionTreeCopy().Serialize(),
		CurrentAction:          int(o.Action()),
	}
}
