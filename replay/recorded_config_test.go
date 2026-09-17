package replay

import (
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simulation"
)

// TestReplayRecordsItsSettings checks a replay file carries the settings
// its simulation ran with, seed included, and that opening it applies
// them even after the active settings have changed.
func TestReplayRecordsItsSettings(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)
	g := *config.GetCurrentGlobals()
	g.MaxLifespan += 123
	g.AttackCosineK = 0.9
	g.Seed = 0
	config.SetGlobals(&g)

	path := filepath.Join(t.TempDir(), "settings.pzr")
	opts := &config.Options{IsHeadless: true, Seed: 77, CheckpointInterval: 10, CheckpointFile: path}
	sim := simulation.NewSimulation(opts)
	for i := 0; i < 30; i++ {
		sim.Update()
	}
	sim.CloseRecorder()

	// Change the active settings; the replay must not pick these up.
	changed := g
	changed.MaxLifespan = 1
	changed.Theme = "light"
	config.SetGlobals(&changed)

	ctrl, err := NewController(path, opts)
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}
	defer ctrl.Close()

	got := ctrl.Globals()
	if got.MaxLifespan != g.MaxLifespan || got.AttackCosineK != 0.9 {
		t.Errorf("recorded max_lifespan %d / attack_cosine_k %v, want %d / 0.9", got.MaxLifespan, got.AttackCosineK, g.MaxLifespan)
	}
	if got.Seed != 77 {
		t.Errorf("recorded seed %d, want the resolved seed 77", got.Seed)
	}
	if got.Theme != "light" {
		t.Errorf("theme %q: display preferences should stay the viewer's own", got.Theme)
	}
	if config.MaxLifespan() != g.MaxLifespan {
		t.Errorf("replay runs with max_lifespan %d, want the recorded %d", config.MaxLifespan(), g.MaxLifespan)
	}
	if err := ctrl.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestSeekKeepsTheSameTrees: a seek reinstalls the decoded descendant
// trees rather than rebuilding them, so the same node objects stay in
// place and views that cache anything keyed on them survive.
func TestSeekKeepsTheSameTrees(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)
	path := filepath.Join(t.TempDir(), "seek.pzr")
	opts := &config.Options{IsHeadless: true, Seed: 53, CheckpointInterval: 25, CheckpointFile: path}
	sim := simulation.NewSimulation(opts)
	for i := 0; i < 150; i++ {
		sim.Update()
	}
	sim.CloseRecorder()

	ctrl, err := NewController(path, opts)
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}
	defer ctrl.Close()

	gen := ctrl.Simulation().TreesGeneration()
	if gen == 0 {
		t.Fatal("a replay should have restored descendant trees")
	}
	node := ctrl.Simulation().GetOrganismTreeNode(1)
	if node == nil {
		t.Skip("no tree node to track in this recording")
	}

	if err := ctrl.SeekToCycle(100); err != nil {
		t.Fatalf("SeekToCycle: %v", err)
	}
	if got := ctrl.Simulation().TreesGeneration(); got != gen {
		t.Errorf("trees generation %d after a seek, want the original %d", got, gen)
	}
	if got := ctrl.Simulation().GetOrganismTreeNode(1); got != node {
		t.Error("a seek replaced the descendant tree nodes")
	}
}
