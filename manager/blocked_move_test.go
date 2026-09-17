package manager

import (
	"math"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// blockedMoveSetup puts a mover at (10,10) facing right, with whatever
// the caller queued into the request map already in place.
func blockedMoveSetup(t *testing.T, walls map[utils.Point]bool) (*OrganismManager, *organism.Organism) {
	t.Helper()
	loadDefaultGlobals(t)
	scores := physiology.Scores{}
	scores[physiology.AbilityMovement] = 50
	scores[physiology.AbilityChemosynthesis] = physiology.PointTotal - 50
	mover := organismWithScores(1, utils.Point{X: 10, Y: 10}, utils.Point{X: 1, Y: 0}, 10, scores)
	m := &OrganismManager{
		api:            &gridStub{walls: walls},
		organismIDGrid: initializeGrid(),
		organisms:      map[int]*organism.Organism{1: mover},
	}
	m.requestManager.ClearMaps()
	m.organismIDGrid[10][10] = 1
	return m, mover
}

// moveCost is what a move charges before any blocked-move penalty.
func moveCost(o *organism.Organism) float64 {
	return effects.MoveCost(config.GetCurrentGlobals(), o.Abilities()[physiology.AbilityMovement], o.Size)
}

// TestBlockedMoveChargesThePenalty: walking into something that was
// already there costs the penalty on top of the move.
func TestBlockedMoveChargesThePenalty(t *testing.T) {
	// A wall ahead means nobody claimed the cell in the decide phase.
	m, mover := blockedMoveSetup(t, map[utils.Point]bool{{X: 11, Y: 10}: true})
	before := mover.Health
	m.applyMove(mover)

	penalty := config.HealthChangeFromBlockedMove() * mover.Size
	want := moveCost(mover) + penalty
	if got := mover.Health - before; math.Abs(got-want) > 1e-9 {
		t.Errorf("blocked by a wall: health changed by %.4f, want %.4f (move %.4f + penalty %.4f)",
			got, want, moveCost(mover), penalty)
	}

	// The penalty is per unit of size, like every other health change,
	// so a bigger organism pays proportionally more for the same misread.
	m2, big := blockedMoveSetup(t, map[utils.Point]bool{{X: 11, Y: 10}: true})
	big.Size = 40
	beforeBig := big.Health
	m2.applyMove(big)
	wantBig := moveCost(big) + config.HealthChangeFromBlockedMove()*big.Size
	if got := big.Health - beforeBig; math.Abs(got-wantBig) > 1e-9 {
		t.Errorf("size 40 blocked by a wall: health changed by %.4f, want %.4f", got, wantBig)
	}
	if mover.Status != organism.StatusMoveBlocked {
		t.Errorf("status %v, want blocked", mover.Status)
	}
}

// TestLosingARaceCostsNoPenalty: the cell was open when the organism
// decided and someone else reached it first. It had no way to know, so
// it pays the move and nothing more.
func TestLosingARaceCostsNoPenalty(t *testing.T) {
	m, mover := blockedMoveSetup(t, nil)
	// Another organism claimed the same empty cell and won it.
	m.requestManager.AddPositionRequest(utils.Point{X: 11, Y: 10}, 2)

	before := mover.Health
	m.applyMove(mover)

	want := moveCost(mover)
	if got := mover.Health - before; math.Abs(got-want) > 1e-9 {
		t.Errorf("lost a race: health changed by %.4f, want just the move cost %.4f", got, want)
	}
	if mover.Status != organism.StatusMoveBlocked {
		t.Errorf("status %v, want blocked", mover.Status)
	}
}

// TestSuccessfulMoveCostsNoPenalty pins the other side: the penalty is
// for a misread, not for moving.
func TestSuccessfulMoveCostsNoPenalty(t *testing.T) {
	m, mover := blockedMoveSetup(t, nil)
	m.requestManager.AddPositionRequest(utils.Point{X: 11, Y: 10}, mover.ID)

	before := mover.Health
	m.applyMove(mover)

	if got, want := mover.Health-before, moveCost(mover); math.Abs(got-want) > 1e-9 {
		t.Errorf("clear move: health changed by %.4f, want just the move cost %.4f", got, want)
	}
	if mover.Status != organism.StatusMoveSuccess {
		t.Errorf("status %v, want success", mover.Status)
	}
}
