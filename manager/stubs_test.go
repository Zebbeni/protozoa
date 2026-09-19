package manager

import (
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// gridStub is an organism.API with no walls or food except where given.
// Any other call panics via the nil embedded interface.
type gridStub struct {
	organism.API
	walls map[utils.Point]bool
	// strengths overrides the default per-wall strength, for tests that
	// care how strong a wall is rather than only that one is there.
	strengths map[utils.Point]int
}

func (s *gridStub) IsWallAtPoint(p utils.Point) bool { return s.walls[p] }

// GetWallStrengthAtPoint and AddWallStrength exist because moving is now
// allowed to go *through* a wall an organism can burrow. A wall with no
// strength recorded reports the maximum, so a test that only says "there
// is a wall here" gets one nothing can shoulder through — which is what
// every such test meant before burrowing existed.
func (s *gridStub) GetWallStrengthAtPoint(p utils.Point) int {
	if st, ok := s.strengths[p]; ok {
		return st
	}
	if s.walls[p] {
		return MaxWallStrength
	}
	return 0
}

func (s *gridStub) AddWallStrength(p utils.Point, delta int) int {
	if s.strengths == nil {
		s.strengths = map[utils.Point]int{}
	}
	st := s.GetWallStrengthAtPoint(p) + delta
	if st <= 0 {
		st = 0
		delete(s.walls, p)
	}
	s.strengths[p] = st
	return st
}
func (s *gridStub) CheckFoodAtPoint(p utils.Point, check organism.FoodCheck) bool {
	return check(nil, false)
}
func (s *gridStub) GetFoodAtPoint(utils.Point) (*food.Item, bool) { return nil, false }
func (s *gridStub) GetPhAtPoint(utils.Point) float64              { return 0 }
