package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

func TestEndCondition(t *testing.T) {
	const min = 10
	const maxCycles = 500
	for _, tc := range []struct {
		name           string
		alive          int
		cycle          int
		armed          bool
		min            int
		maxCycles      int
		replayBytes    int64
		maxReplayBytes int64
		want           EndCondition
	}{
		{"extinct always ends, even unarmed", 0, 1, false, min, 0, 0, 0, EndExtinct},
		{"extinct always ends, even disabled", 0, 1, false, 0, 0, 0, 0, EndExtinct},
		{"founding phase below min does not end", 3, 1, false, min, 0, 0, 0, EndNone},
		{"armed and below min ends", min - 1, 1, true, min, 0, 0, 0, EndBelowMinimum},
		{"armed at exactly min keeps running", min, 1, true, min, 0, 0, 0, EndNone},
		{"armed above min keeps running", 50, 1, true, min, 0, 0, 0, EndNone},
		{"disabled never ends a living population", 1, 1, true, 0, 0, 0, 0, EndNone},

		{"max cycles off runs forever", 50, 1 << 30, true, min, 0, 0, 0, EndNone},
		{"before max cycles keeps running", 50, maxCycles - 1, true, min, maxCycles, 0, 0, EndNone},
		{"at max cycles ends", 50, maxCycles, true, min, maxCycles, 0, 0, EndMaxCycles},
		{"past max cycles ends", 50, maxCycles + 1, true, min, maxCycles, 0, 0, EndMaxCycles},
		// A cycle limit needs no arming: unlike the minimum it can't fire
		// during the founding phase by accident, since cycles only rise.
		{"max cycles applies unarmed", 3, maxCycles, false, min, maxCycles, 0, 0, EndMaxCycles},

		// The replay size cap, in bytes.
		{"under the replay cap keeps running", 50, 1, true, min, 0, 100, 200, EndNone},
		{"at the replay cap ends", 50, 1, true, min, 0, 200, 200, EndMaxReplaySize},
		{"past the replay cap ends", 50, 1, true, min, 0, 300, 200, EndMaxReplaySize},
		{"a 0 cap can't fire", 50, 1, true, min, 0, 1 << 40, 0, EndNone},

		// Extinction wins, so a run that ends with nothing alive says so
		// rather than blaming whichever limit it also crossed.
		{"extinction beats max cycles", 0, maxCycles, true, min, maxCycles, 0, 0, EndExtinct},
		{"a dwindling population beats max cycles", min - 1, maxCycles, true, min, maxCycles, 0, 0, EndBelowMinimum},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := endCondition(tc.alive, tc.cycle, tc.armed, tc.min, tc.maxCycles,
				tc.replayBytes, tc.maxReplayBytes)
			if got != tc.want {
				t.Errorf("endCondition(alive=%d, cycle=%d, armed=%v, min=%d, maxCycles=%d, replay=%d/%d) = %v, want %v",
					tc.alive, tc.cycle, tc.armed, tc.min, tc.maxCycles,
					tc.replayBytes, tc.maxReplayBytes, got, tc.want)
			}
		})
	}
}

func globalsWithMin(t *testing.T, min int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	g.MinOrganisms = min
	config.SetGlobals(&g)
}

// runUntilDone steps a sim the way the headless runner does, recording
// the peak population along the way.
func runUntilDone(t *testing.T, seed, maxCycles int) (sim *Simulation, peak int) {
	t.Helper()
	sim = NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
	for i := 0; i < maxCycles && !sim.IsDone(); i++ {
		sim.Update()
		if n := sim.OrganismCount(); n > peak {
			peak = n
		}
	}
	return sim, peak
}

// tailOffSeed and the small grid below give a run that grows past twice
// the minimum and then crashes below it within about a thousand cycles.
// A small world keeps populations small, so the run is quick and crashes
// are common; at the default grid size worlds have become stable enough
// that no seed searched falls below the minimum in 8000 cycles. Balance
// changes can make a chosen seed survive — the test says so and a fresh
// one is easy to find, since most small-grid seeds still tail off.
const (
	tailOffSeed  = 2
	tailOffGridW = 24
	tailOffGridH = 20
)

// globalsForTailOff loads default globals with the given minimum on the
// small tail-off grid.
func globalsForTailOff(t *testing.T, min int) {
	t.Helper()
	globalsWithMin(t, min)
	config.GetCurrentGlobals().GridUnitsWide = tailOffGridW
	config.GetCurrentGlobals().GridUnitsHigh = tailOffGridH
}

// TestMinOrganismsEndsTailOffRun checks the end condition against a real
// run: it must fire only after the population has doubled the minimum,
// and only once the population is below it — never during the founding
// phase, when every run starts with a single organism.
func TestMinOrganismsEndsTailOffRun(t *testing.T) {
	const min = 10
	globalsForTailOff(t, min)

	sim, peak := runUntilDone(t, tailOffSeed, 20000)
	if !sim.IsDone() {
		t.Fatalf("seed %d no longer falls below the minimum in 20000 cycles; pick a new tailOffSeed", tailOffSeed)
	}
	alive := sim.OrganismCount()
	if alive != 0 {
		if !sim.minOrganismsArmed || peak < 2*min {
			t.Errorf("ended with %d alive before arming (peak %d, needs %d)", alive, peak, 2*min)
		}
		if alive >= min {
			t.Errorf("ended with %d alive, not below the minimum of %d", alive, min)
		}
	}
	t.Logf("ended at cycle %d with %d alive (peak %d)", sim.Cycle(), alive, peak)

	// The same seed with the condition disabled can only run as long or
	// longer — the minimum ends runs early, it never extends them.
	globalsForTailOff(t, 0)
	full, _ := runUntilDone(t, tailOffSeed, 20000)
	if full.Cycle() < sim.Cycle() {
		t.Errorf("disabling min_organisms made the run shorter: %d vs %d cycles", full.Cycle(), sim.Cycle())
	}
	t.Logf("with min_organisms disabled: ended at cycle %d with %d alive", full.Cycle(), full.OrganismCount())
}

// TestMinOrganismsArmedSurvivesSnapshot: a run resumed from a checkpoint
// must keep its armed end condition, or it would silently idle until the
// population doubled again.
func TestMinOrganismsArmedSurvivesSnapshot(t *testing.T) {
	globalsWithMin(t, 10)
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 12345, CheckpointInterval: 1 << 30})
	for i := 0; i < 3000 && !sim.minOrganismsArmed; i++ {
		sim.Update()
	}
	if !sim.minOrganismsArmed {
		t.Skip("population never reached twice the minimum")
	}

	restored, err := RestoreFromSnapshot(sim.CaptureSnapshot(), &config.Options{IsHeadless: true, Seed: 12345, CheckpointInterval: 1 << 30})
	if err != nil {
		t.Fatalf("RestoreFromSnapshot: %v", err)
	}
	if !restored.minOrganismsArmed {
		t.Error("armed end condition lost across snapshot restore")
	}
}

// TestMaxCyclesStopsARunningSim drives a real simulation to the cycle
// limit. The unit test above pins the decision; this pins that the
// decision is actually consulted by the loop, on a population that is
// alive and well and would otherwise keep going.
func TestMaxCyclesStopsARunningSim(t *testing.T) {
	const limit = 300
	globalsForTailOff(t, 0) // minimum off, so only the cycle limit can fire
	config.GetCurrentGlobals().MaxCycles = limit

	sim, _ := runUntilDone(t, 1, limit*4)
	if !sim.IsDone() {
		t.Fatalf("the run passed %d cycles without stopping", limit*4)
	}
	if got := sim.EndCondition(); got != EndMaxCycles {
		t.Errorf("stopped for %v at cycle %d with %d alive, want the cycle limit",
			got, sim.Cycle(), sim.OrganismCount())
	}
	if sim.Cycle() != limit {
		t.Errorf("stopped at cycle %d, want exactly %d", sim.Cycle(), limit)
	}
}

// TestMaxCyclesOffRunsOn: 0 has to mean unlimited, since that is what
// every run did before the setting existed and what every settings file
// written before it will decode to.
func TestMaxCyclesOffRunsOn(t *testing.T) {
	globalsForTailOff(t, 0)
	config.GetCurrentGlobals().MaxCycles = 0

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 1, CheckpointInterval: 1 << 30})
	for c := 0; c < 400; c++ {
		sim.Update()
	}
	if got := sim.EndCondition(); got == EndMaxCycles {
		t.Error("a max_cycles of 0 stopped the run")
	}
}
