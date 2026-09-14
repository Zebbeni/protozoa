package manager

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// gridStub is an organism.API with no walls or food except where given.
// Any other call panics via the nil embedded interface.
type gridStub struct {
	organism.API
	walls map[utils.Point]bool
}

func (s *gridStub) IsWallAtPoint(p utils.Point) bool { return s.walls[p] }
func (s *gridStub) CheckFoodAtPoint(p utils.Point, check organism.FoodCheck) bool {
	return check(nil, false)
}
func (s *gridStub) GetFoodAtPoint(utils.Point) (*food.Item, bool) { return nil, false }

// shoveSetup places a mover facing east at (10,10) and returns the manager
// and the cell ahead. occupied puts another organism in that cell.
func shoveSetup(t *testing.T, size float64, occupied bool, walls map[utils.Point]bool) (*OrganismManager, *organism.Organism, utils.Point) {
	t.Helper()
	loadDefaultGlobals(t)
	m := &OrganismManager{api: &gridStub{walls: walls}, organismIDGrid: initializeGrid()}
	m.requestManager.ClearMaps()

	mover := &organism.Organism{ID: 1, Location: utils.Point{X: 10, Y: 10}, Direction: utils.Point{X: 1, Y: 0}, Size: size}
	mover.SetAction(d.ActMove)
	m.organismIDGrid[10][10] = mover.ID
	ahead := mover.Location.Add(mover.Direction)
	if occupied {
		m.organismIDGrid[ahead.X][ahead.Y] = 2
	}
	return m, mover, ahead
}

// TestLargeMoverShovesOccupant: a large organism moving into an occupied
// cell queues damage on it, tagged with the mover as the source so
// Defense and thorns treat it like an attack.
func TestLargeMoverShovesOccupant(t *testing.T) {
	loadDefaultGlobals(t)
	m, mover, ahead := shoveSetup(t, config.MaximumMaxSize(), true, nil)
	m.updateRequestMapTo(mover, &m.requestManager)

	effects := m.requestManager.GetHealthEffects(ahead)
	if len(effects) != 1 {
		t.Fatalf("expected one shove effect on the occupant, got %v", effects)
	}
	want := config.HealthChangeInflictedByShoving() * mover.Size * mover.AbilityMultiplier(physiology.AbilityDigging)
	if effects[0].Amount != want || want >= 0 {
		t.Errorf("shove amount %.3f, want %.3f (negative)", effects[0].Amount, want)
	}
	if effects[0].SourceID != mover.ID {
		t.Errorf("shove source %d, want the mover %d", effects[0].SourceID, mover.ID)
	}
	if m.requestManager.GetPositionRequest(ahead) != 0 {
		t.Error("a shove must not also claim the occupied cell")
	}
}

// TestOnlyLargeOrganismsShove: small and medium movers just get blocked.
func TestOnlyLargeOrganismsShove(t *testing.T) {
	loadDefaultGlobals(t)
	for _, size := range []float64{1, config.MaximumMaxSize() * 0.5} {
		m, mover, ahead := shoveSetup(t, size, true, nil)
		m.updateRequestMapTo(mover, &m.requestManager)
		if effects := m.requestManager.GetHealthEffects(ahead); len(effects) != 0 {
			t.Errorf("size %.0f mover should not shove, got %v", size, effects)
		}
	}
}

// TestShoveNeedsAnOccupant: moving into empty space claims it, and pushing
// against a wall damages the wall (handled at resolve), not a phantom
// organism.
func TestShoveNeedsAnOccupant(t *testing.T) {
	loadDefaultGlobals(t)
	m, mover, ahead := shoveSetup(t, config.MaximumMaxSize(), false, nil)
	m.updateRequestMapTo(mover, &m.requestManager)
	if len(m.requestManager.GetHealthEffects(ahead)) != 0 {
		t.Error("moving into an empty cell should not shove")
	}
	if m.requestManager.GetPositionRequest(ahead) != mover.ID {
		t.Error("moving into an empty cell should request it")
	}

	wallAhead := map[utils.Point]bool{{X: 11, Y: 10}: true}
	m, mover, ahead = shoveSetup(t, config.MaximumMaxSize(), false, wallAhead)
	m.updateRequestMapTo(mover, &m.requestManager)
	if len(m.requestManager.GetHealthEffects(ahead)) != 0 {
		t.Error("pushing against a wall should not queue organism damage")
	}
}

func (s *gridStub) GetPhAtPoint(utils.Point) float64 { return 0 }

// TestShoveLandsThroughDefense follows a shove from the mover's request to
// the occupant's health: it lands scaled by the occupant's Defense
// multiplier, the same path attack damage takes.
func TestShoveLandsThroughDefense(t *testing.T) {
	loadDefaultGlobals(t)
	m, mover, ahead := shoveSetup(t, config.MaximumMaxSize(), true, nil)
	m.updateRequestMapTo(mover, &m.requestManager)

	occupant := &organism.Organism{ID: 2, Location: ahead, Size: 50, Health: 50}
	m.applyCycleHealthChanges(occupant)

	shove, _ := shoveEffect(mover)
	want := 50 + shove*defenseDamageMult(occupant.Abilities())
	if diff := occupant.Health - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("occupant health %.3f after shove, want %.3f", occupant.Health, want)
	}
	if occupant.Health >= 50 {
		t.Error("the shove did no damage")
	}
}
