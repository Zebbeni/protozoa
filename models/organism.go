package models

import (
	"fmt"
	"math/rand"

	c "../constants"
	d "../decisions"
)

// Organism has stuff
// - location (X, Y)
// - direction (angle)
// - current action (Action)
// - algorithm code (String? or []int?)
// - algorithm (func)
type Organism struct {
	X, Y, DirX, DirY, Score int
	DecisionSequence        d.Sequence
	DecisionTree            d.Node
}

// NewOrganism initializes organism at with random grid location and direction
func NewOrganism() Organism {
	x := rand.Intn(c.GridWidth)
	y := rand.Intn(c.GridHeight)
	dirX := 1
	dirY := 0
	decisionSequence := d.NewRandomSequence()
	decisionNode := d.TreeFromSequence(decisionSequence)
	organism := Organism{
		X: x, Y: y, DirX: dirX, DirY: dirY,
		DecisionSequence: decisionSequence,
		DecisionTree:     decisionNode,
	}
	return organism
}

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	Environment *Environment
	Organisms   [c.NumOrganisms]Organism
	Grid        [c.GridWidth][c.GridHeight]bool
}

// NewOrganismManager creates all Organisms and updates grid
func NewOrganismManager(environment *Environment) OrganismManager {
	organismManager := OrganismManager{Environment: environment}
	for i := range organismManager.Organisms {
		organism := NewOrganism()
		organismManager.Organisms[i] = organism
		organismManager.Grid[organism.X][organism.Y] = true
	}
	return organismManager
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (om *OrganismManager) Update() {
	for o, organism := range om.Organisms {
		action := om.chooseAction(organism, organism.DecisionTree)
		om.applyAction(&om.Organisms[o], action)
	}
}

// doDecisionTree recursively walks through nodes of an organism's
// decision tree, finally applying the chosen action
func (om *OrganismManager) chooseAction(o Organism, tree d.Node) interface{} {
	if tree.IsAction() {
		return tree.NodeType
	}
	condition := tree.NodeType
	if om.isConditionTrue(o, condition) {
		return om.chooseAction(o, *tree.YesNode)
	}
	return om.chooseAction(o, *tree.NoNode)
}

func (om *OrganismManager) isConditionTrue(o Organism, cond interface{}) bool {
	switch cond {
	case d.CanMove:
		return om.canMove(o)
	case d.IsOnFood:
		return om.isOnFood(o)
	}
	return false
}

func (om *OrganismManager) canMove(o Organism) bool {
	newX := o.X + o.DirX
	newY := o.Y + o.DirY
	if newX < 0 || newY < 0 || newX >= c.GridWidth || newY >= c.GridHeight {
		return false
	}
	if om.Grid[newX][newY] {
		return false
	}
	return true
}

func (om *OrganismManager) isOnFood(o Organism) bool {
	value := om.Environment.GetFoodAtGridLocation(o.X, o.Y)
	return value > 0
}

func (om *OrganismManager) applyAction(o *Organism, action interface{}) {
	switch action {
	case d.ActEat:
		om.applyEat(o)
		break
	case d.ActMove:
		om.applyMove(o)
		break
	case d.ActTurnLeft:
		om.applyTurnLeft(o)
		break
	case d.ActTurnRight:
		om.applyTurnLeft(o)
		break
	}
}

func (om *OrganismManager) applyEat(o *Organism) {
	if om.isOnFood(*o) {
		o.Score++
	}
}

func (om *OrganismManager) applyMove(o *Organism) {
	if om.canMove(*o) {
		om.Grid[o.X][o.Y] = false
		o.X += o.DirX
		o.Y += o.DirY
		om.Grid[o.X][o.Y] = true
	}
}

func (om *OrganismManager) applyTurnLeft(o *Organism) {
	if o.DirX == 0 {
		if o.DirY == 1 {
			o.DirX = 1
		} else {
			o.DirX = -1
		}
		o.DirY = 0
	} else if o.DirY == 0 {
		if o.DirX == 1 {
			o.DirY = -1
		} else {
			o.DirY = 1
		}
		o.DirX = 0
	}
}

func (om *OrganismManager) applyTurnRight(o *Organism) {
	if o.DirX == 0 {
		if o.DirY == 1 {
			o.DirX = -1
		} else {
			o.DirX = 1
		}
		o.DirY = 0
	} else if o.DirY == 0 {
		if o.DirX == 1 {
			o.DirY = 1
		} else {
			o.DirY = -1
		}
		o.DirX = 0
	}
}

// For drawing

// GetOrganisms returns an array of all Organisms from organism manager
func (om *OrganismManager) GetOrganisms() [c.NumOrganisms]Organism {
	return om.Organisms
}

// PrintMaxScore prints the highest current score of any Organism (and their index)
func (om *OrganismManager) PrintMaxScore() {
	maxScore := -1
	winningOrganism := -1
	for o, organism := range om.Organisms {
		if organism.Score > maxScore {
			maxScore = organism.Score
			winningOrganism = o
		}
	}
	fmt.Printf("\nBest #%d. Score: %2d", winningOrganism, maxScore)
}
