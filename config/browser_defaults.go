package config

// Browser defaults: a smaller world, because the cost of a cycle tracks the
// POPULATION it carries and wasm runs the same loop several times slower than
// a native build.
//
// These live in an untagged file so the browser's configuration can be
// measured from a native test (simulation's WASM_DEFAULTS probe) rather than
// only by building for wasm. platform_js.go is the one line that applies them.
const (
	wasmGridWide = 60
	wasmGridHigh = 48
)

// BrowserDefaults shrinks the world for the browser.
//
// Applied to the BASELINE only, so it is overridden the way any default is: a
// -config file that names a grid wins, and a replay takes its grid from the
// recording's header, which is what keeps a recording replayable.
func BrowserDefaults(g *Globals) {
	// The per-world counts scale with the area, or the smaller world starts
	// at several times the food density the numbers were chosen for. These
	// are item COUNTS, not densities.
	share := float64(wasmGridWide*wasmGridHigh) / float64(g.GridUnitsWide*g.GridUnitsHigh)
	g.InitialFood = int(float64(g.InitialFood) * share)
	g.InitialBuriedFood = int(float64(g.InitialBuriedFood) * share)
	g.InitialWalls = int(float64(g.InitialWalls) * share)

	g.GridUnitsWide, g.GridUnitsHigh = wasmGridWide, wasmGridHigh
}

// The browser used to raise chemo_ph_effect from 0.01 to 0.025 as well, so the
// pH cycle would show inside the minute or two a visitor watches for. It was
// removed when the defaults became a neural world: the push is the gain times
// the setting times the curve, and max_chemosynthesis_gain went 0.1 -> 0.5 in
// the same adoption, so the browser was pushing about twelve times what the
// 0.025 was chosen against.
//
// Measured over 6 seeds x 4,000 cycles (WASM_DEFAULTS=1), all three arms
// surviving 6/6: with the raised push the browser world held a mean of 130
// organisms, 5% occupancy, against 1,272 and 44% on the baseline push, where
// the desktop grid holds 2,418 at 30%. Both browser arms ended at pH 1.5, so
// the stronger push bought no more pH movement and cost nine tenths of the
// population — a world a visitor opens on and finds empty.
