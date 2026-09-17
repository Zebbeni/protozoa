package physiology

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

func TestScoresFromSlice(t *testing.T) {
	if s, err := ScoresFromSlice([]int{8, 4, 2, 2, 2, 1, 1}); err != nil || s[AbilityEating] != 4 {
		t.Errorf("valid slice: %v, %v", s, err)
	}
	for _, bad := range [][]int{
		{5, 1, 1, 1, 1, 1},     // too short
		{5, 1, 1, 1, 1, 1, 2},  // sums to 12, not the budget
		{-1, 6, 4, 4, 4, 2, 1}, // a negative entry
		{11, 3, 2, 2, 1, 1, 0}, // one entry over the cap
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
	g.InitialAbilityScores = []int{4, 6, 2, 2, 4, 1, 1}
	if s := InitialScores(rng); s != (Scores{4, 6, 2, 2, 4, 1, 1}) {
		t.Errorf("configured scores: got %v", s)
	}

	g.InitialAbilityScores = []int{9, 1, 1, 1, 1, 1, 1}
	if s := InitialScores(rng); s != BalancedScores() {
		t.Errorf("an invalid configured total should fall back to balanced, got %v", s)
	}

	g.RandomInitialAbilities = true
	if s := InitialScores(rng); s.Validate() != nil {
		t.Errorf("random initial scores invalid: %v", s)
	}
}
