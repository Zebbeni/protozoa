package manager

import (
	"math"
	"testing"

	c "github.com/Zebbeni/protozoa/config"

	"github.com/Zebbeni/protozoa/utils"
)

// permTestEnv walls off a one-cell-wide channel and puts a pH spike in
// it, returning the manager and the probe point just outside.
//
// Two wall columns, not one: the world is a torus, so a single barrier
// leaves the way round still open and pH reaches the far side without
// ever going through a wall. An earlier version of this test measured
// exactly that and reported a sealed wall leaking.
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

// phAcross runs the world forward and reports how far the probe outside
// the channel has moved from neutral.
func phAcross(m *EnvironmentManager, probe utils.Point, cycles int) float64 {
	for i := 0; i < cycles; i++ {
		m.Update()
	}
	return math.Abs(m.GetPhAtPoint(probe) - 5)
}

// TestWeakWallsBarelySlowDiffusion is the change: a wall's strength sets
// how much it impedes pH rather than every wall being an absolute
// barrier. A flimsy wall should pass nearly as much as open water.
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

// TestDiffusionFallsAsWallsStrengthen: the whole point is that wearing a
// wall down loosens it gradually. Before this, digging a wall from 100 to
// 1 changed nothing at all until the last point came off.
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
	// Strictly less at the top than the bottom, not merely no greater:
	// the old behaviour blocked every wall equally, which a
	// non-increasing check is satisfied by.
	if !(last < first) {
		t.Errorf("a full-strength wall passed %v against a flimsy one's %v; strength makes no difference", last, first)
	}
}

// TestFullStrengthWallsStillSeal: the old behaviour has to survive at the
// top of the range, or walls stop being walls.
func TestFullStrengthWallsStillSeal(t *testing.T) {
	m, spike, probe := permTestEnv(t, MaxWallStrength)
	wall := utils.Point{X: spike.X + 1, Y: spike.Y}

	before := m.GetPhAtPoint(wall)
	got := phAcross(m, probe, 60)

	if math.Abs(got) > 1e-9 {
		t.Errorf("pH crossed a full-strength wall by %v; it should seal completely", got)
	}
	// And its own pH is held: a sealed wall keeps whatever it was built in.
	if after := m.GetPhAtPoint(wall); math.Abs(after-before) > 1e-9 {
		t.Errorf("a sealed wall's own pH moved from %v to %v", before, after)
	}
}

// TestWallPhStillOutOfTheWaterStats: permeable or not, organisms can't be
// in a wall, so counting one would report pH nothing lives in. This is
// deliberately unchanged by the permeability work.
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
