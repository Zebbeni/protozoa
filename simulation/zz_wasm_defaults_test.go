package simulation

import (
	"os"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestWasmDefaultsProbe runs the world a BROWSER visitor gets, which is the
// shipped defaults with config.BrowserDefaults applied, and reports survival,
// population and pH beside the desktop baseline.
//
// It exists because the browser's configuration used to be measurable only by
// building for wasm, so the one number anyone had for it was the grid size.
// Asserts nothing: what the numbers should be is a balance question.
//
//	WASM_DEFAULTS=1 WASM_DEFAULTS_CYCLES=4000 go test ./simulation/ -run TestWasmDefaultsProbe -v -timeout 30m
func TestWasmDefaultsProbe(t *testing.T) {
	if os.Getenv("WASM_DEFAULTS") == "" {
		t.Skip("set WASM_DEFAULTS=1 to measure the browser world")
	}
	cycles := envInt("WASM_DEFAULTS_CYCLES", 4000)
	seeds := envInt("WASM_DEFAULTS_SEEDS", 6)

	loadDefaultGlobals(t)
	base := *config.GetCurrentGlobals()

	browser := base
	config.BrowserDefaults(&browser)

	// Two arms, not three. There was a third holding chemo_ph_effect at the
	// desktop value, from when BrowserDefaults raised it; it no longer does,
	// so that arm reproduced the browser arm digit for digit and read as a
	// control confirming something. An override is only worth an arm while
	// there is an override.
	for _, arm := range []struct {
		name string
		g    config.Globals
	}{
		{"desktop", base},
		{"browser", browser},
	} {
		g := arm.g
		alive, pops, phs, ends := 0, 0, 0.0, 0
		for seed := 101; seed < 101+seeds; seed++ {
			run := g
			run.Seed = seed
			config.SetGlobals(&run)
			sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
			i := 0
			for ; i < cycles && sim.OrganismCount() > 0; i++ {
				sim.Update()
			}
			ends += i
			if n := sim.OrganismCount(); n > 0 {
				alive++
				pops += n
				phs += sim.AveragePh()
			}
		}
		cells := g.GridUnitsWide * g.GridUnitsHigh
		if alive == 0 {
			t.Logf("%-28s %dx%d: 0/%d survived %d cycles", arm.name,
				g.GridUnitsWide, g.GridUnitsHigh, seeds, cycles)
			continue
		}
		pop := pops / alive
		t.Logf("%-28s %dx%d (%d cells): %d/%d alive, mean pop %d (%.0f%% occupancy), mean pH %.2f, mean cycles %d",
			arm.name, g.GridUnitsWide, g.GridUnitsHigh, cells,
			alive, seeds, pop, 100*float64(pop)/float64(cells), phs/float64(alive), ends/seeds)
	}
}
