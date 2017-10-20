package models

import (
	"fmt"
	"math"
	"math/rand"

	c "../constants"
	d "../decisions"
	u "../utils"
)

// OrganismState defines type of action Organism is doing
type OrganismState int

// Define Organism States
const (
	StateIdle OrganismState = iota
	StateMoving
	StateEating

	LeftTurnAngle  = math.Pi / 2.0
	RightTurnAngle = -1.0 * (math.Pi / 2.0)
)

// Organism has stuff
// - location (X, Y)
// - direction (angle, x & y vectors)
// - current action (Action)
// - algorithm code (String? or []int?)
// - algorithm (func)
type Organism struct {
	Age, DirX, DirY, X, Y, Index int
	Direction, Health, AvgHealth float64
	State                        OrganismState
	DecisionSequence             d.Sequence
	DecisionTree                 d.Node
}

// NewOrganism initializes organism at with random grid location and direction
func NewOrganism(index int) Organism {
	decisionSequence := d.NewRandomSequence()
	decisionNode := d.TreeFromSequence(decisionSequence)
	direction := math.Floor(rand.Float64()*4.0) * math.Pi / 2.0
	dirX := u.CalcDirXForDirection(direction)
	dirY := u.CalcDirYForDirection(direction)
	organism := Organism{
		Age:              0,
		AvgHealth:        50,
		Health:           50,
		Index:            index,
		DecisionSequence: decisionSequence,
		DecisionTree:     decisionNode,
		Direction:        direction,
		DirX:             dirX,
		DirY:             dirY,
		X:                rand.Intn(c.GridWidth),
		Y:                rand.Intn(c.GridHeight),
	}
	return organism
}

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	Environment         *Environment
	Organisms           [c.NumOrganisms]Organism
	Grid                [c.GridWidth][c.GridHeight]int
	BestOrganismCurrent int
	BestAgeCurrent      int
	BestOrganismAllTime int
	BestAgeAllTime      int
	BestSequence        d.Sequence
}

// NewOrganismManager creates all Organisms and updates grid
func NewOrganismManager(environment *Environment) OrganismManager {
	organismManager := OrganismManager{Environment: environment}
	for x := 0; x < c.GridWidth; x++ {
		for y := 0; y < c.GridHeight; y++ {
			organismManager.Grid[x][y] = -1
		}
	}
	for i := range organismManager.Organisms {
		organism := NewOrganism(i)
		organismManager.Organisms[i] = organism
		organismManager.Grid[organism.X][organism.Y] = i
	}
	organismManager.BestSequence = d.NewRandomSequence()
	return organismManager
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (om *OrganismManager) Update() {
	isNewBest := false
	om.BestAgeCurrent = 0
	for i, o := range om.Organisms {
		om.updateOrganism(i, &om.Organisms[i])
		if o.Age > om.BestAgeCurrent {
			om.BestOrganismCurrent = i
			om.BestAgeCurrent = o.Age
			if o.Age > om.BestAgeAllTime {
				om.BestAgeAllTime = o.Age
				om.BestSequence = make(d.Sequence, len(o.DecisionSequence))
				copy(om.BestSequence, o.DecisionSequence)
				if i != om.BestOrganismAllTime {
					isNewBest = true
					om.BestOrganismAllTime = i
				}
			}
		}
	}
	if isNewBest {
		om.PrintBest()
	}
}

// UpdateOrganism update's an Organism's Age, runs its Action cycle, updates
// its Health, and replaces it if its Health <= 0
func (om *OrganismManager) updateOrganism(index int, o *Organism) {
	if o.Health > 0.0 {
		o.Age++
		om.applyAction(o, om.chooseAction(o, o.DecisionTree))
		om.updateHealth(o)
	} else {
		om.replaceOrganism(index)
	}
}

func (om *OrganismManager) replaceOrganism(index int) {
	o := om.Organisms[index]
	om.Grid[o.X][o.Y] = -1
	om.Organisms[index] = NewOrganism(index)
	om.Organisms[index].DecisionSequence = d.MutateSequence(om.BestSequence)
	om.Organisms[index].DecisionTree = d.TreeFromSequence(om.Organisms[index].DecisionSequence)
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
	case d.IsFoodAhead:
		return om.isFoodAhead(o)
	case d.IsFoodLeft:
		return om.isFoodLeft(o)
	case d.IsFoodRight:
		return om.isFoodRight(o)
	}
	return false
}

func (om *OrganismManager) getOrganismAt(x, y int) *Organism {
	if u.IsOnGrid(x, y) && om.Grid[x][y] != -1 {
		index := om.Grid[x][y]
		return &om.Organisms[index]
	}
	return nil
}

func (om *OrganismManager) isFoodAhead(o *Organism) bool {
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if om.Environment.IsFoodAtGridLocation(x, y) {
		return true
	}
	organismAhead := om.getOrganismAt(x, y)
	if organismAhead != nil {
		return organismAhead.Health < c.HealthThresholdForEating
	}
	return false
}

func (om *OrganismManager) isFoodLeft(o *Organism) bool {
	direction := o.Direction + LeftTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	return om.Environment.IsFoodAtGridLocation(x, y)
}

func (om *OrganismManager) isFoodRight(o *Organism) bool {
	direction := o.Direction + RightTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	return om.Environment.IsFoodAtGridLocation(x, y)
}

func (om *OrganismManager) canMove(o *Organism) bool {
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if u.IsOnGrid(x, y) {
		return !(om.Grid[x][y] != -1 || om.Environment.IsFoodAtGridLocation(x, y))
	}
	return false
}

func (om *OrganismManager) applyAction(o *Organism, action interface{}) {
	o.State = StateIdle // default to idle so other functions don't need to
	switch action {
	case d.ActEat:
		om.applyEat(o)
		break
	case d.ActMove:
		om.applyMove(o)
		break
	case d.ActTurnLeft:
		om.applyTurn(o, LeftTurnAngle)
		break
	case d.ActTurnRight:
		om.applyTurn(o, RightTurnAngle)
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
	o.Health = math.Min(o.Health, c.MaxHealth)
	o.AvgHealth = (o.AvgHealth*float64(o.Age-1) + o.Health) / float64(o.Age)
}

func (om *OrganismManager) applyEat(o *Organism) {
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if om.Environment.IsFoodAtGridLocation(x, y) {
		o.State = StateEating
		om.Environment.RemoveFood(x, y)
	} else {
		organismAhead := om.getOrganismAt(x, y)
		if organismAhead != nil {
			o.State = StateEating
			om.replaceOrganism(organismAhead.Index)
		}
	}
}

func (om *OrganismManager) applyMove(o *Organism) {
	o.State = StateMoving
	if om.canMove(o) {
		om.Grid[o.X][o.Y] = -1
		o.X += o.DirX
		o.Y += o.DirY
		om.Grid[o.X][o.Y] = o.Index
	}
}

func (om *OrganismManager) applyTurn(o *Organism, directionChange float64) {
	o.Direction += directionChange
	o.DirX = u.CalcDirXForDirection(o.Direction)
	o.DirY = u.CalcDirYForDirection(o.Direction)
}

// For drawing

// GetOrganisms returns an array of all Organisms from organism manager
func (om *OrganismManager) GetOrganisms() [c.NumOrganisms]Organism {
	return om.Organisms
}

// PrintBest prints the highest current score of any Organism (and their index)
func (om *OrganismManager) PrintBest() {
	fmt.Printf("\nBest #%2d. Age: %d", om.BestOrganismAllTime, om.BestAgeAllTime)
	tree := d.TreeFromSequence(om.BestSequence)
	fmt.Printf("\n%s", d.PrintNode(tree, 1))
}
