package manager

import (
	"testing"

	d "github.com/Zebbeni/protozoa/decision"

	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// eatingRecorder is a phRecorder that also holds food, which applyEat needs
// to find something to eat and to remove it.
type eatingRecorder struct {
	phRecorder
	food map[utils.Point]int
}

func (s *eatingRecorder) GetFoodAtPoint(p utils.Point) (*food.Item, bool) {
	if v, ok := s.food[p]; ok && v > 0 {
		return food.NewItem(p, v), true
	}
	return nil, false
}

func (s *eatingRecorder) RemoveFoodAtPoint(p utils.Point, value int) {
	if s.food[p] -= value; s.food[p] < 0 {
		s.food[p] = 0
	}
}

// eatingOrganism is one standing on food, facing +x, with points in Eating.
func eatingOrganism(t *testing.T) (*OrganismManager, *organism.Organism, *eatingRecorder) {
	t.Helper()
	loadDefaultGlobals(t)
	scores := physiology.Scores{}
	scores[physiology.AbilityEating] = physiology.MaxAbilityScore
	scores[physiology.AbilityTolerance] = physiology.PointTotal - physiology.MaxAbilityScore

	at := utils.Point{X: 10, Y: 10}
	api := &eatingRecorder{phRecorder: phRecorder{ph: 5}, food: map[utils.Point]int{at: 50}}
	m := &OrganismManager{api: api, organismIDGrid: initializeGrid()}
	traits := organism.Traits{IdealPh: 5, Abilities: scores, MaxSize: 100}
	o := organism.Restore(1, 1, 20, 20, 0, 0, 0,
		at, utils.Point{X: 1, Y: 0}, 1,
		traits, nil, d.ActEat, organism.StatusIdle, 0, 0, 0, 0, nil)
	return m, o, api
}

// TestEatingPushesPhAtTheEatersOwnCell: eating and chemosynthesis both act
// on the cell the organism occupies, so both push pH there. The eating push
// landed on the cell BEHIND instead, which put the restoring half of the pH
// cycle somewhere the organism was not.
func TestEatingPushesPhAtTheEatersOwnCell(t *testing.T) {
	m, o, api := eatingOrganism(t)
	behind := o.Location.Sub(o.Direction)

	m.applyEat(o)

	if api.changes[o.Location] <= 0 {
		t.Errorf("eating changed pH at the eater's cell %v by %v; want a rise",
			o.Location, api.changes[o.Location])
	}
	if v, touched := api.changes[behind]; touched {
		t.Errorf("eating also changed pH at the cell behind %v (by %v)", behind, v)
	}
}
