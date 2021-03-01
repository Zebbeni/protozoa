package manager

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"sort"
	"time"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

var descendantsPrintThreshold = 10

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	api organism.API

	organisms             map[int]*organism.Organism
	organismIDGrid        [][]int
	totalOrganismsCreated int

	organismUpdateOrder []int
	newOrganismIDs      []int

	MostReproductiveAllTime  *organism.Organism
	MostReproductiveCurrent  *organism.Organism
	AncestorDescendantsCount map[int]int

	originalAncestorsSorted []int
	originalAncestorColors  map[int]color.Color   // all original ancestor IDs with at least one descendant
	populationHistory       map[int]map[int]int16 // cycle : ancestorId : livingDescendantsCount

	UpdateDuration, ResolveDuration time.Duration
}

// NewOrganismManager creates all Organisms and updates grid
func NewOrganismManager(api organism.API) *OrganismManager {
	grid := initializeGrid()
	organisms := make(map[int]*organism.Organism)
	manager := &OrganismManager{
		api:                      api,
		organismIDGrid:           grid,
		organisms:                organisms,
		organismUpdateOrder:      make([]int, 0, c.MaxOrganisms()),
		newOrganismIDs:           make([]int, 0, 100),
		AncestorDescendantsCount: make(map[int]int),
		MostReproductiveAllTime:  &organism.Organism{},
		MostReproductiveCurrent:  &organism.Organism{},
		originalAncestorColors:   make(map[int]color.Color),
		populationHistory:        make(map[int]map[int]int16),
	}
	return manager
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (m *OrganismManager) Update() {
	m.MostReproductiveCurrent = &organism.Organism{}
	// Periodically add new random organisms if population below a certain amount
	if len(m.organisms) < c.MaxOrganisms() && rand.Float64() < c.ChanceToAddOrganism() {
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
	m.updateHistory()
}

// updateHistory updates the population map for all living organisms
func (m *OrganismManager) updateHistory() {
	cycle := m.api.Cycle()
	if cycle%c.PopulationUpdateInterval() != 0 {
		return
	}

	populationMap := make(map[int]int16)
	for _, o := range m.organisms {
		if o.OriginalAncestorID == o.ID {
			continue
		}

		if _, ok := populationMap[o.OriginalAncestorID]; !ok {
			populationMap[o.OriginalAncestorID] = 0
		}
		populationMap[o.OriginalAncestorID]++
	}

	m.populationHistory[cycle] = populationMap
}

// GetHistory returns the full population history of all original ancestors as a
// map of cycles to maps of ancestorIDs to the living descendants at that time
func (m *OrganismManager) GetHistory() map[int]map[int]int16 {
	return m.populationHistory
}

// GetAncestorColors returns a map all original ancestor IDs to their color
func (m *OrganismManager) GetAncestorColors() map[int]color.Color {
	return m.originalAncestorColors
}

// GetAncestorsSorted returns a list of all original ancestor IDs in order
func (m *OrganismManager) GetAncestorsSorted() []int {
	return m.originalAncestorsSorted
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
	grid := make([][]int, c.GridUnitsWide())
	for r := 0; r < c.GridUnitsWide(); r++ {
		grid[r] = make([]int, c.GridUnitsHigh())
	}
	for x := 0; x < c.GridUnitsWide(); x++ {
		for y := 0; y < c.GridUnitsHigh(); y++ {
			grid[x][y] = -1
		}
	}
	return grid
}

func (m *OrganismManager) updateOrganism(o *organism.Organism) {
	if o.Action() == d.ActAttack {
		m.api.AddUpdatedGridPoint(o.Location)
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
	if o.Children > m.MostReproductiveCurrent.Children {
		decisionTree := o.GetBestDecisionTreeCopy(true)
		if decisionTree == nil {
			decisionTree = o.GetCurrentDecisionTreeCopy(true)
		}
		m.MostReproductiveCurrent = o

		if o.Children > m.MostReproductiveAllTime.Children {
			m.MostReproductiveAllTime = o
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
		o := organism.NewRandom(index, spawnPoint, m.api)
		m.registerNewOrganism(o, index)
	}
}

// SpawnChildOrganism creates a new organism near an existing 'parent' organism
// with a copy of its parent's node library. (No organism created if no room)
// Returns true / false depending on whether a child was actually spawned.
func (m *OrganismManager) SpawnChildOrganism(parent *organism.Organism) bool {
	if spawnPoint, found := m.getChildSpawnLocation(parent); found {
		index := m.totalOrganismsCreated
		o := parent.NewChild(index, spawnPoint, m.api)
		m.registerNewOrganism(o, index)
		m.addToOriginalAncestors(parent)
		return true
	}
	return false
}

func (m *OrganismManager) registerNewOrganism(o *organism.Organism, index int) {
	m.api.AddUpdatedGridPoint(o.Location)

	m.organisms[index] = o
	m.totalOrganismsCreated++
	m.organismIDGrid[o.X()][o.Y()] = index
	m.newOrganismIDs = append(m.newOrganismIDs, index)

	// update originalAncestorColors
	ancestorID := o.OriginalAncestorID
	if ancestorID != o.ID {
		if _, ok := m.AncestorDescendantsCount[ancestorID]; !ok {
			m.AncestorDescendantsCount[ancestorID] = 0
		}
		m.AncestorDescendantsCount[ancestorID]++
	}
}

func (m *OrganismManager) addToOriginalAncestors(o *organism.Organism) {
	if _, ok := m.originalAncestorColors[o.ID]; ok {
		return
	}
	m.originalAncestorColors[o.ID] = o.Color()
	m.originalAncestorsSorted = append(m.originalAncestorsSorted, o.ID)
	sort.Ints(m.originalAncestorsSorted)
}

// returns a random point and whether it is empty
func (m *OrganismManager) getRandomSpawnLocation() (utils.Point, bool) {
	point := utils.GetRandomPoint(c.GridUnitsWide(), c.GridUnitsHigh())
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
	return m.api.CheckFoodAtPoint(point, func(item *food.Item) bool {
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

// GetOrganismInfoAtPoint returns the Organism Info at the given point (nil if none)
func (m *OrganismManager) GetOrganismInfoAtPoint(point utils.Point) *organism.Info {
	if id, found := m.getOrganismIDAt(point); found {
		return m.organisms[id].Info()
	}
	return nil
}

// GetOrganismInfoByID returns the Organism Info for a given Organism ID. (nil if not found)
func (m *OrganismManager) GetOrganismInfoByID(id int) *organism.Info {
	if o, found := m.organisms[id]; found {
		return o.Info()
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
	o.ApplyHealthChange(c.HealthChangePerCycle() * o.Size)
}

func (m *OrganismManager) applyIdle(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromBeingIdle()*o.Size)
}

func (m *OrganismManager) applyHealthChange(o *organism.Organism, amount float64) {
	prevSize := o.Size
	o.ApplyHealthChange(amount)
	if o.Size > prevSize {
		m.api.AddUpdatedGridPoint(o.Location)
	}
}

func (m *OrganismManager) applyAttack(o *organism.Organism) {
	m.api.AddUpdatedGridPoint(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromAttacking()*o.Size)
	targetPoint := o.Location.Add(o.Direction)
	if m.isOrganismAtLocation(targetPoint) {
		targetOrganismIndex := m.organismIDGrid[targetPoint.X][targetPoint.Y]
		targetOrganism := m.organisms[targetOrganismIndex]
		m.applyHealthChange(targetOrganism, c.HealthChangeInflictedByAttack()*o.Size)
		m.removeIfDead(targetOrganism)
	}
}

func (m *OrganismManager) removeIfDead(o *organism.Organism) bool {
	if o.Health > 0.0 {
		return false
	}
	m.api.AddUpdatedGridPoint(o.Location)
	m.organismIDGrid[o.Location.X][o.Location.Y] = -1
	m.api.AddFoodAtPoint(o.Location, int(o.Size))
	delete(m.organisms, o.ID)
	return true
}

func (m *OrganismManager) applySpawn(o *organism.Organism) {
	o.ApplyHealthChange(o.HealthCostToReproduce())
	if success := m.SpawnChildOrganism(o); success {
		o.Children++
	}
}

func (m *OrganismManager) applyFeed(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromFeeding()*o.Size)
	amountToFeed := c.HealthChangeFromFeeding() * o.Size
	targetPoint := o.Location.Add(o.Direction)
	if m.isOrganismAtLocation(targetPoint) {
		targetOrganismIndex := m.organismIDGrid[targetPoint.X][targetPoint.Y]
		targetOrganism := m.organisms[targetOrganismIndex]
		m.applyHealthChange(targetOrganism, amountToFeed)
	} else {
		m.api.AddFoodAtPoint(targetPoint, int(amountToFeed))
	}
}

func (m *OrganismManager) applyEat(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromEatingAttempt()*o.Size)
	targetPoint := o.Location.Add(o.Direction)
	if item := m.api.GetFoodAtPoint(targetPoint); item != nil {
		maxCanEat := o.Size
		amountToEat := math.Min(float64(item.Value), maxCanEat)
		m.api.RemoveFoodAtPoint(targetPoint, int(amountToEat))
		m.applyHealthChange(o, amountToEat)
	}
}

func (m *OrganismManager) applyMove(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromMoving()*o.Size)

	targetPoint := o.Location.Add(o.Direction)
	if m.isGridLocationEmpty(targetPoint) {
		m.api.AddUpdatedGridPoint(o.Location)
		m.api.AddUpdatedGridPoint(targetPoint)

		m.organismIDGrid[o.Location.X][o.Location.Y] = -1
		m.organismIDGrid[targetPoint.X][targetPoint.Y] = o.ID
		o.Location = targetPoint
	}
}

func (m *OrganismManager) applyRightTurn(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromTurning()*o.Size)

	o.Direction = o.Direction.Right()
}

func (m *OrganismManager) applyLeftTurn(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromTurning()*o.Size)

	o.Direction = o.Direction.Left()
}

// GetAllOrganismInfo returns a map of all organisms' Info
func (m *OrganismManager) GetAllOrganismInfo() map[int]*organism.Info {
	infoMap := make(map[int]*organism.Info)
	for id, o := range m.organisms {
		infoMap[id] = o.Info()
	}
	return infoMap
}

// PrintBest prints the highest current score of any Organism (and their index)
func (m *OrganismManager) PrintBest() {
	m.printBestAncestors()
	fmt.Print("\n\n")
	m.printBestCurrent()
}

func (m *OrganismManager) printBestCurrent() {
	fmt.Printf("\n  - Best Organism Current - \n")
	m.printOrganismInfo(m.MostReproductiveCurrent)
}

func (m *OrganismManager) printOrganismInfo(o *organism.Organism) string {
	return fmt.Sprintf("\n      ID: %10d   |         InitialHealth: %4d"+
		"\n     Age: %10d   |      MinHealthToSpawn: %4d"+
		"\nChildren: %10d   |      MinCyclesToSpawn: %4d"+
		"\nAncestor: %10d   |  CyclesToEvaluateTree: %4d"+
		"\n  Health: %10.2f   |   ChanceToMutateTree:  %4.2f"+
		"\n    Size: %10.2f   |              MaxSize:  %4.2f"+
		"\n  Tree:\n%s",
		o.ID, int(o.InitialHealth()),
		o.Age, int(o.MinHealthToSpawn()),
		o.Children, o.MinCyclesBetweenSpawns(),
		o.OriginalAncestorID, o.CyclesToEvaluateDecisionTree(),
		o.Health, o.ChanceToMutateDecisionTree(),
		o.Size, o.MaxSize(),
		o.GetCurrentDecisionTreeCopy(true).Print())
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
