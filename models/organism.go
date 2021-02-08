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

// Traits contains organism-specific values that dictate how and when organisms
// perform certain activities, which are passed down from parents to children.
type Traits struct {
	organismColor color.Color
	// spawnHealthPercent: The percentage of max health this organism and its children
	// start with, also equal to what it loses when spawning a child.
	spawnHealthPercent float64
	// MinHealthToSpawnPercent: a percentage of max health needed in order to spawn-
	// must be greater than SpawnHealth and less than MaxHealth
	minHealthToSpawnPercent      float64
	minCyclesBetweenSpawns       int
	chanceToMutateDecisionTree   float64
	cyclesToEvaluateDecisionTree int
}

func (t *Traits) copyMutated() *Traits {
	// minCycles = previous +1, +0, or +(-1), bounded by 0.0 and MaxCyclesBetweenSpawns
	minCycles := int(math.Min(math.Max(float64(t.minCyclesBetweenSpawns+(rand.Intn(3)-1)), 0), c.MaxCyclesBetweenSpawns))
	// spawnHealthPercent = previous +- <0.01, bounded by 0.0 and 1.0
	spawnHealthPercent := math.Min(math.Max(t.spawnHealthPercent+(rand.Float64()*0.02)-0.01, 0.0), 1.0)
	// minHealthToSpawnPercent = previous +- <0.01, bounded by spawnHealthPercent (calculated above) and MaxHealthToSpawnPercent
	minHealthToSpawnPercent := math.Min(math.Max(t.minHealthToSpawnPercent+(rand.Float64()*0.02)-0.01, spawnHealthPercent), c.MaxHealthToSpawnPercent)
	// chanceToMutateDecisionTree = previous +- <0.01, bounded by 0.0 and MaxChanceToMutateDecisionTree
	chanceToMutateDecisionTree := math.Min(math.Max(t.chanceToMutateDecisionTree+(rand.Float64()*0.02)-0.01, 0.0), c.MaxChanceToMutateDecisionTree)
	// cyclesToEvaluateDecisionTree = previous +1, +0 or +(-1), bounded by 1 and CyclesToEvaluateDecisionTree
	cyclesToEvaluateDecisionTree := int(math.Min(math.Max(float64(t.cyclesToEvaluateDecisionTree+(rand.Intn(3)-1)), 1), float64(c.MaxCyclesToEvaluateDecisionTree)))
	return &Traits{
		organismColor:                u.MutateColor(t.organismColor),
		spawnHealthPercent:           spawnHealthPercent,
		minHealthToSpawnPercent:      minHealthToSpawnPercent,
		minCyclesBetweenSpawns:       minCycles,
		chanceToMutateDecisionTree:   chanceToMutateDecisionTree,
		cyclesToEvaluateDecisionTree: cyclesToEvaluateDecisionTree,
	}
}

func (t *Traits) copy() *Traits {
	return &Traits{
		organismColor:                t.organismColor,
		spawnHealthPercent:           t.spawnHealthPercent,
		minHealthToSpawnPercent:      t.minHealthToSpawnPercent,
		minCyclesBetweenSpawns:       t.minCyclesBetweenSpawns,
		cyclesToEvaluateDecisionTree: t.cyclesToEvaluateDecisionTree,
		chanceToMutateDecisionTree:   t.chanceToMutateDecisionTree,
	}
}

func newRandomTraits() *Traits {
	spawnHealthPercent := rand.Float64() * c.MaxInitialHealthPercent
	return &Traits{
		organismColor:                u.GetRandomColor(),
		spawnHealthPercent:           spawnHealthPercent,
		minHealthToSpawnPercent:      rand.Float64()*(1.0-spawnHealthPercent) + spawnHealthPercent,
		minCyclesBetweenSpawns:       rand.Intn(c.MaxCyclesBetweenSpawns),
		chanceToMutateDecisionTree:   rand.Float64() * c.MaxChanceToMutateDecisionTree,
		cyclesToEvaluateDecisionTree: rand.Intn(c.MaxCyclesToEvaluateDecisionTree),
	}
}

// Organism contains:
// - location (X, Y)
// - direction (angle, x & y vectors)
// - current action (Action)
// - algorithm code (String? or []int?)
// - algorithm (func)
type Organism struct {
	ID, Age, Children, DirX, DirY, X, Y      int
	Direction, Health, PrevHealth, AvgHealth float64
	State                                    OrganismState
	NodeLibrary                              *d.NodeLibrary
	DecisionTree                             *d.Node
	OriginalAncestorID                       int
	CountdownToChangeDecisionTree            int
	CyclesSinceLastSpawn                     int
	traits                                   *Traits
}

// InitialHealth returns the health an organism and its children start life with
func (o *Organism) InitialHealth() float64 {
	return o.traits.spawnHealthPercent * c.MaxHealth
}

// HealthCostToReproduce returns the health to lose upon spawning a child
func (o *Organism) HealthCostToReproduce() float64 {
	return o.traits.spawnHealthPercent * c.MaxHealth
}

// MinHealthToSpawn returns the minimum health required for an organism to spawn a child
func (o *Organism) MinHealthToSpawn() float64 {
	return o.traits.minHealthToSpawnPercent * c.MaxHealth
}

// MinCyclesBetweenSpawns returns the minimum number of cycles needed for an
// organism to spawn
func (o *Organism) MinCyclesBetweenSpawns() int {
	return o.traits.minCyclesBetweenSpawns
}

// ChanceToMutateDecisionTree returns the percent chance this organism will
// mutate its most successful decision tree when switching algorithms.
func (o *Organism) ChanceToMutateDecisionTree() float64 {
	return o.traits.chanceToMutateDecisionTree
}

// CyclesToEvaluateDecisionTree returns the number of cycles this organism will
// wait before changing its decision tree
func (o *Organism) CyclesToEvaluateDecisionTree() int {
	return o.traits.cyclesToEvaluateDecisionTree
}

// Color returns an organism's color
func (o *Organism) Color() color.Color {
	return o.traits.organismColor
}

// NewRandomOrganism initializes organism at with random grid location and direction
func NewRandomOrganism(id, x, y int) *Organism {
	nodeLibrary := d.NewNodeLibrary()
	decisionTree := nodeLibrary.GetRandomNode()
	direction, dirX, dirY := u.GetRandomDirection()
	traits := newRandomTraits()
	initialHealth := traits.spawnHealthPercent * c.MaxHealth
	organism := Organism{
		ID:                            id,
		AvgHealth:                     initialHealth,
		Health:                        initialHealth,
		PrevHealth:                    initialHealth,
		DecisionTree:                  decisionTree,
		Direction:                     direction,
		DirX:                          dirX,
		DirY:                          dirY,
		X:                             x,
		Y:                             y,
		NodeLibrary:                   nodeLibrary,
		OriginalAncestorID:            id,
		CountdownToChangeDecisionTree: traits.cyclesToEvaluateDecisionTree,

		Age:                  0,
		Children:             0,
		CyclesSinceLastSpawn: 0,

		traits: traits,
	}
	return &organism
}

// NewChildOrganism initializes and returns a new organism with a copied NodeLibrary from its parent
func NewChildOrganism(id, x, y int, parent *Organism) *Organism {
	_, parentDecisionTree := parent.NodeLibrary.GetBestNodesForHealth()
	if parentDecisionTree == nil {
		parentDecisionTree = parent.NodeLibrary.Map[parent.DecisionTree.ID]
	}
	decisionTree := d.CopyTreeByValue(parentDecisionTree)
	nodeLibrary := d.NewNodeLibrary()
	nodeLibrary.RegisterAndReturnNewNode(decisionTree)
	direction, dirX, dirY := u.GetRandomDirection()
	traits := parent.traits.copyMutated()
	organism := Organism{
		ID:                            id,
		AvgHealth:                     parent.InitialHealth(),
		Health:                        parent.InitialHealth(),
		PrevHealth:                    parent.InitialHealth(),
		DecisionTree:                  decisionTree,
		Direction:                     direction,
		DirX:                          dirX,
		DirY:                          dirY,
		X:                             x,
		Y:                             y,
		NodeLibrary:                   nodeLibrary,
		OriginalAncestorID:            parent.OriginalAncestorID,
		CountdownToChangeDecisionTree: traits.cyclesToEvaluateDecisionTree,

		Age:                  0,
		Children:             0,
		CyclesSinceLastSpawn: 0,

		traits: traits,
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

	decisionTree := o.DecisionTree
	if bestTopLevel != nil {
		decisionTree = bestTopLevel
	} else if best != nil {
		decisionTree = best
	}

	if rand.Float64() < o.ChanceToMutateDecisionTree() {
		mutatedTree := d.MutateTree(decisionTree)
		o.DecisionTree = o.NodeLibrary.RegisterAndReturnNewNode(mutatedTree)
	} else {
		o.DecisionTree = o.NodeLibrary.RegisterAndReturnNewNode(decisionTree)
	}

	o.DecisionTree.SetUsedInCurrentDecisionTree(true)
	o.CountdownToChangeDecisionTree = o.CyclesToEvaluateDecisionTree()
}

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	NodeLibrary            d.NodeLibrary
	Environment            *Environment
	Organisms              map[int]*Organism
	Grid                   [][]int
	MostChildrenAllTime    int
	LastIndexAdded         int
	LastReportedPopulation int

	MostReproductiveAllTime  *organismInfo
	MostReproductiveCurrent  *organismInfo
	AncestorDescendantsCount map[int]int
}

type organismInfo struct {
	id           int
	ancestorID   int
	age          int
	children     int
	decisionTree string
	traits       *Traits
}

func (o *organismInfo) ID() int {
	return o.id
}

// NewOrganismManager creates all Organisms and updates grid
func NewOrganismManager(environment *Environment) OrganismManager {
	organismManager := OrganismManager{
		Environment: environment,
	}
	organismManager.Grid = make([][]int, c.GridWidth)
	for r := 0; r < c.GridWidth; r++ {
		organismManager.Grid[r] = make([]int, c.GridHeight)
	}
	for x := 0; x < c.GridWidth; x++ {
		for y := 0; y < c.GridHeight; y++ {
			organismManager.Grid[x][y] = -1
		}
	}
	organismManager.Organisms = make(map[int]*Organism)
	for i := 0; i < c.NumInitialOrganisms; i++ {
		organismManager.SpawnRandomOrganism()
	}
	organismManager.LastReportedPopulation = 0
	organismManager.AncestorDescendantsCount = make(map[int]int)

	organismManager.MostReproductiveAllTime = &organismInfo{traits: &Traits{}}
	organismManager.MostReproductiveCurrent = &organismInfo{traits: &Traits{}}
	return organismManager
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (om *OrganismManager) Update() {
	om.MostReproductiveCurrent = &organismInfo{traits: &Traits{}}
	// Periodically add new random organisms if population below a certain amount
	if len(om.Organisms) < c.NumInitialOrganisms {
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

	beneathMaxOrganisms := len(om.Organisms) < c.MaxOrganismsAllowed
	healthyEnough := o.Health > o.MinHealthToSpawn()
	cyclesPassed := o.CyclesSinceLastSpawn >= o.MinCyclesBetweenSpawns()
	if beneathMaxOrganisms && healthyEnough && cyclesPassed {
		om.SpawnChildOrganism(o)
	}

	healthChange := o.Health - o.PrevHealth
	o.DecisionTree.UpdateStats(healthChange, true)
	o.PrevHealth = o.Health

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
	return o.CountdownToChangeDecisionTree <= 0 || o.Health < c.MaxHealth*0.10
}

func (om *OrganismManager) evaluateBest(o *Organism) {
	if o.Children > om.MostReproductiveCurrent.children {
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
			traits:       o.traits.copy(),
		}
		om.MostReproductiveCurrent = organismInfo

		if o.Children > om.MostReproductiveAllTime.children {
			om.MostReproductiveAllTime = organismInfo
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
		parent.CyclesSinceLastSpawn = 0
		parent.Children++
		parent.Health += parent.HealthCostToReproduce()
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
	x := rand.Intn(c.GridWidth)
	y := rand.Intn(c.GridHeight)
	for !om.isGridLocationEmpty(x, y) {
		x = rand.Intn(c.GridWidth)
		y = rand.Intn(c.GridHeight)
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
	width := c.GridWidth
	height := c.GridHeight
	return u.IsOnGrid(x, y, width, height) && om.Grid[x][y] == -1 && !om.Environment.IsFoodAtPoint(Point{X: x, Y: y})
}

func (om *OrganismManager) isOrganismAtLocation(x, y int) bool {
	width := c.GridWidth
	height := c.GridHeight
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
	case d.IsHealthAboveFiftyPercent:
		return o.Health > c.MaxHealth*0.5
	}
	return false
}

func (om *OrganismManager) getOrganismAt(x, y int) *Organism {
	width := c.GridWidth
	height := c.GridHeight
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
	width := c.GridWidth
	height := c.GridHeight
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
	o.CyclesSinceLastSpawn++
}

func (om *OrganismManager) updateHealth(o *Organism) {
	o.Health += c.HealthChangePerCycle
	o.Health = math.Min(o.Health, c.MaxHealth)
	o.AvgHealth = (o.AvgHealth*float64(o.Age-1) + o.Health) / float64(o.Age)
}

func (om *OrganismManager) applyIdle(o *Organism) {
	o.Health += c.HealthChangeFromBeingIdle
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
	o.Health += c.HealthChangeFromAttacking
	o.State = StateAttacking
}

func (om *OrganismManager) applyEat(o *Organism) {
	x := o.X + o.DirX
	y := o.Y + o.DirY
	o.Health += c.HealthChangeFromEatingAttempt
	if om.Environment.IsFoodAtPoint(Point{X: x, Y: y}) {
		om.Environment.RemoveFood(Point{X: x, Y: y})
		o.Health += c.HealthChangeFromConsumingFood
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
	o.Health += c.HealthChangeFromMoving
	o.State = StateMoving
}

func (om *OrganismManager) applyTurn(o *Organism, directionChange float64) {
	o.Direction += directionChange
	o.Health += c.HealthChangeFromTurning
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
	om.printOrganismInfo(om.MostReproductiveCurrent)
}

func (om *OrganismManager) printBestAllTime() {
	fmt.Printf("\n  - Best Organism All Time - \n")
	om.printOrganismInfo(om.MostReproductiveAllTime)
}

func (om *OrganismManager) printOrganismInfo(info *organismInfo) {
	fmt.Printf("\n      ID: %10d   |      InitialHealth:     %4d", info.id, int(info.traits.spawnHealthPercent*c.MaxHealth))
	fmt.Printf("\n     Age: %10d   |   MinHealthToSpawn:     %4d", info.age, int(info.traits.minHealthToSpawnPercent*c.MaxHealth))
	fmt.Printf("\nChildren: %10d   |   MinCyclesToSpawn:     %4d", info.children, info.traits.minCyclesBetweenSpawns)
	fmt.Printf("\nAncestor: %10d   |   CyclesToEvaluateTree: %4d", info.ancestorID, info.traits.cyclesToEvaluateDecisionTree)
	fmt.Printf("\n                       |   ChanceToMutateTree:%.2f", info.traits.chanceToMutateDecisionTree)
	fmt.Printf("\nDecisionTree:\n%s", info.decisionTree)
}

func (om *OrganismManager) printBestAncestors() {
	fmt.Printf("\n - Original Ancestors: %d\n", len(om.AncestorDescendantsCount))
	fmt.Printf("   Best (%d descendants or more) -\n", descendantsPrintThreshold)
	fmt.Print("  Ancestor ID  | Descendants\n")

	// updateThreshold := false
	for ancestorID, descendants := range om.AncestorDescendantsCount {
		if descendants >= descendantsPrintThreshold {
			fmt.Printf("\n%13d  |%12d", ancestorID, descendants)
			// if descendants > descendantsPrintThreshold*2 {
			// 	updateThreshold = true
			// }
		}
	}
	// if updateThreshold {
	// 	descendantsPrintThreshold = int(math.Ceil(float64(descendantsPrintThreshold) * 1.1))
	// }
}
