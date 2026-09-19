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

func TestBalancedScoresValid(t *testing.T) {
	s := BalancedScores()
	if err := s.Validate(); err != nil {
		t.Fatalf("balanced scores invalid: %v", err)
	}
	for _, a := range AllAbilities {
		if s[a] < PointTotal/len(AllAbilities) || s[a] > PointTotal/len(AllAbilities)+1 {
			t.Errorf("%s = %d, not an even split", a.Name(), s[a])
		}
	}
}

// TestCurvesRunBetweenZeroAndOne: effect curves go from 0 at score 0 to 1
// at score 100, and cost curves the other way round.
//
// Each curve is checked under a real shape rather than whatever the
// settings happen to name, because a curve configured ShapeFlat is 1
// everywhere on purpose — that is how a curve says the score shouldn't
// scale it at all. What is being pinned here is the shapes, not the
// configuration, so the configuration is set aside.
func TestCurvesRunBetweenZeroAndOne(t *testing.T) {
	loadGlobals(t)
	base := *config.GetCurrentGlobals()
	for _, id := range AllCurves {
		g := base
		setCurveShape(&g, id, ShapeLinear)
		cv := CurveFor(&g, id)
		atZero, atMax := 0.0, 1.0
		if id.CostStyle() {
			atZero, atMax = 1, 0
		}
		if got := cv.At(0); got != atZero {
			t.Errorf("%s at 0 = %v, want %v", id.Name(), got, atZero)
		}
		if got := cv.At(MaxAbilityScore); got != atMax {
			t.Errorf("%s at %d = %v, want %v", id.Name(), MaxAbilityScore, got, atMax)
		}
	}
}

// TestCostCurvesFallEffectCurvesRise pins the direction for any K: a
// better score always makes a cost cheaper and an effect stronger.
func TestCostCurvesFallEffectCurvesRise(t *testing.T) {
	loadGlobals(t)
	for _, k := range []float64{0, 0.5, 1} {
		g := *config.GetCurrentGlobals()
		for _, id := range AllCurves {
			setCurveShape(&g, id, ShapeCosine)
			setCurveK(&g, id, k)
		}
		for _, id := range AllCurves {
			cv := CurveFor(&g, id)
			prev := cv.At(0)
			for score := 1; score <= MaxAbilityScore; score++ {
				got := cv.At(float64(score))
				if id.CostStyle() && got > prev+1e-12 {
					t.Fatalf("%s (K %.1f) is a cost curve but rose at score %d", id.Name(), k, score)
				}
				if !id.CostStyle() && got < prev-1e-12 {
					t.Fatalf("%s (K %.1f) is an effect curve but fell at score %d", id.Name(), k, score)
				}
				prev = got
			}
		}
	}
}

// setCurveK sets curve id's cosine K in g (test helper).
func setCurveK(g *config.Globals, id CurveID, k float64) {
	switch id {
	case CurveChemosynthesis:
		g.ChemosynthesisCosineK = k
	case CurveEating:
		g.EatingCosineK = k
	case CurveMovementCost:
		g.MovementCostCosineK = k
	case CurveDiggingCost:
		g.DiggingCostCosineK = k
	case CurveDiggingStrength:
		g.DiggingStrengthCosineK = k
	case CurveAttack:
		g.AttackCosineK = k
	case CurveDamageTaken:
		g.DamageTakenCosineK = k
	case CurveThorns:
		g.ThornsCosineK = k
	case CurvePhTolerance:
		g.PhToleranceCosineK = k
	}
}

// setCurveShape sets curve id's shape in g (test helper).
func setCurveShape(g *config.Globals, id CurveID, kind ShapeKind) {
	name := string(kind)
	switch id {
	case CurveChemosynthesis:
		g.ChemosynthesisCurveShape = name
	case CurveEating:
		g.EatingCurveShape = name
	case CurveMovementCost:
		g.MovementCostCurveShape = name
	case CurveDiggingCost:
		g.DiggingCostCurveShape = name
	case CurveDiggingStrength:
		g.DiggingStrengthCurveShape = name
	case CurveAttack:
		g.AttackCurveShape = name
	case CurveDamageTaken:
		g.DamageTakenCurveShape = name
	case CurveThorns:
		g.ThornsCurveShape = name
	case CurvePhTolerance:
		g.PhToleranceCurveShape = name
	case CurveDiggingCreate:
		g.DiggingCreationCurveShape = name
	case CurveChemoPhEffect:
		g.ChemoPhEffectCurveShape = name
	case CurveEatingPhEffect:
		g.EatingPhEffectCurveShape = name
	default:
		// Loud rather than silent: a curve missing from this switch keeps
		// whatever shape the settings gave it, so every test that sets a
		// shape quietly tests something else instead. CurveDiggingCreate
		// sat unhandled here for exactly that reason.
		panic("setCurveShape: no case for curve " + id.Name())
	}
}

// TestCosineShape pins the formula: T = (1 − cos(πp))/2, D = (1 − p)·K·T,
// shape = T − D.
func TestCosineShape(t *testing.T) {
	loadGlobals(t)
	g := *config.GetCurrentGlobals()
	// The cosine shape specifically, whatever the defaults ship with.
	g.AttackCurveShape = string(ShapeCosine)
	g.AttackCosineK = 0.5
	cv := CurveFor(&g, CurveAttack)

	// At the midpoint: T = 0.5, D = 0.5 · 0.5 · 0.5 = 0.125, shape = 0.375.
	mid := float64(MaxAbilityScore) / 2
	if got := cv.At(mid); math.Abs(got-0.375) > 1e-12 {
		t.Errorf("attack at %v = %v, want 0.375", mid, got)
	}

	// K = 0 is a pure cosine: symmetric about the midpoint.
	g.AttackCosineK = 0
	pure := CurveFor(&g, CurveAttack)
	if got := pure.At(mid/2) + pure.At(mid*1.5); math.Abs(got-1) > 1e-12 {
		t.Errorf("pure cosine should be symmetric: at(%v) + at(%v) = %v, want 1", mid/2, mid*1.5, got)
	}

	// More K means less benefit at every score in between.
	g.AttackCosineK = 1
	damped := CurveFor(&g, CurveAttack)
	for _, score := range []float64{1, mid / 2, mid, mid * 1.5, MaxAbilityScore - 1} {
		if !(damped.At(score) < pure.At(score)) {
			t.Errorf("K = 1 should suppress score %v below the pure cosine", score)
		}
	}
}

// TestCostCurvesMirrorEffectCurves: with the same K, a cost curve is 1
// minus the effect curve at every score.
func TestCostCurvesMirrorEffectCurves(t *testing.T) {
	loadGlobals(t)
	g := *config.GetCurrentGlobals()
	g.AttackCurveShape, g.MovementCostCurveShape = string(ShapeCosine), string(ShapeCosine)
	g.AttackCosineK, g.MovementCostCosineK = 0.4, 0.4
	effect, cost := CurveFor(&g, CurveAttack), CurveFor(&g, CurveMovementCost)
	for s := 0; s <= MaxAbilityScore; s += 10 {
		if got, want := cost.At(float64(s)), 1-effect.At(float64(s)); math.Abs(got-want) > 1e-12 {
			t.Errorf("score %d: cost %v, want 1 − effect = %v", s, got, want)
		}
	}
}

// TestCurveKClamps: K is held to [0, 1], so no setting can make a curve
// run backwards.
func TestCurveKClamps(t *testing.T) {
	loadGlobals(t)
	g := *config.GetCurrentGlobals()
	// Each clamp belongs to a shape, so ask for that shape.
	g.AttackCurveShape = string(ShapeCosine)
	g.ChemosynthesisCurveShape = string(ShapeSaturating)
	g.AttackCosineK = 5
	if k := CurveFor(&g, CurveAttack).Shape.(CosineShape).K; k != 1 {
		t.Errorf("K 5 should clamp to 1, got %v", k)
	}
	g.AttackCosineK = -1
	if k := CurveFor(&g, CurveAttack).Shape.(CosineShape).K; k != 0 {
		t.Errorf("K -1 should clamp to 0, got %v", k)
	}
	g.ChemosynthesisSaturatingK = 0.25
	if k := CurveFor(&g, CurveChemosynthesis).Shape.(SaturatingShape).K; k != MinSaturatingK {
		t.Errorf("chemosynthesis K 0.25 should clamp to %v, got %v", MinSaturatingK, k)
	}
	g.ChemosynthesisSaturatingK = 1000
	if k := CurveFor(&g, CurveChemosynthesis).Shape.(SaturatingShape).K; k != MaxSaturatingK {
		t.Errorf("chemosynthesis K 1000 should clamp to %v, got %v", MaxSaturatingK, k)
	}
}

// TestSaturatingShape: chemosynthesis front-loads its gains, reaching half
// the benefit at score √K, and still ends at exactly 1.
func TestSaturatingShape(t *testing.T) {
	loadGlobals(t)
	g := *config.GetCurrentGlobals()
	// The saturating shape specifically, whatever the defaults ship with.
	g.ChemosynthesisCurveShape = string(ShapeSaturating)
	for _, k := range []float64{1, 25, 100} {
		g.ChemosynthesisSaturatingK = k
		cv := CurveFor(&g, CurveChemosynthesis)
		scale := 1 - k/(k+MaxAbilityScore*MaxAbilityScore)
		if got, want := cv.At(math.Sqrt(k)), 0.5/scale; math.Abs(got-want) > 1e-12 {
			t.Errorf("K %v: at score √K = %v, want %v", k, got, want)
		}
		if cv.At(MaxAbilityScore) != 1 || cv.At(0) != 0 {
			t.Errorf("K %v: endpoints %v and %v, want 0 and 1", k, cv.At(0), cv.At(MaxAbilityScore))
		}
		prev := 0.0
		for s := 1; s <= MaxAbilityScore; s++ {
			v := cv.At(float64(s))
			if v < prev {
				t.Fatalf("K %v falls between scores %d and %d", k, s-1, s)
			}
			prev = v
		}
		// Front-loaded: past the halfway score, most of the gain is in.
		if cv.At(50) < 0.95 {
			t.Errorf("K %v: only %v of the gain by score 50", k, cv.At(50))
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
	s := BalancedScores()
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
	s := BalancedScores()
	peak := 0
	for i := 0; i < 100000; i++ {
		s = s.Mutated(rng)
		for _, a := range AllAbilities {
			if a != AbilityChemosynthesis && s[a] > peak {
				peak = s[a]
			}
		}
	}
	if peak < MaxAbilityScore-2 {
		t.Errorf("no non-chemo ability ever reached %d points; drift is too weak "+
			"to produce specialists (peak %d)", MaxAbilityScore-2, peak)
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
		s := BalancedScores()
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
