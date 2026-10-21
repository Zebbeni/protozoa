package replay

import (
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simulation"
)

// TestANeuralRunRecordsAndReplays is the end-to-end claim for the brain that a
// snapshot carries in a field of its own: a recorded neural world reopens as
// the same world, with every organism holding the net it was recorded with.
//
// The other replay tests all run the shipped settings, which means decision
// trees — so without this the Brain field round-trips through nothing but a
// unit test, and the first time a user records a neural run would be the first
// time the path is exercised.
func TestANeuralRunRecordsAndReplays(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)
	g := config.GetCurrentGlobals()
	g.UseNeuralNets = true
	config.SetGlobals(g)

	const targetCycle = 1200
	tempPath := filepath.Join(t.TempDir(), "neural.pzr")

	recordSim := simulation.NewSimulation(&config.Options{
		IsHeadless:         true,
		Seed:               6,
		CheckpointInterval: 400,
		CheckpointFile:     tempPath,
	})
	for recordSim.Cycle() < targetCycle && recordSim.OrganismCount() > 0 {
		recordSim.Update()
	}
	recordSim.CloseRecorder()
	if recordSim.OrganismCount() == 0 {
		t.Fatalf("the recorded world died before cycle %d; nothing to replay", targetCycle)
	}
	recorded := recordSim.GetAllOrganismInfo()

	// The brains as the recording left them, keyed by id.
	recordedBrains := map[int]string{}
	for id := range recorded {
		net := recordSim.GetOrganismNeuralNetByID(id)
		if net == nil {
			t.Fatalf("organism %d was recorded in a neural world with no net", id)
		}
		recordedBrains[id] = string(net.Encode())
	}

	ctrl, err := NewController(tempPath, &config.Options{
		IsHeadless: true,
		ReplayFile: tempPath,
	})
	if err != nil {
		t.Fatalf("open the recorded file: %v", err)
	}
	defer ctrl.Close()

	replaySim := ctrl.Simulation()
	replaySim.Pause(false)
	for replaySim.Cycle() < targetCycle {
		replaySim.Update()
	}

	replayed := replaySim.GetAllOrganismInfo()
	if len(replayed) != len(recorded) {
		t.Fatalf("cycle %d holds %d organisms in the recording and %d on replay",
			targetCycle, len(recorded), len(replayed))
	}

	checked := 0
	for id, want := range recordedBrains {
		net := replaySim.GetOrganismNeuralNetByID(id)
		if net == nil {
			t.Errorf("organism %d came back from the recording with no net", id)
			continue
		}
		if got := string(net.Encode()); got != want {
			t.Errorf("organism %d replayed with a different net than it was recorded with", id)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no organism was compared; the test says nothing")
	}
	t.Logf("%d organisms replayed with the net they were recorded with", checked)
}

// TestSeekingANeuralWorldRestoresItsBrains: a seek rebuilds the organism
// manager from a snapshot rather than playing forward, so it is the path that
// reads the Brain field back off disk.
func TestSeekingANeuralWorldRestoresItsBrains(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)
	g := config.GetCurrentGlobals()
	g.UseNeuralNets = true
	config.SetGlobals(g)

	tempPath := filepath.Join(t.TempDir(), "seek.pzr")
	recordSim := simulation.NewSimulation(&config.Options{
		IsHeadless:         true,
		Seed:               6,
		CheckpointInterval: 250,
		CheckpointFile:     tempPath,
	})
	for recordSim.Cycle() < 1000 && recordSim.OrganismCount() > 0 {
		recordSim.Update()
	}
	recordSim.CloseRecorder()
	if recordSim.OrganismCount() == 0 {
		t.Fatal("the recorded world died; nothing to seek through")
	}

	ctrl, err := NewController(tempPath, &config.Options{
		IsHeadless: true,
		ReplayFile: tempPath,
	})
	if err != nil {
		t.Fatalf("open the recorded file: %v", err)
	}
	defer ctrl.Close()

	// Seek away and back to the same cycle, which is how the panel's own
	// seek test avoids depending on which organisms a balance keeps alive.
	if err := ctrl.SeekToCycle(750); err != nil {
		t.Fatalf("seek to 750: %v", err)
	}
	first := brainsAt(t, ctrl)
	if err := ctrl.SeekToCycle(250); err != nil {
		t.Fatalf("seek to 250: %v", err)
	}
	if err := ctrl.SeekToCycle(750); err != nil {
		t.Fatalf("seek back to 750: %v", err)
	}
	second := brainsAt(t, ctrl)

	if len(first) == 0 {
		t.Fatal("nothing was alive at the sampled cycle")
	}
	if len(first) != len(second) {
		t.Fatalf("the same cycle holds %d organisms on one seek and %d on another",
			len(first), len(second))
	}
	for id, want := range first {
		got, ok := second[id]
		if !ok {
			t.Errorf("organism %d is alive on one seek to the cycle and not the other", id)
			continue
		}
		if got != want {
			t.Errorf("organism %d has a different net after seeking away and back", id)
		}
	}
	t.Logf("%d brains identical across a seek away and back", len(first))
}

// brainsAt is every living organism's encoded net at the controller's current
// cycle.
func brainsAt(t *testing.T, ctrl *Controller) map[int]string {
	t.Helper()
	sim := ctrl.Simulation()
	out := map[int]string{}
	for id := range sim.GetAllOrganismInfo() {
		net := sim.GetOrganismNeuralNetByID(id)
		if net == nil {
			t.Errorf("organism %d has no net in a neural recording", id)
			continue
		}
		out[id] = string(net.Encode())
	}
	return out
}
