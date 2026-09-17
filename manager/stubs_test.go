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
}

func (s *gridStub) IsWallAtPoint(p utils.Point) bool { return s.walls[p] }
func (s *gridStub) CheckFoodAtPoint(p utils.Point, check organism.FoodCheck) bool {
	return check(nil, false)
}
func (s *gridStub) GetFoodAtPoint(utils.Point) (*food.Item, bool) { return nil, false }
func (s *gridStub) GetPhAtPoint(utils.Point) float64              { return 0 }
