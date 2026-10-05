package manager

import (
	"math"
	"testing"

	d "github.com/Zebbeni/protozoa/decision"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// crowdWorld puts chemosynthesising organisms at the given points, all with
// the same scores, and resolves the claim counts for the cycle.
func crowdWorld(t *testing.T, penalty float64, points ...utils.Point) (*OrganismManager, []*organism.Organism) {
	t.Helper()
	loadDefaultGlobals(t)
	g := c.GetCurrentGlobals()
	g.ChemoCrowdingPenalty = penalty
	c.SetGlobals(g)

	scores := physiology.Scores{}
	scores[physiology.AbilityChemosynthesis] = physiology.MaxAbilityScore
	scores[physiology.AbilityTolerance] = physiology.PointTotal - physiology.MaxAbilityScore

	api := &phRecorder{ph: 5}
	m := &OrganismManager{api: api, organismIDGrid: initializeGrid(),
		organisms: map[int]*organism.Organism{}}
	var out []*organism.Organism
	for i, p := range points {
		o := organism.Restore(i+1, 1, 20, 20, 0, 0, 0, p, utils.Point{X: 1, Y: 0}, i+1,
			organism.Traits{IdealPh: 5, Abilities: scores, MaxSize: 100}, nil,
			d.ActChemosynthesis, organism.StatusIdle, 0, 0, 0, 0, nil)
		m.organisms[o.ID] = o
		m.organismIDGrid[p.X][p.Y] = o.ID
		m.organismIds = append(m.organismIds, o.ID)
		out = append(out, o)
	}
	m.buildChemoClaims()
	return m, out
}

// TestCrowdingIsSymmetric: two adjacent chemosynthesisers contest each other
// equally. Counting them in the resolve phase is what makes this hold — in
// the decide phase the answer would depend on which was reached first.
func TestCrowdingIsSymmetric(t *testing.T) {
	m, orgs := crowdWorld(t, 1,
		utils.Point{X: 10, Y: 10}, utils.Point{X: 11, Y: 10})
	a := m.chemoClaimsAround(orgs[0])
	b := m.chemoClaimsAround(orgs[1])

	sum := func(cl [5]int) int {
		total := 0
		for _, n := range cl {
			total += n
		}
		return total
	}
	if sum(a) != sum(b) {
		t.Errorf("neighbours contest each other unequally: %v against %v", a, b)
	}
	// Exactly TWO of the five are shared: each draws on the other's own
	// cell, and nothing else overlaps. The other three neighbours of each are
	// a step further apart than either reaches.
	if sum(a) != 5+2 {
		t.Errorf("claim counts are %v, summing %d; two adjacent organisms share two cells",
			a, sum(a))
	}
}

// TestCrowdingCutsTheGain is the point of the setting: an organism beside
// another chemosynthesiser gains less than one on its own.
func TestCrowdingCutsTheGain(t *testing.T) {
	alone, soloOrgs := crowdWorld(t, 1, utils.Point{X: 4, Y: 4})
	alone.applyChemosynthesis(soloOrgs[0])
	soloGain := soloOrgs[0].Health - 20

	pair, pairOrgs := crowdWorld(t, 1,
		utils.Point{X: 10, Y: 10}, utils.Point{X: 11, Y: 10})
	pair.applyChemosynthesis(pairOrgs[0])
	pairGain := pairOrgs[0].Health - 20

	if !(pairGain < soloGain) {
		t.Errorf("a crowded organism gained %v and a lone one %v; crowding should cost it",
			pairGain, soloGain)
	}
	// Two of five cells halved: four shares of five.
	if want := soloGain * (4.0 / 5); math.Abs(pairGain-want) > 1e-9 {
		t.Errorf("a crowded organism gained %v, want %v", pairGain, want)
	}
}

// TestCrowdingOffLeavesTheGainExactly: the setting at 0 has to reproduce the
// simulation that had no crowding in it, bit for bit.
func TestCrowdingOffLeavesTheGainExactly(t *testing.T) {
	off, offOrgs := crowdWorld(t, 0,
		utils.Point{X: 10, Y: 10}, utils.Point{X: 11, Y: 10})
	off.applyChemosynthesis(offOrgs[0])
	crowded := offOrgs[0].Health

	alone, soloOrgs := crowdWorld(t, 0, utils.Point{X: 4, Y: 4})
	alone.applyChemosynthesis(soloOrgs[0])

	if crowded != soloOrgs[0].Health {
		t.Errorf("with crowding off a neighbour still cost %v", soloOrgs[0].Health-crowded)
	}
	if off.chemoClaims != nil {
		t.Error("the claim grid was built with crowding off; it is per-cycle work for nothing")
	}
}

// TestADyingNeighbourDoesNotCrowd: a dying organism is frozen and takes no
// action, so it draws on nothing.
func TestADyingNeighbourDoesNotCrowd(t *testing.T) {
	m, orgs := crowdWorld(t, 1,
		utils.Point{X: 10, Y: 10}, utils.Point{X: 11, Y: 10})
	orgs[1].Status = organism.StatusDying
	m.buildChemoClaims()

	for _, n := range m.chemoClaimsAround(orgs[0]) {
		if n != 1 {
			t.Errorf("a dying neighbour still contests a cell: claims %v",
				m.chemoClaimsAround(orgs[0]))
			break
		}
	}
}
