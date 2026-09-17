package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

func TestShouldEnd(t *testing.T) {
	const min = 10
	for _, tc := range []struct {
		name  string
		alive int
		armed bool
		min   int
		want  bool
	}{
		{"extinct always ends, even unarmed", 0, false, min, true},
		{"extinct always ends, even disabled", 0, false, 0, true},
		{"founding phase below min does not end", 3, false, min, false},
		{"armed and below min ends", min - 1, true, min, true},
		{"armed at exactly min keeps running", min, true, min, false},
		{"armed above min keeps running", 50, true, min, false},
		{"disabled never ends a living population", 1, true, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldEnd(tc.alive, tc.armed, tc.min); got != tc.want {
				t.Errorf("shouldEnd(alive=%d, armed=%v, min=%d) = %v, want %v",
					tc.alive, tc.armed, tc.min, got, tc.want)
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
	tailOffSeed  = 3
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
