package organism

import (
	"fmt"
	"math"
	"math/rand"

	c "github.com/Zebbeni/protozoa/constants"
	d "github.com/Zebbeni/protozoa/decisions"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/utils"
)

var descendantsPrintThreshold = 1

// Manager contains 2D array of booleans showing if organism present
type Manager struct {
	worldAPI WorldAPI

	organisms      map[int]*Organism
	organismIDGrid [][]int

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

// NewManager creates all Organisms and updates grid
func NewManager(api WorldAPI) *Manager {
	grid := initializeGrid()
	organisms := make(map[int]*Organism)
	manager := &Manager{
		worldAPI:                 api,
		organismIDGrid:           grid,
		organisms:                organisms,
		AncestorDescendantsCount: make(map[int]int),
		MostReproductiveAllTime:  &organismInfo{traits: &Traits{}},
		MostReproductiveCurrent:  &organismInfo{traits: &Traits{}},
	}
	return manager
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (m *Manager) Update() {
	m.MostReproductiveCurrent = &organismInfo{traits: &Traits{}}
	// Periodically add new random organisms if population below a certain amount
	if len(m.organisms) < c.MaxOrganismsAllowed && rand.Float64() < c.ChanceToAddOrganism {
		m.SpawnRandomOrganism()
	}
	// FUTURE: do this multi-threaded
	for _, o := range m.organisms {
		m.updateOrganism(o)
	}
	// FUTURE: Do this in order from lowest to highest id
	for _, o := range m.organisms {
		m.resolveOrganismAction(o)
	}
}

func initializeGrid() [][]int {
	grid := make([][]int, c.GridWidth)
	for r := 0; r < c.GridWidth; r++ {
		grid[r] = make([]int, c.GridHeight)
	}
	for x := 0; x < c.GridWidth; x++ {
		for y := 0; y < c.GridHeight; y++ {
			grid[x][y] = -1
		}
	}
	return grid
}

// UpdateOrganism updates an Organism's Age, runs its Action cycle, updates
// its Health, and replaces it if its Health <= 0
func (m *Manager) updateOrganism(o *Organism) {
	m.updateStats(o)
	healthChange := o.Health - o.PrevHealth

	if o.Health <= 0.0 {
		m.removeOrganism(o.ID)
		return
	}

	beneathMaxOrganisms := len(m.organisms) < c.MaxOrganismsAllowed
	healthyEnough := o.Health > o.MinHealthToSpawn()
	cyclesPassed := o.CyclesSinceLastSpawn >= o.MinCyclesBetweenSpawns()
	if beneathMaxOrganisms && healthyEnough && cyclesPassed {
		spawnSuccess := m.SpawnChildOrganism(o)
		// compensate for reproduction health lost so it doesn't
		// adversely affect decision tree stats
		if spawnSuccess {
			healthChange -= o.HealthCostToReproduce()
		}
	}

	// o.DecisionTree.UpdateStats(healthChange, true, o.CyclesToEvaluateDecisionTree())
	// o.PrevHealth = o.Health

	// if o.shouldChangeDecisionTree() {
	// 	o.UpdateDecisionTree()
	// }

	// action := m.chooseAction(o, o.DecisionTree)

	m.applyAction(o, action)
	m.evaluateBest(o)

	// o.PruneUnusedNodes()
}

func (m *Manager) UpdateOrganism(o *Organism) {
	o.UpdateStats()
	o.UpdateAction()
}

// we want to be able to run this multiple times at once, so any changes we make
// here should be read-only except for modifying the Organism's own state
func (m *Manager) updateOrganismAction(o *Organism) {
	// calculate health change since last cycle
	o.Update()
	// update last-used decision tree stats

	// (possibly) update current decision tree

	// choose action from decision tree

	// prune nodes
}

func (m *Manager) resolveOrganismAction(o *Organism) {
	// if healthy enough to spawn, create child

	// attempt to fulfill requested action

	// if health beneath 0, remove
}

func (m *Manager) evaluateBest(o *Organism) {
	if o.Children > m.MostReproductiveCurrent.children {
		decisionTree := o.GetBestDecisionTreeCopy()
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
		m.MostReproductiveCurrent = organismInfo

		if o.Children > m.MostReproductiveAllTime.children {
			m.MostReproductiveAllTime = organismInfo
		}
	}
}

func (m *Manager) removeOrganism(index int) {
	o, _ := m.organisms[index]
	m.organismIDGrid[o.Location.X][o.Location.Y] = -1
	m.worldAPI.AddFoodAtPoint(o.Location, int(o.Size))
	delete(m.organisms, index)
}

// SpawnRandomOrganism creates an Organism with random position.
//
// Checks random positions on the grid until it finds an empty one. Calls
// NewOrganism to initialize decision tree, other random attributes.
func (m *Manager) SpawnRandomOrganism() {
	index := m.lastIndexAdded + 1
	spawnPoint := m.getRandomSpawnLocation()
	organism := NewRandom(index, spawnPoint, m.worldAPI)
	m.registerNewOrganism(organism, index)
}

// SpawnChildOrganism creates a new organism near an existing 'parent' organism
// with a copy of its parent's node library. (No organism created if no room)
// Returns true / false depending on whether a child was actually spawned.
func (m *Manager) SpawnChildOrganism(parent *Organism) bool {
	index := m.lastIndexAdded + 1
	if x, y, found := m.getChildSpawnLocation(parent); found {
		parent.CyclesSinceLastSpawn = 0
		parent.Children++
		parent.ApplyHealthChange(parent.HealthCostToReproduce())
		organism := NewChild(parent, index, x, y)
		m.registerNewOrganism(organism, index)
		return true
	}
	return false
}

func (m *Manager) registerNewOrganism(o *Organism, index int) {
	m.organisms[index] = o
	m.organismIDGrid[o.X()][o.Y()] = index
	m.lastIndexAdded = index

	ancestorID := o.OriginalAncestorID
	if ancestorID != o.ID {
		if _, ok := m.AncestorDescendantsCount[ancestorID]; !ok {
			m.AncestorDescendantsCount[ancestorID] = 0
		}
		m.AncestorDescendantsCount[ancestorID]++
	}
}

func (m *Manager) getRandomSpawnLocation() utils.Point {
	point := utils.GetRandomPoint()
	for !m.isGridLocationEmpty(point) {
		point = utils.GetRandomPoint()
	}
	return point
}

func (m *Manager) getChildSpawnLocation(parent *Organism) (utils.Point, bool) {
	direction := utils.GetRandomDirection()
	point := parent.Location.Add(direction)
	for i := 0; i < 4; i++ {
		if m.isGridLocationEmpty(point) {
			return point, true
		}
		direction = direction.Left()
		point = parent.Location.Add(direction)
	}
	return point, false
}

func (m *Manager) isGridLocationEmpty(point utils.Point) bool {
	hasFood := m.worldAPI.CheckFoodAtPoint(point, func(item *food.Item) bool {
		return item != nil
	})
	return utils.IsOnGrid(point) && m.organismIDGrid[x][y] == -1 && !hasFood
}

func (m *Manager) isOrganismAtLocation(x, y int) bool {
	width := c.GridWidth
	height := c.GridHeight
	return utils.IsOnGrid(x, y, width, height) && m.organismIDGrid[x][y] != -1
}

// chooseAction walks through nodes of an organism's decision tree, eventually
// returning the chosen action
//
// As chooseAction walks thorugh nodes, it also populates nodes to update metriic
// information for the next update run, diminishing the use value with each level
func (m *Manager) chooseAction(o *Organism, tree *d.Node) interface{} {
	tree.UsedLastCycle = true
	if tree.IsAction() {
		return tree.NodeType
	}
	condition := tree.NodeType
	if m.isConditionTrue(o, condition) {
		return m.chooseAction(o, tree.YesNode)
	}
	return m.chooseAction(o, tree.NoNode)
}

func (m *Manager) isConditionTrue(o *Organism, cond interface{}) bool {
	switch cond {
	case d.CanMove:
		return m.canMove(o)
	case d.IsFoodAhead:
		return m.isFoodAhead(o)
	case d.IsFoodLeft:
		return m.isFoodLeft(o)
	case d.IsFoodRight:
		return m.isFoodRight(o)
	case d.IsOrganismAhead:
		return m.isOrganismAhead(o)
	case d.IsBiggerOrganismAhead:
		return m.isBiggerOrganismAhead(o)
	case d.IsSmallerOrganismAhead:
		return m.isSmallerOrganismAhead(o)
	case d.IsRelatedOrganismAhead:
		return m.isRelatedOrganismAhead(o)
	case d.IsOrganismLeft:
		return m.isOrganismLeft(o)
	case d.IsBiggerOrganismLeft:
		return m.isBiggerOrganismLeft(o)
	case d.IsSmallerOrganismLeft:
		return m.isSmallerOrganismLeft(o)
	case d.IsRelatedOrganismLeft:
		return m.isRelatedOrganismLeft(o)
	case d.IsOrganismRight:
		return m.isOrganismRight(o)
	case d.IsBiggerOrganismRight:
		return m.isBiggerOrganismRight(o)
	case d.IsSmallerOrganismRight:
		return m.isSmallerOrganismRight(o)
	case d.IsRelatedOrganismRight:
		return m.isRelatedOrganismRight(o)
	case d.IsRandomFiftyPercent:
		return rand.Float32() < 0.5
	case d.IsHealthAboveFiftyPercent:
		return o.Health > o.Size*0.5
	}
	return false
}

func (m *Manager) getOrganismAt(point utils.Point) *Organism {
	if !utils.IsOnGrid(point) {
		return nil
	}
	if id, exists := m.getOrganismIDAt(point); exists {
		index := id
		return m.organisms[index]
	}
	return nil
}

func (m *Manager) getOrganismIDAt(point utils.Point) (int, bool) {
	id := m.organismIDGrid[point.X][point.Y]
	if id != -1 {
		return id, true
	}
	return -1, false
}

// CheckOrganismAtPoint returns the result of running a check against any Organism
// found at a given Point.
func (m *Manager) CheckOrganismAtPoint(point utils.Point, checkFunc OrgCheck) bool {
	return checkFunc(m.getOrganismAt(point))
}

func (m *Manager) applyAction(o *Organism, action interface{}) {
	o.State = StateIdle // default to idle so other functions don't need to
	switch action {
	case d.ActIdle:
		m.applyIdle(o)
		break
	case d.ActAttack:
		m.applyAttack(o)
		break
	case d.ActFeed:
		m.applyFeed(o)
		break
	case d.ActEat:
		m.applyEat(o)
		break
	case d.ActMove:
		m.applyMove(o)
		break
	case d.ActTurnLeft:
		m.applyTurn(o, LeftTurnAngle)
		break
	case d.ActTurnRight:
		m.applyTurn(o, RightTurnAngle)
		break
	}
}

func (m *Manager) updateHealth(o *Organism) {
	o.ApplyHealthChange(c.HealthChangePerCycle * o.Size)
}

func (m *Manager) applyIdle(o *Organism) {
	o.State = StateIdle
	o.ApplyHealthChange(c.HealthChangeFromBeingIdle * o.Size)
}

func (m *Manager) applyAttack(o *Organism) {
	o.State = StateAttacking
	o.ApplyHealthChange(c.HealthChangeFromAttacking * o.Size)

	x := o.X + o.DirX
	y := o.Y + o.DirY
	if m.isOrganismAtLocation(x, y) {
		targetOrganismIndex := m.organismIDGrid[x][y]
		targetOrganism := m.organisms[targetOrganismIndex]
		targetOrganism.ApplyHealthChange(c.HealthChangeInflictedByAttack * o.Size)
	}
}

func (m *Manager) applyFeed(o *Organism) {
	o.State = StateFeeding
	o.ApplyHealthChange(c.HealthChangeFromFeeding * o.Size)

	amountToFeed := c.HealthChangeFromFeeding * o.Size
	x := o.X + o.DirX
	y := o.Y + o.DirY
	if m.isOrganismAtLocation(x, y) {
		targetOrganismIndex := m.organismIDGrid[x][y]
		targetOrganism := m.organisms[targetOrganismIndex]
		targetOrganism.ApplyHealthChange(amountToFeed)
	} else {
		m.Environment.AddFoodAtPoint(Point{X: x, Y: y}, int(amountToFeed))
	}
}

func (m *Manager) applyEat(o *Organism) {
	o.State = StateEating
	o.ApplyHealthChange(c.HealthChangeFromEatingAttempt * o.Size)

	x := o.X + o.DirX
	y := o.Y + o.DirY
	if value, exists := m.Environment.GetFoodAtPoint(Point{X: x, Y: y}); exists {
		maxCanEat := o.Size
		amountToEat := math.Min(float64(value), maxCanEat)
		m.Environment.RemoveFood(Point{X: x, Y: y}, int(amountToEat))
		o.ApplyHealthChange(amountToEat)
	}
}

func (m *Manager) applyMove(o *Organism) {
	o.State = StateMoving
	o.ApplyHealthChange(c.HealthChangeFromMoving * o.Size)

	if m.canMove(o) {
		m.organismIDGrid[o.X][o.Y] = -1
		o.X += o.DirX
		o.Y += o.DirY
		m.organismIDGrid[o.X][o.Y] = o.ID
	}
}

func (m *Manager) applyTurn(o *Organism, directionChange float64) {
	o.State = StateTurning
	o.ApplyHealthChange(c.HealthChangeFromTurning * o.Size)

	o.Direction += directionChange
	o.DirX = utils.CalcDirXForDirection(o.Direction)
	o.DirY = utils.CalcDirYForDirection(o.Direction)
}

// GetOrganisms returns an array of all Organisms from organism manager
func (m *Manager) GetOrganisms() map[int]*Organism {
	return m.organisms
}

// PrintBest prints the highest current score of any Organism (and their index)
func (m *Manager) PrintBest() {
	m.printBestAncestors()
	fmt.Print("\n\n")
	m.printBestCurrent()
	fmt.Print("\n\n")
	m.printBestAllTime()
}

func (m *Manager) printBestCurrent() {
	fmt.Printf("\n  - Best Organism Current - \n")
	m.printOrganismInfo(m.MostReproductiveCurrent)
}

func (m *Manager) printBestAllTime() {
	fmt.Printf("\n  - Best Organism All Time - \n")
	m.printOrganismInfo(m.MostReproductiveAllTime)
}

func (m *Manager) printOrganismInfo(info *organismInfo) {
	fmt.Printf("\n      ID: %10d   |         InitialHealth: %4d", info.id, int(info.traits.spawnHealth))
	fmt.Printf("\n     Age: %10d   |      MinHealthToSpawn: %4d", info.age, int(info.traits.minHealthToSpawn))
	fmt.Printf("\nChildren: %10d   |      MinCyclesToSpawn: %4d", info.children, info.traits.minCyclesBetweenSpawns)
	fmt.Printf("\nAncestor: %10d   |  CyclesToEvaluateTree: %4d", info.ancestorID, info.traits.cyclesToEvaluateDecisionTree)
	fmt.Printf("\n  Health: %10.2f   |   ChanceToMutateTree:  %4.2f", info.health, info.traits.chanceToMutateDecisionTree)
	fmt.Printf("\n    Size: %10.2f   |              MaxSize:  %4.2f", info.size, info.traits.maxSize)
	fmt.Printf("\n  DecisionTree:\n%s", info.decisionTree)
}

func (m *Manager) printBestAncestors() {
	fmt.Printf("\n - Original Ancestors: %d\n", len(m.AncestorDescendantsCount))
	fmt.Printf("   Best (%d descendants or more) -\n", descendantsPrintThreshold)
	fmt.Print("  Ancestor ID  | Descendants\n")

	// updateThreshold := false
	for ancestorID, descendants := range m.AncestorDescendantsCount {
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
