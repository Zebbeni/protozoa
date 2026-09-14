package physiology

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

// The curve endpoints and mutation rates live in config; production
// loads them from an embedded FS via main's init, which tests can't
// reach, so decode settings/default.json from disk.
func loadGlobals(t *testing.T) {
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

func TestGenesisScoresValid(t *testing.T) {
	loadGlobals(t)

	s := GenesisScores()
	if err := s.Validate(); err != nil {
		t.Fatalf("genesis scores invalid: %v", err)
	}
	// Chemosynthesis takes the whole remainder, so genesis organisms
	// read as pure chemosynthesizers rather than generalists.
	for _, a := range AllAbilities {
		if a == AbilityChemosynthesis {
			continue
		}
		if s[a] >= s[AbilityChemosynthesis] {
			t.Errorf("%s (%d) should start below chemosynthesis (%d)",
				a.Name(), s[a], s[AbilityChemosynthesis])
		}
	}
	t.Logf("genesis: %v (total %d)", s, s.Total())
}

// TestGenesisOrganismIsNeutral is the load-bearing property: a genesis
// organism must sit at 1.0 in every ability, because that is exactly
// how the pre-scores featureless organism behaved.
//
// Pivoting the curves on the even split instead broke this — genesis is
// deliberately lopsided, so five of six abilities landed below neutral
// and the population collapsed at cycle 2438 on a seed that had
// previously run indefinitely. Pinning it here means any future change
// to the genesis distribution or the curve shape has to keep the
// starting organism balanced, or say loudly that it isn't.
func TestGenesisOrganismIsNeutral(t *testing.T) {
	loadGlobals(t)

	g := GenesisScores()
	for _, a := range AllAbilities {
		got := g.Multiplier(a)
		if math.Abs(got-1.0) > 1e-9 {
			t.Errorf("%s multiplier at genesis score %d = %v, want 1.0", a.Name(), g[a], got)
		}
	}
}

// TestMultiplierPivotsAtNeutral checks the pivot directly for each
// ability, independently of what the genesis distribution happens to be.
func TestMultiplierPivotsAtNeutral(t *testing.T) {
	loadGlobals(t)

	for _, a := range AllAbilities {
		got := multiplierFor(a, neutralScore(a))
		if math.Abs(got-1.0) > 1e-9 {
			t.Errorf("%s multiplier at neutral score = %v, want 1.0", a.Name(), got)
		}
	}
}

// TestMultiplierEndpointsAndMonotonicity checks each curve actually
// reaches its configured endpoints and moves in one direction the whole
// way — a non-monotonic curve would mean some middling score beat both
// extremes, which is never the intent.
func TestMultiplierEndpointsAndMonotonicity(t *testing.T) {
	loadGlobals(t)

	for _, a := range AllAbilities {
		atZero, atMax := curveEndpoints(a)
		if got := multiplierFor(a, 0); math.Abs(got-atZero) > 1e-9 {
			t.Errorf("%s multiplier at 0 = %v, want %v", a.Name(), got, atZero)
		}
		// atMax lands a fixed span above the ability's own neutral, not
		// at the 100-point budget — that's what makes specialising cost
		// the same wherever an ability starts.
		full := neutralScore(a) + float64(config.AbilitySpecializationSpan())
		if got := multiplierFor(a, full); math.Abs(got-atMax) > 1e-9 {
			t.Errorf("%s multiplier at %v = %v, want %v", a.Name(), full, got, atMax)
		}

		rising := atMax > atZero
		prev := multiplierFor(a, 0)
		for score := 1; score <= PointTotal; score++ {
			got := multiplierFor(a, float64(score))
			if rising && got < prev {
				t.Errorf("%s curve should rise but fell at score %d (%v -> %v)",
					a.Name(), score, prev, got)
			}
			if !rising && got > prev {
				t.Errorf("%s curve should fall but rose at score %d (%v -> %v)",
					a.Name(), score, prev, got)
			}
			prev = got
		}
	}
}

// TestCostAbilitiesInvert pins the direction of the cost-style curves.
// Movement is a cost multiplier, so a high score must make moving
// CHEAPER; wiring its endpoints the same way round as chemosynthesis
// would make specialising in movement actively harmful, and nothing
// else in the code would complain.
func TestCostAbilitiesInvert(t *testing.T) {
	loadGlobals(t)

	if multiplierFor(AbilityMovement, 0) <= multiplierFor(AbilityMovement, PointTotal) {
		t.Error("movement is a COST multiplier: score 0 must cost more than score 100")
	}
	if multiplierFor(AbilityDefense, 0) <= multiplierFor(AbilityDefense, PointTotal) {
		t.Error("defense is a DAMAGE-TAKEN multiplier: score 0 must take more than score 100")
	}
	// Effect-style abilities go the other way.
	for _, a := range []Ability{AbilityChemosynthesis, AbilityEating, AbilityDigging, AbilityAttack} {
		if multiplierFor(a, 0) >= multiplierFor(a, PointTotal) {
			t.Errorf("%s is an EFFECT multiplier: score 100 must beat score 0", a.Name())
		}
	}
}

// TestMutationPreservesTotal is the invariant guard. Mutation moves
// points between entries and must never mint or destroy them; a
// lineage that drifted off PointTotal would quietly out-compete
// everything else for reasons no config value explains.
func TestMutationPreservesTotal(t *testing.T) {
	loadGlobals(t)

	rng := simrand.New(1234)
	s := GenesisScores()
	for i := 0; i < 200000; i++ {
		s = s.Mutated(rng)
		if err := s.Validate(); err != nil {
			t.Fatalf("invariant broken after %d mutations: %v", i, err)
		}
	}
	t.Logf("after 200k mutations: %v (total %d)", s, s.Total())
}

// TestMutationReachesSpecialisation confirms the budget can actually be
// concentrated — if the transfer rule could not move a lineage far from
// its start, the scores would be decoration rather than a strategy
// space.
func TestMutationReachesSpecialisation(t *testing.T) {
	loadGlobals(t)

	rng := simrand.New(99)
	s := GenesisScores()
	peak := 0
	for i := 0; i < 100000; i++ {
		s = s.Mutated(rng)
		for _, a := range AllAbilities {
			if a != AbilityChemosynthesis && s[a] > peak {
				peak = s[a]
			}
		}
	}
	if peak < 50 {
		t.Errorf("no non-chemo ability ever exceeded %d points; drift is too weak "+
			"to produce specialists", peak)
	}
	t.Logf("peak non-chemo score reached: %d", peak)
}

// TestMutationIsDeterministic guards replay: the same seed and the same
// starting scores must produce an identical mutation sequence. Map
// iteration or a reject-and-retry picker would break this silently.
func TestMutationIsDeterministic(t *testing.T) {
	loadGlobals(t)

	run := func() []Scores {
		rng := simrand.New(7)
		s := GenesisScores()
		out := make([]Scores, 0, 500)
		for i := 0; i < 500; i++ {
			s = s.Mutated(rng)
			out = append(out, s)
		}
		return out
	}

	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("mutation diverged at step %d: %v vs %v", i, a[i], b[i])
		}
	}
}

// TestSpecializingCostsTheSameEverywhere is the guard on the fitness
// valley that kept every lineage chemosynthetic.
//
// Moving N points into any ability must buy the same fraction of that
// ability's payoff, whichever ability it is. When the top of the curve
// was anchored at the 100-point budget instead, growing a non-chemo
// ability took 90 points while abandoning it took 10 — so the first
// points a lineage invested bought less than they cost, and selection
// removed every organism part-way across.
func TestSpecializingCostsTheSameEverywhere(t *testing.T) {
	loadGlobals(t)

	span := float64(config.AbilitySpecializationSpan())
	const step = 10.0

	fractions := make(map[Ability]float64, len(AllAbilities))
	for _, a := range AllAbilities {
		_, atMax := curveEndpoints(a)
		neutral := neutralScore(a)
		got := multiplierFor(a, neutral+step)
		// How far the multiplier travelled toward its endpoint.
		fractions[a] = (got - 1.0) / (atMax - 1.0)
	}

	want := step / span
	for a, got := range fractions {
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("%s: %v points past neutral moved %.3f of the way to full payoff, want %.3f",
				a.Name(), step, got, want)
		}
	}
}
