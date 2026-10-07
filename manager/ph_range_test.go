package manager

import (
	"math"
	"testing"

	c "github.com/Zebbeni/protozoa/config"

	"github.com/Zebbeni/protozoa/utils"
)

func TestPhRangeTracksTheExtremes(t *testing.T) {
	loadDefaultGlobals(t)
	// Slow diffusion, which is what this test is about: it edits two cells
	// and reads the extremes back. At the shipped ph_diffuse_factor of 1 a
	// spike equalises with its neighbours before the scan ever sees it, so
	// the test would be measuring the diffusion rate rather than the range.
	g := c.GetCurrentGlobals()
	g.PhDiffuseFactor = 0.1
	c.SetGlobals(g)
	m := NewEnvironmentManager(stubEnvAPI{})
	m.Update()

	lo, hi := m.GetPhRange()
	if lo > m.GetAveragePh() || hi < m.GetAveragePh() {
		t.Fatalf("range %v-%v doesn't contain the average %v", lo, hi, m.GetAveragePh())
	}

	m.currentPhMap[1][1] = 0
	m.currentPhMap[5][5] = 10
	// The scan reads the previous map, so it takes an update to swap the edits into place and another to see them.
	m.Update()
	m.Update()

	lo, hi = m.GetPhRange()
	// Diffusion has already spread each spike a little by the time the scan sees it.
	if lo > 3 {
		t.Errorf("lowest pH %v after acidifying a cell to the floor, want it well below neutral", lo)
	}
	if hi < 7 {
		t.Errorf("highest pH %v after raising a cell to the ceiling, want it well above neutral", hi)
	}
	if math.Abs(m.GetAveragePh()-5) > 1 {
		t.Errorf("average %v moved far more than two edited cells should shift it", m.GetAveragePh())
	}
}

// wallStub is an organism.API reporting walls at the given points.
type wallEnvStub struct {
	walls map[utils.Point]bool
	// strengths overrides the per-wall strength, for tests that care how much a wall slows diffusion.
	strengths map[utils.Point]int
}

func (wallEnvStub) Cycle() int                         { return 0 }
func (wallEnvStub) AddPhUpdate(utils.Point)            {}
func (s wallEnvStub) IsWallAtPoint(p utils.Point) bool { return s.walls[p] }

func (s wallEnvStub) GetWallStrengthAtPoint(p utils.Point) int {
	if st, ok := s.strengths[p]; ok {
		return st
	}
	if s.walls[p] {
		return MaxWallStrength
	}
	return 0
}

func TestPhStatsIgnoreWalls(t *testing.T) {
	loadDefaultGlobals(t)
	walled := utils.Point{X: 3, Y: 3}
	m := NewEnvironmentManager(wallEnvStub{walls: map[utils.Point]bool{walled: true}})

	// A frozen extreme inside the wall, and a milder one in the water.
	m.currentPhMap[walled.X][walled.Y] = 0
	m.currentPhMap[20][20] = 8
	m.Update()
	m.Update()

	lo, hi := m.GetPhRange()
	if lo <= 0.5 {
		t.Errorf("lowest water pH %v, want the walled cell's 0 left out", lo)
	}
	if hi < 5 {
		t.Errorf("highest water pH %v, want the raised cell counted", hi)
	}
	avg := m.GetAveragePh()
	if avg < lo || avg > hi {
		t.Errorf("average %v outside the water's range %v-%v", avg, lo, hi)
	}
}
