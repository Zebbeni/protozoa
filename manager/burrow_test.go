package manager

import (
	"math"
	"testing"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// burrowSetup puts a digger at (10,10) facing right, with a wall of the given strength in the cell ahead.
func burrowSetup(t *testing.T, digging int, size float64, wallStrength int) (*OrganismManager, *organism.Organism) {
	t.Helper()
	loadDefaultGlobals(t)
	// The burrow endpoints are set here rather than taken from the shipped
	// settings: these tests are about what a burrow DOES — the hole, the
	// spoil, the repaint — and a configuration that makes the test's wall
	// unbreakable fails them for a reason that has nothing to do with any
	// of that. The wall strengths below are chosen against these.
	bg := c.GetCurrentGlobals()
	bg.WallBreakAtZeroDigging = 0
	bg.WallBreakAtMaxDigging = 10
	bg.WallBreakMultiplier = 1
	c.SetGlobals(bg)
	ahead := utils.Point{X: 11, Y: 10}

	scores := physiology.Scores{}
	scores[physiology.AbilityDigging] = digging
	scores[physiology.AbilityChemosynthesis] = physiology.PointTotal - digging

	digger := organismWithScores(1, utils.Point{X: 10, Y: 10}, utils.Point{X: 1, Y: 0}, size, scores)
	m := &OrganismManager{
		api: &gridStub{
			walls:     map[utils.Point]bool{ahead: true},
			strengths: map[utils.Point]int{ahead: wallStrength},
		},
		organismIDGrid: initializeGrid(),
		organisms:      map[int]*organism.Organism{1: digger},
	}
	m.requestManager.ClearMaps()
	m.organismIDGrid[10][10] = 1
	return m, digger
}

func TestBurrowingMovesThroughAndDestroysTheWall(t *testing.T) {
	ahead := utils.Point{X: 11, Y: 10}
	m, digger := burrowSetup(t, physiology.MaxAbilityScore, 20, 5)

	// The decide phase has to stake the claim, or the resolve phase treats the move as walking into something already there.
	if !m.canOccupy(digger, ahead) {
		t.Fatal("a breakable wall should be a cell the digger can claim")
	}
	m.requestManager.AddPositionRequest(ahead, digger.ID, digger.Size)

	before := digger.Health
	m.applyMove(digger)

	if digger.Status != organism.StatusMoveSuccess {
		t.Errorf("status %v, want success", digger.Status)
	}
	if digger.Location != ahead {
		t.Errorf("ended at %v, want %v", digger.Location, ahead)
	}
	if m.api.IsWallAtPoint(ahead) {
		t.Errorf("the wall survived at strength %d; burrowing destroys it outright",
			m.api.GetWallStrengthAtPoint(ahead))
	}
	// No blocked-move penalty: it didn't misread anything.
	if got, want := digger.Health-before, moveCost(digger); math.Abs(got-want) > 1e-9 {
		t.Errorf("health changed by %.4f, want just the move cost %.4f", got, want)
	}
}

func TestWallTooStrongToBurrowStillBlocks(t *testing.T) {
	ahead := utils.Point{X: 11, Y: 10}
	m, digger := burrowSetup(t, 1, 1, MaxWallStrength)

	if m.canOccupy(digger, ahead) {
		t.Fatal("a wall far beyond the digger should not be claimable")
	}

	m.applyMove(digger)
	if digger.Status != organism.StatusMoveBlocked {
		t.Errorf("status %v, want blocked", digger.Status)
	}
	if digger.Location == ahead {
		t.Error("the digger moved into a wall it can't break")
	}
	if !m.api.IsWallAtPoint(ahead) {
		t.Error("the wall was destroyed by an organism that couldn't break it")
	}
}

func TestBurrowRecheckedAtResolveTime(t *testing.T) {
	ahead := utils.Point{X: 11, Y: 10}
	m, digger := burrowSetup(t, 2, 2, 1)

	if !m.canOccupy(digger, ahead) {
		t.Fatal("the digger should be able to claim a strength-1 wall")
	}
	m.requestManager.AddPositionRequest(ahead, digger.ID, digger.Size)

	m.api.AddWallStrength(ahead, MaxWallStrength)

	m.applyMove(digger)
	if digger.Status != organism.StatusMoveBlocked {
		t.Errorf("status %v, want blocked by the reinforced wall", digger.Status)
	}
	if digger.Location == ahead {
		t.Error("the digger stepped onto a wall that grew out of its reach")
	}
}

func TestCanMoveAndIsWallAheadDisagree(t *testing.T) {
	loadDefaultGlobals(t)
	ahead := utils.Point{X: 11, Y: 10}
	api := &gridStub{
		walls:     map[utils.Point]bool{ahead: true},
		strengths: map[utils.Point]int{ahead: 5},
	}

	scores := physiology.Scores{}
	scores[physiology.AbilityDigging] = physiology.MaxAbilityScore
	scores[physiology.AbilityChemosynthesis] = physiology.PointTotal - physiology.MaxAbilityScore
	// Built with the API attached, since these are the organism's own condition lookups rather than the manager's.
	digger := organism.Restore(1, 1, 20, 20, 0, 0, 0,
		utils.Point{X: 10, Y: 10}, utils.Point{X: 1, Y: 0}, 1,
		organism.Traits{Abilities: scores}, nil, d.ActIdle, organism.StatusIdle, 0, 0, 0, 0, api)

	if !digger.CanBurrowAhead() {
		t.Fatal("this digger should get through a strength-5 wall")
	}
	if !api.IsWallAtPoint(ahead) {
		t.Error("IsWallAhead must stay true for a burrower: there is still a wall there")
	}
}

func TestBurrowRepaintsTheWallItDestroyed(t *testing.T) {
	ahead := utils.Point{X: 11, Y: 10}
	m, digger := burrowSetup(t, 2, 2, 1)
	stub := m.api.(*gridStub)

	m.requestManager.AddPositionRequest(ahead, digger.ID, digger.Size)
	m.applyMove(digger)

	if digger.Location != ahead {
		t.Fatalf("the digger didn't get through: at %v, status %v", digger.Location, digger.Status)
	}
	if m.api.GetWallStrengthAtPoint(ahead) != 0 {
		t.Errorf("the wall survived at strength %d", m.api.GetWallStrengthAtPoint(ahead))
	}
	if !stub.wallUpdates[ahead] {
		t.Error("the destroyed wall's cell was never flagged for the wall layer, " +
			"so the sprite stays up and the organism renders on top of it")
	}
}

func TestBurrowingIsTheOnlyWayThroughAWall(t *testing.T) {
	ahead := utils.Point{X: 11, Y: 10}

	dug, digger := burrowSetup(t, 2, 2, 1)
	ds := dug.api.(*gridStub)
	before := dug.api.GetWallStrengthAtPoint(ahead)
	dug.applyDig(digger)
	if after := dug.api.GetWallStrengthAtPoint(ahead); after != before {
		t.Errorf("digging moved the wall ahead from %d to %d; only burrowing goes through a wall now",
			before, after)
	}
	if ds.wallUpdates[ahead] {
		t.Error("digging flagged the wall cell it no longer changes")
	}

	// Burrowing still does, and still flags the layer so the sprite goes.
	burrowed, burrower := burrowSetup(t, 2, 2, 1)
	bs := burrowed.api.(*gridStub)
	burrowed.requestManager.AddPositionRequest(ahead, burrower.ID, burrower.Size)
	burrowed.applyMove(burrower)
	if burrowed.api.GetWallStrengthAtPoint(ahead) != 0 {
		t.Error("burrowing left the wall standing")
	}
	if !bs.wallUpdates[ahead] {
		t.Error("burrowing left the wall cell unflagged, so the sprite stays up")
	}
}

// spoilSetup burrows a digger through a wall of the given strength with the spoil fraction set.
func spoilSetup(t *testing.T, wallStrength int, fraction float64) (*OrganismManager, *organism.Organism, utils.Point) {
	t.Helper()
	m, digger := burrowSetup(t, 10, 10, wallStrength)
	g := c.GetCurrentGlobals()
	g.BurrowSpoilFraction = fraction
	c.SetGlobals(g)

	ahead := digger.Location.Add(digger.Direction)
	m.requestManager.AddPositionRequest(ahead, digger.ID, digger.Size)
	m.applyMove(digger)
	if digger.Location != ahead {
		t.Fatalf("the digger didn't get through: at %v, status %v", digger.Location, digger.Status)
	}
	return m, digger, ahead
}

func TestBurrowPilesSpoilOnBothFlanks(t *testing.T) {
	const strength = 40
	m, digger, ahead := spoilSetup(t, strength, 0.5)

	left := ahead.Add(digger.Direction.Left())
	right := ahead.Add(digger.Direction.Right())
	want := effects.BurrowSpoilPerSide(c.GetCurrentGlobals(), strength)
	if want <= 0 {
		t.Fatal("the test's fraction produced no spoil; it checks nothing")
	}
	for name, side := range map[string]utils.Point{"left": left, "right": right} {
		if got := m.api.GetWallStrengthAtPoint(side); got != want {
			t.Errorf("%s flank holds %d spoil, want %d", name, got, want)
		}
	}
	// Equal banks either side, not a remainder dumped on one.
	if m.api.GetWallStrengthAtPoint(left) != m.api.GetWallStrengthAtPoint(right) {
		t.Error("the two banks differ; a left-hand bias would show in every tunnel")
	}
	if got := m.api.GetWallStrengthAtPoint(ahead); got != 0 {
		t.Errorf("the tunnel holds %d wall; the burrow should have cleared it", got)
	}
}

func TestBurrowSpoilSkipsOccupiedCells(t *testing.T) {
	m, digger := burrowSetup(t, 10, 10, 40)
	g := c.GetCurrentGlobals()
	g.BurrowSpoilFraction = 0.5
	c.SetGlobals(g)

	ahead := digger.Location.Add(digger.Direction)
	left := ahead.Add(digger.Direction.Left())
	right := ahead.Add(digger.Direction.Right())

	// Put a bystander on the left flank.
	m.gridMutex.Lock()
	m.organismIDGrid[left.X][left.Y] = 2
	m.gridMutex.Unlock()

	m.requestManager.AddPositionRequest(ahead, digger.ID, digger.Size)
	m.applyMove(digger)

	if got := m.api.GetWallStrengthAtPoint(left); got != 0 {
		t.Errorf("spoil of %d was piled onto a cell holding another organism", got)
	}
	if got := m.api.GetWallStrengthAtPoint(right); got <= 0 {
		t.Error("the free flank got no spoil; one blocked side shouldn't cancel the other")
	}
}

func TestBurrowSpoilFractionZeroDestroysTheWall(t *testing.T) {
	m, digger, ahead := spoilSetup(t, 40, 0)
	for _, side := range []utils.Point{
		ahead.Add(digger.Direction.Left()),
		ahead.Add(digger.Direction.Right()),
	} {
		if got := m.api.GetWallStrengthAtPoint(side); got != 0 {
			t.Errorf("with a fraction of 0 a flank still gained %d wall", got)
		}
	}
}

func TestBurrowSpoilFlagsTheWallLayer(t *testing.T) {
	m, digger, ahead := spoilSetup(t, 40, 0.5)
	stub := m.api.(*gridStub)
	for name, side := range map[string]utils.Point{
		"left":  ahead.Add(digger.Direction.Left()),
		"right": ahead.Add(digger.Direction.Right()),
	} {
		if !stub.wallUpdates[side] {
			t.Errorf("the %s bank was never flagged, so its new wall never draws", name)
		}
	}
}
