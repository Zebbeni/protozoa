package manager

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	c "github.com/Zebbeni/protozoa/constants"
	"github.com/Zebbeni/protozoa/decisions"
	d "github.com/Zebbeni/protozoa/decisions"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

var descendantsPrintThreshold = 10

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	worldAPI organism.WorldAPI

	organisms             map[int]*organism.Organism
	organismIDGrid        [][]int
	totalOrganismsCreated int

	organismUpdateOrder []int
	newOrganismIDs      []int

	MostReproductiveAllTime  *organismInfo
	MostReproductiveCurrent  *organismInfo
	AncestorDescendantsCount map[int]int

	UpdateDuration, ResolveDuration time.Duration
}

type organismInfo struct {
	id           int
	size         float64
	health       float64
	ancestorID   int
	age          int
	children     int
	decisionTree string
	traits       organism.Traits
}

func (o *organismInfo) ID() int {
	return o.id
}

// NewOrganismManager creates all Organisms and updates grid
func NewOrganismManager(api organism.WorldAPI) *OrganismManager {
	grid := initializeGrid()
	organisms := make(map[int]*organism.Organism)
	manager := &OrganismManager{
		worldAPI:                 api,
		organismIDGrid:           grid,
		organisms:                organisms,
		organismUpdateOrder:      make([]int, 0, c.MaxOrganisms),
		newOrganismIDs:           make([]int, 0, 100),
		AncestorDescendantsCount: make(map[int]int),
		MostReproductiveAllTime:  &organismInfo{traits: organism.Traits{}},
		MostReproductiveCurrent:  &organismInfo{traits: organism.Traits{}},
	}
	return manager
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (m *OrganismManager) Update() {
	m.MostReproductiveCurrent = &organismInfo{traits: organism.Traits{}}
	// Periodically add new random organisms if population below a certain amount
	if len(m.organisms) < c.MaxOrganisms && rand.Float64() < c.ChanceToAddOrganism {
		m.SpawnRandomOrganism()
	}
	// FUTURE: do this multi-threaded
	start := time.Now()
	for _, id := range m.organismUpdateOrder {
		m.updateOrganism(m.organisms[id])
	}
	m.UpdateDuration = time.Since(start)
	start = time.Now()
	for _, id := range m.organismUpdateOrder {
		m.resolveOrganismAction(m.organisms[id])
	}
	m.ResolveDuration = time.Since(start)
	m.updateOrganismOrder()
}

// updateOrganismOrder creates a new ordered list of all organismIDs that are
// alive after the current cycle, appending any newly spawned organisms.
// This means iterating the full list of organisms again, but this should be
// faster than just deleting the dead IDs and shifting all others to the left
func (m *OrganismManager) updateOrganismOrder() {
	orderedIDs := append(m.organismUpdateOrder, m.newOrganismIDs...)
	organismUpdateOrder := make([]int, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if _, ok := m.organisms[id]; ok {
			organismUpdateOrder = append(organismUpdateOrder, id)
		}
	}
	m.organismUpdateOrder = organismUpdateOrder
	m.newOrganismIDs = make([]int, 0, 100)
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

func (m *OrganismManager) updateOrganism(o *organism.Organism) {
	if o.Action() == decisions.ActAttack {
		m.worldAPI.AddGridPointToUpdate(o.Location)
	}
	o.UpdateStats()
	o.UpdateAction()
}

func (m *OrganismManager) resolveOrganismAction(o *organism.Organism) {
	if o == nil {
		return
	}
	m.updateHealth(o)
	m.applyAction(o)
	m.evaluateBest(o)
	m.removeIfDead(o)
}

func (m *OrganismManager) evaluateBest(o *organism.Organism) {
	if o.Children > m.MostReproductiveCurrent.children {
		decisionTree := o.GetBestDecisionTreeCopy(true)
		if decisionTree == nil {
			decisionTree = o.GetCurrentDecisionTreeCopy(true)
		}
		organismInfo := &organismInfo{
			id:           o.ID,
			size:         o.Size,
			health:       o.Health,
			ancestorID:   o.OriginalAncestorID,
			decisionTree: decisionTree.Print("", true, false),
			age:          o.Age,
			children:     o.Children,
			traits:       o.Traits(),
		}
		m.MostReproductiveCurrent = organismInfo

		if o.Children > m.MostReproductiveAllTime.children {
			m.MostReproductiveAllTime = organismInfo
		}
	}
}

// SpawnRandomOrganism creates an Organism with random position.
//
// Checks random positions on the grid until it finds an empty one. Calls
// NewOrganism to initialize decision tree, other random attributes.
func (m *OrganismManager) SpawnRandomOrganism() {
	if spawnPoint, found := m.getRandomSpawnLocation(); found {
		index := m.totalOrganismsCreated
		organism := organism.NewRandom(index, spawnPoint, m.worldAPI)
		m.registerNewOrganism(organism, index)
	}
}

// SpawnChildOrganism creates a new organism near an existing 'parent' organism
// with a copy of its parent's node library. (No organism created if no room)
// Returns true / false depending on whether a child was actually spawned.
func (m *OrganismManager) SpawnChildOrganism(parent *organism.Organism) bool {
	if spawnPoint, found := m.getChildSpawnLocation(parent); found {
		index := m.totalOrganismsCreated
		organism := parent.NewChild(index, spawnPoint, m.worldAPI)
		m.registerNewOrganism(organism, index)
		return true
	}
	return false
}

func (m *OrganismManager) registerNewOrganism(o *organism.Organism, index int) {
	m.worldAPI.AddGridPointToUpdate(o.Location)

	m.organisms[index] = o
	m.totalOrganismsCreated++
	m.organismIDGrid[o.X()][o.Y()] = index
	m.newOrganismIDs = append(m.newOrganismIDs, index)

	// update ancestors
	ancestorID := o.OriginalAncestorID
	if ancestorID != o.ID {
		if _, ok := m.AncestorDescendantsCount[ancestorID]; !ok {
			m.AncestorDescendantsCount[ancestorID] = 0
		}
		m.AncestorDescendantsCount[ancestorID]++
	}
}

// returns a random point and whether it is empty
func (m *OrganismManager) getRandomSpawnLocation() (utils.Point, bool) {
	point := utils.GetRandomPoint()
	return point, m.isGridLocationEmpty(point)
}

func (m *OrganismManager) getChildSpawnLocation(parent *organism.Organism) (utils.Point, bool) {
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

func (m *OrganismManager) isGridLocationEmpty(point utils.Point) bool {
	return !m.isFoodAtLocation(point) && !m.isOrganismAtLocation(point)
}

func (m *OrganismManager) isFoodAtLocation(point utils.Point) bool {
	return m.worldAPI.CheckFoodAtPoint(point, func(item *food.Item) bool {
		return item != nil
	})
}

func (m *OrganismManager) isOrganismAtLocation(point utils.Point) bool {
	return m.organismIDGrid[point.X][point.Y] != -1
}

func (m *OrganismManager) getOrganismAt(point utils.Point) *organism.Organism {
	if id, exists := m.getOrganismIDAt(point); exists {
		index := id
		return m.organisms[index]
	}
	return nil
}

func (m *OrganismManager) getOrganismIDAt(point utils.Point) (int, bool) {
	id := m.organismIDGrid[point.X][point.Y]
	if id != -1 {
		return id, true
	}
	return -1, false
}

// CheckOrganismAtPoint returns the result of running a check against any Organism
// found at a given Point.
func (m *OrganismManager) CheckOrganismAtPoint(point utils.Point, checkFunc organism.OrgCheck) bool {
	return checkFunc(m.getOrganismAt(point))
}

// GetOrganismAtPoint returns the Organism at the given point (nil if none)
func (m *OrganismManager) GetOrganismAtPoint(point utils.Point) *organism.Organism {
	if id, found := m.getOrganismIDAt(point); found {
		return m.organisms[id]
	}
	return nil
}

// OrganismCount returns the current number of organisms alive in the simulation
func (m *OrganismManager) OrganismCount() int {
	return len(m.organisms)
}

func (m *OrganismManager) applyAction(o *organism.Organism) {
	switch o.Action() {
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
		m.applyLeftTurn(o)
		break
	case d.ActTurnRight:
		m.applyRightTurn(o)
		break
	case d.ActSpawn:
		m.applySpawn(o)
		break
	}
}

func (m *OrganismManager) updateHealth(o *organism.Organism) {
	o.ApplyHealthChange(c.HealthChangePerCycle * o.Size)
}

func (m *OrganismManager) applyIdle(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromBeingIdle*o.Size)
}

func (m *OrganismManager) applyHealthChange(o *organism.Organism, amount float64) {
	prevSize := o.Size
	o.ApplyHealthChange(amount)
	if o.Size > prevSize {
		m.worldAPI.AddGridPointToUpdate(o.Location)
	}
}

func (m *OrganismManager) applyAttack(o *organism.Organism) {
	m.worldAPI.AddGridPointToUpdate(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromAttacking*o.Size)
	targetPoint := o.Location.Add(o.Direction)
	if m.isOrganismAtLocation(targetPoint) {
		targetOrganismIndex := m.organismIDGrid[targetPoint.X][targetPoint.Y]
		targetOrganism := m.organisms[targetOrganismIndex]
		m.applyHealthChange(targetOrganism, c.HealthChangeInflictedByAttack*o.Size)
		m.removeIfDead(targetOrganism)
	}
}

func (m *OrganismManager) removeIfDead(o *organism.Organism) bool {
	if o.Health > 0.0 {
		return false
	}
	m.worldAPI.AddGridPointToUpdate(o.Location)
	m.organismIDGrid[o.Location.X][o.Location.Y] = -1
	m.worldAPI.AddFoodAtPoint(o.Location, int(o.Size))
	delete(m.organisms, o.ID)
	return true
}

func (m *OrganismManager) applySpawn(o *organism.Organism) {
	if success := m.SpawnChildOrganism(o); success {
		o.ApplyHealthChange(o.HealthCostToReproduce())
		o.CyclesSinceLastSpawn = 0
		o.Children++
	}
}

func (m *OrganismManager) applyFeed(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromFeeding*o.Size)
	amountToFeed := c.HealthChangeFromFeeding * o.Size
	targetPoint := o.Location.Add(o.Direction)
	if m.isOrganismAtLocation(targetPoint) {
		targetOrganismIndex := m.organismIDGrid[targetPoint.X][targetPoint.Y]
		targetOrganism := m.organisms[targetOrganismIndex]
		m.applyHealthChange(targetOrganism, amountToFeed)
	} else {
		m.worldAPI.AddFoodAtPoint(targetPoint, int(amountToFeed))
	}
}

func (m *OrganismManager) applyEat(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromEatingAttempt*o.Size)
	targetPoint := o.Location.Add(o.Direction)
	if value, exists := m.worldAPI.GetFoodAtPoint(targetPoint); exists {
		maxCanEat := o.Size
		amountToEat := math.Min(float64(value), maxCanEat)
		m.worldAPI.RemoveFoodAtPoint(targetPoint, int(amountToEat))
		m.applyHealthChange(o, amountToEat)
	}
}

func (m *OrganismManager) applyMove(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromMoving*o.Size)

	targetPoint := o.Location.Add(o.Direction)
	if m.isGridLocationEmpty(targetPoint) {
		m.worldAPI.AddGridPointToUpdate(o.Location)
		m.worldAPI.AddGridPointToUpdate(targetPoint)

		m.organismIDGrid[o.Location.X][o.Location.Y] = -1
		m.organismIDGrid[targetPoint.X][targetPoint.Y] = o.ID
		o.Location = targetPoint
	}
}

func (m *OrganismManager) applyRightTurn(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromTurning*o.Size)

	o.Direction = o.Direction.Right()
}

func (m *OrganismManager) applyLeftTurn(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromTurning*o.Size)

	o.Direction = o.Direction.Left()
}

// GetOrganisms returns an array of all Organisms from organism manager
func (m *OrganismManager) GetOrganisms() map[int]*organism.Organism {
	return m.organisms
}

// PrintBest prints the highest current score of any Organism (and their index)
func (m *OrganismManager) PrintBest() {
	m.printBestAncestors()
	fmt.Print("\n\n")
	m.printBestCurrent()
	fmt.Print("\n\n")
	m.printBestAllTime()
}

func (m *OrganismManager) printBestCurrent() {
	fmt.Printf("\n  - Best Organism Current - \n")
	m.printOrganismInfo(m.MostReproductiveCurrent)
}

func (m *OrganismManager) printBestAllTime() {
	fmt.Printf("\n  - Best Organism All Time - \n")
	m.printOrganismInfo(m.MostReproductiveAllTime)
}

func (m *OrganismManager) printOrganismInfo(info *organismInfo) {
	fmt.Printf("\n      ID: %10d   |         InitialHealth: %4d", info.id, int(info.traits.SpawnHealth))
	fmt.Printf("\n     Age: %10d   |      MinHealthToSpawn: %4d", info.age, int(info.traits.MinHealthToSpawn))
	fmt.Printf("\nChildren: %10d   |      MinCyclesToSpawn: %4d", info.children, info.traits.MinCyclesBetweenSpawns)
	fmt.Printf("\nAncestor: %10d   |  CyclesToEvaluateTree: %4d", info.ancestorID, info.traits.CyclesToEvaluateDecisionTree)
	fmt.Printf("\n  Health: %10.2f   |   ChanceToMutateTree:  %4.2f", info.health, info.traits.ChanceToMutateDecisionTree)
	fmt.Printf("\n    Size: %10.2f   |              MaxSize:  %4.2f", info.size, info.traits.MaxSize)
	fmt.Printf("\n  DecisionTree:\n%s", info.decisionTree)
}

func (m *OrganismManager) printBestAncestors() {
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
