package models

import (
	"fmt"
	"math"
	"math/rand"

	c "../constants"
	d "../decisions"
)

// OrganismState defines type of action Organism is doing
type OrganismState int

// Define Organism States
const (
	StateIdle OrganismState = iota
	StateMoving
	StateEating
)

// Organism has stuff
// - location (X, Y)
// - direction (angle)
// - current action (Action)
// - algorithm code (String? or []int?)
// - algorithm (func)
type Organism struct {
	Age, DirX, DirY, X, Y int
	Health, AvgHealth     float32
	State                 OrganismState
	DecisionSequence      d.Sequence
	DecisionTree          d.Node
}

// NewOrganism initializes organism at with random grid location and direction
func NewOrganism() Organism {
	decisionSequence := d.NewRandomSequence()
	decisionNode := d.TreeFromSequence(decisionSequence)
	organism := Organism{
		Age:              0,
		AvgHealth:        50,
		Health:           50,
		DecisionSequence: decisionSequence,
		DecisionTree:     decisionNode,
		DirX:             1,
		DirY:             0,
		X:                rand.Intn(c.GridWidth),
		Y:                rand.Intn(c.GridHeight),
	}
	return organism
}

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	Environment       *Environment
	Organisms         [c.NumOrganisms]Organism
	Grid              [c.GridWidth][c.GridHeight]bool
	BestOrganismIndex int
	bestAge           int
	bestSequence      d.Sequence
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
	for i, o := range om.Organisms {
		om.updateOrganism(i, &om.Organisms[i])
		if o.Age > om.bestAge {
			om.bestAge = o.Age
			om.bestSequence = o.DecisionSequence
			if i != om.BestOrganismIndex {
				om.BestOrganismIndex = i
				om.PrintOldest()
			}
		}
	}
}

// UpdateOrganism update's an Organism's Age, runs its Action cycle, updates
// its Health, and replaces it if its Health <= 0
func (om *OrganismManager) updateOrganism(index int, o *Organism) {
	o.Age++
	om.applyAction(o, om.chooseAction(o, o.DecisionTree))
	om.updateHealth(o)
	if o.Health <= 0.0 {
		om.replaceOrganism(index)
	}
}

func (om *OrganismManager) replaceOrganism(index int) {
	fmt.Printf("\nDead: #%2d, Age: %d | Best: %d", index, om.Organisms[index].Age, om.bestAge)
	om.Organisms[index] = NewOrganism()
	om.Organisms[index].DecisionSequence = d.MutateSequence(om.bestSequence)
}

// doDecisionTree recursively walks through nodes of an organism's
// decision tree, finally applying the chosen action
func (om *OrganismManager) chooseAction(o *Organism, tree d.Node) interface{} {
	if tree.IsAction() {
		return tree.NodeType
	}
	condition := tree.NodeType
	if om.isConditionTrue(o, condition) {
		return om.chooseAction(o, *tree.YesNode)
	}
	return om.chooseAction(o, *tree.NoNode)
}

func (om *OrganismManager) isConditionTrue(o *Organism, cond interface{}) bool {
	switch cond {
	case d.CanMove:
		return om.canMove(o)
	case d.IsOnFood:
		return om.isOnFood(o)
	}
	return false
}

func (om *OrganismManager) canMove(o *Organism) bool {
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

func (om *OrganismManager) isOnFood(o *Organism) bool {
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

func (om *OrganismManager) updateHealth(o *Organism) {
	switch o.State {
	case StateIdle:
		break
	case StateMoving:
		o.Health += c.HealthChangeFromMoving
		break
	case StateEating:
		o.Health += c.HealthChangeFromEating
		break
	}
	o.Health += c.HealthChangePerTurn
	o.Health = float32(math.Min(float64(o.Health), c.MaxHealth))
	o.AvgHealth = (o.AvgHealth*float32(o.Age-1) + o.Health) / float32(o.Age)
}

func (om *OrganismManager) applyEat(o *Organism) {
	if om.isOnFood(o) {
		o.State = StateEating
	} else {
		o.State = StateIdle
	}
}

func (om *OrganismManager) applyMove(o *Organism) {
	o.State = StateMoving
	if om.canMove(o) {
		om.Grid[o.X][o.Y] = false
		o.X += o.DirX
		o.Y += o.DirY
		om.Grid[o.X][o.Y] = true
	}
}

func (om *OrganismManager) applyTurnLeft(o *Organism) {
	o.State = StateIdle
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
	o.State = StateIdle
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

// PrintOldest prints the highest current score of any Organism (and their index)
func (om *OrganismManager) PrintOldest() {
	index := om.BestOrganismIndex
	fmt.Printf("\nBest #%2d. Age: %d", index, om.Organisms[index].Age)
}
