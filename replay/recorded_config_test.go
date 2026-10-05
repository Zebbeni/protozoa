package replay

import (
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simulation"
)

// TestReplayRecordsItsSettings checks a replay file carries the settings its simulation ran with, seed included.
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

	// Seek first, THEN pick an organism to track. Tracking one chosen before
	// the seek couples this to that organism being alive at both cycles,
	// which is a property of the balance rather than of seeking: under the
	// shipped settings founder 1 is dead by cycle 100 and the lookup
	// returned nil, failing a test about node identity for a reason that has
	// nothing to do with it.
	if err := ctrl.SeekToCycle(100); err != nil {
		t.Fatalf("SeekToCycle: %v", err)
	}
	var id int
	for orgID := range ctrl.Simulation().GetAllOrganismInfo() {
		id = orgID
		break
	}
	if id == 0 {
		t.Skip("nothing alive at the seek target to track")
	}
	node := ctrl.Simulation().GetOrganismTreeNode(id)
	if node == nil {
		t.Skipf("organism %d has no tree node to track", id)
	}

	// Away and back to the SAME cycle, so the organism is alive at both
	// samples: whether a given one survives from 100 to 120 is a property of
	// the balance, and this is about whether a seek rebuilds the trees.
	if err := ctrl.SeekToCycle(50); err != nil {
		t.Fatalf("SeekToCycle: %v", err)
	}
	if err := ctrl.SeekToCycle(100); err != nil {
		t.Fatalf("SeekToCycle: %v", err)
	}
	if got := ctrl.Simulation().TreesGeneration(); got != gen {
		t.Errorf("trees generation %d after a seek, want the original %d", got, gen)
	}
	// Reinstalled, not rebuilt: the graph image and its alive-set cache hold
	// these pointers across a seek.
	if got := ctrl.Simulation().GetOrganismTreeNode(id); got != node {
		t.Errorf("a seek replaced descendant tree node %d: was %p, now %p", id, node, got)
	}
}
