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
	StateFeeding
	StateIdle
	StateMoving
	StateTurning
	StateEating
	StateReproducing

	LeftTurnAngle  = math.Pi / 2.0
	RightTurnAngle = -1.0 * (math.Pi / 2.0)
)

// Traits contains organism-specific values that dictate how and when organisms
// perform certain activities, which are passed down from parents to children.
type Traits struct {
	organismColor color.Color
	// maxSize represents the maximum size an organism can reach.
	maxSize float64
	// spawnHealth: The health value - and size - this organism and its
	// children start with, also equal to what it loses when spawning a child.
	spawnHealth float64
	// minHealthToSpawn: the minimum health needed in order to spawn-
	// must be greater than spawnHealth and less than maxSize
	minHealthToSpawn             float64
	minCyclesBetweenSpawns       int
	chanceToMutateDecisionTree   float64
	cyclesToEvaluateDecisionTree int
}

func (t *Traits) copy() *Traits {
	return &Traits{
		organismColor:                t.organismColor,
		maxSize:                      t.maxSize,
		spawnHealth:                  t.spawnHealth,
		minHealthToSpawn:             t.minHealthToSpawn,
		minCyclesBetweenSpawns:       t.minCyclesBetweenSpawns,
		cyclesToEvaluateDecisionTree: t.cyclesToEvaluateDecisionTree,
		chanceToMutateDecisionTree:   t.chanceToMutateDecisionTree,
	}
}

func newRandomTraits() *Traits {
	organismColor := u.GetRandomColor()
	maxSize := rand.Float64() * c.MaximumMaxSize
	spawnHealth := rand.Float64() * maxSize * c.MaxSpawnHealthPercent
	minHealthToSpawn := spawnHealth + rand.Float64()*(maxSize-spawnHealth)
	minCyclesBetweenSpawns := rand.Intn(c.MaxCyclesBetweenSpawns)
	chanceToMutateDecisionTree := math.Max(c.MinChanceToMutateDecisionTree, rand.Float64()*c.MaxChanceToMutateDecisionTree)
	cyclesToEvaluateDecisionTree := rand.Intn(c.MaxCyclesToEvaluateDecisionTree)
	return &Traits{
		organismColor:                organismColor,
		maxSize:                      maxSize,
		spawnHealth:                  spawnHealth,
		minHealthToSpawn:             minHealthToSpawn,
		minCyclesBetweenSpawns:       minCyclesBetweenSpawns,
		chanceToMutateDecisionTree:   chanceToMutateDecisionTree,
		cyclesToEvaluateDecisionTree: cyclesToEvaluateDecisionTree,
	}
}

func (t *Traits) copyMutated() *Traits {
	organismColor := mutateColor(t.organismColor)
	// maxSize = previous +- previous +- <5.0, bounded by MinimumMaxSize and MaximumMaxSize
	maxSize := mutateFloat(t.maxSize, 5.0, c.MinimumMaxSize, c.MaximumMaxSize)
	// minCyclesBetweenSpawns = previous +- <=5, bounded by 0 and MaxCyclesBetweenSpawns
	minCyclesBetweenSpawns := mutateInt(t.minCyclesBetweenSpawns, 5, 0, c.MaxCyclesBetweenSpawns)
	// spawnHealth = previous +- <0.05, bounded by MinSpawnHealth and maxSize
	spawnHealth := mutateFloat(t.spawnHealth, 0.1, c.MinSpawnHealth, maxSize*c.MaxSpawnHealthPercent)
	// minHealthToSpawn = previous +- <5.0, bounded by spawnHealthPercent and maxSize (both calculated above)
	minHealthToSpawn := mutateFloat(t.minHealthToSpawn, 5.0, spawnHealth, maxSize)
	// chanceToMutateDecisionTree = previous +- <0.01, bounded by MinChanceToMutateDecisionTree and MaxChanceToMutateDecisionTree
	chanceToMutateDecisionTree := mutateFloat(t.chanceToMutateDecisionTree, 0.01, c.MinChanceToMutateDecisionTree, c.MaxChanceToMutateDecisionTree)
	// cyclesToEvaluateDecisionTree = previous +1, +0 or +(-1), bounded by 1 and CyclesToEvaluateDecisionTree
	cyclesToEvaluateDecisionTree := mutateInt(t.cyclesToEvaluateDecisionTree, 1, 1, c.MaxCyclesToEvaluateDecisionTree)
	return &Traits{
		organismColor:                organismColor,
		maxSize:                      maxSize,
		spawnHealth:                  spawnHealth,
		minHealthToSpawn:             minHealthToSpawn,
		minCyclesBetweenSpawns:       minCyclesBetweenSpawns,
		chanceToMutateDecisionTree:   chanceToMutateDecisionTree,
		cyclesToEvaluateDecisionTree: cyclesToEvaluateDecisionTree,
	}
}

func mutateFloat(value, maxChange, min, max float64) float64 {
	mutated := value + maxChange - rand.Float64()*maxChange*2.0
	return math.Min(math.Max(mutated, min), max)
}

func mutateInt(value, maxChange, min, max int) int {
	mutated := math.Round(float64(value) + rand.Float64()*float64(maxChange)*2.0 - (float64(maxChange)))
	return int(math.Min(math.Max(mutated, float64(min)), float64(max)))
}

// MutateColor returns a slight variation on a given color
func mutateColor(originalColor color.Color) color.Color {
	r32, g32, b32, a32 := originalColor.RGBA()
	r := mutateColorValue(r32)
	g := mutateColorValue(g32)
	b := mutateColorValue(b32)
	a := uint8(a32)
	return color.RGBA{r, g, b, a}
}

func mutateColorValue(v uint32) uint8 {
	converted := int(uint8(v)) // cast to uint8 and back to int to avoid overflow
	mutated := math.Max(math.Min(float64(converted+rand.Intn(21)-10), 255), 50)
	return uint8(mutated)
}

// Organism contains:
// - location (X, Y)
// - direction (angle, x & y vectors)
// - current action (Action)
// - algorithm code (String? or []int?)
// - algorithm (func)
type Organism struct {
	ID, Age, Children, DirX, DirY, X, Y            int
	Size, Direction, Health, PrevHealth, AvgHealth float64
	State                                          OrganismState
	NodeLibrary                                    *d.NodeLibrary
	DecisionTree                                   *d.Node
	OriginalAncestorID                             int
	CountdownToChangeDecisionTree                  int
	CyclesSinceLastSpawn                           int
	traits                                         *Traits
}

// InitialHealth returns the health an organism and its children start life with
func (o *Organism) InitialHealth() float64 {
	return o.traits.spawnHealth
}

// HealthCostToReproduce returns the health to lose upon spawning a child
func (o *Organism) HealthCostToReproduce() float64 {
	return o.traits.spawnHealth * -1.0
}

// MinHealthToSpawn returns the minimum health required for an organism to spawn a child
func (o *Organism) MinHealthToSpawn() float64 {
	return o.traits.minHealthToSpawn
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

// MaxSize returns an organism's maximum size
func (o *Organism) MaxSize() float64 {
	return o.traits.maxSize
}

// NewRandomOrganism initializes organism at with random grid location and direction
func NewRandomOrganism(id, x, y int) *Organism {
	nodeLibrary := d.NewNodeLibrary()
	decisionTree := nodeLibrary.GetRandomNode()
	direction, dirX, dirY := u.GetRandomDirection()
	traits := newRandomTraits()
	initialHealth := traits.spawnHealth
	organism := Organism{
		ID:                            id,
		AvgHealth:                     initialHealth,
		Health:                        initialHealth,
		PrevHealth:                    initialHealth,
		Size:                          initialHealth,
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

	nodeLibrary := d.NewNodeLibrary()
	decisionTree := d.CopyTreeByValue(parentDecisionTree)
	decisionTree = nodeLibrary.RegisterAndReturnNewNode(decisionTree)

	direction, dirX, dirY := u.GetRandomDirection()
	traits := parent.traits.copyMutated()
	organism := Organism{
		ID:                            id,
		AvgHealth:                     parent.InitialHealth(),
		Health:                        parent.InitialHealth(),
		PrevHealth:                    parent.InitialHealth(),
		Size:                          parent.InitialHealth(),
		DecisionTree:                  decisionTree,
		Direction:                     direction,
		DirX:                          dirX,
		DirY:                          dirY,
		X:                             x,
		Y:                             y,
		NodeLibrary:                   nodeLibrary,
		OriginalAncestorID:            parent.OriginalAncestorID,
		CountdownToChangeDecisionTree: decisionTree.Complexity * traits.cyclesToEvaluateDecisionTree,

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

// ApplyHealthChange adds a value to the organism's health, bounded by 0 and MaxSize
// If new health is greater than the organism's Size, this is updated too.
func (o *Organism) ApplyHealthChange(change float64) {
	o.Health += change
	if o.Health > o.Size {
		// When eating causes size to increase, increase slowly, not all at once.
		difference := o.Health - o.Size
		o.Size = math.Min(o.Size+(difference*c.GrowthFactor), o.traits.maxSize)
	}
	o.Health = math.Min(math.Max(o.Health, 0.0), o.Size)
}

// UpdateDecisionTree either swaps its current DecisionTree with a new one or,
// if already using the best node for the sought metric, mutates its existing
// algorithm
func (o *Organism) UpdateDecisionTree() {
	o.DecisionTree.SetUsedInCurrentDecisionTree(false)
	best, bestTopLevel := o.NodeLibrary.GetBestNodesForHealth()

	decisionTree := o.DecisionTree
	if rand.Float64() < 0.05 && best != nil {
		decisionTree = best
	} else if bestTopLevel != nil {
		decisionTree = bestTopLevel
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
	size         float64
	health       float64
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
	if len(om.Organisms) < c.MaxOrganismsAllowed && rand.Float64() < c.ChanceToAddOrganism {
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
	healthChange := o.Health - o.PrevHealth

	if o.Health <= 0.0 {
		om.removeOrganism(o.ID)
		return
	}

	beneathMaxOrganisms := len(om.Organisms) < c.MaxOrganismsAllowed
	healthyEnough := o.Health > o.MinHealthToSpawn()
	cyclesPassed := o.CyclesSinceLastSpawn >= o.MinCyclesBetweenSpawns()
	if beneathMaxOrganisms && healthyEnough && cyclesPassed {
		spawnSuccess := om.SpawnChildOrganism(o)
		// compensate for reproduction health lost so it doesn't
		// adversely affect decision tree stats
		if spawnSuccess {
			healthChange -= o.HealthCostToReproduce()
		}
	}

	o.DecisionTree.UpdateStats(healthChange, true, o.CyclesToEvaluateDecisionTree())
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
	isHealthEmergency := o.Health < o.Size*c.HealthPercentToChangeDecisionTree
	return o.CountdownToChangeDecisionTree <= 0 || isHealthEmergency
}

func (om *OrganismManager) evaluateBest(o *Organism) {
	if o.Children > om.MostReproductiveCurrent.children {
		_, decisionTree := o.NodeLibrary.GetBestNodesForHealth()
		if decisionTree == nil {
			decisionTree = o.DecisionTree
		}
		organismInfo := &organismInfo{
			id:           o.ID,
			size:         o.Size,
			health:       o.Health,
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
	organism, _ := om.Organisms[index]
	x, y := organism.X, organism.Y
	om.Grid[x][y] = -1
	om.Environment.AddFoodAtPoint(Point{X: x, Y: y}, int(organism.Size))
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
// Returns true / false depending on whether a child was actually spawned.
func (om *OrganismManager) SpawnChildOrganism(parent *Organism) bool {
	index := om.LastIndexAdded + 1
	if x, y, found := om.getChildSpawnLocation(parent); found {
		parent.CyclesSinceLastSpawn = 0
		parent.Children++
		parent.ApplyHealthChange(parent.HealthCostToReproduce())
		organism := NewChildOrganism(index, x, y, parent)
		om.registerNewOrganism(organism, index)
		return true
	}
	return false
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
	_, hasFood := om.Environment.GetFoodAtPoint(Point{X: x, Y: y})
	return u.IsOnGrid(x, y, width, height) && om.Grid[x][y] == -1 && !hasFood
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
	case d.IsBiggerOrganismAhead:
		return om.isBiggerOrganismAhead(o)
	case d.IsSmallerOrganismAhead:
		return om.isSmallerOrganismAhead(o)
	case d.IsRelatedOrganismAhead:
		return om.isRelatedOrganismAhead(o)
	case d.IsOrganismLeft:
		return om.isOrganismLeft(o)
	case d.IsBiggerOrganismLeft:
		return om.isBiggerOrganismLeft(o)
	case d.IsSmallerOrganismLeft:
		return om.isSmallerOrganismLeft(o)
	case d.IsRelatedOrganismLeft:
		return om.isRelatedOrganismLeft(o)
	case d.IsOrganismRight:
		return om.isOrganismRight(o)
	case d.IsBiggerOrganismRight:
		return om.isBiggerOrganismRight(o)
	case d.IsSmallerOrganismRight:
		return om.isSmallerOrganismRight(o)
	case d.IsRelatedOrganismRight:
		return om.isRelatedOrganismRight(o)
	case d.IsRandomFiftyPercent:
		return rand.Float32() < 0.5
	case d.IsHealthAboveFiftyPercent:
		return o.Health > o.Size*0.5
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
	_, exists := om.Environment.GetFoodAtPoint(Point{X: x, Y: y})
	return exists
}

func (om *OrganismManager) isFoodLeft(o *Organism) bool {
	direction := o.Direction + LeftTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	_, exists := om.Environment.GetFoodAtPoint(Point{X: x, Y: y})
	return exists
}

func (om *OrganismManager) isFoodRight(o *Organism) bool {
	direction := o.Direction + RightTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	_, exists := om.Environment.GetFoodAtPoint(Point{X: x, Y: y})
	return exists
}

func (om *OrganismManager) isOrganismAhead(o *Organism) bool {
	return om.isOrganismAtLocation(o.X+o.DirX, o.Y+o.DirY)
}

func (om *OrganismManager) isBiggerOrganismAhead(o *Organism) bool {
	if organismAhead := om.getOrganismAt(o.X+o.DirX, o.Y+o.DirY); organismAhead != nil {
		return organismAhead.Size > o.Size
	}
	return false
}

func (om *OrganismManager) isSmallerOrganismAhead(o *Organism) bool {
	if organismAhead := om.getOrganismAt(o.X+o.DirX, o.Y+o.DirY); organismAhead != nil {
		return organismAhead.Size < o.Size
	}
	return false
}

func (om *OrganismManager) isRelatedOrganismAhead(o *Organism) bool {
	if organismAhead := om.getOrganismAt(o.X+o.DirX, o.Y+o.DirY); organismAhead != nil {
		return organismAhead.OriginalAncestorID == o.OriginalAncestorID
	}
	return false
}

func (om *OrganismManager) isOrganismLeft(o *Organism) bool {
	direction := o.Direction + LeftTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	return om.isOrganismAtLocation(x, y)
}

func (om *OrganismManager) isBiggerOrganismLeft(o *Organism) bool {
	direction := o.Direction + LeftTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	if organismAhead := om.getOrganismAt(x, y); organismAhead != nil {
		return organismAhead.Size > o.Size
	}
	return false
}

func (om *OrganismManager) isSmallerOrganismLeft(o *Organism) bool {
	direction := o.Direction + LeftTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	if organismAhead := om.getOrganismAt(x, y); organismAhead != nil {
		return organismAhead.Size < o.Size
	}
	return false
}

func (om *OrganismManager) isRelatedOrganismLeft(o *Organism) bool {
	direction := o.Direction + LeftTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	if organismAhead := om.getOrganismAt(x, y); organismAhead != nil {
		return organismAhead.OriginalAncestorID == o.OriginalAncestorID
	}
	return false
}

func (om *OrganismManager) isOrganismRight(o *Organism) bool {
	direction := o.Direction + RightTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	return om.isOrganismAtLocation(x, y)
}

func (om *OrganismManager) isBiggerOrganismRight(o *Organism) bool {
	direction := o.Direction + RightTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	if organismAhead := om.getOrganismAt(x, y); organismAhead != nil {
		return organismAhead.Size > o.Size
	}
	return false
}

func (om *OrganismManager) isSmallerOrganismRight(o *Organism) bool {
	direction := o.Direction + RightTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	if organismAhead := om.getOrganismAt(x, y); organismAhead != nil {
		return organismAhead.Size < o.Size
	}
	return false
}

func (om *OrganismManager) isRelatedOrganismRight(o *Organism) bool {
	direction := o.Direction + RightTurnAngle
	x := o.X + u.CalcDirXForDirection(direction)
	y := o.Y + u.CalcDirYForDirection(direction)
	if organismAhead := om.getOrganismAt(x, y); organismAhead != nil {
		return organismAhead.OriginalAncestorID == o.OriginalAncestorID
	}
	return false
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
	if _, exists := om.Environment.GetFoodAtPoint(Point{X: x, Y: y}); exists {
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
	case d.ActFeed:
		om.applyFeed(o)
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
	o.ApplyHealthChange(c.HealthChangePerCycle * o.Size)
	o.AvgHealth = (o.AvgHealth*float64(o.Age-1) + o.Health) / float64(o.Age)
}

func (om *OrganismManager) applyIdle(o *Organism) {
	o.State = StateIdle
	o.ApplyHealthChange(c.HealthChangeFromBeingIdle * o.Size)
}

func (om *OrganismManager) applyAttack(o *Organism) {
	o.State = StateAttacking
	o.ApplyHealthChange(c.HealthChangeFromAttacking * o.Size)

	x := o.X + o.DirX
	y := o.Y + o.DirY
	if om.isOrganismAtLocation(x, y) {
		targetOrganismIndex := om.Grid[x][y]
		targetOrganism := om.Organisms[targetOrganismIndex]
		targetOrganism.ApplyHealthChange(c.HealthChangeInflictedByAttack * o.Size)
	}
}

func (om *OrganismManager) applyFeed(o *Organism) {
	o.State = StateFeeding
	o.ApplyHealthChange(c.HealthChangeFromFeeding * o.Size)

	amountToFeed := c.HealthChangeFromFeeding * o.Size
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if om.isOrganismAtLocation(x, y) {
		targetOrganismIndex := om.Grid[x][y]
		targetOrganism := om.Organisms[targetOrganismIndex]
		targetOrganism.ApplyHealthChange(amountToFeed)
	} else {
		om.Environment.AddFoodAtPoint(Point{X: x, Y: y}, int(amountToFeed))
	}
}

func (om *OrganismManager) applyEat(o *Organism) {
	o.State = StateEating
	o.ApplyHealthChange(c.HealthChangeFromEatingAttempt * o.Size)

	x := o.X + o.DirX
	y := o.Y + o.DirY
	if value, exists := om.Environment.GetFoodAtPoint(Point{X: x, Y: y}); exists {
		maxCanEat := o.Size
		amountToEat := math.Min(float64(value), maxCanEat)
		om.Environment.RemoveFood(Point{X: x, Y: y}, int(amountToEat))
		o.ApplyHealthChange(amountToEat)
	}
}

func (om *OrganismManager) applyMove(o *Organism) {
	o.State = StateMoving
	o.ApplyHealthChange(c.HealthChangeFromMoving * o.Size)

	if om.canMove(o) {
		om.Grid[o.X][o.Y] = -1
		o.X += o.DirX
		o.Y += o.DirY
		om.Grid[o.X][o.Y] = o.ID
	}
}

func (om *OrganismManager) applyTurn(o *Organism, directionChange float64) {
	o.State = StateTurning
	o.ApplyHealthChange(c.HealthChangeFromTurning * o.Size)

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
	fmt.Printf("\n      ID: %10d   |         InitialHealth: %4d", info.id, int(info.traits.spawnHealth))
	fmt.Printf("\n     Age: %10d   |      MinHealthToSpawn: %4d", info.age, int(info.traits.minHealthToSpawn))
	fmt.Printf("\nChildren: %10d   |      MinCyclesToSpawn: %4d", info.children, info.traits.minCyclesBetweenSpawns)
	fmt.Printf("\nAncestor: %10d   |  CyclesToEvaluateTree: %4d", info.ancestorID, info.traits.cyclesToEvaluateDecisionTree)
	fmt.Printf("\n  Health: %10.2f   |   ChanceToMutateTree:  %4.2f", info.health, info.traits.chanceToMutateDecisionTree)
	fmt.Printf("\n    Size: %10.2f   |              MaxSize:  %4.2f", info.size, info.traits.maxSize)
	fmt.Printf("\n  DecisionTree:\n%s", info.decisionTree)
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
