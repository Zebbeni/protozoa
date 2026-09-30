package manager

import (
	"testing"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// burialManager builds a FoodManager holding the given piles, with no walls or organisms in the way.
func burialManager(t *testing.T, piles map[utils.Point]int) *FoodManager {
	t.Helper()
	loadDefaultGlobals(t)
	m := NewFoodManager(&foodAPIStub{}, simrand.New(1))
	g := c.GetCurrentGlobals()
	g.InitialFood = 0
	g.ChanceToAddFoodItem = 0
	c.SetGlobals(g)
	m.Items = map[utils.Point]*food.Item{}
	for p, v := range piles {
		m.AddFoodAtPoint(p, v)
	}
	return m
}

func totals(m *FoodManager) (surface, buried int) {
	for _, item := range m.Items {
		surface += item.Value
	}
	for _, v := range m.Buried {
		buried += v
	}
	return surface, buried
}

func TestBurialMovesAFlatAmountAndConservesIt(t *testing.T) {
	big := utils.Point{X: 1, Y: 1}
	small := utils.Point{X: 2, Y: 2}
	m := burialManager(t, map[utils.Point]int{big: 100, small: 10})

	for i := 0; i < 10; i++ {
		m.BuryFood(1)
	}

	if got := m.Items[big].Value; got != 90 {
		t.Errorf("the 100 pile is at %d, want 90", got)
	}
	if _, stillThere := m.Items[small]; stillThere {
		t.Errorf("the 10 pile should be gone, it is at %d", m.Items[small].Value)
	}
	if got := m.Buried[big]; got != 10 {
		t.Errorf("buried under the big pile is %d, want 10", got)
	}
	if got := m.Buried[small]; got != 10 {
		t.Errorf("buried under the small pile is %d, want 10", got)
	}

	surface, buried := totals(m)
	if surface+buried != 110 {
		t.Errorf("110 units went in and %d came out (%d surface + %d buried); "+
			"burial must move food, never destroy it", surface+buried, surface, buried)
	}
}

func TestBurialTakesOnlyWhatIsThere(t *testing.T) {
	p := utils.Point{X: 3, Y: 3}
	m := burialManager(t, map[utils.Point]int{p: 2})

	m.BuryFood(5)

	if _, stillThere := m.Items[p]; stillThere {
		t.Error("the pile survived a burial larger than itself")
	}
	if got := m.Buried[p]; got != 2 {
		t.Errorf("buried %d, want the 2 that were actually there", got)
	}
}

func TestBurialStopsAtAFullStore(t *testing.T) {
	p := utils.Point{X: 4, Y: 4}
	m := burialManager(t, map[utils.Point]int{p: 50})
	m.Buried[p] = c.MaxFoodValue()

	m.BuryFood(10)

	if got := m.Items[p].Value; got != 50 {
		t.Errorf("the pile lost %d units into a full store", 50-got)
	}
	if got := m.Buried[p]; got != c.MaxFoodValue() {
		t.Errorf("the buried store went past its cap to %d", got)
	}
}

func TestBurialOnlyFiresOnTheInterval(t *testing.T) {
	p := utils.Point{X: 5, Y: 5}
	m := burialManager(t, map[utils.Point]int{p: 100})
	g := c.GetCurrentGlobals()
	g.BurialAmount, g.BurialInterval = 1, 5
	g.ChanceToAddFoodItem = 0 // no spawns, so only burial moves anything
	c.SetGlobals(g)

	for cycle := 1; cycle <= 4; cycle++ {
		m.Update(cycle)
	}
	if got := m.Buried[p]; got != 0 {
		t.Errorf("after 4 cycles of a 5-cycle interval, %d is buried; want 0", got)
	}
	m.Update(5)
	if got := m.Buried[p]; got != 1 {
		t.Errorf("at cycle 5, %d is buried; want 1", got)
	}
	m.Update(10)
	if got := m.Buried[p]; got != 2 {
		t.Errorf("at cycle 10, %d is buried; want 2", got)
	}
}

func TestBurialCanBeSwitchedOff(t *testing.T) {
	p := utils.Point{X: 6, Y: 6}
	m := burialManager(t, map[utils.Point]int{p: 100})
	g := c.GetCurrentGlobals()
	g.BurialInterval = 0
	g.ChanceToAddFoodItem = 0
	c.SetGlobals(g)

	for cycle := 1; cycle <= 50; cycle++ {
		m.Update(cycle)
	}
	if got := m.Buried[p]; got != 0 {
		t.Errorf("burial ran with the interval off: %d buried", got)
	}
	if got := m.Items[p].Value; got != 100 {
		t.Errorf("the pile changed with burial off: %d", got)
	}
}

func TestUnburyReportsWhatItMoved(t *testing.T) {
	p := utils.Point{X: 7, Y: 7}
	m := burialManager(t, nil)
	m.Buried[p] = 3

	if got := m.Unbury(p, 10); got != 3 {
		t.Errorf("unburied %d, want the 3 that were there", got)
	}
	if item, ok := m.getFood(p); !ok || item.Value != 3 {
		t.Error("the unburied food did not arrive in the food layer")
	}
	if _, stillBuried := m.Buried[p]; stillBuried {
		t.Error("an emptied buried store was left behind as a zero entry")
	}
	if got := m.Unbury(p, 10); got != 0 {
		t.Errorf("a spent hole yielded %d; digging recovers food, it does not create it", got)
	}
}

func TestUnburyLeavesWhatDoesNotFit(t *testing.T) {
	p := utils.Point{X: 8, Y: 8}
	m := burialManager(t, map[utils.Point]int{p: c.MaxFoodValue() - 2})
	m.Buried[p] = 10

	moved := m.Unbury(p, 10)
	if moved != 2 {
		t.Errorf("moved %d into a cell with room for 2", moved)
	}
	if got := m.Buried[p]; got != 8 {
		t.Errorf("%d left buried, want the 8 that did not fit", got)
	}
}

// digSetup puts a digger at (10,10) facing right with the given amount buried under it and the given amount buried in the cell ahead.
func digSetup(t *testing.T, here, ahead int) (*OrganismManager, *organism.Organism, utils.Point, utils.Point) {
	t.Helper()
	loadDefaultGlobals(t)
	at := utils.Point{X: 10, Y: 10}
	front := utils.Point{X: 11, Y: 10}

	scores := physiology.Scores{}
	scores[physiology.AbilityDigging] = physiology.MaxAbilityScore
	scores[physiology.AbilityChemosynthesis] = physiology.PointTotal - physiology.MaxAbilityScore

	o := organismWithScores(1, at, utils.Point{X: 1, Y: 0}, 20, scores)
	m := &OrganismManager{
		api:            &gridStub{buried: map[utils.Point]int{at: here, front: ahead}},
		organismIDGrid: initializeGrid(),
		organisms:      map[int]*organism.Organism{1: o},
	}
	m.requestManager.ClearMaps()
	m.organismIDGrid[at.X][at.Y] = 1
	return m, o, at, front
}

func TestDiggingUnburiesUnderItself(t *testing.T) {
	m, o, at, front := digSetup(t, 40, 40)
	stub := m.api.(*gridStub)

	m.applyDig(o)

	if stub.foods[at] <= 0 {
		t.Error("digging brought nothing up under the organism")
	}
	if stub.foods[front] != 0 {
		t.Errorf("digging put %d food in the cell ahead; it should not touch it", stub.foods[front])
	}
	if stub.buried[front] != 40 {
		t.Errorf("the buried store ahead changed to %d", stub.buried[front])
	}
}

func TestDiggingASpentHoleProducesNothing(t *testing.T) {
	m, o, at, _ := digSetup(t, 0, 0)
	stub := m.api.(*gridStub)

	before := o.Health
	m.applyDig(o)

	if stub.foods[at] != 0 {
		t.Errorf("a spent hole produced %d food out of nothing", stub.foods[at])
	}
	if o.Health >= before {
		t.Error("the dig was free; it should still cost health")
	}
}

func TestEatingTakesFoodFromUnderTheOrganism(t *testing.T) {
	loadDefaultGlobals(t)
	at := utils.Point{X: 10, Y: 10}
	front := utils.Point{X: 11, Y: 10}

	scores := physiology.Scores{}
	scores[physiology.AbilityEating] = physiology.MaxAbilityScore
	scores[physiology.AbilityChemosynthesis] = physiology.PointTotal - physiology.MaxAbilityScore

	o := organismWithScores(1, at, utils.Point{X: 1, Y: 0}, 20, scores)
	m := &OrganismManager{
		api:            &gridStub{foods: map[utils.Point]int{at: 50, front: 50}},
		organismIDGrid: initializeGrid(),
		organisms:      map[int]*organism.Organism{1: o},
	}
	m.requestManager.ClearMaps()
	m.organismIDGrid[at.X][at.Y] = 1

	m.applyEat(o)

	if o.Status != organism.StatusEatSuccess {
		t.Errorf("status %v, want an eat success on the cell it is standing on", o.Status)
	}
	if got := m.removedFrom(at); got <= 0 {
		t.Error("nothing was eaten from under the organism")
	}
	if got := m.removedFrom(front); got != 0 {
		t.Errorf("%d was taken from the cell ahead; eating is no longer aimed there", got)
	}
}

// removedFrom reports how much food the stub has lost at a point, against the 50 each cell starts with in the eating test above.
func (m *OrganismManager) removedFrom(p utils.Point) int {
	stub := m.api.(*gridStub)
	return 50 - stub.foods[p]
}

func TestRaisingAWallBuriesTheFoodThere(t *testing.T) {
	loadDefaultGlobals(t)
	at := utils.Point{X: 10, Y: 10}
	left := at.Add(utils.Point{X: 0, Y: -1})
	right := at.Add(utils.Point{X: 0, Y: 1})

	scores := physiology.Scores{}
	scores[physiology.AbilityDigging] = physiology.MaxAbilityScore
	scores[physiology.AbilityChemosynthesis] = physiology.PointTotal - physiology.MaxAbilityScore

	g := c.GetCurrentGlobals()
	// Make sure a wall actually goes up on the flanks, whatever the shipped creation settings happen to be.
	g.WallCreatedAtZero = 3
	g.WallCreatedSmall, g.WallCreatedMedium, g.WallCreatedLarge = 3, 3, 3
	c.SetGlobals(g)

	o := organismWithScores(1, at, utils.Point{X: 1, Y: 0}, 20, scores)
	stub := &gridStub{foods: map[utils.Point]int{left: 30, right: 12}}
	m := &OrganismManager{
		api:            stub,
		organismIDGrid: initializeGrid(),
		organisms:      map[int]*organism.Organism{1: o},
	}
	m.requestManager.ClearMaps()
	m.organismIDGrid[at.X][at.Y] = 1

	m.applyDig(o)

	for p, want := range map[utils.Point]int{left: 30, right: 12} {
		if got := stub.foods[p]; got != 0 {
			t.Errorf("%v still has %d food on the surface under a new wall", p, got)
		}
		if got := stub.buried[p]; got != want {
			t.Errorf("%v has %d buried, want the %d that was displaced; "+
				"raising a wall must not destroy food", p, got, want)
		}
	}
}

func TestBuryAllIgnoresTheStoreCap(t *testing.T) {
	p := utils.Point{X: 9, Y: 9}
	m := burialManager(t, map[utils.Point]int{p: 40})
	m.Buried[p] = c.MaxFoodValue()

	moved := m.BuryAllAt(p)

	if moved != 40 {
		t.Errorf("displaced %d of the 40 that was there", moved)
	}
	if _, stillThere := m.Items[p]; stillThere {
		t.Error("food was left on the surface where a wall went up")
	}
	if got, want := m.Buried[p], c.MaxFoodValue()+40; got != want {
		t.Errorf("buried store is %d, want %d; displacement must not destroy the overflow", got, want)
	}
}

func TestUnburyWorksUnderTheDiggerItself(t *testing.T) {
	loadDefaultGlobals(t)
	cell := utils.Point{X: 3, Y: 3}

	api := &foodAPIStub{organisms: map[utils.Point]bool{cell: true}}
	m := NewFoodManager(api, simrand.New(1))
	g := c.GetCurrentGlobals()
	g.InitialFood, g.ChanceToAddFoodItem = 0, 0
	c.SetGlobals(g)
	m.Items = map[utils.Point]*food.Item{}
	m.Buried[cell] = 3

	// One unit, the amount a size-1 organism at Digging 0 raises.
	if moved := m.Unbury(cell, 1); moved != 1 {
		t.Fatalf("unburied %d under a standing organism, want 1", moved)
	}
	if item, ok := m.getFood(cell); !ok || item.Value != 1 {
		t.Error("the unburied unit did not reach the food layer the organism is standing on")
	}
	if got := m.Buried[cell]; got != 2 {
		t.Errorf("%d left buried, want 2", got)
	}

	m.Unbury(cell, 1)
	m.Unbury(cell, 1)
	if _, stillBuried := m.Buried[cell]; stillBuried {
		t.Error("repeated digging did not empty the store")
	}
	if moved := m.Unbury(cell, 1); moved != 0 {
		t.Errorf("a spent hole under an organism yielded %d", moved)
	}
}

// TestFoodStillNeverArrivesUnderAnOrganismByItself: the fix must not have opened the door the guard exists for.
func TestFoodStillNeverArrivesUnderAnOrganismByItself(t *testing.T) {
	loadDefaultGlobals(t)
	cell := utils.Point{X: 4, Y: 4}

	api := &foodAPIStub{organisms: map[utils.Point]bool{cell: true}}
	m := NewFoodManager(api, simrand.New(1))
	g := c.GetCurrentGlobals()
	g.InitialFood, g.ChanceToAddFoodItem = 0, 0
	c.SetGlobals(g)
	m.Items = map[utils.Point]*food.Item{}

	m.AddFoodAtPoint(cell, 50)
	if _, ok := m.getFood(cell); ok {
		t.Error("AddFoodAtPoint dropped food onto a living organism")
	}
}

func TestInitialBuriedFoodSeedsTheGround(t *testing.T) {
	loadDefaultGlobals(t)
	g := c.GetCurrentGlobals()
	g.InitialFood, g.ChanceToAddFoodItem = 0, 0
	g.InitialBuriedFood = 500
	g.MinInitialBuriedValue, g.MaxInitialBuriedValue = 10, 50
	c.SetGlobals(g)

	m := NewFoodManager(&foodAPIStub{}, simrand.New(1))

	cells, total := 0, 0
	for _, v := range m.Buried {
		cells++
		total += v
		if v < 10 {
			t.Errorf("a cell holds %d buried, below the minimum of 10", v)
		}
	}
	if cells == 0 {
		t.Fatal("no cells were seeded with buried food")
	}
	// Cells can coincide, so the count is a ceiling rather than an equality.
	if cells > 500 {
		t.Errorf("seeded %d cells from a count of 500", cells)
	}
	if cells < 400 {
		t.Errorf("only %d of 500 draws landed on distinct cells on an 8000-cell grid; "+
			"that is too many collisions to be chance", cells)
	}
	if total == 0 {
		t.Error("cells were seeded with no food in them")
	}
	// Nothing on the surface: this seeds the buried layer only.
	if len(m.Items) != 0 {
		t.Errorf("%d surface food items appeared; initial_buried_food is the buried layer alone", len(m.Items))
	}
}

func TestInitialBuriedFoodOffDrawsNothing(t *testing.T) {
	loadDefaultGlobals(t)
	g := c.GetCurrentGlobals()
	g.InitialFood, g.ChanceToAddFoodItem = 0, 0
	g.InitialBuriedFood = 0
	c.SetGlobals(g)

	withSetting := simrand.New(7)
	NewFoodManager(&foodAPIStub{}, withSetting)

	// A second RNG from the same seed, untouched.
	untouched := simrand.New(7)
	for i := 0; i < 16; i++ {
		if a, b := withSetting.Intn(1<<30), untouched.Intn(1<<30); a != b {
			t.Fatalf("draw %d differs (%d vs %d): a count of 0 consumed random values", i, a, b)
		}
	}
	if len(NewFoodManager(&foodAPIStub{}, simrand.New(1)).Buried) != 0 {
		t.Error("a count of 0 still seeded buried food")
	}
}
