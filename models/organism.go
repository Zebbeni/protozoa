package models

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"

	c "github.com/Zebbeni/protozoa/constants"
	d "github.com/Zebbeni/protozoa/decisions"
	u "github.com/Zebbeni/protozoa/utils"
)

// OrganismState defines type of action Organism is doing
type OrganismState int

// Define Organism States
const (
	StateAttacking OrganismState = iota
	StateIdle
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
	Direction, Health, AvgHealth        float64
	State                               OrganismState
	NodeLibrary                         *d.NodeLibrary
	DecisionTree                        *d.Node
	NodeToUpdateStats                   *d.Node
	CountdownToChangeDecisionTree       int
	MetricsValues                       map[d.Metric]float64
}

// OrganismConfig contains all attributes needed to set up OrganismManager
type OrganismConfig struct {
	NumInitialOrganisms           int
	MaxOrganisms                  int
	InitialHealth                 float64
	MaxHealth                     float64
	HealthChangePerTurn           float64
	HealthChangeFromAttacking     float64
	HealthChangeFromBeingAttacked float64
	HealthChangeFromMoving        float64
	HealthChangeFromEating        float64
	HealthChangeFromReproducing   float64
	GridWidth, GridHeight         int
}

// NewOrganism initializes organism at with random grid location and direction
func NewOrganism(index, x, y int, health float64, nodeLibrary *d.NodeLibrary) *Organism {
	decisionNode := nodeLibrary.GetRandomNode()
	direction := math.Floor(rand.Float64()*4.0) * math.Pi / 2.0
	dirX := u.CalcDirXForDirection(direction)
	dirY := u.CalcDirYForDirection(direction)
	organism := Organism{
		Age:               0,
		AvgHealth:         health,
		Health:            health,
		Children:          0,
		Color:             decisionNode.Color,
		ID:                index,
		DecisionTree:      decisionNode,
		Direction:         direction,
		DirX:              dirX,
		DirY:              dirY,
		X:                 x,
		Y:                 y,
		NodeLibrary:       nodeLibrary,
		NodeToUpdateStats: decisionNode,
		MetricsValues: map[d.Metric]float64{
			d.MetricHealth: 0.0,
		},
	}
	organism.DecisionTree.UpdateNumOrganismsUsing(1)
	return &organism
}

// UpdateDecisionTree either swaps its current DecisionTree with a new one
//
// If already using the best node for the sought metric, mutates its existing
// algorithm
func (o *Organism) UpdateDecisionTree(bestNodesForMetrics map[d.Metric]*d.Node) {
	o.DecisionTree.UpdateNumOrganismsUsing(-1)
	current := o.DecisionTree
	best := bestNodesForMetrics[d.MetricHealth]
	didMutate := false
	if best != nil && best.ID != current.ID {
		o.DecisionTree = best
	} else {
		didMutate = true
		mutatedTree := d.MutateTree(current)
		o.DecisionTree = o.NodeLibrary.RegisterAndReturnNewNode(mutatedTree)
	}
	o.DecisionTree.UpdateNumOrganismsUsing(1)
	o.Color = o.DecisionTree.Color
	if didMutate {
		if best != nil {
			fmt.Printf("\nBEST:\n%sAVG:%f\nTOTAL: %f\nUSES: %f\n", best.Print("", true), best.MetricsAvgs[d.MetricHealth], best.Metrics[d.MetricHealth], best.Uses)
		}
		fmt.Printf("\nMUTATED TO:\n%sAVG:%f\nTOTAL: %f\nUSES: %f\n", o.DecisionTree.Print("", true), o.DecisionTree.MetricsAvgs[d.MetricHealth], o.DecisionTree.Metrics[d.MetricHealth], o.DecisionTree.Uses)
		fmt.Printf("\nNodeLibrary Length: %d\n", len(o.NodeLibrary.Map))
	}
}

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	config                 OrganismConfig
	NodeLibrary            d.NodeLibrary
	BestNodesForMetrics    map[d.Metric]*d.Node
	Environment            *Environment
	Organisms              map[int]*Organism
	Grid                   [][]int
	BestOrganismCurrent    int
	BestAgeCurrent         int
	MostChildrenCurrent    int
	BestOrganismAllTime    int
	BestAgeAllTime         int
	MostChildrenAllTime    int
	LastIndexAdded         int
	LastReportedPopulation int
}

// NewOrganismManager creates all Organisms and updates grid
func NewOrganismManager(environment *Environment, config OrganismConfig) OrganismManager {
	organismManager := OrganismManager{
		NodeLibrary: d.NewNodeLibrary(),
		Environment: environment,
		config:      config,
	}
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
	organismManager.LastReportedPopulation = 0
	return organismManager
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (om *OrganismManager) Update() {
	isNewBest := false
	om.MostChildrenCurrent = 0
	// Periodically add new random organisms if population below a certain amount
	if len(om.Organisms) < om.config.MaxOrganisms/10 {
		om.AddNewOrganism()
	}
	om.BestNodesForMetrics = om.NodeLibrary.GetBestNodesForMetrics()
	for k, o := range om.Organisms {
		om.updateOrganism(k, om.Organisms[k])
		if o.Children > om.MostChildrenCurrent {
			om.BestOrganismCurrent = k
			om.MostChildrenCurrent = o.Children
			if o.Children > om.MostChildrenAllTime {
				om.MostChildrenAllTime = o.Children
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
	om.NodeLibrary.PruneUnusedNodes()
}

// UpdateOrganism updates an Organism's Age, runs its Action cycle, updates
// its Health, and replaces it if its Health <= 0
func (om *OrganismManager) updateOrganism(index int, o *Organism) {
	if o.Health > 0.0 {
		o.Age++
		// if o.Age%c.AgeToSpawn == 0 {
		// 	om.spawnNewOrganism(o)
		// }
		// TODO: Do next 2 lines in a for loop for all matrices
		o.MetricsValues[d.MetricHealth] = o.Health
		o.CountdownToChangeDecisionTree--
		if o.CountdownToChangeDecisionTree <= 0 {
			o.UpdateDecisionTree(om.BestNodesForMetrics)
			o.CountdownToChangeDecisionTree = rand.Intn(200) * o.DecisionTree.Complexity
		}
		o.NodeToUpdateStats.UpdateStats(o.MetricsValues)
		o.NodeToUpdateStats = o.DecisionTree
		action := om.chooseAction(o, o.DecisionTree)
		om.applyAction(o, action)
		om.updateHealth(o)
	} else {
		om.removeOrganism(index)
	}
}

func (om *OrganismManager) removeOrganism(index int) {
	o := om.Organisms[index]
	om.Grid[o.X][o.Y] = -1
	om.Environment.AddFoodAtPoint(Point{X: o.X, Y: o.Y})
	o.DecisionTree.UpdateNumOrganismsUsing(-1)
	delete(om.Organisms, index)
}

func (om *OrganismManager) spawnNewOrganism(parent *Organism) {
	index := om.LastIndexAdded + 1
	x, y := om.getSpawnLocation(parent)
	if x != -1 && y != -1 {
		child := *NewOrganism(index, x, y, om.config.MaxHealth, &om.NodeLibrary)
		child.Age = 0
		child.NodeLibrary = &om.NodeLibrary
		child.DecisionTree = parent.DecisionTree
		child.NodeToUpdateStats = parent.NodeToUpdateStats
		child.Color = parent.DecisionTree.Color
		child.Health = parent.Health
		om.Grid[x][y] = index
		om.Organisms[index] = &child
		om.LastIndexAdded = index
		parent.Children++
	}
}

// AddNewOrganism creates an Organism with random position.
//
// Checks random positions on the grid until it finds an empty one. Calls
// NewOrganism to initialize decision tree, other random attributes.
func (om *OrganismManager) AddNewOrganism() {
	index := om.LastIndexAdded + 1
	isPlaced := false
	for isPlaced == false {
		x := rand.Intn(om.config.GridWidth)
		y := rand.Intn(om.config.GridHeight)
		if om.isGridLocationEmpty(x, y) {
			organism := *NewOrganism(index, x, y, om.config.MaxHealth/2, &om.NodeLibrary)
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
	return u.IsOnGrid(x, y, width, height) && om.Grid[x][y] == -1 && !om.Environment.IsFoodAtPoint(Point{X: x, Y: y})
}

func (om *OrganismManager) isOrganismAtLocation(x, y int) bool {
	width := om.config.GridWidth
	height := om.config.GridHeight
	return u.IsOnGrid(x, y, width, height) && om.Grid[x][y] != -1
}

// chooseAction walks through nodes of an organism's decision tree, eventually
// returning the chosen action
//
// As chooseAction walks thorugh nodes, it also populates nodes to update metriic
// information for the next update run, diminishing the use value with each level
func (om *OrganismManager) chooseAction(o *Organism, tree *d.Node) interface{} {
	if tree.IsAction() {
		return tree.NodeType
	}
	condition := tree.NodeType
	if om.isConditionTrue(o, condition) {
		tree.UsedYes = true
		return om.chooseAction(o, tree.YesNode)
	}
	tree.UsedNo = true
	return om.chooseAction(o, tree.NoNode)
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
	case d.IsOrganismAhead:
		return om.isOrganismAhead(o)
	case d.IsOrganismLeft:
		return om.isOrganismLeft(o)
	case d.IsOrganismRight:
		return om.isOrganismRight(o)
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
	if om.Environment.IsFoodAtPoint(Point{X: x, Y: y}) {
		return true
	}
	return false
}

func (om *OrganismManager) isFoodLeft(o *Organism) bool {
	direction := o.Direction + LeftTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	return om.Environment.IsFoodAtPoint(Point{X: x, Y: y})
}

func (om *OrganismManager) isFoodRight(o *Organism) bool {
	direction := o.Direction + RightTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	return om.Environment.IsFoodAtPoint(Point{X: x, Y: y})
}

func (om *OrganismManager) isOrganismAhead(o *Organism) bool {
	x := o.X + o.DirX
	y := o.Y + o.DirY
	return om.isOrganismAtLocation(x, y)
}

func (om *OrganismManager) isOrganismLeft(o *Organism) bool {
	direction := o.Direction + LeftTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	return om.isOrganismAtLocation(x, y)
}

func (om *OrganismManager) isOrganismRight(o *Organism) bool {
	direction := o.Direction + RightTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	return om.isOrganismAtLocation(x, y)
}

func (om *OrganismManager) canMove(o *Organism) bool {
	width := om.config.GridWidth
	height := om.config.GridHeight
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if !u.IsOnGrid(x, y, width, height) {
		return false
	}
	if om.Grid[x][y] > -1 {
		return false
	}
	if om.Environment.IsFoodAtPoint(Point{X: x, Y: y}) {
		return false
	}
	return true
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
	case d.ActAttack:
		om.applyAttack(o)
		break
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
	o.Health += om.config.HealthChangePerTurn
	o.Health = math.Min(o.Health, om.config.MaxHealth)
	o.AvgHealth = (o.AvgHealth*float64(o.Age-1) + o.Health) / float64(o.Age)
}

func (om *OrganismManager) applyAttack(o *Organism) {
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if om.isOrganismAtLocation(x, y) {
		targetOrganismIndex := om.Grid[x][y]
		targetOrganism := om.Organisms[targetOrganismIndex]
		targetOrganism.Health += c.HealthChangeFromBeingAttacked
	}
	o.Health += om.config.HealthChangeFromAttacking
	o.State = StateAttacking
}

func (om *OrganismManager) applyEat(o *Organism) {
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if om.Environment.IsFoodAtPoint(Point{X: x, Y: y}) {
		om.Environment.RemoveFood(Point{X: x, Y: y})
		o.Health += om.config.HealthChangeFromEating
	}
	o.State = StateEating
}

func (om *OrganismManager) applyMove(o *Organism) {
	if om.canMove(o) {
		om.Grid[o.X][o.Y] = -1
		o.X += o.DirX
		o.Y += o.DirY
		om.Grid[o.X][o.Y] = o.ID
	}
	o.Health += om.config.HealthChangeFromMoving
	o.State = StateMoving
}

func (om *OrganismManager) applyReproduce(o *Organism) {
	if om.canReproduce(o) {
		om.spawnNewOrganism(o)
	}
	o.Health += om.config.HealthChangeFromReproducing
	o.State = StateReproducing
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
}
