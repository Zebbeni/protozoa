package organism

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// phLookup is a LookupAPI whose whole world sits at one pH.
type phLookup struct{ ph float64 }

func (l phLookup) CheckFoodAtPoint(utils.Point, FoodCheck) bool    { return false }
func (l phLookup) CheckOrganismAtPoint(utils.Point, OrgCheck) bool { return false }
func (l phLookup) GetFoodAtPoint(utils.Point) (*food.Item, bool)   { return nil, false }
func (l phLookup) GetPhAtPoint(utils.Point) float64                { return l.ph }
func (l phLookup) GetPhMap() [][]float64                           { return nil }
func (l phLookup) IsWallAtPoint(utils.Point) bool                  { return false }
func (l phLookup) GetWallStrengthAtPoint(utils.Point) int          { return 0 }
func (l phLookup) OrganismCount() int                              { return 1 }
func (l phLookup) FoodCount() int                                  { return 0 }
func (l phLookup) WallCount() int                                  { return 0 }
func (l phLookup) Cycle() int                                      { return 0 }
func (l phLookup) GetSelected() int                                { return -1 }

func globalsWithChemoWidth(t *testing.T, chemoScore int) *config.Globals {
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
	return config.GetCurrentGlobals()
}

// testPhScore is the Chemosynthesis and Tolerance score the pH-condition
// organism carries: half the budget each, so both abilities are well
// inside their curves rather than at an endpoint.
const testPhScore = physiology.MaxAbilityScore / 2

// organismAtPhDistance builds an organism sitting dist away from its
// ideal pH, splitting its budget between Chemosynthesis and Tolerance.
func organismAtPhDistance(dist float64) *Organism {
	const ideal = 5.0
	scores := physiology.Scores{}
	scores[physiology.AbilityChemosynthesis] = testPhScore
	scores[physiology.AbilityTolerance] = testPhScore
	return &Organism{
		traits:    Traits{IdealPh: ideal, Abilities: scores},
		lookupAPI: phLookup{ph: ideal + dist},
	}
}

// TestPhConditionsAreIndependent pins the split: each condition asks
// about its own ability — "safe here" about Tolerance's width T, "can
// chemosynthesize here" about Chemosynthesis's width C — so they disagree
// wherever the two widths differ.
func TestPhConditionsAreIndependent(t *testing.T) {
	g := globalsWithChemoWidth(t, testPhScore)
	chemo := effects.ChemoWidth(g, testPhScore)
	bearable := effects.PhToleranceWidth(g, testPhScore)
	if chemo >= bearable {
		t.Fatalf("test assumes a narrower feeding width than bearable band: C %v, T %v", chemo, bearable)
	}

	// Between the two: the water is bearable, but feeding no longer pays.
	between := (chemo + bearable) / 2
	o := organismAtPhDistance(between)
	if !o.isConditionTrue(d.IsHealthyPhHere) {
		t.Errorf("at distance %.2f (inside T = %.2f) pH should be safe", between, bearable)
	}
	if o.isConditionTrue(d.CanChemosynthesizeHere) {
		t.Errorf("at distance %.2f (past C = %.2f) chemosynthesis should not pay off", between, chemo)
	}

	// Inside both, and outside both.
	if o := organismAtPhDistance(chemo / 2); !o.isConditionTrue(d.IsHealthyPhHere) || !o.isConditionTrue(d.CanChemosynthesizeHere) {
		t.Error("close to its ideal pH an organism should be both safe and able to feed")
	}
	if o := organismAtPhDistance(bearable * 2); o.isConditionTrue(d.IsHealthyPhHere) || o.isConditionTrue(d.CanChemosynthesizeHere) {
		t.Error("far from its ideal pH an organism should be neither safe nor able to feed")
	}
}

// TestChemosynthesisIsOneCurve: the same curve covers the whole attempt —
// a gain at the ideal pH, breaking even at C, a loss past it — so nothing
// needs a separate "failed" case.
func TestChemosynthesisIsOneCurve(t *testing.T) {
	g := globalsWithChemoWidth(t, physiology.MaxAbilityScore)
	const score = 60
	width := effects.ChemoWidth(g, score)

	best := effects.ChemosynthesisGain(g, score, 1, 0)
	if math.Abs(best-g.MaxChemosynthesisGain) > 1e-12 {
		t.Errorf("at its ideal pH an organism gains %v, want the full %v", best, g.MaxChemosynthesisGain)
	}
	if edge := effects.ChemosynthesisGain(g, score, 1, width); math.Abs(edge) > 1e-12 {
		t.Errorf("at C the attempt should break even, got %v", edge)
	}
	if past := effects.ChemosynthesisGain(g, score, 1, width*1.5); past >= 0 {
		t.Errorf("past C the attempt should cost health, got %v", past)
	}
	// The loss is floored at what a perfect attempt would have gained.
	if far := effects.ChemosynthesisGain(g, score, 1, 100); far < -g.MaxChemosynthesisGain-1e-12 {
		t.Errorf("far from ideal the loss is %v, want no worse than %v", far, -g.MaxChemosynthesisGain)
	}
	// A wider ability feeds where a narrower one can't.
	if narrow := effects.ChemosynthesisGain(g, 2, 1, width); narrow >= 0 {
		t.Errorf("a score of 2 should not pay off at %v pH, got %v", width, narrow)
	}
}

// TestPhDamageNeverReachesZero: there is no safe band — only water at
// exactly an organism's ideal pH is free — and Tolerance widens what an
// organism bears without ever cancelling the cost.
func TestPhDamageNeverReachesZero(t *testing.T) {
	g := globalsWithChemoWidth(t, physiology.MaxAbilityScore)

	if got := effects.PhDamage(g, physiology.MaxAbilityScore, 10, 0); got != 0 {
		t.Errorf("at its ideal pH an organism should pay nothing, got %v", got)
	}
	for _, distance := range []float64{0.1, 1, 3, 6} {
		none := effects.PhDamage(g, 0, 1, distance)
		full := effects.PhDamage(g, physiology.MaxAbilityScore, 1, distance)
		if none >= 0 || full >= 0 {
			t.Errorf("distance %v: damage %v / %v, want both negative", distance, none, full)
		}
		if !(full > none) {
			t.Errorf("distance %v: full Tolerance pays %v, no Tolerance %v; tolerance should help", distance, full, none)
		}
	}

	// Squared in distance, and scaled by size.
	near, far := effects.PhDamage(g, 40, 1, 1), effects.PhDamage(g, 40, 1, 2)
	if math.Abs(far-4*near) > 1e-12 {
		t.Errorf("1 pH costs %v and 2 pH costs %v, want four times as much", near, far)
	}
	if big := effects.PhDamage(g, 40, 3, 1); math.Abs(big-3*near) > 1e-12 {
		t.Errorf("three times the size costs %v, want %v", big, 3*near)
	}
}
