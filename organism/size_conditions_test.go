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

// neighbourLookup puts one organism of a chosen size in every direction.
type neighbourLookup struct{ size float64 }

func (l neighbourLookup) CheckOrganismAtPoint(_ utils.Point, check OrgCheck) bool {
	if l.size == 0 {
		return check(nil)
	}
	return check(&Organism{Size: l.size})
}
func (l neighbourLookup) CheckFoodAtPoint(utils.Point, FoodCheck) bool  { return false }
func (l neighbourLookup) GetBuriedFoodAtPoint(utils.Point) int          { return 0 }
func (l neighbourLookup) GetFoodAtPoint(utils.Point) (*food.Item, bool) { return nil, false }
func (l neighbourLookup) GetPhAtPoint(utils.Point) float64              { return 5 }
func (l neighbourLookup) GetPhMap() [][]float64                         { return nil }
func (l neighbourLookup) IsWallAtPoint(utils.Point) bool                { return false }
func (l neighbourLookup) GetWallStrengthAtPoint(utils.Point) int        { return 0 }
func (l neighbourLookup) OrganismCount() int                            { return 2 }
func (l neighbourLookup) FoodCount() int                                { return 0 }
func (l neighbourLookup) BuriedFoodCount() int                          { return 0 }
func (l neighbourLookup) WallCount() int                                { return 0 }
func (l neighbourLookup) Cycle() int                                    { return 0 }
func (l neighbourLookup) GetSelected() int                              { return -1 }

func loadSizeGlobals(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	config.SetGlobals(&g)
}

func sizeSubject(t *testing.T, size, neighbourSize float64) *Organism {
	t.Helper()
	loadSizeGlobals(t)
	return &Organism{
		Size:      size,
		Location:  utils.Point{X: 5, Y: 5},
		Direction: utils.Point{X: 1, Y: 0},
		lookupAPI: neighbourLookup{size: neighbourSize},
	}
}

// cellLookup answers the surface and buried food questions per cell, and nothing else.
type cellLookup struct {
	surface map[utils.Point]int
	buried  map[utils.Point]int
}

func (l cellLookup) CheckFoodAtPoint(p utils.Point, check FoodCheck) bool {
	if v, ok := l.surface[p]; ok && v > 0 {
		return check(food.NewItem(p, v), true)
	}
	return check(nil, false)
}
func (l cellLookup) GetBuriedFoodAtPoint(p utils.Point) int          { return l.buried[p] }
func (l cellLookup) CheckOrganismAtPoint(utils.Point, OrgCheck) bool { return false }
func (l cellLookup) GetFoodAtPoint(p utils.Point) (*food.Item, bool) {
	if v, ok := l.surface[p]; ok && v > 0 {
		return food.NewItem(p, v), true
	}
	return nil, false
}
func (l cellLookup) GetPhAtPoint(utils.Point) float64       { return 5 }
func (l cellLookup) GetPhMap() [][]float64                  { return nil }
func (l cellLookup) IsWallAtPoint(utils.Point) bool         { return false }
func (l cellLookup) GetWallStrengthAtPoint(utils.Point) int { return 0 }
func (l cellLookup) OrganismCount() int                     { return 1 }
func (l cellLookup) FoodCount() int                         { return 0 }
func (l cellLookup) BuriedFoodCount() int                   { return 0 }
func (l cellLookup) WallCount() int                         { return 0 }
func (l cellLookup) Cycle() int                             { return 0 }
func (l cellLookup) GetSelected() int                       { return -1 }

var muchBiggerConditions = []d.Condition{
	d.IsMuchBiggerOrganismAhead, d.IsMuchBiggerOrganismLeft, d.IsMuchBiggerOrganismRight,
}

var muchSmallerConditions = []d.Condition{
	d.IsMuchSmallerOrganismAhead, d.IsMuchSmallerOrganismLeft, d.IsMuchSmallerOrganismRight,
}

// TestMuchBiggerNeedsTheWholeRatio: the threshold is a multiple of the asker's own size, not an absolute.
func TestMuchBiggerNeedsTheWholeRatio(t *testing.T) {
	loadSizeGlobals(t)
	const mine = 10.0
	ratio := config.MuchBiggerSizeRatio()

	// Just under the ratio: bigger, but not much bigger.
	o := sizeSubject(t, mine, mine*ratio-0.01)
	for _, c := range muchBiggerConditions {
		if o.isConditionTrue(c) {
			t.Errorf("%s is true just under the %vx threshold", d.Names[c], ratio)
		}
	}
	if !o.isConditionTrue(d.IsBiggerOrganismAhead) {
		t.Error("the plain bigger-than condition should be true here; the two would otherwise agree")
	}

	o = sizeSubject(t, mine, mine*ratio+0.01)
	for _, c := range muchBiggerConditions {
		if !o.isConditionTrue(c) {
			t.Errorf("%s is false just over the %vx threshold", d.Names[c], ratio)
		}
	}
}

func TestMuchSmallerNeedsTheWholeRatio(t *testing.T) {
	loadSizeGlobals(t)
	const mine = 10.0
	ratio := config.MuchSmallerSizeRatio()

	o := sizeSubject(t, mine, mine*ratio+0.01)
	for _, c := range muchSmallerConditions {
		if o.isConditionTrue(c) {
			t.Errorf("%s is true just over the %vx threshold", d.Names[c], ratio)
		}
	}

	o = sizeSubject(t, mine, mine*ratio-0.01)
	for _, c := range muchSmallerConditions {
		if !o.isConditionTrue(c) {
			t.Errorf("%s is false just under the %vx threshold", d.Names[c], ratio)
		}
	}
}

func TestMuchBiggerAndSmallerAreTwoSidesOfOneRelationship(t *testing.T) {
	loadSizeGlobals(t)
	big, small := config.MuchBiggerSizeRatio(), config.MuchSmallerSizeRatio()
	if big*small != 1 {
		t.Errorf("the shipped ratios are %v and %v, which are not reciprocals", big, small)
	}

	const a, b = 5.0, 25.0 // b is exactly 5x a, past a 4x threshold
	if !sizeSubject(t, a, b).isConditionTrue(d.IsMuchBiggerOrganismAhead) {
		t.Error("the smaller organism does not see the larger as much bigger")
	}
	if !sizeSubject(t, b, a).isConditionTrue(d.IsMuchSmallerOrganismAhead) {
		t.Error("the larger organism does not see the smaller as much smaller")
	}
}

// TestEmptyWaterIsNeitherBigNorSmall: the conditions ask about a neighbour, and an empty cell has none.
func TestEmptyWaterIsNeitherBigNorSmall(t *testing.T) {
	o := sizeSubject(t, 10, 0)
	for _, c := range append(append([]d.Condition{}, muchBiggerConditions...), muchSmallerConditions...) {
		if o.isConditionTrue(c) {
			t.Errorf("%s is true against an empty cell", d.Names[c])
		}
	}
}

func TestSizeRatioConditionsFollowTheirSettings(t *testing.T) {
	o := sizeSubject(t, 10, 20) // twice my size: not "much" at the 4x default
	if o.isConditionTrue(d.IsMuchBiggerOrganismAhead) {
		t.Fatal("2x should not read as much bigger at the shipped 4x ratio")
	}

	g := config.GetCurrentGlobals()
	g.MuchBiggerSizeRatio = 1.5
	config.SetGlobals(g)
	if !o.isConditionTrue(d.IsMuchBiggerOrganismAhead) {
		t.Error("lowering the ratio to 1.5 did not make 2x read as much bigger")
	}
}

func TestFoodHereConditionsReadTheOwnCell(t *testing.T) {
	loadSizeGlobals(t)
	here := utils.Point{X: 5, Y: 5}
	ahead := utils.Point{X: 6, Y: 5}

	subject := func(surface map[utils.Point]int, buried map[utils.Point]int) *Organism {
		return &Organism{
			Size:      10,
			Location:  here,
			Direction: utils.Point{X: 1, Y: 0},
			lookupAPI: cellLookup{surface: surface, buried: buried},
		}
	}

	// Empty cell, food only in front: both conditions false.
	o := subject(map[utils.Point]int{ahead: 50}, map[utils.Point]int{ahead: 50})
	if o.isConditionTrue(d.IsFoodHere) {
		t.Error("IsFoodHere is true for food in the cell AHEAD")
	}
	if o.isConditionTrue(d.IsFoodBuriedHere) {
		t.Error("IsFoodBuriedHere is true for food buried in the cell AHEAD")
	}
	if !o.isConditionTrue(d.IsFoodAhead) {
		t.Error("IsFoodAhead should still read the cell ahead")
	}

	o = subject(map[utils.Point]int{here: 50}, nil)
	if !o.isConditionTrue(d.IsFoodHere) {
		t.Error("IsFoodHere missed food in its own cell")
	}
	if o.isConditionTrue(d.IsFoodBuriedHere) {
		t.Error("IsFoodBuriedHere is true with nothing buried")
	}

	// Buried only: worth digging, nothing to eat yet.
	o = subject(nil, map[utils.Point]int{here: 7})
	if o.isConditionTrue(d.IsFoodHere) {
		t.Error("IsFoodHere is true for food that is still buried")
	}
	if !o.isConditionTrue(d.IsFoodBuriedHere) {
		t.Error("IsFoodBuriedHere missed the buried store under it")
	}
}

func TestMuchFoodIsRelativeToTheAsker(t *testing.T) {
	loadSizeGlobals(t)
	here := utils.Point{X: 5, Y: 5}
	ratio := config.MuchFoodPerSize()

	subject := func(size float64, surface, buried int) *Organism {
		return &Organism{
			Size:      size,
			Location:  here,
			Direction: utils.Point{X: 1, Y: 0},
			lookupAPI: cellLookup{
				surface: map[utils.Point]int{here: surface},
				buried:  map[utils.Point]int{here: buried},
			},
		}
	}

	const pile = 40
	small := float64(pile)/ratio - 1 // needs less than the pile to qualify
	large := float64(pile)/ratio + 1

	if o := subject(small, pile, pile); !o.isConditionTrue(d.IsMuchFoodHere) {
		t.Errorf("a pile of %d is not much to an organism of size %v", pile, small)
	}
	if o := subject(large, pile, pile); o.isConditionTrue(d.IsMuchFoodHere) {
		t.Errorf("a pile of %d reads as much to an organism of size %v", pile, large)
	}
	// The buried half reads the same way against the same threshold.
	if o := subject(small, 0, pile); !o.isConditionTrue(d.IsMuchFoodBuriedHere) {
		t.Errorf("%d buried is not much to an organism of size %v", pile, small)
	}
	if o := subject(large, 0, pile); o.isConditionTrue(d.IsMuchFoodBuriedHere) {
		t.Errorf("%d buried reads as much to an organism of size %v", pile, large)
	}
}

func TestMuchFoodCannotBeTrueWithoutFood(t *testing.T) {
	loadSizeGlobals(t)
	here := utils.Point{X: 5, Y: 5}

	for _, ratio := range []float64{0, 0.5, 1, 100} {
		g := config.GetCurrentGlobals()
		g.MuchFoodPerSize = ratio
		config.SetGlobals(g)

		o := &Organism{
			Size:      10,
			Location:  here,
			Direction: utils.Point{X: 1, Y: 0},
			lookupAPI: cellLookup{},
		}
		if o.isConditionTrue(d.IsMuchFoodHere) {
			t.Errorf("ratio %v: IsMuchFoodHere is true on an empty cell", ratio)
		}
		if o.isConditionTrue(d.IsMuchFoodBuriedHere) {
			t.Errorf("ratio %v: IsMuchFoodBuriedHere is true with nothing buried", ratio)
		}
	}
}

func TestMuchFoodFollowsItsSetting(t *testing.T) {
	loadSizeGlobals(t)
	here := utils.Point{X: 5, Y: 5}
	o := &Organism{
		Size:      10,
		Location:  here,
		Direction: utils.Point{X: 1, Y: 0},
		lookupAPI: cellLookup{surface: map[utils.Point]int{here: 15}},
	}

	g := config.GetCurrentGlobals()
	g.MuchFoodPerSize = 2 // needs 20 units for a size-10 organism
	config.SetGlobals(g)
	if o.isConditionTrue(d.IsMuchFoodHere) {
		t.Error("15 units read as much at 2 per unit of size 10")
	}

	g.MuchFoodPerSize = 1 // needs 10
	config.SetGlobals(g)
	if !o.isConditionTrue(d.IsMuchFoodHere) {
		t.Error("15 units did not read as much at 1 per unit of size 10")
	}
}
