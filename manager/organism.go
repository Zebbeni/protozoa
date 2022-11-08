package manager

import (
	"fmt"
	"image/color"
	"math"
	"sync"
	"time"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	api            organism.API
	requestManager RequestManager

	organisms             map[int]*organism.Organism
	organismIDGrid        [][]int
	totalOrganismsCreated int

	organismIds []int

	originalAncestors      []int
	originalAncestorColors map[int]color.Color   // all original ancestor IDs with at least one descendant
	populationHistory      map[int]map[int]int16 // cycle : ancestorId : livingDescendantsCount

	UpdateDuration, ResolveDuration time.Duration

	ancestorMutex sync.RWMutex
	gridMutex     sync.RWMutex
	organismMutex sync.RWMutex
}

// NewOrganismManager creates all Organisms and updates grid
func NewOrganismManager(api organism.API) *OrganismManager {
	grid := initializeGrid()
	organisms := make(map[int]*organism.Organism)
	manager := &OrganismManager{
		api:                    api,
		requestManager:         RequestManager{},
		organismIDGrid:         grid,
		organisms:              organisms,
		organismIds:            make([]int, 0, c.MaxOrganisms()),
		originalAncestorColors: make(map[int]color.Color),
		populationHistory:      make(map[int]map[int]int16),
	}
	manager.InitializeOrganisms(c.InitialOrganisms())
	return manager
}

func (m *OrganismManager) InitializeOrganisms(count int) {
	for i := 0; i < count; i++ {
		m.SpawnRandomOrganism()
	}
}

// Update walks through decision tree of each organism and applies the
// chosen action to the organism, the grid, and the environment
func (m *OrganismManager) Update() {
	m.requestManager.ClearMaps()
	m.organismIds = make([]int, 0, c.MaxOrganisms())

	m.updateOrganismActions()
	m.resolveOrganismActions()

	m.updateHistory()
}

func (m *OrganismManager) updateOrganismActions() {
	start := time.Now()

	m.organismMutex.RLock()
	orgsToUpdate := make(chan int, len(m.organisms))
	m.organismMutex.RUnlock()

	numWorkers := 8
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			for k := range orgsToUpdate {
				m.organismMutex.RLock()
				o := m.organisms[k]
				m.organismMutex.RUnlock()

				m.updateOrganismAction(o)
			}
			wg.Done()
		}()
	}

	for k := range m.organisms {
		orgsToUpdate <- k
	}
	close(orgsToUpdate)

	// wait for all worker threads to call Done()
	wg.Wait()

	m.UpdateDuration = time.Since(start)
}

func (m *OrganismManager) resolveOrganismActions() {
	start := time.Now()

	orgsToResolve := make(chan *organism.Organism, len(m.organismIds))

	numWorkers := 8
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			for o := range orgsToResolve {
				m.resolveOrganismAction(o)
			}
			wg.Done()
		}()
	}

	for _, id := range m.organismIds {
		m.organismMutex.RLock()
		orgsToResolve <- m.organisms[id]
		m.organismMutex.RUnlock()
	}
	close(orgsToResolve)
	// wait for all worker threads to call Done()
	wg.Wait()

	m.ResolveDuration = time.Since(start)
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

func (m *OrganismManager) updateRequestMap(o *organism.Organism) {
	switch o.Action() {
	case d.ActEat:
		m.addFoodRequest(o)
	case d.ActMove:
		m.addMoveRequest(o)
	case d.ActSpawn:
		m.addSpawnRequest(o)
	case d.ActAttack:
		m.addAttackRequest(o)
	case d.ActFeed:
		m.addFeedRequest(o)
	default:
		return
	}
}

func (m *OrganismManager) addAttackRequest(o *organism.Organism) {
	effect := m.calculateAttackEffect(o)
	target := o.Location.Add(o.Direction)
	m.requestManager.AddHealthEffectRequest(target, effect)
}

func (m *OrganismManager) addFeedRequest(o *organism.Organism) {
	// the feed effect constant is negative so multiply by -1 to
	// get a positive health benefit for the beneficiary organism
	effect := -1 * m.calculateFeedEffect(o)
	target := o.Location.Add(o.Direction)
	m.requestManager.AddHealthEffectRequest(target, effect)
}

func (m *OrganismManager) addSpawnRequest(o *organism.Organism) {
	target, ok := m.getChildSpawnLocation(o)
	if ok {
		m.requestManager.AddPositionRequest(target, o.ID)
	}
}

// determine the new position required for a move action add 1 to the number of
// requests for this position in positionRequests
func (m *OrganismManager) addMoveRequest(o *organism.Organism) {
	target := o.Location.Add(o.Direction)

	// Only make request if empty, to avoid complications resolving it later
	if empty := m.isGridLocationEmpty(target); empty {
		m.requestManager.AddPositionRequest(target, o.ID)
	}
}

// calculate the amount of food the given organism requests to eat at a target
// location. Add this to the food item stored for this location representing
// the total eat requests made here.
func (m *OrganismManager) addFoodRequest(o *organism.Organism) {
	target := o.Location.Add(o.Direction)
	value := int(math.Ceil(m.calculateValueToEat(o, target)))
	m.requestManager.AddFoodRequest(target, value)
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

// GetAncestors returns a list of all original ancestor IDs
func (m *OrganismManager) GetAncestors() []int {
	return m.originalAncestors
}

func (m *OrganismManager) addUpdatedPoint(point utils.Point) {
	m.api.AddOrganismUpdate(point)
}

func (m *OrganismManager) applyOrganismPhGrowthEffect(o *organism.Organism) {
	m.api.AddPhChangeAtPoint(o.Location, o.Traits().PhGrowthEffect*o.Size)
}

func (m *OrganismManager) updateOrganismAction(o *organism.Organism) {
	// if previous action was attack, allow the screen to render white
	if o.Action() == d.ActAttack {
		m.addUpdatedPoint(o.Location)
	}
	o.UpdateStats()
	o.UpdateAction()
	m.updateRequestMap(o)
	m.addToOrganismIds(o)
}

func (m *OrganismManager) addToOrganismIds(o *organism.Organism) {
	m.organismMutex.Lock()
	m.organismIds = append(m.organismIds, o.ID)
	m.organismMutex.Unlock()
}

func (m *OrganismManager) resolveOrganismAction(o *organism.Organism) {
	if o == nil || o.Age == 0 {
		return
	}
	m.applyCycleHealthChanges(o)
	m.applyAction(o)
	m.removeIfDead(o)
}

// SpawnRandomOrganism creates an Organism with random position.
//
// Checks random positions on the grid until it finds an empty one. Calls
// NewOrganism to initialize decision tree, other random attributes.
func (m *OrganismManager) SpawnRandomOrganism() {
	if spawnPoint, found := m.getRandomSpawnLocation(); found {
		id := m.generateId()
		o := organism.NewRandom(id, spawnPoint, m.api)
		m.registerNewOrganism(o, id)
	}
}

// SpawnChildOrganism creates a new organism near an existing 'parent' organism
// with a copy of its parent's node library. (No organism created if no room or
// if more than 1 organism made a request to spawn and/or move into the desired
// location.
// Returns true / false depending on whether a child was actually spawned.
func (m *OrganismManager) SpawnChildOrganism(parent *organism.Organism) bool {
	spawnPoint, found := m.getChildSpawnLocation(parent)
	if found == false {
		return false
	}
	if m.isMatchingPositionRequest(spawnPoint, parent.ID) == false {
		return false
	}
	id := m.generateId()
	o := parent.NewChild(id, spawnPoint, m.api)
	m.registerNewOrganism(o, id)
	m.addToOriginalAncestors(parent)
	return true
}

// return true iff the request id stored in the position requests matches
// the id of the organism checking
func (m *OrganismManager) isMatchingPositionRequest(p utils.Point, id int) bool {
	requestId := m.requestManager.GetPositionRequest(p)
	return id == requestId
}

func (m *OrganismManager) generateId() int {
	m.organismMutex.Lock()
	defer m.organismMutex.Unlock()

	m.totalOrganismsCreated++
	return m.totalOrganismsCreated
}

func (m *OrganismManager) registerNewOrganism(o *organism.Organism, index int) {
	m.addUpdatedPoint(o.Location)

	m.gridMutex.Lock()
	m.organismMutex.Lock()

	m.organisms[index] = o
	m.organismIDGrid[o.X()][o.Y()] = index

	m.gridMutex.Unlock()
	m.organismMutex.Unlock()
}

func (m *OrganismManager) addToOriginalAncestors(o *organism.Organism) {
	m.ancestorMutex.RLock()
	_, ok := m.originalAncestorColors[o.ID]
	m.ancestorMutex.RUnlock()
	if ok {
		return
	}

	m.ancestorMutex.Lock()
	m.originalAncestorColors[o.ID] = o.Color()
	m.originalAncestors = append(m.originalAncestors, o.ID)
	m.ancestorMutex.Unlock()
}

// returns a random point and whether it is empty
func (m *OrganismManager) getRandomSpawnLocation() (utils.Point, bool) {
	point := utils.GetRandomPoint(c.GridUnitsWide(), c.GridUnitsHigh())
	isEmpty := m.isGridLocationEmpty(point)
	return point, isEmpty
}

func (m *OrganismManager) getChildSpawnLocation(parent *organism.Organism) (utils.Point, bool) {
	var point utils.Point
	direction := parent.Direction
	for i := 0; i < 4; i++ {
		direction = direction.Left()
		point = parent.Location.Add(direction)

		empty := m.isGridLocationEmpty(point)
		if empty {
			return point, true
		}
	}

	return point, false
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

func (m *OrganismManager) isGridLocationEmpty(point utils.Point) bool {
	return !point.IsWall() && !m.isFoodAtLocation(point) && !m.isOrganismAtLocation(point)
}

func (m *OrganismManager) isFoodAtLocation(point utils.Point) bool {
	return m.api.CheckFoodAtPoint(point, func(_ *food.Item, exists bool) bool {
		return exists
	})
}

func (m *OrganismManager) isOrganismAtLocation(point utils.Point) bool {
	m.gridMutex.RLock()
	id := m.organismIDGrid[point.X][point.Y]
	m.gridMutex.RUnlock()

	return id != -1
}

func (m *OrganismManager) getOrganismAt(point utils.Point) *organism.Organism {
	if id, exists := m.getOrganismIDAt(point); exists {
		index := id

		m.organismMutex.RLock()
		defer m.organismMutex.RUnlock()

		return m.organisms[index]
	}
	return nil
}

func (m *OrganismManager) getOrganismIDAt(point utils.Point) (int, bool) {
	m.gridMutex.RLock()
	id := m.organismIDGrid[point.X][point.Y]
	m.gridMutex.RUnlock()

	return id, id != -1
}

// CheckOrganismAtPoint returns the result of running a check against any Organism
// found at a given Point.
func (m *OrganismManager) CheckOrganismAtPoint(point utils.Point, checkFunc organism.OrgCheck) bool {
	return checkFunc(m.getOrganismAt(point))
}

// GetOrganismInfoAtPoint returns the Organism Info at the given point (nil if none)
func (m *OrganismManager) GetOrganismInfoAtPoint(point utils.Point) *organism.Info {
	if id, found := m.getOrganismIDAt(point); found {
		m.organismMutex.RLock()
		defer m.organismMutex.RUnlock()

		if o, ok := m.organisms[id]; ok {
			return o.Info()
		}
	}
	return nil
}

// GetOrganismDecisionTreeByID returns a copy of the currently-used decision tree of the
// given organism (nil if no organism found)
func (m *OrganismManager) GetOrganismDecisionTreeByID(id int) *d.Tree {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()

	if o, ok := m.organisms[id]; ok {
		return o.GetDecisionTreeCopy()
	}
	return nil
}

// GetOrganismInfoByID returns the Organism Info for a given Organism ID. (nil if not found)
func (m *OrganismManager) GetOrganismInfoByID(id int) *organism.Info {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()

	if o, found := m.organisms[id]; found {
		return o.Info()
	}
	return nil
}

// GetOrganismTraitsByID returns the Organism Traits for a given Organism ID and whether it was
// successfully found
func (m *OrganismManager) GetOrganismTraitsByID(id int) (organism.Traits, bool) {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()

	if o, found := m.organisms[id]; found {
		return o.Traits(), true
	}
	return organism.Traits{}, false
}

// OrganismCount returns the current number of organisms alive in the simulation
func (m *OrganismManager) OrganismCount() int {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()

	return len(m.organisms)
}

// DeadCount returns the total number of organisms that have died in the simulation
func (m *OrganismManager) DeadCount() int {
	return m.totalOrganismsCreated - len(m.organisms)
}

func (m *OrganismManager) applyAction(o *organism.Organism) {
	switch o.Action() {
	case d.ActChemosynthesis:
		m.applyChemosynthesis(o)
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

func (m *OrganismManager) applyCycleHealthChanges(o *organism.Organism) {
	decisionsEffect := c.HealthChangePerDecisionTreeNode() * float64(o.GetCurrentDecisionTreeLength())
	phEffect := 0.0
	// Subtract health if organism is too far away from its ideal ph
	phDist := math.Abs(o.Traits().IdealPh - m.api.GetPhAtPoint(o.Location))
	if phDist > o.Traits().PhTolerance {
		phEffect = (phDist - o.Traits().PhTolerance) * c.HealthChangePerUnhealthyPh()
	}
	// Add effects due to feeding and/or attack (not related to organism size)
	healthEffects := m.requestManager.GetHealthEffects(o.Location)
	m.applyHealthChange(o, o.Size*(decisionsEffect+phEffect)+healthEffects)
}

// add a positive health change if organism attempts chemosynthesis in a
// favorable ph environment
func (m *OrganismManager) applyChemosynthesis(o *organism.Organism) {
	ph := m.api.GetPhAtPoint(o.Location)
	ideal := o.Traits().IdealPh
	tolerance := o.Traits().PhTolerance
	if math.Abs(ideal-ph) < tolerance {
		m.applyHealthChange(o, c.HealthChangeFromChemosynthesis()*o.Size)
	}
}

func (m *OrganismManager) applyHealthChange(o *organism.Organism, amount float64) {
	prevSize := o.Size
	o.ApplyHealthChange(amount)
	if o.Size > prevSize {
		m.addUpdatedPoint(o.Location)
		// Organism growth affects ph
		m.applyOrganismPhGrowthEffect(o)
	}
}

func (m *OrganismManager) applyAttack(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromAttacking()*o.Size)
}

func (m *OrganismManager) calculateAttackEffect(o *organism.Organism) float64 {
	return c.HealthChangeInflictedByAttack() * o.Size
}

func (m *OrganismManager) removeIfDead(o *organism.Organism) bool {
	if o.Health > 0.0 {
		return false
	}

	m.gridMutex.Lock()
	m.organismMutex.Lock()

	m.organismIDGrid[o.Location.X][o.Location.Y] = -1
	delete(m.organisms, o.ID)

	m.gridMutex.Unlock()
	m.organismMutex.Unlock()

	m.api.AddFoodAtPoint(o.Location, int(o.Size))
	m.addUpdatedPoint(o.Location)

	return true
}

func (m *OrganismManager) applySpawn(o *organism.Organism) {
	if success := m.SpawnChildOrganism(o); success {
		m.applyHealthChange(o, o.HealthCostToReproduce())
		o.Children++
	}
}

func (m *OrganismManager) applyFeed(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromFeeding()*o.Size)
}

func (m *OrganismManager) calculateFeedEffect(o *organism.Organism) float64 {
	return c.HealthChangeFromFeeding() * o.Size
}

func (m *OrganismManager) applyEat(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromEatingAttempt()*o.Size)
	target := o.Location.Add(o.Direction)

	// apply health change but don't delete food until all eat requests have been
	// processed. This may result in more total food being consumed by nearby organisms
	// than exists at a given point, but this seems preferable right now to denying
	// the eat request altogether or coming up with some perfect way to divvy it up.
	amountToEat := m.calculateValueToEat(o, target)
	m.api.RemoveFoodAtPoint(target, int(math.Ceil(amountToEat)))
	m.applyHealthChange(o, amountToEat)
}

func (m *OrganismManager) calculateValueToEat(o *organism.Organism, target utils.Point) float64 {
	if item, found := m.api.GetFoodAtPoint(target); found {
		maxCanEat := o.Size
		return math.Min(float64(item.Value), maxCanEat)
	}
	return 0
}

func (m *OrganismManager) applyMove(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromMoving()*o.Size)

	targetPoint := o.Location.Add(o.Direction)
	if m.isMatchingPositionRequest(targetPoint, o.ID) == false {
		return
	}

	m.addUpdatedPoint(o.Location)
	m.addUpdatedPoint(targetPoint)

	m.gridMutex.Lock()
	m.organismIDGrid[o.Location.X][o.Location.Y] = -1
	m.organismIDGrid[targetPoint.X][targetPoint.Y] = o.ID
	m.gridMutex.Unlock()

	o.Location = targetPoint
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
	m.organismMutex.RLock()
	for id, o := range m.organisms {
		info := o.Info()
		infoMap[id] = info
	}
	m.organismMutex.RUnlock()
	return infoMap
}

func (m *OrganismManager) printOrganismInfo(o *organism.Organism) string {
	return fmt.Sprintf("\n      ID: %10d   |         InitialHealth: %4d"+
		"\n     Age: %10d   |      MinHealthToSpawn: %4d"+
		"\nChildren: %10d   |      MinCyclesToSpawn: %4d"+
		"\nAncestor: %10d   |  "+
		"\n  Health: %10.2f   |   ChanceToMutateTree:  %4.2f"+
		"\n    CalcAndUpdateSize: %10.2f   |              MaxSize:  %4.2f"+
		"\n  Tree:\n%s",
		o.ID, int(o.InitialHealth()),
		o.Age, int(o.MinHealthToSpawn()),
		o.Children, o.MinCyclesBetweenSpawns(),
		o.OriginalAncestorID,
		o.Health, o.ChanceToMutateDecisionTree(),
		o.Size, o.MaxSize(),
		o.GetDecisionTreeCopy().Print())
}
