package simulation

import (
	"fmt"
	"image/color"

	"github.com/Zebbeni/protozoa/checkpoint"
	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
	"github.com/lucasb-eyer/go-colorful"
)

// RestoreFromCheckpoint opens a .pzr file and restores the simulation from
// the last snapshot. The simulation can then continue running from that point.
func RestoreFromCheckpoint(path string, options *config.Options) (*Simulation, error) {
	reader, err := checkpoint.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open checkpoint: %w", err)
	}
	defer reader.Close()

	if reader.SnapshotCount() == 0 {
		return nil, fmt.Errorf("checkpoint file has no snapshots")
	}

	// Read the last snapshot
	snap, err := reader.ReadSnapshot(reader.SnapshotCount() - 1)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot: %w", err)
	}

	return RestoreFromSnapshot(snap, options)
}

// RestoreFromSnapshot rebuilds a full Simulation from a snapshot payload.
func RestoreFromSnapshot(snap *checkpoint.SnapshotPayload, options *config.Options) (*Simulation, error) {
	// Restore RNG
	rng, err := simrand.RestoreFromState(snap.RNGState)
	if err != nil {
		return nil, fmt.Errorf("failed to restore RNG state: %w", err)
	}

	sim := &Simulation{
		options: options,
		rng:     rng,
		cycle:   snap.Cycle,
	}

	sim.updateManager = manager.NewUpdateManager()
	sim.environmentManager = restoreEnvironment(sim, snap)
	sim.foodManager = restoreFood(sim, rng, snap)
	sim.organismManager = restoreOrganisms(sim, rng, snap)

	return sim, nil
}

// ResetFromSnapshot replaces this simulation's internal state from a snapshot
// while preserving the pointer identity (so all UI references remain valid).
func (s *Simulation) ResetFromSnapshot(snap *checkpoint.SnapshotPayload) error {
	rng, err := simrand.RestoreFromState(snap.RNGState)
	if err != nil {
		return fmt.Errorf("failed to restore RNG state: %w", err)
	}

	s.rng = rng
	s.cycle = snap.Cycle
	s.updateManager = manager.NewUpdateManager()
	s.environmentManager = restoreEnvironment(s, snap)
	s.foodManager = restoreFood(s, rng, snap)
	s.organismManager = restoreOrganisms(s, rng, snap)

	return nil
}

func restoreEnvironment(sim *Simulation, snap *checkpoint.SnapshotPayload) *manager.EnvironmentManager {
	return manager.RestoreEnvironmentManager(sim, snap.CurrentPhMap, snap.PreviousPhMap)
}

func restoreFood(sim *Simulation, rng *simrand.RNG, snap *checkpoint.SnapshotPayload) *manager.FoodManager {
	return manager.RestoreFoodManager(sim, rng, snap.FoodItems)
}

func restoreOrganisms(sim *Simulation, rng *simrand.RNG, snap *checkpoint.SnapshotPayload) *manager.OrganismManager {
	// Build organisms from records
	organisms := make(map[int]*organism.Organism)
	for _, rec := range snap.Organisms {
		o := recordToOrganism(rec, sim)
		organisms[o.ID] = o
	}

	// Rebuild ancestor data
	ancestorIDs := make([]int, 0, len(snap.Ancestors))
	ancestorColors := make(map[int]color.Color)
	for _, a := range snap.Ancestors {
		ancestorIDs = append(ancestorIDs, a.ID)
		ancestorColors[a.ID] = colorful.Color{R: a.ColorR, G: a.ColorG, B: a.ColorB}
	}

	return manager.RestoreOrganismManager(sim, rng, organisms, snap.OrganismGrid,
		snap.TotalOrganismsCreated, ancestorIDs, ancestorColors)
}

func recordToOrganism(rec checkpoint.OrganismRecord, api organism.LookupAPI) *organism.Organism {
	traits := organism.Traits{
		OrganismColor:              colorful.Color{R: rec.ColorR, G: rec.ColorG, B: rec.ColorB},
		MaxSize:                    rec.MaxSize,
		SpawnHealth:                rec.SpawnHealth,
		MinHealthToSpawn:           rec.MinHealthToSpawn,
		MinCyclesBetweenSpawns:     rec.MinCyclesBetweenSpawns,
		ChanceToMutateDecisionTree: rec.ChanceToMutateDecisionTree,
		IdealPh:                    rec.IdealPh,
		PhTolerance:                rec.PhTolerance,
		PhGrowthEffect:             rec.PhGrowthEffect,
		MaxLifespan:                rec.MaxLifespan,
	}

	tree := d.DeserializeTree(rec.DecisionTree)

	return organism.Restore(
		rec.ID, rec.Age, rec.Health, rec.Size, rec.Children,
		rec.TraveledDist, rec.CyclesSinceLastSpawn,
		utils.Point{X: rec.LocationX, Y: rec.LocationY},
		utils.Point{X: rec.DirectionX, Y: rec.DirectionY},
		rec.OriginalAncestorID,
		traits, tree, d.Action(rec.CurrentAction), api,
	)
}
