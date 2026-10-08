//go:build js

package config

// Browser defaults: a smaller world, because the cost of a cycle tracks the
// POPULATION it carries and wasm runs the same loop several times slower
// than a native build.
//
// Measured natively over 6 seeds x 3000 cycles, cycles per second against
// the desktop grid: see TestGridPerf (GRID_PERF=1).
const (
	wasmGridWide = 60
	wasmGridHigh = 48
	// Against 0.01 on the desktop baseline.
	wasmChemoPhEffect = 0.025
)

// platformDefaults shrinks the world for the browser.
//
// Applied to the BASELINE only, so it is overridden the way any default is:
// a -config file that names a grid wins, and a replay takes its grid from
// the recording's header, which is what keeps a recording replayable.
func platformDefaults(g *Globals) {
	// The per-world counts scale with the area, or the smaller world starts
	// at several times the food density the numbers were chosen for. These
	// are item COUNTS, not densities.
	share := float64(wasmGridWide*wasmGridHigh) / float64(g.GridUnitsWide*g.GridUnitsHigh)
	g.InitialFood = int(float64(g.InitialFood) * share)
	g.InitialBuriedFood = int(float64(g.InitialBuriedFood) * share)
	g.InitialWalls = int(float64(g.InitialWalls) * share)

	g.GridUnitsWide, g.GridUnitsHigh = wasmGridWide, wasmGridHigh

	// A stronger chemosynthesis push, so the pH cycle is visible inside the
	// minute or two a browser visitor watches for.
	g.ChemoPhEffect = wasmChemoPhEffect
}
