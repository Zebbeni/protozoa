package manager

import (
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// gridStub is an organism.API with no walls or food except where given.
type gridStub struct {
	organism.API
	walls map[utils.Point]bool
	// strengths overrides the default per-wall strength, for tests that care how strong a wall is.
	strengths map[utils.Point]int
	// wallUpdates records the cells flagged for the wall layer to repaint.
	wallUpdates map[utils.Point]bool
	// foods is the food items on the grid, keyed by cell, with the value each holds.
	foods map[utils.Point]int
	// buried is the food that has settled out of reach, which only a dig brings back.
	buried map[utils.Point]int
}

func (s *gridStub) GetBuriedFoodAtPoint(p utils.Point) int { return s.buried[p] }

// UnburyFoodAtPoint mirrors FoodManager.Unbury closely enough for the manager tests.
func (s *gridStub) UnburyFoodAtPoint(p utils.Point, value int) int {
	if value <= 0 || s.buried[p] <= 0 {
		return 0
	}
	if value > s.buried[p] {
		value = s.buried[p]
	}
	s.buried[p] -= value
	if s.buried[p] == 0 {
		delete(s.buried, p)
	}
	if s.foods == nil {
		s.foods = map[utils.Point]int{}
	}
	s.foods[p] += value
	return value
}

func (s *gridStub) AddWallUpdate(p utils.Point) {
	if s.wallUpdates == nil {
		s.wallUpdates = map[utils.Point]bool{}
	}
	s.wallUpdates[p] = true
}

func (s *gridStub) IsWallAtPoint(p utils.Point) bool { return s.walls[p] }

// GetWallStrengthAtPoint and AddWallStrength exist because moving is now allowed to go *through* a wall an organism can burrow.
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
	if v, ok := s.foods[p]; ok {
		item := food.NewItem(p, v)
		return check(item, true)
	}
	return check(nil, false)
}

func (s *gridStub) GetFoodAtPoint(p utils.Point) (*food.Item, bool) {
	if v, ok := s.foods[p]; ok {
		return food.NewItem(p, v), true
	}
	return nil, false
}

// RemoveFoodAtPoint / AddFoodAtPoint / AddPhChangeAtPoint / AddFoodUpdate make the food layer writable.
func (s *gridStub) RemoveFoodAtPoint(p utils.Point, value int) {
	if s.foods == nil {
		return
	}
	s.foods[p] -= value
	if s.foods[p] <= 0 {
		delete(s.foods, p)
	}
}

func (s *gridStub) AddFoodAtPoint(p utils.Point, value int) {
	if s.foods == nil {
		s.foods = map[utils.Point]int{}
	}
	s.foods[p] += value
}

func (s *gridStub) BuryAllFoodAtPoint(p utils.Point) int {
	moved := s.foods[p]
	if moved <= 0 {
		return 0
	}
	delete(s.foods, p)
	if s.buried == nil {
		s.buried = map[utils.Point]int{}
	}
	s.buried[p] += moved
	return moved
}

func (s *gridStub) AddPhChangeAtPoint(utils.Point, float64) {}
func (s *gridStub) AddFoodUpdate(utils.Point)               {}

func (s *gridStub) GetPhAtPoint(utils.Point) float64 { return 0 }
