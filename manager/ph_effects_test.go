package manager

import (
	"math"
	"testing"

	d "github.com/Zebbeni/protozoa/decision"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// phRecorder is an organism.API that records the pH it was asked to
// change and reports a fixed pH everywhere.
type phRecorder struct {
	organism.API
	ph      float64
	changes map[utils.Point]float64
}

func (s *phRecorder) GetPhAtPoint(utils.Point) float64 { return s.ph }
func (s *phRecorder) AddPhChangeAtPoint(p utils.Point, v float64) {
	if s.changes == nil {
		s.changes = map[utils.Point]float64{}
	}
	s.changes[p] += v
}
func (s *phRecorder) IsWallAtPoint(utils.Point) bool { return false }
func (s *phRecorder) AddPhUpdate(utils.Point)        {}
func (s *phRecorder) AddOrganismUpdate(utils.Point)  {}

// chemoOrganism is one sitting at ph, with the given Chemosynthesis score.
func chemoOrganism(t *testing.T, score int, ph float64) (*OrganismManager, *organism.Organism, *phRecorder) {
	t.Helper()
	loadDefaultGlobals(t)
	scores := physiology.Scores{}
	scores[physiology.AbilityChemosynthesis] = score
	// The rest of the budget parks in Tolerance first (no single ability
	// can hold it all), which keeps pH damage out of these chemo tests.
	scores[physiology.AbilityTolerance] = min(physiology.MaxAbilityScore, physiology.PointTotal-score)
	scores[physiology.AbilityEating] = physiology.PointTotal - score - scores[physiology.AbilityTolerance]
	api := &phRecorder{ph: ph}
	m := &OrganismManager{api: api, organismIDGrid: initializeGrid()}
	traits := organism.Traits{IdealPh: 5, Abilities: scores, MaxSize: 100}
	o := organism.Restore(1, 1, 20, 20, 0, 0, 0,
		utils.Point{X: 10, Y: 10}, utils.Point{X: 1, Y: 0}, 1,
		traits, nil, d.ActIdle, organism.StatusIdle, 0, 0, 0, 0, nil)
	return m, o, api
}

// TestChemoPhEffectFollowsHealthGained: the pH a chemosynthesizing
// organism pushes out is its health gain times the setting, so a marginal
// attempt barely moves the water and one that gained nothing doesn't move
// it at all.
func TestChemoPhEffectFollowsHealthGained(t *testing.T) {
	const score = 60
	m, o, api := chemoOrganism(t, score, 5)
	g := config.GetCurrentGlobals()
	// The action's gain, not the health that survives growth: part of a
	// gain goes into size.
	gain := effects.ChemosynthesisGain(g, score, o.Size, 0)
	m.applyChemosynthesis(o)
	want := g.ChemoPhEffect * gain
	if got := -api.changes[o.Location]; math.Abs(got-want) > 1e-9 || want <= 0 {
		t.Errorf("at its ideal pH: pushed pH by %v, want %v (gain %v)", got, want, gain)
	}

	// Further off, the same organism gains less and acidifies less.
	width := effects.ChemoWidth(g, score)
	m2, o2, api2 := chemoOrganism(t, score, 5+width/2)
	gain2 := effects.ChemosynthesisGain(g, score, o2.Size, width/2)
	m2.applyChemosynthesis(o2)
	if !(gain2 > 0 && gain2 < gain) {
		t.Fatalf("off-ideal gain %v, want between 0 and %v", gain2, gain)
	}
	if got := -api2.changes[o2.Location]; !(got > 0 && got < -api.changes[o.Location]) {
		t.Errorf("off-ideal pushed pH by %v, want less than %v", got, -api.changes[o.Location])
	}

	// Past its width the attempt costs health and moves no pH at all.
	m3, o3, api3 := chemoOrganism(t, score, 5+width*2)
	before3 := o3.Health
	m3.applyChemosynthesis(o3)
	if o3.Health >= before3 {
		t.Fatalf("past its width the attempt should cost health, got %v", o3.Health-before3)
	}
	if got := api3.changes[o3.Location]; got != 0 {
		t.Errorf("a failed attempt moved pH by %v, want 0", got)
	}
}
