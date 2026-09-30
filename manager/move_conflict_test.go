package manager

import (
	"testing"

	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// moverInto sets up an organism that has claimed the cell ahead of it, the way the decide phase does.
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
	m.requestManager.AddPositionRequest(ahead, mover.ID, mover.Size)
	return m, mover, ahead
}

func TestMoveRecheckedAgainstWallsDugThisCycle(t *testing.T) {
	walls := map[utils.Point]bool{}
	m, mover, ahead := moverInto(t, walls)

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

func TestTheLargestClaimantWinsTheCell(t *testing.T) {
	cell := utils.Point{X: 4, Y: 4}

	type claim struct {
		id   int
		size float64
	}
	for _, tc := range []struct {
		name     string
		a, b     claim
		wantWins int
	}{
		{"bigger wins over younger", claim{id: 1, size: 30}, claim{id: 2, size: 10}, 1},
		{"bigger wins over older", claim{id: 9, size: 30}, claim{id: 2, size: 10}, 9},
		{"equal size falls back to the higher ID", claim{id: 3, size: 20}, claim{id: 7, size: 20}, 7},
	} {
		for _, swap := range []bool{false, true} {
			first, second := tc.a, tc.b
			if swap {
				first, second = tc.b, tc.a
			}
			var rm RequestManager
			rm.ClearMaps()
			rm.AddPositionRequest(cell, first.id, first.size)
			rm.AddPositionRequest(cell, second.id, second.size)
			if got := rm.GetPositionRequest(cell); got != tc.wantWins {
				t.Errorf("%s (claimed %d then %d): cell went to %d, want %d",
					tc.name, first.id, second.id, got, tc.wantWins)
			}
		}
	}
}

func TestAnUnclaimedCellReportsNoClaimant(t *testing.T) {
	var rm RequestManager
	rm.ClearMaps()

	never := utils.Point{X: 1, Y: 2}
	if got := rm.GetPositionRequest(never); got != -1 {
		t.Errorf("an unclaimed cell reports claimant %d, want -1", got)
	}
	if rm.HasPositionRequest(never) {
		t.Error("an unclaimed cell reports having a claim")
	}

	rm.AddPositionRequest(never, 0, 5)
	if got := rm.GetPositionRequest(never); got != 0 {
		t.Errorf("organism 0's real claim reports %d; 0 has to be a usable ID", got)
	}
}
