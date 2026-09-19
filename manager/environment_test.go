package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/utils"
)

// stubEnvAPI is a minimal environment.API: no walls, and pH-update
// notifications are discarded. Enough to drive diffusion in isolation.
type stubEnvAPI struct{}

func (stubEnvAPI) Cycle() int                             { return 0 }
func (stubEnvAPI) AddPhUpdate(p utils.Point)              {}
func (stubEnvAPI) IsWallAtPoint(p utils.Point) bool       { return false }
func (stubEnvAPI) GetWallStrengthAtPoint(utils.Point) int { return 0 }

// loadDefaultGlobals wires settings/default.json into the process-wide
// config. Production reads these from an embedded FS via main's init;
// tests can't reach that, so decode straight from disk.
func loadDefaultGlobals(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	config.SetGlobals(&g)
}

// TestDiffusionIsSymmetric pins the core property of pH diffusion: with
// no walls, a spike spreads equally in all four directions.
//
// Asymmetry here would mean pH drifts across the map for no modelled
// reason, which is close to invisible by eye — a slow bias looks like
// ordinary simulation noise — but would quietly skew every lineage
// toward one edge of the world.
//
// The equality is exact, not approximate: the four neighbours are
// summed in a fixed order (neighbourOffsets) and divided once, so
// symmetric inputs give bit-identical outputs. If this ever needs a
// tolerance, something has started depending on iteration order.
func TestDiffusionIsSymmetric(t *testing.T) {
	loadDefaultGlobals(t)

	origin := utils.Point{X: 20, Y: 20}
	m := NewEnvironmentManager(stubEnvAPI{})
	m.currentPhMap[origin.X][origin.Y] = 10.0

	for i := 0; i < 12; i++ {
		m.Update()
	}

	east := m.currentPhMap[origin.X+4][origin.Y]
	west := m.currentPhMap[origin.X-4][origin.Y]
	north := m.currentPhMap[origin.X][origin.Y-4]
	south := m.currentPhMap[origin.X][origin.Y+4]

	for _, tc := range []struct {
		name string
		a, b float64
	}{
		{"east vs west", east, west},
		{"north vs south", north, south},
		{"east vs north", east, north},
	} {
		if tc.a != tc.b {
			t.Errorf("diffusion should be symmetric, %s: %g != %g", tc.name, tc.a, tc.b)
		}
	}
}

// TestDiffusionConservesPh checks that diffusion only moves pH around,
// never creates or destroys it. The step is a weighted average of
// neighbours, so total pH across the grid should hold steady; a drift
// would compound over the tens of thousands of cycles a real run takes
// and slowly acidify or alkalise the whole world on its own.
func TestDiffusionConservesPh(t *testing.T) {
	loadDefaultGlobals(t)

	m := NewEnvironmentManager(stubEnvAPI{})
	m.currentPhMap[20][20] = 9.0
	m.currentPhMap[5][40] = 1.0

	total := func() float64 {
		sum := 0.0
		for _, col := range m.currentPhMap {
			for _, v := range col {
				sum += v
			}
		}
		return sum
	}

	before := total()
	for i := 0; i < 200; i++ {
		m.Update()
	}
	after := total()

	// Floating-point summation over 8000 cells won't be bit-exact, so
	// allow a relative slack far tighter than any drift that would
	// matter over a real run.
	if rel := (after - before) / before; rel > 1e-9 || rel < -1e-9 {
		t.Errorf("diffusion changed total pH by %.3e relative (%v -> %v)", rel, before, after)
	}
}
