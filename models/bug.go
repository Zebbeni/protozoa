package models

import (
	"math/rand"

	c "../constants"
)

// Action is the custom type for all Bug actions
type Action int
type Condition int

// Define all possible actions for Bug
const (
	ActEat Action = iota
	ActIdle
	ActMove
	ActTurnLeft
	ActTurnRight
	IsCold Condition = iota
)

// Bug has stuff
// - location (X, Y)
// - direction (angle)
// - current action (Action)
// - algorithm code (String? or []int?)
// - algorithm (func)
type Bug struct {
	X, Y, DirX, DirY int
	CurrentAction    Action
}

// NewBug initializes bug at with random location and direction on grid
func NewBug() Bug {
	x := rand.Intn(c.GridWidth)
	y := rand.Intn(c.GridHeight)
	dirX := rand.Intn(2)*2 - 1
	dirY := rand.Intn(2)*2 - 1
	bug := Bug{X: x, Y: y, DirX: dirX, DirY: dirY}
	return bug
}

// UpdateAction sets bug's CurrentAction
func (bug *Bug) UpdateAction() {
	bug.CurrentAction = ActIdle
}

// isAction returns true if the object passed in is an Action
func isAction(v interface{}) bool {
	switch v.(type) {
	case Action:
		return true
	default:
		return false
	}
}
