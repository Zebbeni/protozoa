package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// Measures how much the close-time sections add on top of the bytes
// written during a run, so the size projection uses a measured figure
// rather than a guess. Skipped unless SIZE_PROBE=1.
func TestReplaySizeProbe(t *testing.T) {
	if os.Getenv("SIZE_PROBE") != "1" {
		t.Skip("set SIZE_PROBE=1")
	}
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	var g config.Globals
	json.Unmarshal(data, &g)
	config.SetGlobals(&g)

	for _, cycles := range []int{2000, 6000, 12000, 20000} {
		path := filepath.Join(t.TempDir(), "probe.pzr")
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 1,
			CheckpointFile: path, CheckpointInterval: 1000})
		for i := 0; i < cycles && !sim.IsDone(); i++ {
			sim.Update()
		}
		during := sim.recorder.BytesWritten()
		nodes := sim.organismManager.TotalOrganismsCreated()
		histCycles := sim.cycle
		sim.CloseRecorder()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		final := info.Size()
		t.Logf("cycles %6d: during %9d final %9d closeAdds %8d nodes %7d bytes/node %.2f histCycles %d",
			cycles, during, final, final-during, nodes,
			float64(final-during)/float64(max(nodes, 1)), histCycles)
	}
}

// TestSizeEstimateBeatsTheRealFile is the check that matters for a cap:
// the projection must not come in *under* the file that actually lands
// on disk, or a run overruns the limit the user set. It is allowed to
// overshoot — stopping slightly early is the safe direction.
func TestSizeEstimateBeatsTheRealFile(t *testing.T) {
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	var g config.Globals
	json.Unmarshal(data, &g)
	config.SetGlobals(&g)

	for _, cycles := range []int{1000, 5000, 12000} {
		path := filepath.Join(t.TempDir(), "probe.pzr")
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 1,
			CheckpointFile: path, CheckpointInterval: 1000})
		for i := 0; i < cycles && !sim.IsDone(); i++ {
			sim.Update()
		}
		estimate := sim.EstimatedReplayBytes()
		sim.CloseRecorder()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		actual := info.Size()

		if estimate < actual {
			t.Errorf("at %d cycles the estimate was %d but the file is %d — a cap set on this would overrun",
				cycles, estimate, actual)
		}
		// And not so far over that the cap stops runs well short of the
		// size the user asked for. Only checked once the file is big
		// enough for that to mean anything: on a 6KB file the fixed
		// close-time overhead is most of the estimate, and the smallest
		// cap the user can set is 50MB.
		const bigEnoughToMatter = 1 << 20
		if actual > bigEnoughToMatter && float64(estimate) > 1.5*float64(actual) {
			t.Errorf("at %d cycles the estimate %d is more than 1.5x the real %d",
				cycles, estimate, actual)
		}
	}
}
