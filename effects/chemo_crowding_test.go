package effects

import (
	"math"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

func crowdGlobals(t *testing.T, penalty float64) *config.Globals {
	t.Helper()
	g := digGlobals(t)
	g.ChemoCrowdingPenalty = penalty
	config.SetGlobals(g)
	return g
}

func claims(n ...int) [ChemoCrowdingCells]int {
	var out [ChemoCrowdingCells]int
	for i := range out {
		if i < len(n) {
			out[i] = n[i]
		} else {
			out[i] = 1
		}
	}
	return out
}

// TestAnUncontestedChemosynthesiserKeepsEverything: alone, every one of the
// five cells it draws on has exactly one claimant — itself.
func TestAnUncontestedChemosynthesiserKeepsEverything(t *testing.T) {
	for _, p := range []float64{0, 0.5, 1} {
		g := crowdGlobals(t, p)
		if got := ChemoCrowdingFactor(g, claims(1, 1, 1, 1, 1)); got != 1 {
			t.Errorf("penalty %.1f: an uncontested organism keeps %v, want all of it", p, got)
		}
	}
}

// TestAFullyBoxedChemosynthesiserKeepsAFifth: boxed in on four sides, every
// cell it draws on is drawn on by five organisms, so each yields a fifth of
// its share.
func TestAFullyBoxedChemosynthesiserKeepsAFifth(t *testing.T) {
	g := crowdGlobals(t, 1)
	if got := ChemoCrowdingFactor(g, claims(5, 5, 5, 5, 5)); math.Abs(got-0.2) > 1e-12 {
		t.Errorf("a fully contested organism keeps %v, want a fifth", got)
	}
}

// TestCrowdingSharesEachCellSeparately: the cells are divided one at a time,
// so an organism contested on one side only loses that side's share.
func TestCrowdingSharesEachCellSeparately(t *testing.T) {
	g := crowdGlobals(t, 1)
	// Its own cell and three neighbours to itself; one neighbour shared with
	// one other: 4 whole shares plus a half, out of five.
	want := (4 + 0.5) / 5
	if got := ChemoCrowdingFactor(g, claims(1, 2, 1, 1, 1)); math.Abs(got-want) > 1e-12 {
		t.Errorf("one contested cell of five gives %v, want %v", got, want)
	}
}

// TestCrowdingPenaltyZeroIsExactlyOff is the compatibility claim, and it has
// to be exact rather than close: the factor multiplies a gain, so anything
// but 1 changes every chemosynthesis in the world.
func TestCrowdingPenaltyZeroIsExactlyOff(t *testing.T) {
	g := crowdGlobals(t, 0)
	for _, cl := range [][ChemoCrowdingCells]int{
		claims(1, 1, 1, 1, 1), claims(5, 5, 5, 5, 5), claims(2, 3, 1, 4, 5),
	} {
		if got := ChemoCrowdingFactor(g, cl); got != 1 {
			t.Errorf("with the penalty off, claimants %v give %v, want exactly 1", cl, got)
		}
	}
}

// TestCrowdingPenaltyCarriesBetweenTheTwo: the setting is a strength, since
// the split itself has no knob in it.
func TestCrowdingPenaltyCarriesBetweenTheTwo(t *testing.T) {
	boxed := claims(5, 5, 5, 5, 5)
	full := ChemoCrowdingFactor(crowdGlobals(t, 1), boxed)
	half := ChemoCrowdingFactor(crowdGlobals(t, 0.5), boxed)
	if !(full < half && half < 1) {
		t.Errorf("half a penalty gives %v, want it between the full %v and 1", half, full)
	}
	if want := 1 - 0.5*(1-full); math.Abs(half-want) > 1e-12 {
		t.Errorf("half a penalty gives %v, want the midpoint %v", half, want)
	}
}

// TestCrowdingNeverInventsGain: a claimant count below 1 would mean a cell
// nobody draws on, which cannot happen — the organism itself always does —
// but a bad count must not multiply the gain UP.
func TestCrowdingNeverInventsGain(t *testing.T) {
	g := crowdGlobals(t, 1)
	if got := ChemoCrowdingFactor(g, claims(0, 0, 0, 0, 0)); got != 1 {
		t.Errorf("empty claimant counts give %v, want 1 rather than more", got)
	}
}

// TestCrowdingDoesNotSoftenAFailedAttempt: past the band an organism can
// feed in, the gain is negative — the attempt cost it health. That loss is
// its own wasted effort, not something drawn from the ground, so sharing it
// would make failing hurt LESS for being surrounded.
func TestCrowdingDoesNotSoftenAFailedAttempt(t *testing.T) {
	g := crowdGlobals(t, 1)
	boxed := claims(5, 5, 5, 5, 5)
	for _, loss := range []float64{-0.001, -1, -42} {
		if got := ChemoCrowdedGain(g, loss, boxed); got != loss {
			t.Errorf("a failed attempt costing %v cost %v when crowded; the cost is not shared",
				loss, got)
		}
	}
	// A gain is still shared.
	if got := ChemoCrowdedGain(g, 10, boxed); got >= 10 {
		t.Errorf("a crowded gain of 10 came out %v, want less", got)
	}
	// And zero stays zero rather than becoming a tiny something.
	if got := ChemoCrowdedGain(g, 0, boxed); got != 0 {
		t.Errorf("a zero gain became %v", got)
	}
}
