package physiology

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

func TestScoresFromSlice(t *testing.T) {
	if s, err := ScoresFromSlice([]int{40, 20, 10, 10, 10, 10}); err != nil || s[AbilityEating] != 20 {
		t.Errorf("valid slice: %v, %v", s, err)
	}
	for _, bad := range [][]int{
		{50, 10, 10, 10, 10},     // too short
		{50, 10, 10, 10, 10, 20}, // sums to 110
		{-10, 30, 20, 20, 20, 20},
	} {
		if _, err := ScoresFromSlice(bad); err == nil {
			t.Errorf("%v should be rejected", bad)
		}
	}
}

// TestRandomScoresKeepTheBudget: every draw is a valid distribution, the
// draws actually vary, and a seed reproduces them.
func TestRandomScoresKeepTheBudget(t *testing.T) {
	rng := simrand.New(7)
	seen := map[Scores]bool{}
	for i := 0; i < 500; i++ {
		s := RandomScores(rng)
		if err := s.Validate(); err != nil {
			t.Fatalf("draw %d invalid: %v (%v)", i, s, err)
		}
		seen[s] = true
	}
	if len(seen) < 450 {
		t.Errorf("only %d distinct distributions in 500 draws", len(seen))
	}
	a, b := simrand.New(99), simrand.New(99)
	if RandomScores(a) != RandomScores(b) {
		t.Error("the same seed should draw the same scores")
	}
}

func TestInitialScoresFollowsConfig(t *testing.T) {
	loadGlobals(t)
	g := config.GetCurrentGlobals()
	rng := simrand.New(1)

	g.RandomInitialAbilities = false
	g.InitialAbilityScores = []int{20, 30, 10, 10, 20, 10}
	if s := InitialScores(rng); s != (Scores{20, 30, 10, 10, 20, 10}) {
		t.Errorf("configured scores: got %v", s)
	}

	g.InitialAbilityScores = []int{90, 10, 10, 10, 10, 10}
	if s := InitialScores(rng); s != GenesisScores() {
		t.Errorf("an invalid configured total should fall back to genesis, got %v", s)
	}

	g.RandomInitialAbilities = true
	if s := InitialScores(rng); s.Validate() != nil {
		t.Errorf("random initial scores invalid: %v", s)
	}
}
