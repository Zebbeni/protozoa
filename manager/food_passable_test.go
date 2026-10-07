package manager

import (
	"github.com/Zebbeni/protozoa/effects"
	"testing"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// foodSetup puts an organism at (10,10) facing right with a food pile in the cell ahead.
func foodSetup(t *testing.T) (*OrganismManager, *organism.Organism, utils.Point) {
	t.Helper()
	loadDefaultGlobals(t)

	ahead := utils.Point{X: 11, Y: 10}
	scores := physiology.Scores{}
	scores[physiology.AbilityChemosynthesis] = physiology.PointTotal

	o := organismWithScores(1, utils.Point{X: 10, Y: 10}, utils.Point{X: 1, Y: 0}, 20, scores)
	m := &OrganismManager{
		api:            &gridStub{foods: map[utils.Point]int{ahead: 40}},
		organismIDGrid: initializeGrid(),
		organisms:      map[int]*organism.Organism{1: o},
	}
	m.requestManager.ClearMaps()
	m.organismIDGrid[10][10] = 1
	return m, o, ahead
}

func TestFoodCanBeWalkedOnto(t *testing.T) {
	m, o, ahead := foodSetup(t)
	if !m.canOccupy(o, ahead) {
		t.Fatal("a cell holding food should be claimable")
	}
	m.requestManager.AddPositionRequest(ahead, o.ID, o.Size)

	m.applyMove(o)

	if o.Status != organism.StatusMoveSuccess {
		t.Errorf("status %v, want success", o.Status)
	}
	if o.Location != ahead {
		t.Errorf("ended at %v, want %v", o.Location, ahead)
	}
	if m.organismIDGrid[ahead.X][ahead.Y] != o.ID {
		t.Error("the organism grid was not updated to the food cell")
	}
	// Walking over a pile is not eating it.
	if _, found := m.api.GetFoodAtPoint(ahead); !found {
		t.Error("the food vanished when the organism stepped on it")
	}
}

func TestWalkingOntoFoodIsNotChargedAsBlocked(t *testing.T) {
	m, o, ahead := foodSetup(t)
	m.requestManager.AddPositionRequest(ahead, o.ID, o.Size)

	before := o.Health
	m.applyMove(o)
	spent := before - o.Health

	// Against the move cost itself, not against the penalty alone: the move
	// cost can exceed the penalty under some settings, and then "spent less
	// than the penalty" is unsatisfiable however the penalty is charged.
	// What this is about is whether the PENALTY was added on top.
	moveCost := -effects.MoveCost(c.GetCurrentGlobals(),
		o.Abilities()[physiology.AbilityMovement], o.Size)
	blockPenalty := -c.HealthChangeFromBlockedMove() * o.Size
	if blockPenalty <= 0 {
		t.Fatalf("expected a positive blocked-move penalty magnitude, got %v", blockPenalty)
	}
	if spent > moveCost+blockPenalty/2 {
		t.Errorf("spent %v health against a %v move cost, so the %v blocked-move "+
			"penalty was charged; stepping onto food is not a misread",
			spent, moveCost, blockPenalty)
	}
}

func TestFoodDoesNotPenAnOrganismIn(t *testing.T) {
	m, o, _ := foodSetup(t)
	stub := m.api.(*gridStub)
	for _, d := range []utils.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		stub.foods[o.Location.Add(d)] = 30
	}
	for _, d := range []utils.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		if !m.canOccupy(o, o.Location.Add(d)) {
			t.Errorf("ringed by food, the cell at %v is still closed", d)
		}
	}
	if _, _, found := m.getChildSpawnLocation(o); !found {
		t.Error("a parent ringed by food has nowhere to put a child")
	}
}

func TestFoodStillBlocksNothingButOrganismsDo(t *testing.T) {
	m, o, ahead := foodSetup(t)
	other := organismWithScores(2, ahead, utils.Point{X: 1, Y: 0}, 20, o.Traits().Abilities)
	m.organisms[2] = other
	m.organismIDGrid[ahead.X][ahead.Y] = 2

	if m.canOccupy(o, ahead) {
		t.Error("an occupied cell was claimable")
	}
	if m.canPlaceOrganismAt(ahead) {
		t.Error("a child would be placed onto a living organism")
	}
}

// TestWallsStillBlockWhatCannotBurrow: the food change must not have loosened the wall rule along with it.
func TestWallsStillBlockWhatCannotBurrow(t *testing.T) {
	m, o, ahead := foodSetup(t)
	stub := m.api.(*gridStub)
	stub.walls = map[utils.Point]bool{ahead: true}
	stub.strengths = map[utils.Point]int{ahead: MaxWallStrength}

	if m.canOccupy(o, ahead) {
		t.Error("a full-strength wall was claimable by a non-digger")
	}
	if m.canPlaceOrganismAt(ahead) {
		t.Error("a child would be placed inside a wall")
	}
}

func TestIsGridLocationEmptyStillCountsFood(t *testing.T) {
	m, _, ahead := foodSetup(t)
	if m.isGridLocationEmpty(ahead) {
		t.Error("isGridLocationEmpty ignored food; applyDig depends on it not doing that")
	}
	if !m.canPlaceOrganismAt(ahead) {
		t.Error("canPlaceOrganismAt should allow a cell holding only food")
	}
}
