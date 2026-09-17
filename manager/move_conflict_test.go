package manager

import (
	"testing"

	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// moverInto sets up an organism that has claimed the cell ahead of it,
// the way the decide phase does, with walls wherever walls says.
func moverInto(t *testing.T, walls map[utils.Point]bool) (*OrganismManager, *organism.Organism, utils.Point) {
	t.Helper()
	loadDefaultGlobals(t)
	m := &OrganismManager{api: &gridStub{walls: walls}, organismIDGrid: initializeGrid()}
	m.requestManager.ClearMaps()

	start := utils.Point{X: 10, Y: 10}
	mover := organismWithScores(1, start, utils.Point{X: 1, Y: 0}, 10, physiology.Scores{})
	mover.SetAction(d.ActMove)
	m.organismIDGrid[start.X][start.Y] = mover.ID

	ahead := start.Add(mover.Direction)
	m.requestManager.AddPositionRequest(ahead, mover.ID)
	return m, mover, ahead
}

// TestMoveRecheckedAgainstWallsDugThisCycle is the regression test for
// organisms standing on walls: a mover claims an empty cell in the decide
// phase, another organism digs a wall into it before the mover resolves,
// and the mover must be blocked rather than stepping onto the wall.
func TestMoveRecheckedAgainstWallsDugThisCycle(t *testing.T) {
	walls := map[utils.Point]bool{}
	m, mover, ahead := moverInto(t, walls)

	// The dig lands after the claim, before this organism resolves.
	walls[ahead] = true

	m.applyMove(mover)
	if mover.Location != (utils.Point{X: 10, Y: 10}) {
		t.Errorf("mover stepped to %v, onto a wall dug this cycle", mover.Location)
	}
	if mover.Status != organism.StatusMoveBlocked {
		t.Errorf("status %v, want blocked", mover.Status)
	}
	if id := m.organismIDGrid[ahead.X][ahead.Y]; id == mover.ID {
		t.Error("the mover claimed the walled cell on the grid")
	}
}

// TestMoveIntoClearCellStillWorks: the extra check only stops a move that
// would land on something.
func TestMoveIntoClearCellStillWorks(t *testing.T) {
	m, mover, ahead := moverInto(t, map[utils.Point]bool{})

	m.applyMove(mover)
	if mover.Location != ahead {
		t.Errorf("mover at %v, want %v", mover.Location, ahead)
	}
	if mover.Status != organism.StatusMoveSuccess {
		t.Errorf("status %v, want success", mover.Status)
	}
	if id := m.organismIDGrid[ahead.X][ahead.Y]; id != mover.ID {
		t.Errorf("grid holds %d at the target, want the mover %d", id, mover.ID)
	}
}

// TestMoveBlockedByOrganismArrivingFirst: two claims can't overlap, but a
// cell can still be taken by the time a mover resolves; it must not move
// onto another organism.
func TestMoveBlockedByOrganismArrivingFirst(t *testing.T) {
	m, mover, ahead := moverInto(t, map[utils.Point]bool{})
	m.organismIDGrid[ahead.X][ahead.Y] = 99

	m.applyMove(mover)
	if mover.Location == ahead {
		t.Error("mover stepped onto an occupied cell")
	}
	if mover.Status != organism.StatusMoveBlocked {
		t.Errorf("status %v, want blocked", mover.Status)
	}
}
