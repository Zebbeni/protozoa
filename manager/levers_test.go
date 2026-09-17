package manager

import (
	"testing"

	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

func (s *gridStub) AddOrganismUpdate(utils.Point) {}

// TestAverageAbilityScores: the running log's per-ability columns are the
// mean across living organisms, and an empty world reports zeroes rather
// than dividing by none.
func TestAverageAbilityScores(t *testing.T) {
	loadDefaultGlobals(t)
	m := &OrganismManager{organisms: map[int]*organism.Organism{}, organismIDGrid: initializeGrid()}
	for _, got := range m.AverageAbilityScores() {
		if got != 0 {
			t.Fatalf("an empty world reports %v, want zeroes", got)
		}
	}

	first := physiology.Scores{}
	first[physiology.AbilityChemosynthesis] = 60
	first[physiology.AbilityEating] = 40
	second := physiology.Scores{}
	second[physiology.AbilityChemosynthesis] = 20
	second[physiology.AbilityEating] = 80
	m.organisms[1] = organismWithScores(1, utils.Point{X: 1, Y: 1}, utils.Point{X: 1, Y: 0}, 10, first)
	m.organisms[2] = organismWithScores(2, utils.Point{X: 2, Y: 2}, utils.Point{X: 1, Y: 0}, 10, second)

	avg := m.AverageAbilityScores()
	if got := avg[physiology.AbilityChemosynthesis]; got != 40 {
		t.Errorf("average Chemosynthesis %v, want 40", got)
	}
	if got := avg[physiology.AbilityEating]; got != 60 {
		t.Errorf("average Eating %v, want 60", got)
	}
	if got := avg[physiology.AbilityAttack]; got != 0 {
		t.Errorf("average Attack %v, want 0", got)
	}
}

// organismWithScores builds a test organism carrying the given scores.
func organismWithScores(id int, location, direction utils.Point, size float64, scores physiology.Scores) *organism.Organism {
	return organism.Restore(id, 1, size, size, 0, 0, 0, location, direction, id,
		organism.Traits{Abilities: scores}, nil, d.ActIdle, organism.StatusIdle, 0, 0, 0, 0, nil)
}
