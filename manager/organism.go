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
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// HistoryType identifies the type of per-cycle history data.
type HistoryType int

const (
	HistoryPopulation     HistoryType = iota // cycle : ancestorId : livingDescendantsCount
	HistoryPhDistribution                    // cycle : phBucket : gridCellCount
)

// OrganismManager contains 2D array of booleans showing if organism present
type OrganismManager struct {
	api            organism.API
	rng            *simrand.RNG
	requestManager RequestManager

	organisms             map[int]*organism.Organism
	organismIDGrid        [][]int
	totalOrganismsCreated int

	organismIds []int

	oldestId         int
	oldestAge        int
	mostChildrenId   int
	mostChildren     int
	mostTraveledId   int
	mostTraveledDist int
	mostAggressiveId   int
	mostAggressiveHits int

	originalAncestors      []int
	originalAncestorColors map[int]color.Color              // all original ancestor IDs with at least one descendant
	descendantTrees        map[int]*organism.DescendantNode // ancestorId : root node
	// descendantNodeIndex maps every node ID (alive or dead) to its
	// DescendantNode for O(1) lookup. Populated alongside descendantTrees
	// (RestoreDescendantTrees seeds it from the loaded payload; spawn
	// paths append to it as new organisms are born). Without this,
	// GetTreeNodeByID for a dead organism would walk every tree from its
	// root every call — quadratic in tree size when called per render
	// frame from the descendant-highlight code.
	descendantNodeIndex map[int]*organism.DescendantNode
	// mostSuccessful is the precomputed set of IDs whose descendant tree
	// node has AllBranchesDeadCycle == 0 (lineage survives to end) or
	// equal to the maximum AllBranchesDeadCycle across the loaded tree.
	// Populated once in RestoreDescendantTrees so the per-tick "Most
	// Successful" select mode just does an O(1) lookup per live
	// organism instead of walking the full tree every frame. Empty
	// (nil) in pure live mode where the tree is built up dynamically;
	// callers fall back to the walking implementation when nil.
	mostSuccessful map[int]struct{}
	history map[HistoryType]map[int]map[int]int32 // type : cycle : key : count

	UpdateDuration, ResolveDuration, SortDuration, HistoryDuration time.Duration

	// Sub-timings within Decide phase
	DecideStatsDuration, DecideTreeDuration, DecideRequestDuration time.Duration
	// Sub-timings within Resolve phase
	ResolveHealthDuration, ResolveActionDuration, ResolveDeadDuration time.Duration
	ResolveSpawnDuration                                              time.Duration
	SpawnCount                                                        int

	ancestorMutex sync.RWMutex
	historyMutex  sync.RWMutex
	gridMutex     sync.RWMutex
	organismMutex sync.RWMutex
}

// NewOrganismManager creates all Organisms and updates grid
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
		history: map[HistoryType]map[int]map[int]int32{
			HistoryPopulation:     make(map[int]map[int]int32),
			HistoryPhDistribution: make(map[int]map[int]int32),
		},
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

	m.resetInterestingStats()

	m.updateOrganismActions()
	m.resolveOrganismActions()

	m.updateHistory()
}

// updateOrganismActions runs each organism's per-cycle stats update,
// decision-tree walk, and request submission. Sequential by design:
// an earlier parallel-by-CPU implementation made the simulation
// non-deterministic across replays — replays starting from different
// snapshots reached different states at the same target cycle, which
// broke step-back/forward repeatability and caused organism IDs to be
// assigned to different organisms across timeline jumps. The race
// wasn't on shared writes (workers only modified their own organism)
// but somewhere in the read paths through lookupAPI, so until that's
// pinned down, sequential execution is the source of truth.
func (m *OrganismManager) updateOrganismActions() {
	start := time.Now()

	// Drain any organisms that finished their dying cycle in the
	// previous tick. finalizeDeaths advances Dying → Decaying and
	// removes Decaying ones (dropping food at their cell). Runs
	// before sort/UpdateStats so removed organisms don't appear in
	// this cycle's iteration order.
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
		// Dying organisms are frozen — no decisions, no requests,
		// no stats updates. They sit on the grid for the cycle
		// transition that runs the death animation; finalizeDeaths
		// removes them at the start of the next cycle.
		if o.Status == organism.StatusDying {
			m.organismIds = append(m.organismIds, o.ID)
			continue
		}
		if o.Action() == d.ActAttack {
			m.addUpdatedPoint(o.Location)
		}
		o.UpdateStats()
		o.UpdateAction()
		// Promote to ActSpawn only if the organism is eligible AND
		// the grid actually has room for a child. Doing this at
		// decide time (rather than the old "always promote, fall
		// back at resolve" approach) keeps the request map aligned
		// with what will run and removes any out-of-order side
		// effects later in the cycle.
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

	var accHealth, accAction, accDead, accSpawn time.Duration
	spawnCount := 0

	for _, id := range m.organismIds {
		o, ok := m.organisms[id]
		if !ok || o == nil || o.Age == 0 {
			continue
		}

		t0 := time.Now()
		// Dying organisms skip the resolve loop entirely —
		// finalizeDeaths drives their removal at the start of the
		// next cycle, not the per-organism applyCycleHealthChanges
		// / applyAction path. Without this guard, applyCycleHealthChanges
		// would push their health further negative and the post-action
		// markDyingIfDead call would no-op on an already-dying organism.
		if o.Status == organism.StatusDying {
			continue
		}
		m.applyCycleHealthChanges(o)
		t1 := time.Now()
		// If health-change just killed the organism, skip its
		// action this cycle. markDyingIfDead returns true when it
		// flagged the organism dying; we leave the apply loop here
		// rather than running a stale action.
		if m.markDyingIfDead(o) {
			t2 := time.Now()
			accHealth += t1.Sub(t0)
			accDead += t2.Sub(t1)
			continue
		}
		if o.Action() == d.ActSpawn {
			m.applySpawn(o)
			t2 := time.Now()
			accSpawn += t2.Sub(t1)
			spawnCount++
			t1 = t2
		} else {
			m.applyAction(o)
		}
		t2 := time.Now()
		// Post-action death check: an action like Attack with
		// extreme cost could drop the organism below the threshold.
		m.markDyingIfDead(o)
		m.updateInterestingStats(o)
		t3 := time.Now()

		accHealth += t1.Sub(t0)
		accAction += t2.Sub(t1)
		accDead += t3.Sub(t2)
	}

	m.ResolveHealthDuration = accHealth
	m.ResolveActionDuration = accAction
	m.ResolveSpawnDuration = accSpawn
	m.ResolveDeadDuration = accDead
	m.SpawnCount = spawnCount
	m.ResolveDuration = time.Since(start)
}

// updateHistory updates the population map, average phEffect per ancestor,
// and pH distribution for all living organisms and the environment
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
	m.HistoryDuration = time.Since(start)
}

func (m *OrganismManager) updateRequestMap(o *organism.Organism) {
	m.updateRequestMapTo(o, &m.requestManager)
}

func (m *OrganismManager) updateRequestMapTo(o *organism.Organism, rm *RequestManager) {
	switch o.Action() {
	case d.ActEat:
		target := o.Location.Add(o.Direction)
		value := int(math.Ceil(m.calculateValueToEat(o, target)))
		rm.AddFoodRequest(target, value)
	case d.ActMove:
		target := o.Location.Add(o.Direction)
		if m.isGridLocationEmpty(target) {
			rm.AddPositionRequest(target, o.ID)
		}
	case d.ActSpawn:
		if target, _, ok := m.getChildSpawnLocation(o); ok {
			rm.AddPositionRequest(target, o.ID)
		}
	case d.ActAttack:
		effect := m.calculateAttackEffect(o)
		target := o.Location.Add(o.Direction)
		rm.AddHealthEffectRequest(target, effect)
		// Track attack stats: every attack counts toward AttackTotal,
		// and only those landing on a real organism count as a hit.
		// Checked at decision-time so "in front of you when you decided
		// to attack" is the criterion the user described.
		o.AttackTotal++
		if m.isOrganismAtLocation(target) {
			o.AttackHits++
		}
	case d.ActSting:
		// Sting is an area-of-effect attack hitting all 4 cardinal
		// neighbours with reduced per-cell damage. AttackTotal
		// increments once per sting cycle (consistent with attack as
		// an action count), AttackHits increments per occupied
		// adjacent cell — a 4-target sting can credit up to 4 hits in
		// one cycle, so the hits/total stat properly rewards stingers
		// that find crowds.
		stingEffect := m.calculateStingEffect(o)
		o.AttackTotal++
		for _, offset := range utils.Directions {
			target := o.Location.Add(offset)
			rm.AddHealthEffectRequest(target, stingEffect)
			if m.isOrganismAtLocation(target) {
				o.AttackHits++
			}
		}
	}
}

func (m *OrganismManager) addAttackRequest(o *organism.Organism) {
	effect := m.calculateAttackEffect(o)
	target := o.Location.Add(o.Direction)
	m.requestManager.AddHealthEffectRequest(target, effect)
}

func (m *OrganismManager) addSpawnRequest(o *organism.Organism) {
	target, _, ok := m.getChildSpawnLocation(o)
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

// GetHistory returns a history map by type. Caller must hold history read lock.
func (m *OrganismManager) GetHistory(histType HistoryType) map[int]map[int]int32 {
	return m.history[histType]
}

// GetAncestorColors returns a map all original ancestor IDs to their color
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

// GetAncestors returns a list of all original ancestor IDs
func (m *OrganismManager) GetAncestors() []int {
	return m.originalAncestors
}

func (m *OrganismManager) addUpdatedPoint(point utils.Point) {
	m.api.AddOrganismUpdate(point)
}

func (m *OrganismManager) addToOrganismIds(o *organism.Organism) {
	m.organismIds = append(m.organismIds, o.ID)
}

// SpawnRandomOrganism creates an Organism with random position.
//
// Checks random positions on the grid until it finds an empty one. Calls
// NewOrganism to initialize decision tree, other random attributes.
func (m *OrganismManager) SpawnRandomOrganism() {
	if spawnPoint, found := m.getRandomSpawnLocation(); found {
		id := m.generateId()
		o := organism.NewRandom(m.rng, id, spawnPoint, m.api)

		// In replay/resume modes the descendant tree is pre-loaded from the
		// recorded simulation. Reuse the existing node for this ID instead of
		// creating a duplicate, so the population graph isn't double-counted.
		node, exists := m.descendantTrees[id]
		if !exists {
			node = &organism.DescendantNode{
				ID:         id,
				Color:      o.Color(),
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

// SpawnChildOrganism creates a new organism near an existing 'parent' organism
// with a copy of its parent's node library. (No organism created if no room or
// if more than 1 organism made a request to spawn and/or move into the desired
// location.
// Returns true / false depending on whether a child was actually spawned.
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
	// The birth-animation path synthesises Frame.FromLocation =
	// child.Location.Sub(child.Direction), so as long as Direction is
	// the outward step, the 2-cell move sprite draws parent→child
	// correctly without anyone having to remember the parent's cell.
	o := parent.NewChild(m.rng, id, spawnPoint, spawnDirection, m.api)

	// In replay/resume modes the parent's existing tree (loaded from the
	// recorded simulation) already has a child with this ID. Reuse it so we
	// don't double the node under the same parent.
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

// getRandomSpawnLocation returns a random empty grid point with at
// least one empty cardinal neighbour. Retries up to maxSpawnAttempts —
// a single try used to fail silently when InitialFood was high enough
// that the first random pick collided with food, and the entire
// simulation would end at cycle -1 because no initial organism got
// placed. The neighbour check avoids spawning a chemosynthesiser fully
// walled in by food, which can't reproduce and ends the run early.
func (m *OrganismManager) getRandomSpawnLocation() (utils.Point, bool) {
	const maxSpawnAttempts = 1000
	for i := 0; i < maxSpawnAttempts; i++ {
		point := utils.GetRandomPoint(m.rng, c.GridUnitsWide(), c.GridUnitsHigh())
		if m.isGridLocationEmpty(point) && m.hasEmptyNeighbor(point) {
			return point, true
		}
	}
	return utils.Point{}, false
}

// hasEmptyNeighbor reports whether at least one of point's four
// cardinal neighbours is empty (no wall, food, or organism). Used to
// pick initial spawn locations that aren't fully boxed in.
func (m *OrganismManager) hasEmptyNeighbor(point utils.Point) bool {
	direction := utils.Point{X: 0, Y: -1}
	for i := 0; i < 4; i++ {
		if m.isGridLocationEmpty(point.Add(direction)) {
			return true
		}
		direction = direction.Left()
	}
	return false
}

// getChildSpawnLocation returns an empty cell adjacent to the parent and the
// unit cardinal step from parent→cell. The step is returned separately
// because computing it via spawnPoint.Sub(parent.Location) would wrap on
// grid edges (e.g. (-1, 0) → (gridWidth-1, 0)) and break direction-based
// sprite rotation.
func (m *OrganismManager) getChildSpawnLocation(parent *organism.Organism) (utils.Point, utils.Point, bool) {
	var point utils.Point
	direction := parent.Direction
	for i := 0; i < 4; i++ {
		direction = direction.Left()
		point = parent.Location.Add(direction)

		empty := m.isGridLocationEmpty(point)
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

func (m *OrganismManager) isGridLocationEmpty(point utils.Point) bool {
	return !m.api.IsWallAtPoint(point) && !m.isFoodAtLocation(point) && !m.isOrganismAtLocation(point)
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

// GetOrganismTreeNode returns the tree node for the given organism ID, or nil.
func (m *OrganismManager) GetOrganismTreeNode(id int) *organism.DescendantNode {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()
	if o, ok := m.organisms[id]; ok {
		return o.TreeNode
	}
	return nil
}

// GetTreeNodeByID returns the DescendantNode for the given ID — alive
// or dead — in O(1) via descendantNodeIndex. Used by UI render paths
// like the descendant-highlight walk that runs every frame.
func (m *OrganismManager) GetTreeNodeByID(id int) *organism.DescendantNode {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()
	if o, ok := m.organisms[id]; ok {
		return o.TreeNode
	}
	return m.descendantNodeIndex[id]
}

// GetOrganismInfoAtPoint returns the Organism Info at the given point
// (nil if none). The fast path is the organismIDGrid lookup; a slower
// fallback scans m.organisms by Location so a stray grid/organisms
// inconsistency doesn't make a visibly-rendered organism unhoverable.
// Production code shouldn't normally hit the fallback — if it does,
// there's a bug elsewhere that's letting the grid drift from m.organisms.
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
	// Fallback: linear scan. Catches the case where the grid says
	// nothing's at this cell but an organism is logically here.
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()
	for _, o := range m.organisms {
		if o.Location == point {
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

// GetOldestId returns the id of the oldest living organism
func (m *OrganismManager) GetOldestId() int {
	return m.oldestId
}

// GetMostChildrenId returns the id of the most traveled organism
func (m *OrganismManager) GetMostChildrenId() int {
	return m.mostChildrenId
}

// GetMostTraveledId returns the id of the most traveled organism
func (m *OrganismManager) GetMostTraveledId() int {
	return m.mostTraveledId
}

// GetMostAggressiveId returns the id of the organism with the most
// attack hits (attacks that landed on a target organism). -1 if none.
func (m *OrganismManager) GetMostAggressiveId() int {
	if m.mostAggressiveHits <= 0 {
		return -1
	}
	return m.mostAggressiveId
}

// mostSuccessfulSet returns the set of "most successful" tree node IDs.
// In replay mode this is precomputed at restore time. In live mode the
// recording hasn't ended yet, so we compute it on the fly: every node
// whose subtree still contains an organism alive right now (EndCycle
// == 0). EndCycle is set once by MarkDead and never overwritten, so
// the walk is O(tree) and stable cycle-to-cycle.
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

// GetMostSuccessfulIds returns the IDs of currently-living organisms on
// the most-successful lineage — last alive at recording end (or, in live
// mode, currently alive) plus every ancestor of one. Result is sorted
// by ID for stable rendering.
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

// Organisms returns a snapshot slice of currently-living organisms.
// Used by post-restore initialisation that needs to walk every
// organism after the manager is wired into its parent simulation.
func (m *OrganismManager) Organisms() []*organism.Organism {
	m.organismMutex.RLock()
	defer m.organismMutex.RUnlock()
	out := make([]*organism.Organism, 0, len(m.organisms))
	for _, o := range m.organisms {
		out = append(out, o)
	}
	return out
}

// IsMostSuccessful reports whether the given organism ID is on the
// surviving (or, in extinct recordings, longest-lived) lineage. Used
// by panel UI to badge nodes in the descendant-tree view.
func (m *OrganismManager) IsMostSuccessful(id int) bool {
	set := m.mostSuccessfulSet()
	if set == nil {
		return false
	}
	_, ok := set[id]
	return ok
}

// GetMostSuccessfulId returns the ID of the oldest currently-living
// organism on the most-successful lineage, or -1 if none qualify.
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
	action := o.Action()
	// Junk-DNA fallback: a decision tree node may pick an action the
	// organism no longer has the physiology to resolve (e.g. the tree
	// kept ActMove from an ancestor but the lineage has since lost
	// FeatCilia). Re-route those to applyIdle and rewrite o.Action so
	// the renderer's sprite matches actual behaviour rather than
	// reporting the unrealisable intent. Re-gaining the feature later
	// naturally reactivates the original action.
	if !o.Traits().Features.ActionAvailable(action) {
		o.SetAction(d.ActIdle)
		m.applyIdle(o)
		return
	}
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
	case d.ActSting:
		m.applySting(o)
	case d.ActDig:
		m.applyDig(o)
	case d.ActBurrow:
		m.applyBurrow(o)
	case d.ActHunker:
		m.applyHunker(o)
	case d.ActFlare:
		m.applyFlare(o)
	case d.ActHide:
		m.applyHide(o)
	}
}

func (m *OrganismManager) applyCycleHealthChanges(o *organism.Organism) {
	traits := o.TraitsRef()
	phEffect := 0.0
	tolerance := c.PhTolerance()
	// Subtract health if organism is too far away from its ideal ph
	phDist := math.Abs(traits.IdealPh - m.api.GetPhAtPoint(o.Location))
	if phDist > tolerance {
		phEffect = (phDist - tolerance) * c.HealthChangePerUnhealthyPh()
	}
	// Add effects due to attack (not related to organism size).
	// AttackDamageTakenMult is the defender's passive tradeoff:
	// shells reduce incoming damage, etc. HunkerDamageTakenMult is
	// the per-cycle active modifier — multiplied on top when the
	// defender chose ActHunker this cycle. healthEffects is already
	// negative for damage, so multiplying scales magnitude.
	damageMult := o.Tradeoffs().AttackDamageTakenMult
	if o.Status == organism.StatusHunkering {
		damageMult *= c.HunkerDamageTakenMult()
	}
	healthEffects := m.requestManager.GetHealthEffects(o.Location) * damageMult
	m.applyHealthChange(o, o.Size*phEffect+healthEffects)

	// Lifespan enforcement: when MaxLifespan > 0 every organism dies
	// at that age. removeIfDead runs later in the same cycle and cleans
	// up as usual.
	if maxLifespan := c.MaxLifespan(); maxLifespan > 0 && o.Age >= maxLifespan {
		o.Health = 0
	}
}

// applyIdle is the resolution for ActIdle: the organism holds its current
// location and direction and pays only the configured idle health cost
// (default 0). Per-cycle pH effects and any damage from attacks are still
// applied separately by applyCycleHealthChanges.
func (m *OrganismManager) applyIdle(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromIdle()*o.Size)
	o.Status = organism.StatusIdle
}

// add a positive health change if organism attempts chemosynthesis in a
// favorable ph environment. The chemo-viable window is a scaled subset
// (or superset) of the organism's general pH tolerance: PhTolerance is
// multiplied by ChemosynthesisTolerance so the config can make
// chemosynthesis strictly harder than merely surviving (factor < 1) or
// easier (factor > 1). Also records whether the attempt failed so the
// renderer can play the chemofail sprite in place of the normal one.
func (m *OrganismManager) applyChemosynthesis(o *organism.Organism) {
	traits := o.TraitsRef()
	ph := m.api.GetPhAtPoint(o.Location)
	chemoTolerance := c.PhTolerance() * c.ChemosynthesisTolerance()
	if math.Abs(traits.IdealPh-ph) < chemoTolerance {
		// ChemoEfficiencyMult is the dominant chemo tradeoff: most
		// non-base features carry a small chemo penalty so adopting
		// new physiology costs the lineage some self-feeding rate.
		m.applyHealthChange(o, c.HealthChangeFromChemosynthesis()*o.Size*o.Tradeoffs().ChemoEfficiencyMult)
		o.Status = organism.StatusChemoSuccess
		// Successful chemo pushes local pH down, scaled by organism
		// size — bigger organisms acidify faster. Accumulate the
		// magnitude on the organism for the per-organism tinting.
		delta := c.ChemoPhEffectPerSize() * o.Size
		m.api.AddPhChangeAtPoint(o.Location, -delta)
		o.PhNegative += delta
	} else {
		m.applyHealthChange(o, c.HealthChangeFromFailedChemosynthesis()*o.Size)
		o.Status = organism.StatusChemoFailed
	}
}

func (m *OrganismManager) applyHealthChange(o *organism.Organism, amount float64) {
	prevSize := o.Size
	o.ApplyHealthChange(amount)
	if o.Size > prevSize {
		m.addUpdatedPoint(o.Location)
	}
}

func (m *OrganismManager) applyAttack(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromAttacking()*o.Size)
	o.Status = organism.StatusAttacking
}

func (m *OrganismManager) calculateAttackEffect(o *organism.Organism) float64 {
	// AttackDamageDealtMult is the attacker's passive tradeoff
	// (Fangs etc.). FlareDamageDealtMult is the per-cycle active
	// modifier — multiplied on top when the attacker chose ActFlare
	// this cycle. Defender's AttackDamageTakenMult and any active
	// HunkerDamageTakenMult are applied on the receive side in
	// applyCycleHealthChanges.
	mult := o.Tradeoffs().AttackDamageDealtMult
	if o.Status == organism.StatusFlaring {
		mult *= c.FlareDamageDealtMult()
	}
	return c.HealthChangeInflictedByAttack() * o.Size * mult
}

// calculateStingEffect returns the per-cell damage that a sting
// inflicts on each of its 4 adjacent targets. Uses its own absolute
// damage value (HealthChangeInflictedBySting) — typically smaller
// than a single attack so the niche is in volume, not per-target
// punch. Same attacker-side AttackDamageDealtMult applies; the
// defender-side mult applies on the receive side. Flare boosts
// sting damage symmetrically with single-target attack.
func (m *OrganismManager) calculateStingEffect(o *organism.Organism) float64 {
	mult := o.Tradeoffs().AttackDamageDealtMult
	if o.Status == organism.StatusFlaring {
		mult *= c.FlareDamageDealtMult()
	}
	return c.HealthChangeInflictedBySting() * o.Size * mult
}

// sizeStrengthDelta returns the amount of wall strength a digger or
// burrower of the given Size adds or removes per action, looked up
// from the WallStrengthDelta* config knobs. Bracket cutoffs match
// the renderer's thirds-of-MaximumMaxSize bins so the size class an
// organism reads as visually maps 1:1 to the strength delta it does.
func sizeStrengthDelta(size float64) int {
	maxSize := c.MaximumMaxSize()
	switch {
	case size < maxSize*(1.0/3.0):
		return c.WallStrengthDeltaSmall()
	case size < maxSize*(2.0/3.0):
		return c.WallStrengthDeltaMedium()
	default:
		return c.WallStrengthDeltaLarge()
	}
}

// applyDig resolves ActDig: pays the attack-equivalent cost, then
// removes either the wall or the food in front of the organism.
// Wall: strength -= size delta (clamped at 0; the WallManager removes
// the entry when it hits 0). Food: deleted outright — the digger
// doesn't eat, just clears terrain. Empty cell in front: cost paid,
// no effect.
func (m *OrganismManager) applyDig(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromDigging()*o.Size)
	o.Status = organism.StatusDigging

	target := o.Location.Add(o.Direction)
	delta := sizeStrengthDelta(o.Size)
	if m.api.IsWallAtPoint(target) {
		m.api.AddWallStrength(target, -delta)
		m.api.AddWallUpdate(target)
		return
	}
	// Fallback: if no wall, dig will scoop out food at that cell.
	if item, ok := m.api.GetFoodAtPoint(target); ok && item != nil {
		m.api.RemoveFoodAtPoint(target, item.Value)
	}
}

// applyBurrow resolves ActBurrow: pays the attack-equivalent cost,
// then independently tries to add wall strength to the cells on the
// organism's left and right (relative to its facing). Each side:
//   - organism present  → skip (no wall placed, no food destroyed)
//   - food present      → food deleted, wall placed
//   - wall present      → strength += delta (clamped at MaxWallStrength)
//   - empty cell        → new wall created at delta strength
func (m *OrganismManager) applyBurrow(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromBurrowing()*o.Size)
	o.Status = organism.StatusBurrowing

	delta := sizeStrengthDelta(o.Size)
	for _, side := range []utils.Point{
		o.Location.Add(o.Direction.Left()),
		o.Location.Add(o.Direction.Right()),
	} {
		if m.isOrganismAtLocation(side) {
			continue
		}
		if item, ok := m.api.GetFoodAtPoint(side); ok && item != nil {
			m.api.RemoveFoodAtPoint(side, item.Value)
		}
		m.api.AddWallStrength(side, delta)
		m.api.AddWallUpdate(side)
	}
}

// applySting pays the size-scaled health cost of stinging — damage
// to neighbours is delivered through the request manager (added in
// updateRequestMap during decide) and applied on the defender's
// applyCycleHealthChanges pass.
func (m *OrganismManager) applySting(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromStinging()*o.Size)
	o.Status = organism.StatusStinging
}

// applyHunker raises the organism's defensive posture for this
// cycle: pays the cost and sets Status = StatusHunkering, which
// applyCycleHealthChanges consults to scale incoming damage by an
// additional HunkerDamageTakenMult.
func (m *OrganismManager) applyHunker(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromHunkering()*o.Size)
	o.Status = organism.StatusHunkering
}

// applyFlare extends spikes for this cycle: pays the cost and sets
// Status = StatusFlaring, which calculateAttackEffect consults to
// boost outgoing damage and isBiggerOrganismAtPoint consults to add
// to the organism's perceived size.
func (m *OrganismManager) applyFlare(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromFlaring()*o.Size)
	o.Status = organism.StatusFlaring
}

// applyHide goes invisible for this cycle: pays the cost and sets
// Status = StatusHiding. Other organisms' sensor-conditioned
// CheckOrganismAtPoint lookups treat the hidden cell as empty, but
// the manager's grid integrity is unaffected — a blind attack still
// lands.
func (m *OrganismManager) applyHide(o *organism.Organism) {
	m.addUpdatedPoint(o.Location)
	m.applyHealthChange(o, c.HealthChangeFromHiding()*o.Size)
	o.Status = organism.StatusHiding
}

// deathHealthEpsilon is the smallest Health value that still counts as
// alive. Below this the organism is dead. The threshold matches the
// panel's %5.2f display rounding cutoff, so anything that renders as
// "0.00" is killed off — without it, Size-scaled eat / chemo costs can
// converge asymptotically toward 0 in float64 and leave organisms
// parked in a twilight "dead but not dying" state for thousands of
// cycles.
const deathHealthEpsilon = 0.005

// markDyingIfDead transitions an organism into the dying state when
// its health drops below the alive threshold. The organism stays on
// the grid for one more cycle (rendered as AnimDie via Status); the
// descendant tree node is marked dead immediately so lineage stats
// reflect death-time accurately rather than one-cycle-later. Food
// drop and grid removal are deferred to finalizeDeaths.
//
// Returns true when the organism is in (or just entered) the dying
// state — i.e. "do not run further apply logic for it". Returns
// false when the organism is healthy.
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

// finalizeDeaths removes the dying organisms at the start of every
// cycle: capture their (location, size) for the food drop, then
// clear the grid cell, delete from the organism map, and add food.
// Runs before sort/UpdateStats so removed organisms don't appear in
// the cycle's iteration order. The death animation has already
// played during the prior cycle transition, and the death sprite is
// designed to morph cleanly into the food sprite.
func (m *OrganismManager) finalizeDeaths() {
	type drop struct {
		id   int
		loc  utils.Point
		size int
	}
	var drops []drop
	for _, o := range m.organisms {
		if o.Status != organism.StatusDying {
			continue
		}
		drops = append(drops, drop{id: o.ID, loc: o.Location, size: int(o.Size)})
	}
	if len(drops) == 0 {
		return
	}
	m.gridMutex.Lock()
	m.organismMutex.Lock()
	for _, dr := range drops {
		m.organismIDGrid[dr.loc.X][dr.loc.Y] = -1
		delete(m.organisms, dr.id)
	}
	m.gridMutex.Unlock()
	m.organismMutex.Unlock()
	// AddFoodAtPoint takes the food-manager lock; do it outside
	// the grid/organism critical section.
	for _, dr := range drops {
		m.api.AddFoodAtPoint(dr.loc, dr.size)
		m.addUpdatedPoint(dr.loc)
	}
}

func (m *OrganismManager) applySpawn(o *organism.Organism) {
	if success := m.SpawnChildOrganism(o); success {
		m.applyHealthChange(o, o.HealthCostToReproduce())
		o.Children++
	}
	o.Status = organism.StatusSpawning
	// SpawnChildOrganism can still fail here despite the decide-phase
	// "has empty neighbour" check (e.g. a higher-ID organism with the
	// same target won the position-request priority), but the
	// trapped-organism case the user reported is handled at decide
	// time now: organisms without an empty neighbour resolve through
	// their tree-picked action with real side effects, not ActSpawn.
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
	if amountToEat > 0 {
		o.Status = organism.StatusEatSuccess
		// Successful eating pushes local pH up, scaled by amount eaten.
		// Accumulate the magnitude on the organism for the per-organism
		// tinting.
		delta := c.EatingPhEffectPerFood() * amountToEat
		m.api.AddPhChangeAtPoint(target, delta)
		o.PhPositive += delta
	} else {
		o.Status = organism.StatusEatFailed
	}
}

func (m *OrganismManager) calculateValueToEat(o *organism.Organism, target utils.Point) float64 {
	if item, found := m.api.GetFoodAtPoint(target); found {
		maxCanEat := o.Size
		return math.Min(float64(item.Value), maxCanEat)
	}
	return 0
}

func (m *OrganismManager) applyMove(o *organism.Organism) {
	// MoveCostMult: Cilia makes movement cheaper, Shell/Spikes make
	// it more expensive (dragging mass through the world).
	m.applyHealthChange(o, c.HealthChangeFromMoving()*o.Size*o.Tradeoffs().MoveCostMult)

	targetPoint := o.Location.Add(o.Direction)
	if m.isMatchingPositionRequest(targetPoint, o.ID) == false {
		o.Status = organism.StatusMoveBlocked
		return
	}

	m.addUpdatedPoint(o.Location)
	m.addUpdatedPoint(targetPoint)

	o.TraveledDist++

	m.gridMutex.Lock()
	m.organismIDGrid[o.Location.X][o.Location.Y] = -1
	m.organismIDGrid[targetPoint.X][targetPoint.Y] = o.ID
	m.gridMutex.Unlock()

	o.Location = targetPoint
	o.Status = organism.StatusMoveSuccess
}

func (m *OrganismManager) applyRightTurn(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromTurning()*o.Size)

	o.Direction = o.Direction.Right()
	o.Status = organism.StatusTurnRight
}

func (m *OrganismManager) applyLeftTurn(o *organism.Organism) {
	m.applyHealthChange(o, c.HealthChangeFromTurning()*o.Size)

	o.Direction = o.Direction.Left()
	o.Status = organism.StatusTurnLeft
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
