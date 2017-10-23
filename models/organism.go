package models

import (
	"fmt"
	"image/color"
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
	StateReproducing

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
	ID, Age, Children, DirX, DirY, X, Y int
	Color                               color.RGBA
	Direction                           float64
	Health, AvgHealth                   float32
	State                               OrganismState
	DecisionSequence                    d.Sequence
	DecisionTree                        d.Node
}

// OrganismConfig contains all attributes needed to set up OrganismManager
type OrganismConfig struct {
	NumInitialOrganisms         int
	MaxOrganisms                int
	InitialHealth               float32
	MaxHealth                   float32
	HealthChangePerTurn         float32
	HealthChangeFromMoving      float32
	HealthChangeFromEating      float32
	HealthChangeFromReproducing float32
	HealthThresholdForEating    float32
	GridWidth, GridHeight       int
}

// NewOrganism initializes organism at with random grid location and direction
func NewOrganism(index, x, y int, health float32) *Organism {
	decisionSequence := d.NewRandomSequence()
	decisionNode := d.TreeFromSequence(decisionSequence)
	direction := math.Floor(rand.Float64()*4.0) * math.Pi / 2.0
	dirX := u.CalcDirXForDirection(direction)
	dirY := u.CalcDirYForDirection(direction)
	r := uint8(55 + rand.Intn(200))
	g := uint8(55 + rand.Intn(200))
	b := uint8(55 + rand.Intn(200))
	color := color.RGBA{r, g, b, 255}
	organism := Organism{
		Age:              0,
		AvgHealth:        health,
		Health:           health,
		Children:         0,
		Color:            color,
		ID:               index,
		DecisionSequence: decisionSequence,
		DecisionTree:     decisionNode,
		Direction:        direction,
		DirX:             dirX,
		DirY:             dirY,
		X:                x,
		Y:                y,
	}
	return &organism
}

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	config                 OrganismConfig
	Environment            *Environment
	Organisms              map[int]*Organism
	Grid                   [][]int
	BestOrganismCurrent    int
	BestAgeCurrent         int
	MostChildrenCurrent    int
	BestOrganismAllTime    int
	BestAgeAllTime         int
	MostChildrenAllTime    int
	BestSequence           d.Sequence
	LastIndexAdded         int
	LastReportedPopulation int
}

// NewOrganismManager creates all Organisms and updates grid
func NewOrganismManager(environment *Environment, config OrganismConfig) OrganismManager {
	organismManager := OrganismManager{Environment: environment, config: config}
	organismManager.Grid = make([][]int, config.GridWidth)
	for r := 0; r < config.GridWidth; r++ {
		organismManager.Grid[r] = make([]int, config.GridHeight)
	}
	for x := 0; x < config.GridWidth; x++ {
		for y := 0; y < config.GridHeight; y++ {
			organismManager.Grid[x][y] = -1
		}
	}
	organismManager.Organisms = make(map[int]*Organism)
	for i := 0; i < config.NumInitialOrganisms; i++ {
		organismManager.AddNewOrganism()
	}
	organismManager.BestSequence = d.NewRandomSequence()
	organismManager.LastReportedPopulation = 0
	return organismManager
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (om *OrganismManager) Update() {
	isNewBest := false
	om.MostChildrenCurrent = 0
	// Periodically add new random organisms if population below a certain amount
	if len(om.Organisms) < om.config.MaxOrganisms/4 {
		om.AddNewOrganism()
	}
	for k, o := range om.Organisms {
		om.updateOrganism(k, om.Organisms[k])
		if o.Children > om.MostChildrenCurrent {
			om.BestOrganismCurrent = k
			om.MostChildrenCurrent = o.Children
			if o.Children > om.MostChildrenAllTime {
				om.MostChildrenAllTime = o.Children
				om.BestSequence = make(d.Sequence, len(o.DecisionSequence))
				copy(om.BestSequence, o.DecisionSequence)
				if k != om.BestOrganismAllTime {
					isNewBest = true
					om.BestOrganismAllTime = k
				}
			}
		}
	}
	if isNewBest {
		// om.PrintBest()
	}
	// om.ReportPopulation()
}

// UpdateOrganism update's an Organism's Age, runs its Action cycle, updates
// its Health, and replaces it if its Health <= 0
func (om *OrganismManager) updateOrganism(index int, o *Organism) {
	if o.Health > 0.0 {
		o.Age++
		om.applyAction(o, om.chooseAction(o, o.DecisionTree))
		om.updateHealth(o)
	} else {
		om.removeOrganism(index)
	}
}

func (om *OrganismManager) removeOrganism(index int) {
	o := om.Organisms[index]
	om.Grid[o.X][o.Y] = -1
	delete(om.Organisms, index)
}

func (om *OrganismManager) spawnNewOrganism(parent *Organism) {
	index := om.LastIndexAdded + 1
	x, y := om.getSpawnLocation(parent)
	if x != -1 && y != -1 {
		child := *NewOrganism(index, x, y, om.config.MaxHealth)
		child.DecisionSequence = d.MutateSequence(parent.DecisionSequence)
		child.DecisionTree = d.TreeFromSequence(child.DecisionSequence)
		child.Color = u.MutateColor(parent.Color)
		child.Health = parent.Health
		om.Grid[x][y] = index
		om.Organisms[index] = &child
		om.LastIndexAdded = index
		parent.Children++
	}
}

func (om *OrganismManager) AddNewOrganism() {
	index := om.LastIndexAdded + 1
	isPlaced := false
	for isPlaced == false {
		x := rand.Intn(om.config.GridWidth)
		y := rand.Intn(om.config.GridHeight)
		if om.isGridLocationEmpty(x, y) {
			organism := *NewOrganism(index, x, y, om.config.MaxHealth)
			om.Organisms[index] = &organism
			om.Grid[x][y] = index
			om.LastIndexAdded = index
			isPlaced = true
		}
	}
}

func (om *OrganismManager) getSpawnLocation(parent *Organism) (x, y int) {
	direction := math.Floor(rand.Float64()*4.0) * math.Pi / 2.0
	for i := 0; i < 4; i++ {
		dirX := u.CalcDirXForDirection(direction)
		dirY := u.CalcDirYForDirection(direction)
		x := parent.X + dirX
		y := parent.Y + dirY
		if om.isGridLocationEmpty(x, y) {
			return x, y
		}
		direction += LeftTurnAngle
	}
	return -1, -1
}

func (om *OrganismManager) isGridLocationEmpty(x, y int) bool {
	width := om.config.GridWidth
	height := om.config.GridHeight
	return u.IsOnGrid(x, y, width, height) && om.Grid[x][y] == -1 && !om.Environment.IsFoodAtGridLocation(x, y)
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
	case d.CanReproduce:
		return om.canReproduce(o)
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
	width := om.config.GridWidth
	height := om.config.GridHeight
	if u.IsOnGrid(x, y, width, height) && om.Grid[x][y] != -1 {
		index := om.Grid[x][y]
		return om.Organisms[index]
	}
	return nil
}

func (om *OrganismManager) isFoodAhead(o *Organism) bool {
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if om.Environment.IsFoodAtGridLocation(x, y) {
		return true
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
	width := om.config.GridWidth
	height := om.config.GridHeight
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if u.IsOnGrid(x, y, width, height) {
		return !(om.Grid[x][y] != -1 || om.Environment.IsFoodAtGridLocation(x, y))
	}
	return false
}

func (om *OrganismManager) canReproduce(o *Organism) bool {
	if o.Health+om.config.HealthChangeFromReproducing < 0 {
		return false
	}
	return len(om.Organisms) < om.config.MaxOrganisms
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
	case d.ActReproduce:
		om.applyReproduce(o)
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
		o.Health += om.config.HealthChangeFromMoving
		break
	case StateEating:
		o.Health += om.config.HealthChangeFromEating
		break
	case StateReproducing:
		o.Health += om.config.HealthChangeFromReproducing
		break
	}
	o.Health += om.config.HealthChangePerTurn
	o.Health = float32(math.Min(float64(o.Health), float64(om.config.MaxHealth)))
	o.AvgHealth = (o.AvgHealth*float32(o.Age-1) + o.Health) / float32(o.Age)
}

func (om *OrganismManager) applyEat(o *Organism) {
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if om.Environment.IsFoodAtGridLocation(x, y) {
		o.State = StateEating
		om.Environment.RemoveFood(x, y)
	}
}

func (om *OrganismManager) applyMove(o *Organism) {
	o.State = StateMoving
	if om.canMove(o) {
		om.Grid[o.X][o.Y] = -1
		o.X += o.DirX
		o.Y += o.DirY
		om.Grid[o.X][o.Y] = o.ID
	}
}

func (om *OrganismManager) applyReproduce(o *Organism) {
	if om.canReproduce(o) {
		o.State = StateReproducing
		om.spawnNewOrganism(o)
	}
}

func (om *OrganismManager) applyTurn(o *Organism, directionChange float64) {
	o.Direction += directionChange
	o.DirX = u.CalcDirXForDirection(o.Direction)
	o.DirY = u.CalcDirYForDirection(o.Direction)
}

// GetOrganisms returns an array of all Organisms from organism manager
func (om *OrganismManager) GetOrganisms() map[int]*Organism {
	return om.Organisms
}

// PrintBest prints the highest current score of any Organism (and their index)
func (om *OrganismManager) PrintBest() {
	fmt.Printf("\nBest #%2d. Children: %d", om.BestOrganismAllTime, om.MostChildrenAllTime)
	tree := d.TreeFromSequence(om.BestSequence)
	fmt.Printf("\n%s", d.PrintNode(tree, 1))
}

// ReportPopulation prints the current population
func (om *OrganismManager) ReportPopulation() {
	currentPopulation := len(om.Organisms)
	difference := currentPopulation - om.LastReportedPopulation
	if math.Abs(float64(difference)) > c.PopulationDifferenceToReport {
		if difference < 0 {
			fmt.Printf("\nPopulation at %d, down %d\n", currentPopulation, difference)
		} else {
			fmt.Printf("\nPopulation at %d, up %d\n", currentPopulation, difference)
		}
		om.LastReportedPopulation = currentPopulation
	}
}
