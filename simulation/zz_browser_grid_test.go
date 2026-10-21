package simulation

import (
	"fmt"
	"os"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestBrowserGridProbe asks what the browser's smaller world costs, which the
// WASM_DEFAULTS probe measures at one grid and one founder count and so
// cannot separate: a world that is too small for this balance, and a world
// that simply founds badly.
//
// It reports the PEAK population beside the final one, because the two
// failures look identical in a mean taken at the end — a world that never
// grew and one that grew and crashed.
//
//	BROWSER_GRID=1 go test ./simulation/ -run TestBrowserGridProbe -v -timeout 60m
func TestBrowserGridProbe(t *testing.T) {
	if os.Getenv("BROWSER_GRID") == "" {
		t.Skip("set BROWSER_GRID=1 to measure the browser grid against the desktop one")
	}
	cycles := envInt("BROWSER_GRID_CYCLES", 6000)
	seeds := envInt("BROWSER_GRID_SEEDS", 6)

	loadDefaultGlobals(t)
	base := *config.GetCurrentGlobals()
	baseCells := float64(base.GridUnitsWide * base.GridUnitsHigh)

	type arm struct{ w, h, founders int }
	arms := []arm{
		{60, 48, 1}, {60, 48, 5}, {60, 48, 10}, {60, 48, 20},
		{80, 64, 1}, {80, 64, 5},
		{100, 80, 1}, {100, 80, 5},
	}

	for _, a := range arms {
		g := base
		g.GridUnitsWide, g.GridUnitsHigh = a.w, a.h
		g.InitialOrganisms = a.founders
		// The per-world counts scale with the area exactly as BrowserDefaults
		// does them, so the only thing changing between arms is the room.
		share := float64(a.w*a.h) / baseCells
		g.InitialFood = int(float64(base.InitialFood) * share)
		g.InitialBuriedFood = int(float64(base.InitialBuriedFood) * share)
		g.InitialWalls = int(float64(base.InitialWalls) * share)

		alive, finals, peaks, ends, phs := 0, 0, 0, 0, 0.0
		var deaths []int
		for seed := 101; seed < 101+seeds; seed++ {
			run := g
			run.Seed = seed
			config.SetGlobals(&run)
			sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
			peak, i := 0, 0
			for ; i < cycles && sim.OrganismCount() > 0; i++ {
				sim.Update()
				peak = max(peak, sim.OrganismCount())
			}
			ends += i
			peaks += peak
			if n := sim.OrganismCount(); n > 0 {
				alive++
				finals += n
				phs += sim.AveragePh()
			} else {
				deaths = append(deaths, i)
			}
		}
		cells := a.w * a.h
		final, ph := 0, 0.0
		if alive > 0 {
			final, ph = finals/alive, phs/float64(alive)
		}
		t.Logf("%3dx%-3d (%5d cells) founders %2d: %d/%d alive, mean peak %5d (%3.0f%%), mean final %5d (%2.0f%%), pH %.2f, mean cycles %d, deaths at %v",
			a.w, a.h, cells, a.founders, alive, seeds,
			peaks/seeds, 100*float64(peaks/seeds)/float64(cells),
			final, 100*float64(final)/float64(cells),
			ph, ends/seeds, fmt.Sprint(deaths))
	}
}
