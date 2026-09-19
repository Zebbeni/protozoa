package manager

import (
	"math"
	"testing"

	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// burrowSetup puts a digger at (10,10) facing right, with a wall of the
// given strength in the cell ahead.
func burrowSetup(t *testing.T, digging int, size float64, wallStrength int) (*OrganismManager, *organism.Organism) {
	t.Helper()
	loadDefaultGlobals(t)
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

// TestBurrowingMovesThroughAndDestroysTheWall is the mechanic end to
// end: a digger strong enough for the wall in front of it claims that
// cell in the decide phase, steps into it, and leaves no wall behind.
// Destroyed outright rather than worn down — that is what separates
// burrowing from digging.
func TestBurrowingMovesThroughAndDestroysTheWall(t *testing.T) {
	ahead := utils.Point{X: 11, Y: 10}
	m, digger := burrowSetup(t, physiology.MaxAbilityScore, 20, 5)

	// The decide phase has to stake the claim, or the resolve phase
	// treats the move as walking into something already there.
	if !m.canOccupy(digger, ahead) {
		t.Fatal("a breakable wall should be a cell the digger can claim")
	}
	m.requestManager.AddPositionRequest(ahead, digger.ID)

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

// TestWallTooStrongToBurrowStillBlocks: the threshold has to actually
// stop something, or burrowing is just "walls don't exist".
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

// TestBurrowRecheckedAtResolveTime: the claim is staked against the
// world as it was in the decide phase. Another organism resolving
// earlier this cycle can raise the wall out of reach, and the mover has
// to notice rather than stepping onto it.
func TestBurrowRecheckedAtResolveTime(t *testing.T) {
	ahead := utils.Point{X: 11, Y: 10}
	m, digger := burrowSetup(t, 2, 2, 1)

	if !m.canOccupy(digger, ahead) {
		t.Fatal("the digger should be able to claim a strength-1 wall")
	}
	m.requestManager.AddPositionRequest(ahead, digger.ID)

	// Someone reinforces it before this organism resolves.
	m.api.AddWallStrength(ahead, MaxWallStrength)

	m.applyMove(digger)
	if digger.Status != organism.StatusMoveBlocked {
		t.Errorf("status %v, want blocked by the reinforced wall", digger.Status)
	}
	if digger.Location == ahead {
		t.Error("the digger stepped onto a wall that grew out of its reach")
	}
}

// TestCanMoveAndIsWallAheadDisagree is the condition pair the tree sees:
// a burrower reads CanMove true *and* IsWallAhead true. They answer
// different questions — "is there a wall" and "can I get through it" —
// and collapsing them would make the second unaskable.
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
	// Built with the API attached, since these are the organism's own
	// condition lookups rather than the manager's.
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
