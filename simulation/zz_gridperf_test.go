package simulation

import (
	"os"
	"testing"
	"time"

	"github.com/Zebbeni/protozoa/config"
)

// GRID_PERF=1 go test ./simulation/ -run TestGridPerf -v
//
// Cycles per second at several grid sizes, with the per-world counts scaled
// by area so each world starts at the same density. Native speed is not wasm
// speed; the RATIO between sizes is what this is for.
func TestGridPerf(t *testing.T) {
	if os.Getenv("GRID_PERF") == "" {
		t.Skip("set GRID_PERF=1")
	}
	loadDefaultGlobals(t)
	base := config.GetCurrentGlobals()
	baseCells := float64(base.GridUnitsWide * base.GridUnitsHigh)

	for _, d := range [][3]int{{60, 48, 1}, {60, 48, 5}, {60, 48, 10}, {60, 48, 20}} {
		g := *base
		g.GridUnitsWide, g.GridUnitsHigh = d[0], d[1]
		g.InitialOrganisms = d[2]
		share := float64(d[0]*d[1]) / baseCells
		g.InitialFood = int(float64(base.InitialFood) * share)
		g.InitialBuriedFood = int(float64(base.InitialBuriedFood) * share)
		config.SetGlobals(&g)

		const cycles = 3000
		alive, pops, rate := 0, 0, 0.0
		for seed := 101; seed <= 106; seed++ {
			sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
			start := time.Now()
			for i := 0; i < cycles && sim.OrganismCount() > 0; i++ {
				sim.Update()
			}
			rate += float64(cycles) / time.Since(start).Seconds()
			if n := sim.OrganismCount(); n > 0 {
				alive++
				pops += n
			}
		}
		mean := 0
		if alive > 0 {
			mean = pops / alive
		}
		t.Logf("%3dx%-3d founders %2d: %5.0f cycles/sec, survived %d/6, mean pop %d",
			d[0], d[1], d[2], rate/6, alive, mean)
	}
}
