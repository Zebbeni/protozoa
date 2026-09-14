package organism

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
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

func globalsWithChemoTolerance(t *testing.T, tol float64) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	g.ChemosynthesisTolerance = tol
	config.SetGlobals(&g)
}

func organismAtPhDistance(dist float64) *Organism {
	const ideal = 5.0
	return &Organism{traits: Traits{IdealPh: ideal}, lookupAPI: phLookup{ph: ideal + dist}}
}

// TestPhConditionsAreIndependent pins the split: "safe here" is the
// damage boundary and "can chemosynthesize here" is the feeding boundary,
// and they disagree in both directions depending on which window is wider.
func TestPhConditionsAreIndependent(t *testing.T) {
	// Feeding window wider than the damage-free tolerance: between the
	// two boundaries an organism can feed but is being hurt.
	globalsWithChemoTolerance(t, 3.0)
	between := (config.PhTolerance() + config.ChemosynthesisPhWindow()) / 2
	o := organismAtPhDistance(between)
	if o.isConditionTrue(d.IsHealthyPhHere) {
		t.Errorf("at distance %.2f (past tolerance %.2f) pH should not be safe", between, config.PhTolerance())
	}
	if !o.isConditionTrue(d.CanChemosynthesizeHere) {
		t.Errorf("at distance %.2f (inside window %.2f) chemosynthesis should be possible", between, config.ChemosynthesisPhWindow())
	}

	// Feeding window narrower: between them an organism is safe but
	// can't feed.
	globalsWithChemoTolerance(t, 0.25)
	between = (config.PhTolerance() + config.ChemosynthesisPhWindow()) / 2
	o = organismAtPhDistance(between)
	if !o.isConditionTrue(d.IsHealthyPhHere) {
		t.Errorf("at distance %.2f (inside tolerance %.2f) pH should be safe", between, config.PhTolerance())
	}
	if o.isConditionTrue(d.CanChemosynthesizeHere) {
		t.Errorf("at distance %.2f (past window %.2f) chemosynthesis should not be possible", between, config.ChemosynthesisPhWindow())
	}
}
