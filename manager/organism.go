package manager

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"sync"
	"time"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

type HistoryType int

const (
	HistoryPopulation     HistoryType = iota // cycle : ancestorId : livingDescendantsCount
	HistoryPhDistribution                    // cycle : phBucket : gridCellCount
	HistoryFood                              // cycle : 0 : totalFoodItemCount
	HistoryWalls                             // cycle : 0 : totalWallCellCount
	// HistoryBuriedFood counts CELLS holding buried food, not units, to match HistoryFood.
	HistoryBuriedFood // cycle : 0 : totalBuriedFoodCellCount
)

// AllHistoryTypes is every history series, and the canonical list the maps are built from.
var AllHistoryTypes = []HistoryType{
	HistoryPopulation,
	HistoryPhDistribution,
	HistoryFood,
	HistoryWalls,
	HistoryBuriedFood,
}

func newHistoryMaps() map[HistoryType]map[int]map[int]int32 {
	h := make(map[HistoryType]map[int]map[int]int32, len(AllHistoryTypes))
	for _, t := range AllHistoryTypes {
		h[t] = make(map[int]map[int]int32)
	}
	return h
}

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	// chemoClaims counts how many chemosynthesising organisms draw on each cell this cycle, row-major. Nil when crowding is off.
	chemoClaims    []int32
	api            organism.API
	rng            *simrand.RNG
	requestManager RequestManager

	// designs are the saved organism designs this simulation founds from (config initial_designs), loaded once and dealt round-robin.
	designs       []organism.Design
	designsLoaded bool
	designsDealt  int

	organisms             map[int]*organism.Organism
	organismIDGrid        [][]int
	totalOrganismsCreated int

	organismIds []int

	oldestId           int
	oldestAge          int
	mostChildrenId     int
	mostChildren       int
	mostTraveledId     int
	mostTraveledDist   int
	mostAggressiveId   int
	mostAggressiveHits int

	originalAncestors      []int
	originalAncestorColors map[int]color.Color              // all original ancestor IDs with at least one descendant
	descendantTrees        map[int]*organism.DescendantNode // ancestorId : root node
	// descendantNodeIndex maps every node ID (alive or dead) to its DescendantNode for O(1) lookup.
	descendantNodeIndex map[int]*organism.DescendantNode
	// mostSuccessful is the precomputed set of IDs whose descendant tree node has AllBranchesDeadCycle == 0 (lineage survives to end) or equal to the maximum AllBranchesDeadCycle across the loaded tree.
	mostSuccessful map[int]struct{}
	// recordedEndCycle is the end of the recorded run, set alongside each node's LineageEndCycle when trees are restored.
	recordedEndCycle int
	// treesGeneration identifies the decoded trees installed here; see DescendantTrees.
	treesGeneration int
	history         map[HistoryType]map[int]map[int]int32 // type : cycle : key : count

	UpdateDuration, ResolveDuration, SortDuration, HistoryDuration time.Duration

	DecideStatsDuration, DecideTreeDuration, DecideRequestDuration    time.Duration
	ResolveHealthDuration, ResolveActionDuration, ResolveDeadDuration time.Duration
	ResolveSpawnDuration                                              time.Duration
	SpawnCount                                                        int

	ancestorMutex sync.RWMutex
	historyMutex  sync.RWMutex
	gridMutex     sync.RWMutex
	organismMutex sync.RWMutex
}

func NewOrganismManager(api organism.API, rng *simrand.RNG) *OrganismManager {
	grid := initializeGrid()
	organisms := make(map[int]*organism.Organism)
	manager := &OrganismManager{
		api:                    api,
		rng:                    rng,
		requestManager:         RequestManager{},
		organismIDGrid:         grid,
		organisms:              organisms,
		organismIds:            make([]int, 0, c.MaxOrganisms()),
		originalAncestorColors: make(map[int]color.Color),
		descendantTrees:        make(map[int]*organism.DescendantNode),
		descendantNodeIndex:    make(map[int]*organism.DescendantNode),
		history:                newHistoryMaps(),
	}
	manager.InitializeOrganisms(c.InitialOrganisms())
	return manager
}

func (m *OrganismManager) InitializeOrganisms(count int) {
	for i := 0; i < count; i++ {
		m.SpawnRandomOrganism()
	}
}

// Update walks through decision tree of each organism and applies the chosen action to the organism, the grid.
func (m *OrganismManager) Update() {
	m.requestManager.ClearMaps()
	m.organismIds = make([]int, 0, c.MaxOrganisms())

	m.resetInterestingStats()

	m.updateOrganismActions()
	m.resolveOrganismActions()

	m.updateHistory()
}

// updateOrganismActions runs each organism's per-cycle stats update, decision-tree walk, and request submission.
func (m *OrganismManager) updateOrganismActions() {
	start := time.Now()

	// Drain any organisms that finished their dying cycle in the previous tick.
	m.finalizeDeaths()

	// Collect IDs in deterministic sorted order
	ids := make([]int, 0, len(m.organisms))
	for k := range m.organisms {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	m.SortDuration = time.Since(start)

	m.requestManager.ClearMaps()
	for _, id := range ids {
		o := m.organisms[id]
		// Dying organisms are frozen — no decisions, no requests, no stats updates.
		if o.Status == organism.StatusDying {
			m.organismIds = append(m.organismIds, o.ID)
			continue
		}
		if o.Action() == d.ActAttack {
			m.addUpdatedPoint(o.Location)
		}
		// Cleared here, before ANY organism resolves: thorns and predation
		// credit the attacker's ledger while the defender resolves, so a
		// reset inside the resolve loop wiped whichever of the two came
		// later by id. Dying organisms are skipped above and keep the
		// ledger of the cycle that killed them.
		o.ResetHealthLedger()
		o.UpdateStats()
		o.UpdateAction()
		// Promote to ActSpawn only if the organism is eligible AND the grid actually has room for a child.
		if o.ShouldSpawn() {
			if _, _, ok := m.getChildSpawnLocation(o); ok {
				o.PromoteToSpawn()
			}
		}
		m.updateRequestMapTo(o, &m.requestManager)
		m.organismIds = append(m.organismIds, o.ID)
	}
	m.DecideStatsDuration = time.Since(start) - m.SortDuration
	m.DecideTreeDuration = 0
	m.DecideRequestDuration = 0
	m.UpdateDuration = time.Since(start)
}

func (m *OrganismManager) resetInterestingStats() {
	m.oldestId = -1
	m.oldestAge = -1
	m.mostChildrenId = -1
	m.mostChildren = -1
	m.mostTraveledId = -1
	m.mostTraveledDist = -1
	m.mostAggressiveId = -1
	m.mostAggressiveHits = -1
}

func (m *OrganismManager) updateInterestingStats(o *organism.Organism) {
	if o.Age > m.oldestAge || (o.Age == m.oldestAge && o.ID < m.oldestId) {
		m.oldestAge = o.Age
		m.oldestId = o.ID
	}
	if o.Children > m.mostChildren || (o.Children == m.mostChildren && o.ID < m.mostChildrenId) {
		m.mostChildren = o.Children
		m.mostChildrenId = o.ID
	}
	if o.TraveledDist > m.mostTraveledDist || o.TraveledDist == m.mostTraveledDist && o.ID < m.mostTraveledId {
		m.mostTraveledDist = o.TraveledDist
		m.mostTraveledId = o.ID
	}
	if o.AttackHits > m.mostAggressiveHits || (o.AttackHits == m.mostAggressiveHits && o.ID < m.mostAggressiveId) {
		m.mostAggressiveHits = o.AttackHits
		m.mostAggressiveId = o.ID
	}
}

func (m *OrganismManager) resolveOrganismActions() {
	start := time.Now()

	m.buildChemoClaims()

	var accHealth, accAction, accDead, accSpawn time.Duration
	spawnCount := 0

	for _, id := range m.organismIds {
		o, ok := m.organisms[id]
		if !ok || o == nil || o.Age == 0 {
			continue
		}

		t0 := time.Now()
		// Dying organisms skip the resolve loop entirely.
		if o.Status == organism.StatusDying {
			continue
		}
		m.applyCycleHealthChanges(o)
		t1 := time.Now()
		// If health-change just killed the organism, skip its action this cycle.
		if m.markDyingIfDead(o) {
			t2 := time.Now()
			accHealth += t1.Sub(t0)
			accDead += t2.Sub(t1)
			continue
		}
		switch {
		case o.Action() == d.ActSpawn:
			m.applySpawn(o)
			t2 := time.Now()
			accSpawn += t2.Sub(t1)
			spawnCount++
			t1 = t2
		default:
			m.applyAction(o)
		}
		t2 := time.Now()
		// Post-action death check: an action like Attack with extreme cost could drop the organism below the threshold.
		m.markDyingIfDead(o)
		m.updateInterestingStats(o)
		t3 := time.Now()

		accHealth += t1.Sub(t0)
		accAction += t2.Sub(t1)
		accDead += t3.Sub(t2)
	}

	m.applyKillClaims()

	m.ResolveHealthDuration = accHealth
	m.ResolveActionDuration = accAction
	m.ResolveSpawnDuration = accSpawn
	m.ResolveDeadDuration = accDead
	m.SpawnCount = spawnCount
	m.ResolveDuration = time.Since(start)
}

// updateHistory updates the population map, average phEffect per ancestor.
func (m *OrganismManager) updateHistory() {
	cycle := m.api.Cycle()
	if cycle%c.PopulationUpdateInterval() != 0 {
		m.HistoryDuration = 0
		return
	}
	start := time.Now()

	populationMap := make(map[int]int32)
	for _, o := range m.organisms {
		populationMap[o.OriginalAncestorID]++
	}

	m.historyMutex.Lock()
	m.history[HistoryPopulation][cycle] = populationMap
	m.historyMutex.Unlock()

	// compute pH distribution and average across grid cells using 0.5 pH-wide buckets
	phMap := m.api.GetPhMap()
	phDist := make(map[int]int32)
	phBucketWidth := 0.5
	numPhBuckets := int(c.MaxPh() / phBucketWidth)
	for x := range phMap {
		for _, ph := range phMap[x] {
			bucket := int(ph / phBucketWidth)
			if bucket < 0 {
				bucket = 0
			} else if bucket >= numPhBuckets {
				bucket = numPhBuckets - 1
			}
			phDist[bucket]++
		}
	}
	m.historyMutex.Lock()
	m.history[HistoryPhDistribution][cycle] = phDist
	m.historyMutex.Unlock()

	// Record the total food-item and wall counts, each under a single fixed key (0).
	m.historyMutex.Lock()
	m.history[HistoryFood][cycle] = map[int]int32{0: int32(m.api.FoodCount())}
	m.history[HistoryWalls][cycle] = map[int]int32{0: int32(m.api.WallCount())}
	m.history[HistoryBuriedFood][cycle] = map[int]int32{0: int32(m.api.BuriedFoodCount())}
	m.historyMutex.Unlock()

	m.HistoryDuration = time.Since(start)
}

func (m *OrganismManager) updateRequestMap(o *organism.Organism) {
	m.updateRequestMapTo(o, &m.requestManager)
}

func (m *OrganismManager) updateRequestMapTo(o *organism.Organism, rm *RequestManager) {
	switch o.Action() {
	case d.ActMove:
		target := o.Location.Add(o.Direction)
		if m.canOccupy(o, target) {
			rm.AddPositionRequest(target, o.ID, o.Size)
		}
	case d.ActSpawn:
		if target, _, ok := m.getChildSpawnLocation(o); ok {
			rm.AddPositionRequest(target, o.ID, o.Size)
		}
	case d.ActAttack:
		effect := m.calculateAttackEffect(o)
		target := o.Location.Add(o.Direction)
		rm.AddAttackRequest(target, effect, o.ID)
		// An attack into open water carries the attacker forward, so it has
		// to claim the cell the way a move does or two of them lunge into
		// the same one.
		if m.isOpenForLunge(target) {
			rm.AddPositionRequest(target, o.ID, o.Size)
		}
		// Track attack stats: every attack counts toward AttackTotal.
		o.AttackTotal++
		if m.isOrganismAtLocation(target) {
			o.AttackHits++
		}
	}
}

func (m *OrganismManager) GetHistory(histType HistoryType) map[int]map[int]int32 {
	return m.history[histType]
}

func (m *OrganismManager) GetAncestorColors() map[int]color.Color {
	return m.originalAncestorColors
}

// LockHistoryForReading acquires a read lock on the history maps.
func (m *OrganismManager) LockHistoryForReading() {
	m.historyMutex.RLock()
}

// UnlockHistoryForReading releases the read lock on the history maps.
func (m *OrganismManager) UnlockHistoryForReading() {
	m.historyMutex.RUnlock()
}

// GetDescendantTrees returns the root node of each ancestor's family tree
func (m *OrganismManager) GetDescendantTrees() map[int]*organism.DescendantNode {
	return m.descendantTrees
}

func (m *OrganismManager) GetAncestors() []int {
	return m.originalAncestors
}

func (m *OrganismManager) addUpdatedPoint(point utils.Point) {
	m.api.AddOrganismUpdate(point)
}

func (m *OrganismManager) addToOrganismIds(o *organism.Organism) {
	m.organismIds = append(m.organismIds, o.ID)
}

func (m *OrganismManager) SpawnRandomOrganism() {
	if spawnPoint, found := m.getRandomSpawnLocation(); found {
		id := m.generateId()
		o := m.newFoundingOrganism(id, spawnPoint)

		// In replay/resume modes the descendant tree is pre-loaded from the recorded simulation.
		node, exists := m.descendantTrees[id]
		if !exists {
			node = &organism.DescendantNode{
				ID:         id,
				Color:      o.Color(),
				Abilities:  o.Traits().Abilities,
				StartCycle: m.api.Cycle(),
			}
			m.descendantTrees[id] = node
			m.descendantNodeIndex[id] = node
		}
		o.TreeNode = node

		m.registerNewOrganism(o, id)
		m.addToOriginalAncestors(o)
	}
}

// SpawnChildOrganism creates a new organism near an existing 'parent' organism with a copy of its parent's node library.
func (m *OrganismManager) SpawnChildOrganism(parent *organism.Organism) bool {
	spawnPoint, spawnDirection, found := m.getChildSpawnLocation(parent)
	if found == false {
		return false
	}
	if m.isMatchingPositionRequest(spawnPoint, parent.ID) == false {
		return false
	}
	id := m.generateId()
	// Face the child away from its parent (parent→child step vector).
	o := parent.NewChild(m.rng, id, spawnPoint, spawnDirection, m.api)

	// In replay/resume modes the parent's existing tree (loaded from the recorded simulation) already has a child with this ID.
	var node *organism.DescendantNode
	if parent.TreeNode != nil {
		parent.TreeNode.ForEachChild(func(child *organism.DescendantNode) {
			if child.ID == id {
				node = child
			}
		})
	}
	if node == nil {
		node = &organism.DescendantNode{
			ID:         id,
			Color:      o.Color(),
			Abilities:  o.Traits().Abilities,
			StartCycle: m.api.Cycle(),
		}
		if parent.TreeNode != nil {
			parent.TreeNode.AddChild(node)
		}
	}
	m.descendantNodeIndex[id] = node
	o.TreeNode = node

	m.registerNewOrganism(o, id)
	return true
}

// return true iff the request id stored in the position requests matches the id of the organism checking
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

// getRandomSpawnLocation returns a random empty grid point with at least one empty cardinal neighbour.
func (m *OrganismManager) getRandomSpawnLocation() (utils.Point, bool) {
	const maxSpawnAttempts = 1000
	for i := 0; i < maxSpawnAttempts; i++ {
		point := utils.GetRandomPoint(m.rng, c.GridUnitsWide(), c.GridUnitsHigh())
		if m.canPlaceOrganismAt(point) && m.hasEmptyNeighbor(point) {
			return point, true
		}
	}
	return utils.Point{}, false
}

// hasEmptyNeighbor reports whether at least one of point's four cardinal neighbours is empty (no wall, food, or organism).
func (m *OrganismManager) hasEmptyNeighbor(point utils.Point) bool {
	direction := utils.Point{X: 0, Y: -1}
	for i := 0; i < 4; i++ {
		if m.canPlaceOrganismAt(point.Add(direction)) {
			return true
		}
		direction = direction.Left()
	}
	return false
}

// getChildSpawnLocation returns an empty cell adjacent to the parent and the unit cardinal step from parent→cell.
func (m *OrganismManager) getChildSpawnLocation(parent *organism.Organism) (utils.Point, utils.Point, bool) {
	var point utils.Point
	direction := parent.Direction
	for i := 0; i < 4; i++ {
		direction = direction.Left()
		point = parent.Location.Add(direction)

		empty := m.canPlaceOrganismAt(point)
		if empty {
			return point, direction, true
		}
	}

	return point, utils.Point{}, false
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

func (m *OrganismManager) canPlaceOrganismAt(point utils.Point) bool {
	return !m.api.IsWallAtPoint(point) && !m.isOrganismAtLocation(point)
}

func (m *OrganismManager) isGridLocationEmpty(point utils.Point) bool {
	return !m.api.IsWallAtPoint(point) && !m.isFoodAtLocation(point) && !m.isOrganismAtLocation(point)
}

// canOccupy reports whether o could end this cycle standing on point.
func (m *OrganismManager) canOccupy(o *organism.Organism, point utils.Point) bool {
	if m.isOrganismAtLocation(point) {
		return false
	}
	strength := m.api.GetWallStrengthAtPoint(point)
	if strength <= 0 {
		return true
	}
	return m.canBurrow(o, strength)
}

// isOpenForLunge reports whether an attack into this cell carries the
// attacker into it: nothing alive there and no wall at all.
//
// Deliberately NOT canOccupy, which lets a strong digger through a wall.
// Burrowing belongs to ActMove; an attack does not open terrain.
func (m *OrganismManager) isOpenForLunge(point utils.Point) bool {
	return !m.isOrganismAtLocation(point) && m.api.GetWallStrengthAtPoint(point) <= 0
}

// canBurrow is the manager-side wall-break check.
func (m *OrganismManager) canBurrow(o *organism.Organism, wallStrength int) bool {
	return effects.CanBreakWall(c.GetCurrentGlobals(),
		o.Abilities()[physiology.AbilityDigging], o.Size, wallStrength)
}

func (m *OrganismManager) isFoodAtLocation(point utils.Point) bool {
	return m.api.CheckFoodAtPoint(point, func(_ *food.Item, exists bool) bool {
		return exists
	})
}

func (m *OrganismManager) IsOrganismAtPoint(point utils.Point) bool {
	return m.isOrganismAtLocation(point)
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

// CheckOrganismAtPoint returns the result of running a check against any Organism found at a given Point.
func (m *OrganismManager) CheckOrganismAtPoint(point utils.Point, checkFunc organism.OrgCheck) bool {
	return checkFunc(m.getOrganismAt(point))
}

// GetOrganismTreeNode returns the tree node for the given organism ID, or nil.
func (m *OrganismManager) GetOrganismTreeNode(id int) *organism.DescendantNode {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()
	if o, ok := m.organisms[id]; ok {
		return o.TreeNode
	}
	return nil
}

// GetTreeNodeByID returns the DescendantNode for the given ID — alive or dead — in O(1) via descendantNodeIndex.
func (m *OrganismManager) GetTreeNodeByID(id int) *organism.DescendantNode {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()
	if o, ok := m.organisms[id]; ok {
		return o.TreeNode
	}
	return m.descendantNodeIndex[id]
}

// GetOrganismInfoAtPoint returns the Organism Info at the given point (nil if none).
func (m *OrganismManager) GetOrganismInfoAtPoint(point utils.Point) *organism.Info {
	if id, found := m.getOrganismIDAt(point); found {
		m.organismMutex.RLock()
		if o, ok := m.organisms[id]; ok {
			info := o.Info()
			m.organismMutex.RUnlock()
			return info
		}
		m.organismMutex.RUnlock()
	}
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()
	for _, o := range m.organisms {
		if o.Location == point {
			return o.Info()
		}
	}
	return nil
}

// GetOrganismDecisionTreeByID returns a copy of the currently-used decision tree of the given organism (nil if no organism found)
func (m *OrganismManager) GetOrganismDecisionTreeByID(id int) *d.Tree {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()

	if o, ok := m.organisms[id]; ok {
		return o.GetDecisionTreeCopy()
	}
	return nil
}

func (m *OrganismManager) GetOrganismInfoByID(id int) *organism.Info {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()

	if o, found := m.organisms[id]; found {
		return o.Info()
	}
	return nil
}

func (m *OrganismManager) GetOrganismTraitsByID(id int) (organism.Traits, bool) {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()

	if o, found := m.organisms[id]; found {
		return o.Traits(), true
	}
	return organism.Traits{}, false
}

func (m *OrganismManager) GetOldestId() int {
	return m.oldestId
}

func (m *OrganismManager) GetMostChildrenId() int {
	return m.mostChildrenId
}

func (m *OrganismManager) GetMostTraveledId() int {
	return m.mostTraveledId
}

// GetMostAggressiveId returns the id of the organism with the most attack hits (attacks that landed on a target organism).
func (m *OrganismManager) GetMostAggressiveId() int {
	if m.mostAggressiveHits <= 0 {
		return -1
	}
	return m.mostAggressiveId
}

func (m *OrganismManager) mostSuccessfulSet() map[int]struct{} {
	if m.mostSuccessful != nil {
		return m.mostSuccessful
	}
	if len(m.descendantTrees) == 0 {
		return nil
	}
	set := make(map[int]struct{})
	var walk func(*organism.DescendantNode) bool
	walk = func(n *organism.DescendantNode) bool {
		hasSurvivor := n.EndCycle == 0
		n.ForEachChild(func(c *organism.DescendantNode) {
			if walk(c) {
				hasSurvivor = true
			}
		})
		if hasSurvivor {
			set[n.ID] = struct{}{}
		}
		return hasSurvivor
	}
	for _, root := range m.descendantTrees {
		if root != nil {
			walk(root)
		}
	}
	return set
}

func (m *OrganismManager) GetMostSuccessfulIds() []int {
	set := m.mostSuccessfulSet()
	if set == nil {
		return nil
	}
	m.organismMutex.RLock()
	living := make([]int, 0, len(set))
	for id := range m.organisms {
		if _, ok := set[id]; ok {
			living = append(living, id)
		}
	}
	m.organismMutex.RUnlock()
	sort.Ints(living)
	return living
}

func (m *OrganismManager) Organisms() []*organism.Organism {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()
	out := make([]*organism.Organism, 0, len(m.organisms))
	for _, o := range m.organisms {
		out = append(out, o)
	}
	return out
}

// IsMostSuccessful reports whether the given organism ID is on the surviving (or, in extinct recordings, longest-lived) lineage.
func (m *OrganismManager) IsMostSuccessful(id int) bool {
	set := m.mostSuccessfulSet()
	if set == nil {
		return false
	}
	_, ok := set[id]
	return ok
}

// GetMostSuccessfulId returns the ID of the oldest currently-living organism on the most-successful lineage, or -1 if none qualify.
func (m *OrganismManager) GetMostSuccessfulId() int {
	set := m.mostSuccessfulSet()
	if set == nil {
		return -1
	}
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()
	oldestID := -1
	oldestAge := -1
	for id, o := range m.organisms {
		if _, ok := set[id]; !ok {
			continue
		}
		if o.Age > oldestAge || (o.Age == oldestAge && o.ID < oldestID) {
			oldestAge = o.Age
			oldestID = o.ID
		}
	}
	return oldestID
}

// OrganismCount returns the current number of organisms alive in the simulation AverageAbilityScores is the mean of every living organism's scores, ability by ability.
func (m *OrganismManager) AverageAbilityScores() [physiology.AbilityCount]float64 {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()

	var totals [physiology.AbilityCount]float64
	if len(m.organisms) == 0 {
		return totals
	}
	for _, o := range m.organisms {
		scores := o.Abilities()
		for _, a := range physiology.AllAbilities {
			totals[a] += float64(scores[a])
		}
	}
	for i := range totals {
		totals[i] /= float64(len(m.organisms))
	}
	return totals
}

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
	// No junk-DNA fallback: under ability scores every organism can resolve every action.
	action := o.Action()
	switch action {
	case d.ActChemosynthesis:
		m.applyChemosynthesis(o)
	case d.ActAttack:
		m.applyAttack(o)
	case d.ActEat:
		m.applyEat(o)
	case d.ActMove:
		m.applyMove(o)
	case d.ActTurnLeft:
		m.applyLeftTurn(o)
	case d.ActTurnRight:
		m.applyRightTurn(o)
	case d.ActSpawn:
		m.applySpawn(o)
	case d.ActIdle:
		m.applyIdle(o)
	case d.ActDig:
		m.applyDig(o)
	}
}

func (m *OrganismManager) applyCycleHealthChanges(o *organism.Organism) {
	traits := o.TraitsRef()
	// Water away from an organism's ideal pH costs it health, more sharply the further off it is, less so the more Tolerance it has.
	phDist := math.Abs(traits.IdealPh - m.api.GetPhAtPoint(o.Location))
	phEffect := effects.PhDamage(c.GetCurrentGlobals(), o.Abilities()[physiology.AbilityTolerance], o.Size, phDist)
	// Add effects due to attack (not related to organism size).
	defense := o.Abilities()[physiology.AbilityDefense]
	damageMult := defenseDamageMult(o.Abilities())

	// Walk the incoming effects individually rather than taking the sum.
	healthEffects := 0.0
	// Health left to take, so a hit pays its attacker only for what was
	// actually there: damage runs far past what an organism holds.
	remaining := o.Health
	strongest, strongestScore := -1, -1
	for _, e := range m.requestManager.GetHealthEffects(o.Location) {
		landed := e.Amount * damageMult
		healthEffects += landed
		m.creditAttacker(o, e, landed, &remaining)
		m.applyThorns(o, e, defense)
		// Several attackers can land on one cell in a cycle; the cell goes to
		// the strongest, with the lower ID breaking a tie so map order cannot
		// decide it.
		if e.Amount < 0 && e.SourceID >= 0 && e.SourceID != o.ID {
			if a, ok := m.organisms[e.SourceID]; ok {
				if sc := a.Abilities()[physiology.AbilityAttack]; sc > strongestScore ||
					(sc == strongestScore && e.SourceID < strongest) {
					strongest, strongestScore = e.SourceID, sc
				}
			}
		}
	}
	o.RecordHealth(organism.HealthFromPh, o.Size*phEffect)
	o.RecordHealth(organism.HealthFromAttack, healthEffects)
	m.applyRecordedHealthChange(o, o.Size*phEffect+healthEffects)
	// Only an attack that actually killed hands its cell over; starving
	// after being hit is not a kill.
	o.KilledBy = -1
	if strongest >= 0 && o.Health <= deathHealthEpsilon {
		o.KilledBy = strongest
	}

	// Lifespan enforcement: when MaxLifespan > 0 every organism dies at that age.
	if maxLifespan := c.MaxLifespan(); maxLifespan > 0 && o.Age >= maxLifespan {
		o.Health = 0
	}
}

func defenseDamageMult(scores physiology.Scores) float64 {
	return effects.DamageTakenMultiplier(c.GetCurrentGlobals(), scores[physiology.AbilityDefense])
}

func (m *OrganismManager) applyIdle(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromIdle()*o.Size, organism.HealthFromAction)
	o.Status = organism.StatusIdle
}

// add a positive health change if organism attempts chemosynthesis in a favorable ph environment.
func (m *OrganismManager) applyChemosynthesis(o *organism.Organism) {
	g := c.GetCurrentGlobals()
	traits := o.TraitsRef()
	distance := math.Abs(traits.IdealPh - m.api.GetPhAtPoint(o.Location))
	gain := effects.ChemosynthesisGain(g, o.Abilities()[physiology.AbilityChemosynthesis], o.Size, distance)
	gain = effects.ChemoCrowdedGain(g, gain, m.chemoClaimsAround(o))
	m.applyHealthChange(o, gain, organism.HealthFromChemo)
	if gain <= 0 {
		o.Status = organism.StatusChemoFailed
		return
	}
	o.Status = organism.StatusChemoSuccess

	// Chemosynthesis pushes local pH down in proportion to the health it just gained.
	delta := effects.ChemoPhPush(g, o.Traits().Abilities[physiology.AbilityChemosynthesis], gain)
	m.api.AddPhChangeAtPoint(o.Location, -delta)
	o.PhNegative += delta
}

// buildChemoClaims counts, for every cell, how many chemosynthesising
// organisms draw on it this cycle — itself if it stands there, plus any
// standing in the four cells around it.
//
// Built ONCE per cycle at the top of the resolve phase, where every
// organism's action is already decided: UpdateAction runs for all of them
// before any of this, so the counts do not depend on resolve order and two
// neighbours contest each other symmetrically. Per organism it would be 25
// lookups; per cycle it is one pass over the chemosynthesisers.
//
// Nil when crowding is off, which is what applyChemosynthesis checks.
func (m *OrganismManager) buildChemoClaims() {
	if c.ChemoCrowdingPenalty() <= 0 {
		m.chemoClaims = nil
		return
	}
	w, h := c.GridUnitsWide(), c.GridUnitsHigh()
	if len(m.chemoClaims) != w*h {
		m.chemoClaims = make([]int32, w*h)
	} else {
		for i := range m.chemoClaims {
			m.chemoClaims[i] = 0
		}
	}
	for _, id := range m.organismIds {
		o, ok := m.organisms[id]
		if !ok || o == nil || o.Status == organism.StatusDying {
			continue
		}
		if o.Action() != d.ActChemosynthesis {
			continue
		}
		// It draws on its own cell and the four around it.
		m.chemoClaims[o.Location.Y*w+o.Location.X]++
		for _, dir := range utils.Directions {
			p := o.Location.Add(dir)
			m.chemoClaims[p.Y*w+p.X]++
		}
	}
}

// chemoClaimsAround is the claimant count for the five cells an organism
// draws on, in Directions order after its own.
func (m *OrganismManager) chemoClaimsAround(o *organism.Organism) [effects.ChemoCrowdingCells]int {
	var out [effects.ChemoCrowdingCells]int
	for i := range out {
		out[i] = 1
	}
	if m.chemoClaims == nil {
		return out
	}
	w := c.GridUnitsWide()
	out[0] = int(m.chemoClaims[o.Location.Y*w+o.Location.X])
	for i, dir := range utils.Directions {
		p := o.Location.Add(dir)
		out[i+1] = int(m.chemoClaims[p.Y*w+p.X])
	}
	return out
}

func (m *OrganismManager) applyHealthChange(o *organism.Organism, amount float64, src organism.HealthSource) {
	o.RecordHealth(src, amount)
	m.applyRecordedHealthChange(o, amount)
}

// applyRecordedHealthChange is for a caller that has already split its own
// accounting across sources. The ledger is an observation recorded beside a
// health change rather than through it, so that no accounting can alter the
// float arithmetic a replay depends on — which is why the pH damage and the
// attacks landing on a cell stay one addition here.
func (m *OrganismManager) applyRecordedHealthChange(o *organism.Organism, amount float64) {
	prevSize := o.Size
	o.ApplyHealthChange(amount)
	if o.Size > prevSize {
		m.addUpdatedPoint(o.Location)
	}
}

func (m *OrganismManager) applyAttack(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	m.applyHealthChange(o, effects.AttackCost(c.GetCurrentGlobals(),
		o.Abilities()[physiology.AbilityAttack], o.Size), organism.HealthFromAction)
	o.Status = organism.StatusAttacking
	// Attacks land on organisms only, through the health-effect request map.

	// An attack into open water carries the attacker into the cell: a lunge
	// rather than a swing at nothing. The claim was staked in the decide
	// phase against the world as it was then, so the cell is re-checked
	// here exactly as applyMove re-checks its own.
	target := o.Location.Add(o.Direction)
	if m.isMatchingPositionRequest(target, o.ID) && m.isOpenForLunge(target) {
		m.relocate(o, target)
		o.Status = organism.StatusAttackMove
	}
}

// calculateAttackEffect is the health change o's attack inflicts before the target's Defense.
func (m *OrganismManager) calculateAttackEffect(o *organism.Organism) float64 {
	return -effects.AttackDamage(c.GetCurrentGlobals(), o.Abilities()[physiology.AbilityAttack], o.Size)
}

// applyDig resolves ActDig: pays the dig cost, brings buried food back up under the organism, AND reinforces / adds walls on both sides.
func (m *OrganismManager) applyDig(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	// Digging cuts both ways: a specialist pays less per dig AND shifts more wall strength.
	g := c.GetCurrentGlobals()
	digScore := o.Abilities()[physiology.AbilityDigging]
	m.applyHealthChange(o, effects.DigCost(g, digScore, o.Size), organism.HealthFromAction)
	o.Status = organism.StatusDigging

	created := effects.DigWallCreated(g, digScore, o.Size)

	if amount := effects.DigFood(g, digScore, o.Size); amount > 0 {
		m.api.UnburyFoodAtPoint(o.Location, amount)
	}

	// An organism that raises nothing leaves its flanks alone entirely.
	if created <= 0 {
		return
	}
	for _, side := range []utils.Point{
		o.Location.Add(o.Direction.Left()),
		o.Location.Add(o.Direction.Right()),
	} {
		if m.isOrganismAtLocation(side) {
			continue
		}
		// A wall going up buries the food that was there rather than destroying it.
		if item, ok := m.api.GetFoodAtPoint(side); ok && item != nil {
			m.api.BuryAllFoodAtPoint(side)
		}
		m.api.AddWallStrength(side, created)
		m.api.AddWallUpdate(side)
	}
}

// applyThorns hurts whoever dealt an incoming hit.
// creditAttacker feeds an attacker back a share of the health its hit took.
//
// remaining is the defender's health not yet claimed by an earlier hit this
// cycle, so two attackers splitting a kill are paid for their halves rather
// than each for a whole one. It is decremented as it is spent.
//
// Reaches across to the attacker by SourceID exactly as thorns does, and for
// the same reason: the cycle's request map has been consumed by the time
// damage resolves, so there is nowhere left to queue it.
func (m *OrganismManager) creditAttacker(defender *organism.Organism, hit HealthEffect, landed float64, remaining *float64) {
	if hit.Amount >= 0 || hit.SourceID < 0 || hit.SourceID == defender.ID {
		return
	}
	took := math.Min(-landed, *remaining)
	if took <= 0 {
		return
	}
	*remaining -= took
	gain := effects.AttackHealthGained(c.GetCurrentGlobals(), took)
	if gain <= 0 {
		return
	}
	attacker, ok := m.organisms[hit.SourceID]
	if !ok {
		// Died this cycle; nothing left to feed.
		return
	}
	m.applyHealthChange(attacker, gain, organism.HealthFromPredation)
	m.addUpdatedPoint(attacker.Location)
}

func (m *OrganismManager) applyThorns(defender *organism.Organism, hit HealthEffect, defense int) {
	// Only damage from another organism triggers thorns.
	if hit.Amount >= 0 || hit.SourceID < 0 || hit.SourceID == defender.ID {
		return
	}
	damage := thornsDamage(defense, defender.Size)
	if damage >= 0 {
		return
	}
	attacker, ok := m.organisms[hit.SourceID]
	if !ok {
		// Attacker already died this cycle; nothing to hurt back.
		return
	}
	m.applyHealthChange(attacker, damage, organism.HealthFromThorns)
	m.addUpdatedPoint(attacker.Location)
}

// thornsDamage is the health change a defender's thorns inflict per hit taken.
func thornsDamage(defense int, defenderSize float64) float64 {
	return -effects.ThornsDamage(c.GetCurrentGlobals(), defense, defenderSize)
}

// deathHealthEpsilon is the smallest Health value that still counts as alive.
const deathHealthEpsilon = 0.005

// claimKilledCell moves a killer onto the cell its victim has just vacated,
// so a predator ends up standing on the body it made — which is what it has
// to do to eat it.
//
// Everything is re-checked here because a cycle has passed since the kill:
// the killer may have died, moved away, or had the cell taken. It reports
// whether the move happened, so one killer cannot take two cells.
// corpseValue is the food an organism's body is worth when it dies.
func corpseValue(o *organism.Organism) int {
	return int(o.Size * c.CorpseFoodMultiplier())
}

// applyKillClaims hands each victim's cell to the organism that killed it,
// in the same cycle as the kill.
//
// After the resolve loop, not during it: an organism that changes cell
// mid-phase would be read for health effects at a cell it was not standing
// in when they were queued, and would take its own attack damage. Running
// here also means the attacker's own action has already stamped its status,
// so StatusAttackMove is the one the renderer sees.
//
// The body keeps rendering at the cell for the rest of the cycle so its
// death animation plays; what it gives up is the grid square, which is the
// only part that was holding the killer back.
func (m *OrganismManager) applyKillClaims() {
	claimed := map[int]bool{}
	// m.organismIds is sorted, so which of two bodies a shared killer takes
	// does not depend on map order.
	for _, id := range m.organismIds {
		victim, ok := m.organisms[id]
		if !ok || victim.Status != organism.StatusDying || victim.KilledBy < 0 {
			continue
		}
		killer := victim.KilledBy
		// Consumed within the cycle that set it; a body lingers for its
		// death animation and must not be claimed from twice.
		victim.KilledBy = -1
		if claimed[killer] {
			continue
		}
		if m.claimKilledCell(killer, victim) {
			claimed[killer] = true
		}
	}
}

// claimKilledCell moves an attacker into the cell of the organism it killed,
// dropping the corpse there first so the killer ends up standing on it.
func (m *OrganismManager) claimKilledCell(killerID int, victim *organism.Organism) bool {
	attacker, ok := m.organisms[killerID]
	if !ok || attacker.Status == organism.StatusDying {
		return false
	}
	cell := victim.Location
	if attacker.Location == cell {
		return false
	}
	adjacent := false
	for _, dir := range utils.Directions {
		if attacker.Location.Add(dir) == cell {
			adjacent = true
			break
		}
	}
	if !adjacent {
		return false
	}

	m.gridMutex.Lock()
	// The cell is the victim's to give up; anything else standing there is
	// someone else's and is not taken.
	free := m.organismIDGrid[cell.X][cell.Y] == victim.ID ||
		m.organismIDGrid[cell.X][cell.Y] == -1
	if free {
		m.organismIDGrid[cell.X][cell.Y] = -1
	}
	m.gridMutex.Unlock()
	if !free {
		return false
	}

	// Dropped while the cell reads empty, since food is never placed on a
	// living organism and the attacker is about to be one. This is the whole
	// point of the mechanic: a kill pays in a corpse, and the killer has to
	// be standing on it to eat it.
	m.api.AddFoodAtPoint(cell, corpseValue(victim))

	m.gridMutex.Lock()
	m.organismIDGrid[attacker.Location.X][attacker.Location.Y] = -1
	m.organismIDGrid[cell.X][cell.Y] = attacker.ID
	m.gridMutex.Unlock()

	m.addUpdatedPoint(attacker.Location)
	attacker.Location = cell
	attacker.TraveledDist++
	attacker.Status = organism.StatusAttackMove
	m.addUpdatedPoint(cell)
	return true
}

// markDyingIfDead transitions an organism into the dying state when its health drops below the alive threshold.
func (m *OrganismManager) markDyingIfDead(o *organism.Organism) bool {
	if o.Status == organism.StatusDying {
		return true
	}
	if o.Health > deathHealthEpsilon {
		return false
	}
	if o.TreeNode != nil {
		o.TreeNode.MarkDead(m.api.Cycle())
	}
	o.Status = organism.StatusDying
	m.addUpdatedPoint(o.Location)
	return true
}

// finalizeDeaths removes the dying organisms at the start of every cycle.
func (m *OrganismManager) finalizeDeaths() {
	type drop struct {
		id   int
		loc  utils.Point
		size int
		// yielded: a killer took this body's square in the cycle it died, and
		// dropped its corpse there before moving in.
		//
		// Read off the GRID rather than remembered from last cycle. Nothing
		// else clears a dying organism's own square, so the grid no longer
		// naming it is exactly the claim having happened — and the grid is
		// snapshot state, where a flag held on the manager between the claim
		// and here is not. Snapshots are taken between cycles, which is
		// precisely that gap, so a restore lost it and this cleared a square
		// holding a living killer.
		yielded bool
	}
	var drops []drop
	for _, o := range m.organisms {
		if o.Status != organism.StatusDying {
			continue
		}
		drops = append(drops, drop{
			id: o.ID, loc: o.Location, size: corpseValue(o),
			yielded: m.organismIDGrid[o.Location.X][o.Location.Y] != o.ID,
		})
	}
	// Sorted because a killer can only take one cell and two bodies can name
	// the same one: map order would decide which, differently each run.
	sort.Slice(drops, func(i, j int) bool { return drops[i].id < drops[j].id })
	if len(drops) == 0 {
		return
	}
	m.gridMutex.Lock()
	m.organismMutex.Lock()
	for _, dr := range drops {
		// A yielded square holds its killer now, so clearing it would erase
		// a living organism from the grid.
		if !dr.yielded {
			m.organismIDGrid[dr.loc.X][dr.loc.Y] = -1
		}
		delete(m.organisms, dr.id)
	}
	m.gridMutex.Unlock()
	m.organismMutex.Unlock()
	// AddFoodAtPoint takes the food-manager lock; do it outside the grid/organism critical section.
	for _, dr := range drops {
		// The claim dropped the corpse before it moved in, so a second one
		// here would land on the killer, where the guard refuses it anyway.
		if !dr.yielded {
			m.api.AddFoodAtPoint(dr.loc, dr.size)
		}
		m.addUpdatedPoint(dr.loc)
	}
}

func (m *OrganismManager) applySpawn(o *organism.Organism) {
	if success := m.SpawnChildOrganism(o); success {
		m.applyHealthChange(o, o.HealthCostToReproduce(), organism.HealthFromSpawn)
		o.Children++
	}
	o.Status = organism.StatusSpawning
	// SpawnChildOrganism can still fail here despite the decide-phase "has empty neighbour" check (e.g. a higher-ID organism with the same target won the position-request priority), but the trapped-organism case the user reported is handled at decide time now.
}

// applyEat resolves ActEat against the cell the organism is STANDING ON, not the one ahead.
func (m *OrganismManager) applyEat(o *organism.Organism) {
	m.applyHealthChange(o, effects.EatCost(c.GetCurrentGlobals(),
		o.Abilities()[physiology.AbilityEating], o.Size), organism.HealthFromAction)
	target := o.Location

	amountToEat := m.calculateValueToEat(o, target)
	m.api.RemoveFoodAtPoint(target, int(math.Ceil(amountToEat)))
	// Food removed and health gained are two different quantities.
	gain := effects.HealthFromFood(c.GetCurrentGlobals(), amountToEat)
	// A meal's overflow past the eater's size grows it at EatingGrowthFactor rather than the general GrowthFactor.
	prevSize := o.Size
	o.RecordHealth(organism.HealthFromFood, gain)
	o.ApplyHealthChangeWithGrowth(gain, c.EatingGrowthFactor())
	if o.Size > prevSize {
		m.addUpdatedPoint(o.Location)
	}
	if amountToEat > 0 {
		o.Status = organism.StatusEatSuccess
		// Pushes pH up at the organism's own cell, in proportion to the health the meal gave, like chemosynthesis pushes it down there.
		delta := effects.EatingPhPush(c.GetCurrentGlobals(),
			o.Traits().Abilities[physiology.AbilityEating], gain)
		m.api.AddPhChangeAtPoint(o.Location, delta)
		o.PhPositive += delta
	} else {
		o.Status = organism.StatusEatFailed
	}
}

func (m *OrganismManager) calculateValueToEat(o *organism.Organism, target utils.Point) float64 {
	if item, found := m.api.GetFoodAtPoint(target); found {
		// A high Eating score means more health per action.
		maxCanEat := effects.MaxFoodPerEat(c.GetCurrentGlobals(), o.Abilities()[physiology.AbilityEating], o.Size)
		return math.Min(float64(item.Value), maxCanEat)
	}
	return 0
}

func (m *OrganismManager) applyMove(o *organism.Organism) {
	// A high Movement score makes travel cheap; a low one still moves the organism, just at a steep price.
	m.applyHealthChange(o, effects.MoveCost(c.GetCurrentGlobals(), o.Abilities()[physiology.AbilityMovement], o.Size), organism.HealthFromAction)

	targetPoint := o.Location.Add(o.Direction)
	if m.isMatchingPositionRequest(targetPoint, o.ID) == false {
		// Nobody claimed the cell, so nobody found it empty when they decided.
		if !m.requestManager.HasPositionRequest(targetPoint) {
			m.applyHealthChange(o, c.HealthChangeFromBlockedMove()*o.Size, organism.HealthFromAction)
		}
		o.Status = organism.StatusMoveBlocked
		return
	}
	// The claim was staked in the decide phase, against the world as it was then.
	if m.isOrganismAtLocation(targetPoint) {
		o.Status = organism.StatusMoveBlocked
		return
	}
	if strength := m.api.GetWallStrengthAtPoint(targetPoint); strength > 0 {
		if !m.canBurrow(o, strength) {
			o.Status = organism.StatusMoveBlocked
			return
		}
		// Through it: the wall is destroyed outright rather than worn down.
		m.api.AddWallStrength(targetPoint, -strength)
		// Flag the WALL layer, not just the organism layer.
		m.api.AddWallUpdate(targetPoint)
		m.addUpdatedPoint(targetPoint)
		m.pileBurrowSpoil(o, targetPoint, strength)
	}

	m.relocate(o, targetPoint)
	o.Status = organism.StatusMoveSuccess
}

// relocate moves an organism into a cell it has already been cleared for,
// updating the grid and flagging both cells for repaint.
func (m *OrganismManager) relocate(o *organism.Organism, target utils.Point) {
	m.addUpdatedPoint(o.Location)
	m.addUpdatedPoint(target)

	o.TraveledDist++

	m.gridMutex.Lock()
	m.organismIDGrid[o.Location.X][o.Location.Y] = -1
	m.organismIDGrid[target.X][target.Y] = o.ID
	m.gridMutex.Unlock()

	o.Location = target
}

// pileBurrowSpoil banks part of a burrowed wall onto the two cells flanking the tunnel.
func (m *OrganismManager) pileBurrowSpoil(o *organism.Organism, target utils.Point, strength int) {
	spoil := effects.BurrowSpoilPerSide(c.GetCurrentGlobals(), strength)
	if spoil <= 0 {
		return
	}
	for _, side := range []utils.Point{
		target.Add(o.Direction.Left()),
		target.Add(o.Direction.Right()),
	} {
		if m.isOrganismAtLocation(side) {
			continue
		}
		// A wall going up buries the food that was there rather than destroying it.
		if item, ok := m.api.GetFoodAtPoint(side); ok && item != nil {
			m.api.BuryAllFoodAtPoint(side)
		}
		m.api.AddWallStrength(side, spoil)
		m.api.AddWallUpdate(side)
		m.addUpdatedPoint(side)
	}
}

func (m *OrganismManager) applyRightTurn(o *organism.Organism) {
	m.applyHealthChange(o, effects.TurnCost(c.GetCurrentGlobals(), o.Abilities()[physiology.AbilityMovement], o.Size), organism.HealthFromAction)

	o.Direction = o.Direction.Right()
	o.Status = organism.StatusTurnRight
}

func (m *OrganismManager) applyLeftTurn(o *organism.Organism) {
	m.applyHealthChange(o, effects.TurnCost(c.GetCurrentGlobals(), o.Abilities()[physiology.AbilityMovement], o.Size), organism.HealthFromAction)

	o.Direction = o.Direction.Left()
	o.Status = organism.StatusTurnLeft
}

// newFoundingOrganism builds one of the simulation's initial organisms.
func (m *OrganismManager) newFoundingOrganism(id int, point utils.Point) *organism.Organism {
	designs := m.foundingDesigns()
	if len(designs) == 0 {
		return organism.NewRandom(m.rng, id, point, m.api)
	}
	ds := designs[m.designsDealt%len(designs)]
	m.designsDealt++
	o, err := organism.NewDesigned(m.rng, id, point, m.api, ds)
	if err != nil {
		fmt.Printf("\nWarning: design %q could not be used (%v); founding with a random organism instead.",
			ds.Name, err)
		return organism.NewRandom(m.rng, id, point, m.api)
	}
	return o
}

func (m *OrganismManager) foundingDesigns() []organism.Design {
	if m.designsLoaded {
		return m.designs
	}
	m.designsLoaded = true
	names := c.InitialDesigns()
	if len(names) == 0 {
		return m.designs
	}
	found := organism.DesignsByName(organism.DesignsDir, names)
	if len(found) < len(names) {
		fmt.Printf("\nWarning: %d of %d configured designs were not found in %s/.",
			len(names)-len(found), len(names), organism.DesignsDir)
	}
	// A design whose tree is over the configured limit is dropped rather than founded with.
	for _, ds := range found {
		if ds.ExceedsTreeLimit(c.MaxDecisionTreeSize()) {
			fmt.Printf("\nWarning: design %q has %d decision-tree nodes, over the %d-node limit; not founding with it.",
				ds.Name, ds.TreeSize(), c.MaxDecisionTreeSize())
			continue
		}
		m.designs = append(m.designs, ds)
	}
	return m.designs
}

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
		"\nAncestor: %10d"+
		"\n  Health: %10.2f"+
		"\n    CalcAndUpdateSize: %10.2f   |              MaxSize:  %4.2f"+
		"\n  Tree:\n%s",
		o.ID, int(o.InitialHealth()),
		o.Age, int(o.MinHealthToSpawn()),
		o.Children, o.MinCyclesBetweenSpawns(),
		o.OriginalAncestorID,
		o.Health,
		o.Size, o.MaxSize(),
		o.GetDecisionTreeCopy().Print())
}
