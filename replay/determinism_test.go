package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simulation"
)

// loadDefaultGlobalsFromDisk wires the simulation defaults into the process-wide config from the on-disk settings/default.json.
func loadDefaultGlobalsFromDisk(t *testing.T) {
	t.Helper()
	path := filepath.Join("..", "settings", "default.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	// testdata/last.pzr was recorded before a replay carried the settings it
	// ran with, so it falls back to whatever is active — and the shipped
	// defaults are a NEURAL world while the fixture is a tree one. Pinning
	// the brain here is standing in for the header that file does not have:
	// a recording made today replays as its own world without this.
	g.UseNeuralNets = false
	config.SetGlobals(&g)
}

// loadShippedGlobals installs settings/default.json exactly as it ships, for a
// test that records its own run rather than reading the fixture.
func loadShippedGlobals(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	config.SetGlobals(&g)
}

// hasUsedLastCycle reports whether any node in the tree is marked as having been visited on the most recent chooseAction.
func hasUsedLastCycle(n *d.Node) bool {
	tree := &d.Tree{Node: n}
	for _, line := range tree.PrintLines() {
		if line.UsedLastCycle {
			return true
		}
	}
	return false
}

func TestPhMapSurvivesSnapshotRoundtrip(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)

	opts := &config.Options{
		IsHeadless:         true,
		Seed:               53,
		CheckpointInterval: 1000,
	}
	sim := simulation.NewSimulation(opts)
	for i := 0; i < 100; i++ {
		sim.Update()
	}

	beforeSnap := sim.CaptureSnapshot()
	if beforeSnap == nil {
		t.Fatal("CaptureSnapshot returned nil")
	}
	restored, err := simulation.RestoreFromSnapshot(beforeSnap, opts)
	if err != nil {
		t.Fatalf("RestoreFromSnapshot: %v", err)
	}

	beforePh := sim.GetPhMap()
	afterPh := restored.GetPhMap()
	mismatches := 0
	maxDiff := 0.0
	for x := range beforePh {
		for y := range beforePh[x] {
			diff := beforePh[x][y] - afterPh[x][y]
			if diff < 0 {
				diff = -diff
			}
			if diff > 0 {
				mismatches++
				if diff > maxDiff {
					maxDiff = diff
				}
			}
		}
	}
	if mismatches > 0 {
		t.Errorf("pH map differs after snapshot round-trip: %d cells differ, max abs diff = %g",
			mismatches, maxDiff)
	}
}

// TestFreshRecordReplayRoundtrip records a fresh sim with the current simulation code and verifies that replaying that recording reaches the exact same state at the same cycle.
func TestFreshRecordReplayRoundtrip(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)

	const targetCycle = 2000
	tempPath := filepath.Join(t.TempDir(), "fresh.pzr")

	recordOpts := &config.Options{
		IsHeadless:         true,
		Seed:               53,
		CheckpointInterval: 1000,
		CheckpointFile:     tempPath,
	}
	recordSim := simulation.NewSimulation(recordOpts)
	for recordSim.Cycle() < targetCycle {
		recordSim.Update()
	}
	recordSim.CloseRecorder()
	recordedAlive := recordSim.GetAllOrganismInfo()

	replayOpts := &config.Options{
		IsHeadless: true,
		ReplayFile: tempPath,
	}
	ctrl, err := NewController(tempPath, replayOpts)
	if err != nil {
		t.Fatalf("open recorded file: %v", err)
	}
	defer ctrl.Close()
	replaySim := ctrl.Simulation()
	replaySim.Pause(false)
	for replaySim.Cycle() < targetCycle {
		replaySim.Update()
	}
	replayAlive := replaySim.GetAllOrganismInfo()

	if len(recordedAlive) != len(replayAlive) {
		t.Errorf("alive counts differ at cycle %d: record=%d replay=%d",
			targetCycle, len(recordedAlive), len(replayAlive))
	}
	missing, extra := 0, 0
	for id := range recordedAlive {
		if _, ok := replayAlive[id]; !ok {
			missing++
		}
	}
	for id := range replayAlive {
		if _, ok := recordedAlive[id]; !ok {
			extra++
		}
	}
	if missing > 0 || extra > 0 {
		t.Errorf("at cycle %d: %d IDs in record missing from replay, %d extra in replay",
			targetCycle, missing, extra)
	}
}

func TestRingSnapshotsAreIsolatedFromForwardPlay(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)

	const targetCycle = 200
	tempPath := filepath.Join(t.TempDir(), "fresh.pzr")

	recordOpts := &config.Options{
		IsHeadless:         true,
		Seed:               53,
		CheckpointInterval: 100,
		CheckpointFile:     tempPath,
	}
	recordSim := simulation.NewSimulation(recordOpts)
	for recordSim.Cycle() < targetCycle {
		recordSim.Update()
	}
	recordSim.CloseRecorder()

	replayOpts := &config.Options{IsHeadless: true, ReplayFile: tempPath}
	ctrl, err := NewController(tempPath, replayOpts)
	if err != nil {
		t.Fatalf("open recorded file: %v", err)
	}
	defer ctrl.Close()
	sim := ctrl.Simulation()
	sim.Pause(false)
	// Use ctrl.StepForward so pushRing populates the in-memory ring.
	for sim.Cycle() < targetCycle {
		ctrl.StepForward()
	}

	startCycle := sim.Cycle()
	firstHash := stateHash(sim)
	// Step back 5, forward 5: we should land back at startCycle with identical state.
	for round := 0; round < 5; round++ {
		for i := 0; i < 5; i++ {
			if !ctrl.StepBackward() {
				t.Fatalf("round %d step-back %d failed at cycle %d", round, i, sim.Cycle())
			}
		}
		for i := 0; i < 5; i++ {
			ctrl.StepForward()
		}
		if sim.Cycle() != startCycle {
			t.Fatalf("round %d: sim.Cycle()=%d, want %d", round, sim.Cycle(), startCycle)
		}
		if got := stateHash(sim); got != firstHash {
			t.Fatalf("round %d at cycle %d: state hash drifted (was %x, got %x)",
				round, sim.Cycle(), firstHash, got)
		}
	}
}

// stateHash returns an FNV-1a hash of the alive-organism set keyed by id, location, direction.
func stateHash(sim *simulation.Simulation) uint64 {
	var hash uint64 = 1469598103934665603
	mix := func(v uint64) {
		hash ^= v
		hash *= 1099511628211
	}
	infos := sim.GetAllOrganismInfo()
	ids := make([]int, 0, len(infos))
	for id := range infos {
		ids = append(ids, id)
	}
	// Sort ids so map iteration order can't bleed into the hash.
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1] > ids[j]; j-- {
			ids[j-1], ids[j] = ids[j], ids[j-1]
		}
	}
	for _, id := range ids {
		o := infos[id]
		mix(uint64(id))
		mix(uint64(o.Location.X))
		mix(uint64(o.Location.Y))
		mix(uint64(int8(o.Direction.X)) & 0xFF)
		mix(uint64(int8(o.Direction.Y)) & 0xFF)
	}
	return hash
}

// TestFreshSimulationDescendantTreeConsistent runs a fresh simulation from default settings + seed 53 and verifies the resulting descendant tree is structurally consistent.
func TestFreshSimulationDescendantTreeConsistent(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)

	opts := &config.Options{
		IsHeadless:         true,
		Seed:               53,
		CheckpointInterval: 1000,
	}
	sim := simulation.NewSimulation(opts)

	const totalCycles = 2000
	for i := 0; i < totalCycles; i++ {
		sim.Update()
	}

	trees := sim.GetDescendantTrees()
	issues := 0
	const maxIssues = 20

	var walk func(n *organism.DescendantNode)
	walk = func(n *organism.DescendantNode) {
		if n.Parent != nil && issues < maxIssues {
			if n.Parent.StartCycle > n.StartCycle {
				t.Errorf("node %d StartCycle=%d born before parent %d StartCycle=%d",
					n.ID, n.StartCycle, n.Parent.ID, n.Parent.StartCycle)
				issues++
			}
			if n.Parent.EndCycle != 0 && n.Parent.EndCycle < n.StartCycle {
				t.Errorf("node %d StartCycle=%d but parent %d died at EndCycle=%d",
					n.ID, n.StartCycle, n.Parent.ID, n.Parent.EndCycle)
				issues++
			}
		}
		if n.EndCycle != 0 && n.StartCycle > n.EndCycle && issues < maxIssues {
			t.Errorf("node %d has StartCycle=%d > EndCycle=%d", n.ID, n.StartCycle, n.EndCycle)
			issues++
		}
		n.ForEachChild(walk)
	}
	for _, root := range trees {
		if root != nil {
			walk(root)
		}
	}
}

// TestDecisionTreeFlagsRestoredAfterSnapshot verifies that an organism's decision-tree flags (WasTravelled / UsedLastCycle) are populated after restoring from a snapshot.
func TestDecisionTreeFlagsRestoredAfterSnapshot(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)

	opts := &config.Options{
		IsHeadless:         true,
		Seed:               53,
		CheckpointInterval: 1000,
	}
	sim := simulation.NewSimulation(opts)
	for i := 0; i < 200; i++ {
		sim.Update()
	}

	snap := sim.CaptureSnapshot()
	if snap == nil {
		t.Fatal("CaptureSnapshot returned nil")
	}

	restored, err := simulation.RestoreFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("RestoreFromSnapshot: %v", err)
	}

	allInfo := restored.GetAllOrganismInfo()
	if len(allInfo) == 0 {
		t.Skip("no organisms to test")
	}
	withFlags := 0
	for id := range allInfo {
		tree := restored.GetOrganismDecisionTreeByID(id)
		if tree == nil {
			continue
		}
		if hasUsedLastCycle(tree.Node) {
			withFlags++
		}
	}
	if withFlags == 0 {
		t.Errorf("after restore, no organism has UsedLastCycle flags set on its decision tree")
	}
}

// TestResumeMostSuccessfulAliveAtTargetCycle: after opening a recording and
// playing to a cycle, the Most Successful select mode has something to select.
// The set is built from the recording's descendant trees, and an empty one
// leaves that mode dead with nothing on screen to say why.
//
// It records its OWN run rather than reading testdata/last.pzr. That fixture
// carries no settings of its own, so a replay of it runs whatever balance is
// shipped today and the organisms it calls successful die somewhere in the
// 1,567 cycles — which made this a test of the shipped BALANCE rather than of
// the select mode, and it duly broke when the defaults changed. A recording
// made here carries its own header, so the replay reproduces the run exactly
// and "who was successful" means the same thing on both sides.
func TestResumeMostSuccessfulAliveAtTargetCycle(t *testing.T) {
	loadShippedGlobals(t)

	const targetCycle = 2000
	path := filepath.Join(t.TempDir(), "successful.pzr")

	recorded := simulation.NewSimulation(&config.Options{
		IsHeadless:         true,
		Seed:               53,
		CheckpointInterval: 500,
		CheckpointFile:     path,
	})
	for recorded.Cycle() < targetCycle && recorded.OrganismCount() > 0 {
		recorded.Update()
	}
	recorded.CloseRecorder()
	if recorded.OrganismCount() == 0 {
		t.Skip("the recorded world died, so there is no successful lineage to find")
	}

	ctrl, err := NewController(path, &config.Options{IsHeadless: true, ReplayFile: path})
	if err != nil {
		t.Fatalf("open the recording: %v", err)
	}
	defer ctrl.Close()

	sim := ctrl.Simulation()
	sim.Pause(false)
	for sim.Cycle() < targetCycle {
		sim.Update()
	}

	if living := sim.GetMostSuccessfulIds(); len(living) == 0 {
		t.Fatalf("at cycle %d: no organisms in most-successful set are alive (total alive=%d)",
			targetCycle, sim.OrganismCount())
	}
}
