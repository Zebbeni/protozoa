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

func (stubEnvAPI) Cycle() int                      { return 0 }
func (stubEnvAPI) AddPhUpdate(p utils.Point)       {}
func (stubEnvAPI) IsWallAtPoint(p utils.Point) bool { return false }

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

// setUniformFlow points every cell's current in the same direction at
// full magnitude, bypassing CirculateFlowAtPoint's accumulate-and-clamp
// so the test controls the field exactly.
func setUniformFlow(m *EnvironmentManager, v utils.Vector) {
	for x := range m.flowMap {
		for y := range m.flowMap[x] {
			m.flowMap[x][y] = v
		}
	}
}

// TestFlowCarriesPhDownstream is the regression guard on the sign of
// the anisotropic diffusion weight. A pH spike sitting in an eastward
// current must spread further EAST than west: the weighting favours
// each cell's upstream neighbour, so every cell adopts more of what
// lies behind the flow, and the pattern therefore travels along the
// vector rather than against it.
//
// Getting the sign backwards still produces a plausible-looking,
// perfectly stable simulation — the pattern just drifts the wrong way
// — which is exactly why this is worth pinning down in a test.
func TestFlowCarriesPhDownstream(t *testing.T) {
	loadDefaultGlobals(t)

	const spikePh = 10.0
	origin := utils.Point{X: 20, Y: 20}

	m := NewEnvironmentManager(stubEnvAPI{})
	neutral := m.currentPhMap[origin.X][origin.Y]

	// East is +x. Full-magnitude current so the bias is unmistakable.
	setUniformFlow(m, utils.Vector{X: 1, Y: 0})
	m.currentPhMap[origin.X][origin.Y] = spikePh

	// decayFlow would erode the field as we step, so re-assert it each
	// cycle: this test is about the diffusion weighting, not decay.
	for i := 0; i < 12; i++ {
		setUniformFlow(m, utils.Vector{X: 1, Y: 0})
		m.Update()
	}

	east := m.currentPhMap[origin.X+4][origin.Y] - neutral
	west := m.currentPhMap[origin.X-4][origin.Y] - neutral
	north := m.currentPhMap[origin.X][origin.Y-4] - neutral

	if east <= west {
		t.Errorf("eastward current should carry pH east: east deviation %g, west %g "+
			"(sign of the flow weight in diffusePhLevels is likely inverted)", east, west)
	}
	// Cross-stream spread should sit between the two: the flow biases
	// the x axis only, leaving y at weight 1.
	if !(east > north && north > west) {
		t.Errorf("expected east > north > west, got east=%g north=%g west=%g", east, north, west)
	}
}

// TestZeroFlowDiffusesSymmetrically pins the other half of the
// contract: with a still field the weights all collapse to 1 and
// diffusion is exactly the isotropic mean it was before currents
// existed. Any asymmetry here means the weighting leaks into the
// no-current case.
func TestZeroFlowDiffusesSymmetrically(t *testing.T) {
	loadDefaultGlobals(t)

	const spikePh = 10.0
	origin := utils.Point{X: 20, Y: 20}

	m := NewEnvironmentManager(stubEnvAPI{})
	m.currentPhMap[origin.X][origin.Y] = spikePh

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
			t.Errorf("still water should diffuse symmetrically, %s: %g != %g", tc.name, tc.a, tc.b)
		}
	}
}

// TestCirculateAccumulatesAndClamps covers the write path: repeated
// pushes in one direction build toward full magnitude and saturate
// there, and an opposing push cancels rather than stacking.
func TestCirculateAccumulatesAndClamps(t *testing.T) {
	loadDefaultGlobals(t)

	p := utils.Point{X: 5, Y: 5}
	east := utils.Point{X: 1, Y: 0}
	west := utils.Point{X: -1, Y: 0}

	m := NewEnvironmentManager(stubEnvAPI{})

	if got := m.GetFlowAtPoint(p); !got.IsZero() {
		t.Fatalf("flow should start still, got %+v", got)
	}

	// Far more pushes than needed to reach magnitude 1, to prove the
	// clamp holds rather than letting the vector grow without bound.
	for i := 0; i < 50; i++ {
		m.CirculateFlowAtPoint(p, east, config.CirculateStrength())
	}
	got := m.GetFlowAtPoint(p)
	if l := got.Length(); l > 1.0000001 {
		t.Errorf("flow magnitude should clamp at 1, got %g", l)
	}
	if got.X <= 0 {
		t.Errorf("eastward pushes should give positive X, got %+v", got)
	}

	// An opposing push must reduce the magnitude, not add to it.
	before := m.GetFlowAtPoint(p).Length()
	m.CirculateFlowAtPoint(p, west, config.CirculateStrength())
	if after := m.GetFlowAtPoint(p).Length(); after >= before {
		t.Errorf("opposing push should cancel: %g -> %g", before, after)
	}
}

// TestFlowDecaysToStill verifies an un-tended current fades and snaps
// cleanly to zero, so IsZero stays a meaningful fast path instead of
// cells carrying vanishing residue forever.
func TestFlowDecaysToStill(t *testing.T) {
	loadDefaultGlobals(t)

	p := utils.Point{X: 5, Y: 5}
	m := NewEnvironmentManager(stubEnvAPI{})
	m.CirculateFlowAtPoint(p, utils.Point{X: 1, Y: 0}, 1.0)

	if m.GetFlowAtPoint(p).IsZero() {
		t.Fatal("flow should be non-zero after a full-strength push")
	}

	prev := m.GetFlowAtPoint(p).Length()
	for i := 0; i < 5; i++ {
		m.decayFlow()
		got := m.GetFlowAtPoint(p).Length()
		if got >= prev {
			t.Fatalf("decay should shrink the vector: %g -> %g", prev, got)
		}
		prev = got
	}

	// Enough cycles to cross flowRestThreshold from magnitude 1 at the
	// configured decay rate.
	for i := 0; i < 2000; i++ {
		m.decayFlow()
	}
	if got := m.GetFlowAtPoint(p); !got.IsZero() {
		t.Errorf("flow should snap to exactly still below the rest threshold, got %+v", got)
	}
}
