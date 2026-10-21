package config

// A world size is a grid and the per-world item stocks that go with it. The
// three are one world at three scales: each dimension steps by 1.25, so the
// area steps by 1.5625, and the stocks step with the area — a smaller world
// has to start with proportionally less in it or it starts denser than the
// numbers were chosen for. Every figure below is a whole number because of
// that ratio, so no size is a rounded approximation of another.
//
// MEDIUM IS THE SHIPPED DEFAULT. settings/default.json holds exactly these
// four values, and TestTheMediumWorldIsTheShippedDefault enforces it, because
// a second set of numbers that drifted from the file would be invisible: the
// screen would offer a MEDIUM button that did not restore what the file ships.
//
// This file is untagged so the world a BROWSER visitor gets can be measured
// from a native test rather than only by building for wasm; platform_js.go is
// the one line that applies it.
type WorldSize int

const (
	WorldSmall WorldSize = iota
	WorldMedium
	WorldLarge
)

// AllWorldSizes is every size, smallest first, and is the canonical list: the
// buttons, the lookup and the tests all walk it.
var AllWorldSizes = []WorldSize{WorldSmall, WorldMedium, WorldLarge}

// worldSizeSpec is what a size sets. Deliberately NOT initial_walls: the world
// ships wall-free and every wall in it is one an organism dug, so there is no
// stock to scale, and a preset that silently rewrote a terrain count would be
// doing more than its label says.
type worldSizeSpec struct {
	name       string
	wide, high int
	food       int
	buriedFood int
}

var worldSizes = map[WorldSize]worldSizeSpec{
	WorldSmall:  {"SMALL", 80, 64, 6400, 6400},
	WorldMedium: {"MEDIUM", 100, 80, 10000, 10000},
	WorldLarge:  {"LARGE", 125, 100, 15625, 15625},
}

// BrowserWorldSize is what a browser visitor gets. Small, because the cost of
// a cycle tracks the POPULATION a grid carries and wasm runs the same loop
// several times slower than a native build.
//
// It is a size rather than its own set of numbers so that the browser runs a
// world the desktop can also be put into — one button on the settings screen —
// instead of a configuration only reachable by building for wasm.
const BrowserWorldSize = WorldSmall

// Name is the label the settings screen puts on this size's button.
func (s WorldSize) Name() string { return worldSizes[s].name }

// Grid is the size's grid in cells.
func (s WorldSize) Grid() (wide, high int) {
	w := worldSizes[s]
	return w.wide, w.high
}

// Cells is how many grid cells the size holds.
func (s WorldSize) Cells() int {
	w := worldSizes[s]
	return w.wide * w.high
}

// ApplyWorldSize sets the grid and the two food stocks from the size.
//
// It writes fixed values rather than scaling whatever is already there, so
// clicking through the buttons cannot compound: small then large then small
// lands back on small exactly, where a relative scaling would drift by the
// rounding at each step.
func ApplyWorldSize(g *Globals, size WorldSize) {
	w, ok := worldSizes[size]
	if !ok {
		return
	}
	g.GridUnitsWide, g.GridUnitsHigh = w.wide, w.high
	g.InitialFood = w.food
	g.InitialBuriedFood = w.buriedFood
}

// WorldSizeOf reports which size a configuration is currently on, and false
// when it matches none of them — an edited grid or food stock is a custom
// world, and the buttons say so rather than lighting the nearest one.
func WorldSizeOf(g *Globals) (WorldSize, bool) {
	for _, size := range AllWorldSizes {
		w := worldSizes[size]
		if g.GridUnitsWide == w.wide && g.GridUnitsHigh == w.high &&
			g.InitialFood == w.food && g.InitialBuriedFood == w.buriedFood {
			return size, true
		}
	}
	return WorldMedium, false
}

// BrowserDefaults puts a configuration into the browser's world size.
//
// Applied to the BASELINE only, so it is overridden the way any default is: a
// -config file that names a grid wins, and a replay takes its grid from the
// recording's header, which is what keeps a recording replayable.
func BrowserDefaults(g *Globals) { ApplyWorldSize(g, BrowserWorldSize) }

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
