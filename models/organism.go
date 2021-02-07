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

var descendantsPrintThreshold = 1

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

// Organism contains:
// - location (X, Y)
// - direction (angle, x & y vectors)
// - current action (Action)
// - algorithm code (String? or []int?)
// - algorithm (func)
type Organism struct {
	ID, Age, Children, DirX, DirY, X, Y int
	Color                               color.Color
	Direction, Health, AvgHealth        float64
	State                               OrganismState
	NodeLibrary                         *d.NodeLibrary
	DecisionTree                        *d.Node
	CountdownToChangeDecisionTree       int
	OriginalAncestorID                  int
}

// OrganismConfig contains all attributes needed to set up OrganismManager
type OrganismConfig struct {
	NumInitialOrganisms           int
	MaxOrganisms                  int
	InitialHealth                 float64
	MaxHealth                     float64
	HealthChangePerCycle          float64
	HealthChangeFromAttacking     float64
	HealthChangeFromBeingAttacked float64
	HealthChangeFromMoving        float64
	HealthChangeFromEatingAttempt float64
	HealthChangeFromConsumingFood float64
	HealthChangeFromTurning       float64
	HealthChangeFromBeingIdle     float64
	HealthChangeFromReproducing   float64
	GridWidth, GridHeight         int
}

// NewRandomOrganism initializes organism at with random grid location and direction
func NewRandomOrganism(id, x, y int) *Organism {
	nodeLibrary := d.NewNodeLibrary()
	direction, dirX, dirY := u.GetRandomDirection()
	color := u.GetRandomColor()
	organism := Organism{
		Age:                0,
		AvgHealth:          c.InitialHealth,
		Health:             c.InitialHealth,
		Children:           0,
		Color:              color,
		ID:                 id,
		Direction:          direction,
		DirX:               dirX,
		DirY:               dirY,
		X:                  x,
		Y:                  y,
		NodeLibrary:        nodeLibrary,
		OriginalAncestorID: id,
	}
	organism.setDecisionTree(nodeLibrary.GetRandomNode())
	return &organism
}

// NewChildOrganism initializes and returns a new organism with a copied NodeLibrary from its parent
func NewChildOrganism(id, x, y int, parent *Organism) *Organism {
	_, decisionNode := parent.NodeLibrary.GetBestNodesForHealth()
	if decisionNode == nil {
		decisionNode = parent.NodeLibrary.Map[parent.DecisionTree.ID]
	}
	childDecisionNode := d.CopyTreeByValue(decisionNode)
	nodeLibrary := d.NewNodeLibrary()
	nodeLibrary.RegisterAndReturnNewNode(childDecisionNode)
	direction, dirX, dirY := u.GetRandomDirection()
	color := u.MutateColor(parent.Color)
	organism := Organism{
		Age:                0,
		AvgHealth:          c.InitialHealth,
		Health:             c.InitialHealth,
		Children:           0,
		Color:              color,
		ID:                 id,
		DecisionTree:       decisionNode,
		Direction:          direction,
		DirX:               dirX,
		DirY:               dirY,
		X:                  x,
		Y:                  y,
		NodeLibrary:        nodeLibrary,
		OriginalAncestorID: parent.OriginalAncestorID,
	}
	return &organism
}

func (o *Organism) setDecisionTree(decisionTree *d.Node) {
	if o.DecisionTree != nil {
		o.DecisionTree.SetUsedInCurrentDecisionTree(false)
	}
	o.DecisionTree = decisionTree
	o.DecisionTree.SetUsedInCurrentDecisionTree(true)
}

// UpdateDecisionTree either swaps its current DecisionTree with a new one or,
// if already using the best node for the sought metric, mutates its existing
// algorithm
func (o *Organism) UpdateDecisionTree() {
	o.DecisionTree.SetUsedInCurrentDecisionTree(false)
	best, bestTopLevel := o.NodeLibrary.GetBestNodesForHealth()
	current := o.DecisionTree
	choice := rand.Float32()
	if choice < 0.90 && bestTopLevel != nil {
		o.DecisionTree = bestTopLevel
	} else if choice < 0.95 && best != nil {
		o.DecisionTree = best
	} else {
		mutatedTree := d.MutateTree(current)
		o.DecisionTree = o.NodeLibrary.RegisterAndReturnNewNode(mutatedTree)
	}
	o.DecisionTree.SetUsedInCurrentDecisionTree(true)
	o.CountdownToChangeDecisionTree = o.DecisionTree.Complexity * 2
}

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	config                 OrganismConfig
	NodeLibrary            d.NodeLibrary
	Environment            *Environment
	Organisms              map[int]*Organism
	Grid                   [][]int
	MostChildrenAllTime    int
	LastIndexAdded         int
	LastReportedPopulation int

	OldestOrganismAllTime    *organismInfo
	OldestOrganismCurrent    *organismInfo
	AncestorDescendantsCount map[int]int
}

type organismInfo struct {
	id           int
	ancestorID   int
	age          int
	children     int
	decisionTree string
}

// NewOrganismManager creates all Organisms and updates grid
func NewOrganismManager(environment *Environment, config OrganismConfig) OrganismManager {
	organismManager := OrganismManager{
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
		organismManager.SpawnRandomOrganism()
	}
	organismManager.LastReportedPopulation = 0
	organismManager.AncestorDescendantsCount = make(map[int]int)

	organismManager.OldestOrganismAllTime = &organismInfo{}
	organismManager.OldestOrganismCurrent = &organismInfo{}
	return organismManager
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (om *OrganismManager) Update() {
	om.OldestOrganismCurrent = &organismInfo{}
	// Periodically add new random organisms if population below a certain amount
	if len(om.Organisms) < om.config.MaxOrganisms/10 {
		om.SpawnRandomOrganism()
	}
	for _, o := range om.Organisms {
		om.updateOrganism(o)
	}
}

// UpdateOrganism updates an Organism's Age, runs its Action cycle, updates
// its Health, and replaces it if its Health <= 0
func (om *OrganismManager) updateOrganism(o *Organism) {
	om.updateStats(o)

	if o.Health <= 0.0 {
		om.removeOrganism(o.ID)
		return
	}

	oldEnough := o.Age > c.MinAgeToSpawn
	healthyEnough := o.Health > c.MinHealthToSpawn*c.MaxHealth
	beneathMax := len(om.Organisms) < c.MaxOrganismsAllowed
	if oldEnough && healthyEnough && beneathMax {
		om.SpawnChildOrganism(o)
	}

	o.DecisionTree.UpdateStats(o.Health, true)

	if o.shouldChangeDecisionTree() {
		o.UpdateDecisionTree()
	}

	action := om.chooseAction(o, o.DecisionTree)

	om.applyAction(o, action)
	om.NodeLibrary.PruneUnusedNodes()
	om.evaluateBest(o)
}

func (o *Organism) shouldChangeDecisionTree() bool {
	o.CountdownToChangeDecisionTree--
	return o.CountdownToChangeDecisionTree <= 0
}

func (om *OrganismManager) evaluateBest(o *Organism) {
	if o.Age > om.OldestOrganismCurrent.age {
		_, decisionTree := o.NodeLibrary.GetBestNodesForHealth()
		if decisionTree == nil {
			decisionTree = o.DecisionTree
		}
		organismInfo := &organismInfo{
			id:           o.ID,
			ancestorID:   o.OriginalAncestorID,
			decisionTree: decisionTree.Print("", true, false),
			age:          o.Age,
			children:     o.Children,
		}
		om.OldestOrganismCurrent = organismInfo

		if o.Age > om.OldestOrganismAllTime.age {
			om.OldestOrganismAllTime = organismInfo
		}
	}
}

func (om *OrganismManager) removeOrganism(index int) {
	x, y := om.Organisms[index].X, om.Organisms[index].Y
	om.Grid[x][y] = -1
	om.Environment.AddFoodAtPoint(Point{X: x, Y: y})
	om.Organisms[index].NodeLibrary = nil
	delete(om.Organisms, index)
}

// SpawnRandomOrganism creates an Organism with random position.
//
// Checks random positions on the grid until it finds an empty one. Calls
// NewOrganism to initialize decision tree, other random attributes.
func (om *OrganismManager) SpawnRandomOrganism() {
	index := om.LastIndexAdded + 1
	x, y := om.getRandomSpawnLocation()
	organism := NewRandomOrganism(index, x, y)
	om.registerNewOrganism(organism, index)
}

// SpawnChildOrganism creates a new organism near an existing 'parent' organism
// with a copy of its parent's node library. (No organism created if no room)
func (om *OrganismManager) SpawnChildOrganism(parent *Organism) {
	index := om.LastIndexAdded + 1
	if x, y, found := om.getChildSpawnLocation(parent); found {
		parent.Children++
		parent.Health += om.config.HealthChangeFromReproducing
		organism := NewChildOrganism(index, x, y, parent)
		om.registerNewOrganism(organism, index)
	}
}

func (om *OrganismManager) registerNewOrganism(organism *Organism, index int) {
	om.Organisms[index] = organism
	om.Grid[organism.X][organism.Y] = index
	om.LastIndexAdded = index

	ancestorID := organism.OriginalAncestorID
	if ancestorID != organism.ID {
		if _, ok := om.AncestorDescendantsCount[ancestorID]; !ok {
			om.AncestorDescendantsCount[ancestorID] = 0
		}
		om.AncestorDescendantsCount[ancestorID]++
	}
}

func (om *OrganismManager) getRandomSpawnLocation() (int, int) {
	x := rand.Intn(om.config.GridWidth)
	y := rand.Intn(om.config.GridHeight)
	for !om.isGridLocationEmpty(x, y) {
		x = rand.Intn(om.config.GridWidth)
		y = rand.Intn(om.config.GridHeight)
	}
	return x, y
}

func (om *OrganismManager) getChildSpawnLocation(parent *Organism) (int, int, bool) {
	direction := math.Floor(rand.Float64()*4.0) * math.Pi / 2.0
	for i := 0; i < 4; i++ {
		dirX := u.CalcDirXForDirection(direction)
		dirY := u.CalcDirYForDirection(direction)
		x := parent.X + dirX
		y := parent.Y + dirY
		if om.isGridLocationEmpty(x, y) {
			return x, y, true
		}
		direction += LeftTurnAngle
	}
	return -1, -1, false
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
	tree.UsedLastCycle = true
	if tree.IsAction() {
		return tree.NodeType
	}
	condition := tree.NodeType
	if om.isConditionTrue(o, condition) {
		return om.chooseAction(o, tree.YesNode)
	}
	return om.chooseAction(o, tree.NoNode)
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
	case d.IsOrganismAhead:
		return om.isOrganismAhead(o)
	case d.IsOrganismLeft:
		return om.isOrganismLeft(o)
	case d.IsOrganismRight:
		return om.isOrganismRight(o)
	case d.IsRandomFiftyPercent:
		return rand.Float32() < 0.5
	case d.IsHealthAboveNinetyPercent:
		return o.Health > om.config.MaxHealth*0.9
	case d.IsHealthAboveFiftyPercent:
		return o.Health > om.config.MaxHealth*0.5
	case d.IsHealthAboveTenPercent:
		return o.Health > om.config.MaxHealth*0.1
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

func (om *OrganismManager) applyAction(o *Organism, action interface{}) {
	o.State = StateIdle // default to idle so other functions don't need to
	switch action {
	case d.ActIdle:
		om.applyIdle(o)
		break
	case d.ActAttack:
		om.applyAttack(o)
		break
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

func (om *OrganismManager) updateStats(o *Organism) {
	om.updateAge(o)
	om.updateHealth(o)
}

func (om *OrganismManager) updateAge(o *Organism) {
	o.Age++
}

func (om *OrganismManager) updateHealth(o *Organism) {
	o.Health += om.config.HealthChangePerCycle
	o.Health = math.Min(o.Health, om.config.MaxHealth)
	o.AvgHealth = (o.AvgHealth*float64(o.Age-1) + o.Health) / float64(o.Age)
}

func (om *OrganismManager) applyIdle(o *Organism) {
	o.Health += om.config.HealthChangeFromBeingIdle
	o.State = StateIdle
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
	o.Health += om.config.HealthChangeFromEatingAttempt
	if om.Environment.IsFoodAtPoint(Point{X: x, Y: y}) {
		om.Environment.RemoveFood(Point{X: x, Y: y})
		o.Health += om.config.HealthChangeFromConsumingFood
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

func (om *OrganismManager) applyTurn(o *Organism, directionChange float64) {
	o.Direction += directionChange
	o.Health += om.config.HealthChangeFromTurning
	o.DirX = u.CalcDirXForDirection(o.Direction)
	o.DirY = u.CalcDirYForDirection(o.Direction)
}

// GetOrganisms returns an array of all Organisms from organism manager
func (om *OrganismManager) GetOrganisms() map[int]*Organism {
	return om.Organisms
}

// PrintBest prints the highest current score of any Organism (and their index)
func (om *OrganismManager) PrintBest() {
	om.printBestAncestors()
	fmt.Print("\n\n")
	om.printBestCurrent()
	fmt.Print("\n\n")
	om.printBestAllTime()
}

func (om *OrganismManager) printBestCurrent() {
	fmt.Printf("\n  - Best Organism Current - \n")
	om.printOrganismInfo(om.OldestOrganismCurrent)
}

func (om *OrganismManager) printBestAllTime() {
	fmt.Printf("\n  - Best Organism All Time - \n")
	om.printOrganismInfo(om.OldestOrganismAllTime)
}

func (om *OrganismManager) printOrganismInfo(info *organismInfo) {
	fmt.Printf("\n      ID: %10d", info.id)
	fmt.Printf("\n     Age: %10d", info.age)
	fmt.Printf("\nChildren: %10d", info.children)
	fmt.Printf("\nAncestor: %10d", info.ancestorID)
	fmt.Printf("\nDecisionTree:\n%s", info.decisionTree)
}

func (om *OrganismManager) printBestAncestors() {
	fmt.Printf("\n - Original Ancestors: %d\n", len(om.AncestorDescendantsCount))
	fmt.Printf("   Best (%d descendants or more) -\n", descendantsPrintThreshold)
	fmt.Print("  Ancestor ID  | Descendants\n")

	updateThreshold := false
	for ancestorID, descendants := range om.AncestorDescendantsCount {
		if descendants >= descendantsPrintThreshold {
			fmt.Printf("\n%13d  |%12d", ancestorID, descendants)
			if descendants > descendantsPrintThreshold*2 {
				updateThreshold = true
			}
		}
	}
	if updateThreshold {
		descendantsPrintThreshold = int(math.Ceil(float64(descendantsPrintThreshold) * 1.1))
	}
}
