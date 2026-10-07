package physiology

import (
	"testing"

	"github.com/Zebbeni/protozoa/simrand"
)

func TestMutationStopsAtTheCap(t *testing.T) {
	loadGlobals(t)

	rng := simrand.New(5)
	var s Scores
	s[AbilityChemosynthesis] = MaxAbilityScore
	s[AbilityEating] = PointTotal - MaxAbilityScore
	if err := s.Validate(); err != nil {
		t.Fatalf("starting scores invalid: %v", err)
	}

	sawDrop := false
	for i := 0; i < 20000; i++ {
		s = s.Mutated(rng)
		if err := s.Validate(); err != nil {
			t.Fatalf("invariant broken after %d mutations: %v", i, err)
		}
		if s[AbilityChemosynthesis] < MaxAbilityScore {
			sawDrop = true
		}
	}
	if !sawDrop {
		t.Error("a maxed ability never gave any points away; the cap should block " +
			"gains, not transfers out")
	}
}

func TestRandomScoresRespectTheCap(t *testing.T) {
	rng := simrand.New(11)
	capped := 0
	for i := 0; i < 2000; i++ {
		s := RandomScores(rng)
		if err := s.Validate(); err != nil {
			t.Fatalf("draw %d invalid: %v (%v)", i, s, err)
		}
		for _, a := range AllAbilities {
			if s[a] == MaxAbilityScore {
				capped++
			}
		}
	}
	// If nothing ever landed on the cap the spill path isn't being exercised and this test proves nothing.
	if capped == 0 {
		t.Error("no draw ever reached the cap; the spill path is untested")
	}
	t.Logf("%d capped entries in 2000 draws", capped)
}
