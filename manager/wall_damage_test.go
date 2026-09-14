package manager

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// Representative sizes for each bracket (thirds of MaximumMaxSize).
func smallSize() float64 { return 1 }
func largeSize() float64 { return config.MaximumMaxSize() }

// scoresWithAbility builds a valid distribution with ability a at score,
// taking the difference out of chemosynthesis.
func scoresWithAbility(t *testing.T, a physiology.Ability, score int) physiology.Scores {
	t.Helper()
	s := physiology.GenesisScores()
	s[physiology.AbilityChemosynthesis] -= score - s[a]
	s[a] = score
	if err := s.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}
	return s
}

// hitsToDestroy hits a wall of the given strength until it breaks, with
// rolls from a seeded RNG, and returns the number of hits taken.
func hitsToDestroy(size float64, scores physiology.Scores, a physiology.Ability, strength int, rng *simrand.RNG) int {
	hits := 0
	for strength > 0 && hits < 1000 {
		strength -= wallDamage(size, scores, a, rng.Float64())
		hits++
	}
	return hits
}

// TestWallBreakCalibration pins the two reference points the mechanic is
// tuned to: a large organism with 10 Digging or Attack, and a small one
// with 50, each need exactly 7 hits to destroy a strength-7 wall. Exact,
// not on average: at the calibration points every hit removes one point,
// with nothing left over to roll for.
func TestWallBreakCalibration(t *testing.T) {
	loadDefaultGlobals(t)

	for _, a := range []physiology.Ability{physiology.AbilityDigging, physiology.AbilityAttack} {
		for _, tc := range []struct {
			name  string
			size  float64
			score int
		}{
			{"large with 10", largeSize(), 10},
			{"small with 50", smallSize(), 50},
		} {
			for seed := uint64(1); seed <= 20; seed++ {
				got := hitsToDestroy(tc.size, scoresWithAbility(t, a, tc.score), a, MaxWallStrength, simrand.New(seed))
				if got != 7 {
					t.Errorf("%v, %s: %d hits to destroy a strength-%d wall (seed %d), want 7",
						a, tc.name, got, MaxWallStrength, seed)
					break
				}
			}
		}
	}
}

// TestWallDamageFollowsItsInputs pins each input's direction: bigger
// organisms and higher scores break walls in fewer hits.
func TestWallDamageFollowsItsInputs(t *testing.T) {
	loadDefaultGlobals(t)
	a := physiology.AbilityAttack
	avgHits := func(size float64, score int) float64 {
		total := 0
		for seed := uint64(1); seed <= 200; seed++ {
			total += hitsToDestroy(size, scoresWithAbility(t, a, score), a, MaxWallStrength, simrand.New(seed))
		}
		return float64(total) / 200
	}

	if small, large := avgHits(smallSize(), 10), avgHits(largeSize(), 10); large >= small {
		t.Errorf("a large organism should need fewer hits than a small one: large %.1f, small %.1f", large, small)
	}
	if low, high := avgHits(smallSize(), 10), avgHits(smallSize(), 50); high >= low {
		t.Errorf("a higher score should need fewer hits: score 50 %.1f, score 10 %.1f", high, low)
	}
}

// TestWeakHittersStillWearWallsDown: a small organism with no points in
// the ability does well under a point per hit. Rounding would make it
// unable to ever break a wall; the roll must land a point often enough
// that the average matches the formula.
func TestWeakHittersStillWearWallsDown(t *testing.T) {
	loadDefaultGlobals(t)
	a := physiology.AbilityDigging
	scores := scoresWithAbility(t, a, 0)

	want := physiology.MultiplierAt(a, 0) / physiology.MultiplierAt(a, config.WallBreakScoreSmall())
	if want >= 1 {
		t.Fatalf("test assumes a fractional damage, got %.3f", want)
	}
	const n = 2000
	total := 0
	for i := 0; i < n; i++ {
		total += wallDamage(smallSize(), scores, a, (float64(i)+0.5)/n)
	}
	if got := float64(total) / n; got < want-0.01 || got > want+0.01 {
		t.Errorf("average damage %.3f, want %.3f", got, want)
	}
}

// wallStub is an organism.API that only knows about walls. Any other call
// panics via the nil embedded interface, which flags the test reaching
// further than intended.
type wallStub struct {
	organism.API
	walls   map[utils.Point]int
	updated []utils.Point
}

func (s *wallStub) GetWallStrengthAtPoint(p utils.Point) int { return s.walls[p] }
func (s *wallStub) AddWallStrength(p utils.Point, delta int) int {
	s.walls[p] = max(0, s.walls[p]+delta)
	return s.walls[p]
}
func (s *wallStub) AddWallUpdate(p utils.Point) { s.updated = append(s.updated, p) }

// TestDamageWallAheadHitsOnlyTheCellAhead wires the formula to the grid:
// the wall in front loses strength and is marked for redraw, and nothing
// else is touched.
func TestDamageWallAheadHitsOnlyTheCellAhead(t *testing.T) {
	loadDefaultGlobals(t)

	o := &organism.Organism{Location: utils.Point{X: 10, Y: 10}, Direction: utils.Point{X: 1, Y: 0}, Size: largeSize()}
	ahead := utils.Point{X: 11, Y: 10}
	behind := utils.Point{X: 9, Y: 10}
	stub := &wallStub{walls: map[utils.Point]int{ahead: 3, behind: 3}}
	m := &OrganismManager{api: stub, rng: simrand.New(1)}

	m.damageWallAhead(o, physiology.AbilityAttack)

	if stub.walls[ahead] != 2 {
		t.Errorf("wall ahead strength %d, want 2 after one calibrated hit", stub.walls[ahead])
	}
	if stub.walls[behind] != 3 {
		t.Error("the wall behind the organism was touched")
	}
	if len(stub.updated) != 1 || stub.updated[0] != ahead {
		t.Errorf("expected one redraw at %v, got %v", ahead, stub.updated)
	}

	stub.walls = map[utils.Point]int{}
	stub.updated = nil
	m.damageWallAhead(o, physiology.AbilityAttack)
	if len(stub.updated) != 0 {
		t.Errorf("no wall ahead should request no redraw, got %v", stub.updated)
	}
}
