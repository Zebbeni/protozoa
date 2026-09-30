package manager

import (
	"math"
	"testing"

	c "github.com/Zebbeni/protozoa/config"

	"github.com/Zebbeni/protozoa/utils"
)

// permTestEnv walls off a one-cell-wide channel and puts a pH spike in it, returning the manager and the probe point just outside.
func permTestEnv(t *testing.T, strength int) (*EnvironmentManager, utils.Point, utils.Point) {
	t.Helper()
	loadDefaultGlobals(t)

	const left, mid, right = 5, 6, 7
	stub := wallEnvStub{
		walls:     map[utils.Point]bool{},
		strengths: map[utils.Point]int{},
	}
	if strength > 0 {
		for y := 0; y < c.GridUnitsHigh(); y++ {
			for _, x := range []int{left, right} {
				p := utils.Point{X: x, Y: y}
				stub.walls[p] = true
				stub.strengths[p] = strength
			}
		}
	}
	m := NewEnvironmentManager(stub)

	spike := utils.Point{X: mid, Y: 10}
	probe := utils.Point{X: right + 1, Y: 10}
	m.setPhAtPoint(spike, 10)
	return m, spike, probe
}

// phAcross runs the world forward and reports how far the probe outside the channel has moved from neutral.
func phAcross(m *EnvironmentManager, probe utils.Point, cycles int) float64 {
	for i := 0; i < cycles; i++ {
		m.Update()
	}
	return math.Abs(m.GetPhAtPoint(probe) - 5)
}

func TestWeakWallsBarelySlowDiffusion(t *testing.T) {
	open, _, openProbe := permTestEnv(t, 0)
	weak, _, weakProbe := permTestEnv(t, 1)

	gotOpen := phAcross(open, openProbe, 40)
	gotWeak := phAcross(weak, weakProbe, 40)

	if gotOpen <= 0 {
		t.Fatalf("pH didn't cross open water at all (%v); the test can't measure a wall", gotOpen)
	}
	if gotWeak <= 0 {
		t.Errorf("nothing crossed a strength-1 wall (%v); it should be nearly water", gotWeak)
	}
	if gotWeak > gotOpen {
		t.Errorf("a wall passed more than open water: %v against %v", gotWeak, gotOpen)
	}
	// "Nearly water" means most of it, not a trace.
	if gotWeak < gotOpen*0.5 {
		t.Errorf("a strength-1 wall passed only %v of open water's %v", gotWeak, gotOpen)
	}
}

func TestDiffusionFallsAsWallsStrengthen(t *testing.T) {
	var first, last float64
	prev := math.Inf(1)
	for i, strength := range []int{1, 25, 50, 75, 100} {
		m, _, probe := permTestEnv(t, strength)
		got := phAcross(m, probe, 40)
		if got > prev {
			t.Errorf("strength %d passed %v, more than the weaker wall's %v", strength, got, prev)
		}
		if i == 0 {
			first = got
		}
		last = got
		prev = got
	}
	// Strictly less at the top than the bottom, not merely no greater.
	if !(last < first) {
		t.Errorf("a full-strength wall passed %v against a flimsy one's %v; strength makes no difference", last, first)
	}
}

func TestFullStrengthWallsStillSeal(t *testing.T) {
	m, spike, probe := permTestEnv(t, MaxWallStrength)
	wall := utils.Point{X: spike.X + 1, Y: spike.Y}

	before := m.GetPhAtPoint(wall)
	got := phAcross(m, probe, 60)

	if math.Abs(got) > 1e-9 {
		t.Errorf("pH crossed a full-strength wall by %v; it should seal completely", got)
	}
	if after := m.GetPhAtPoint(wall); math.Abs(after-before) > 1e-9 {
		t.Errorf("a sealed wall's own pH moved from %v to %v", before, after)
	}
}

func TestWallPhStillOutOfTheWaterStats(t *testing.T) {
	loadDefaultGlobals(t)
	wall := utils.Point{X: 5, Y: 5}
	stub := wallEnvStub{
		walls:     map[utils.Point]bool{wall: true},
		strengths: map[utils.Point]int{wall: 1}, // as permeable as a wall gets
	}
	m := NewEnvironmentManager(stub)
	m.setPhAtPoint(wall, 0)

	m.Update()
	m.Update()

	if lo, _ := m.GetPhRange(); lo < 1 {
		t.Errorf("lowest pH %v — the wall's own value reached the water stats", lo)
	}
}

// phAcrossTuned builds a walled world with the two wall-blocking settings installed and measures what crosses it.
func phAcrossTuned(t *testing.T, strength int, atMax, curve float64, cycles int) float64 {
	t.Helper()
	m, _, probe := permTestEnv(t, strength)
	g := c.GetCurrentGlobals()
	g.WallPhBlockAtMax = atMax
	g.WallPhBlockCurve = curve
	c.SetGlobals(g)
	return phAcross(m, probe, cycles)
}

func TestWallBlockDefaultsKeepTheLinearRamp(t *testing.T) {
	loadDefaultGlobals(t)
	g := c.GetCurrentGlobals()
	if g.WallPhBlockAtMax != 1 || g.WallPhBlockCurve != 1 {
		t.Fatalf("shipped defaults are %v / %v, want 1 / 1", g.WallPhBlockAtMax, g.WallPhBlockCurve)
	}

	for _, strength := range []int{1, 25, 50, 75, 100} {
		got := phAcrossTuned(t, strength, 1, 1, 40)

		plain, _, plainProbe := permTestEnv(t, strength)
		want := phAcross(plain, plainProbe, 40)

		if got != want {
			t.Errorf("strength %d: %v with the settings at 1/1, %v with the shipped defaults", strength, got, want)
		}
	}
}

// TestWallBlockAtMaxOpensWalls: the setting's whole job is to say how much of a barrier a full-strength wall is.
func TestWallBlockAtMaxOpensWalls(t *testing.T) {
	open, _, openProbe := permTestEnv(t, 0)
	want := phAcross(open, openProbe, 40)

	if got := phAcrossTuned(t, MaxWallStrength, 0, 1, 40); got != want {
		t.Errorf("at block 0 a full-strength wall passed %v, open water passes %v", got, want)
	}

	mid := phAcrossTuned(t, MaxWallStrength, 0.5, 1, 40)
	if !(mid > 0 && mid < want) {
		t.Errorf("at block 0.5 a full-strength wall passed %v; want between 0 and open water's %v", mid, want)
	}
}

func TestWallBlockCurveShapesTheRamp(t *testing.T) {
	const half = MaxWallStrength / 2

	lo := phAcrossTuned(t, half, 1, 0.5, 40)
	mid := phAcrossTuned(t, half, 1, 1, 40)
	hi := phAcrossTuned(t, half, 1, 2, 40)

	// A curve above 1 pushes the blocking toward full strength, so a half-strength wall blocks LESS and passes more.
	if !(lo < mid && mid < hi) {
		t.Errorf("half-strength wall passed %v / %v / %v at curve 0.5 / 1 / 2; want strictly increasing", lo, mid, hi)
	}
}

func TestWallBlockCurveZeroRestoresAbsoluteBarriers(t *testing.T) {
	for _, strength := range []int{1, 50, MaxWallStrength} {
		if got := phAcrossTuned(t, strength, 1, 0, 60); math.Abs(got) > 1e-9 {
			t.Errorf("at curve 0 a strength-%d wall leaked %v; every wall should seal", strength, got)
		}
	}
}
