package simulation

import (
	"fmt"
	d "github.com/Zebbeni/protozoa/decision"
	"image/color"
	"sort"
	"time"

	"github.com/Zebbeni/protozoa/checkpoint"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	options *config.Options
	rng     *simrand.RNG

	cycle    int
	isPaused bool

	selectedID int

	organismManager    *manager.OrganismManager
	foodManager        *manager.FoodManager
	environmentManager *manager.EnvironmentManager
	updateManager      *manager.UpdateManager

	// Checkpoint recording (nil if not recording)
	recorder *checkpoint.Writer

	// Per-cycle timing (latest)
	UpdateTime, EnvironmentUpdateTime, FoodUpdateTime, OrganismUpdateTime time.Duration
	OrganismUpdateLoopTime, OrganismResolveLoopTime                       time.Duration

	// Accumulated timing for periodic summaries
	accEnv, accFood, accSort, accHistory, accCheckpoint time.Duration
	accDecideStats, accDecideTree, accDecideRequest     time.Duration
	accResolveHealth, accResolveAction, accResolveDead  time.Duration
	accResolveSpawn                                     time.Duration
	accTotal                                            time.Duration
	accCycles, accSpawnCount                            int
}

// NewSimulation returns a simulation with generated world and organisms
// cycle increments at the beginning of Update() so start at -1 to ensure
// first actions are attributed to cycle 0
func NewSimulation(options *config.Options) *Simulation {
	rng := simrand.New(uint64(options.Seed))
	sim := &Simulation{
		options:  options,
		rng:      rng,
		cycle:    -1,
		isPaused: false,
	}
	sim.updateManager = manager.NewUpdateManager()
	sim.environmentManager = manager.NewEnvironmentManager(sim)
	sim.foodManager = manager.NewFoodManager(sim, rng)
	sim.organismManager = manager.NewOrganismManager(sim, rng)

	header := checkpoint.FileHeader{
		Seed:               uint64(options.Seed),
		CheckpointInterval: options.CheckpointInterval,
		GridUnitsWide:      config.GridUnitsWide(),
		GridUnitsHigh:      config.GridUnitsHigh(),
	}
	w, err := checkpoint.NewWriter(options.CheckpointFile, header)
	if err != nil {
		fmt.Printf("\nWarning: failed to create checkpoint file: %v", err)
	} else {
		sim.recorder = w
	}

	return sim
}

// Update calls Update functions for controllers in simulation
func (s *Simulation) Update() {
	if s.isPaused {
		return
	}

	s.cycle++
	start := time.Now()

	s.updateEnvironment()
	s.updateFood()
	s.updateOrganisms()

	s.UpdateTime = time.Since(start)

	// Write checkpoint snapshot at configured intervals
	checkpointStart := time.Now()
	if s.recorder != nil && s.cycle%s.options.CheckpointInterval == 0 {
		s.writeSnapshot()
	}
	checkpointTime := time.Since(checkpointStart)

	// Accumulate timing for periodic summaries
	om := s.organismManager
	s.accEnv += s.EnvironmentUpdateTime
	s.accFood += s.FoodUpdateTime
	s.accSort += om.SortDuration
	s.accDecideStats += om.DecideStatsDuration
	s.accDecideTree += om.DecideTreeDuration
	s.accDecideRequest += om.DecideRequestDuration
	s.accResolveHealth += om.ResolveHealthDuration
	s.accResolveAction += om.ResolveActionDuration
	s.accResolveSpawn += om.ResolveSpawnDuration
	s.accResolveDead += om.ResolveDeadDuration
	s.accHistory += om.HistoryDuration
	s.accCheckpoint += checkpointTime
	s.accTotal += s.UpdateTime + checkpointTime
	s.accSpawnCount += om.SpawnCount
	s.accCycles++
}

func (s *Simulation) writeSnapshot() {
	rngState, err := s.rng.MarshalState()
	if err != nil {
		fmt.Printf("\nWarning: failed to marshal RNG state: %v", err)
		return
	}

	currentPh, previousPh := s.environmentManager.CapturePhMaps()

	snap := &checkpoint.SnapshotPayload{
		Cycle:                 s.cycle,
		RNGState:              rngState,
		TotalOrganismsCreated: s.organismManager.TotalOrganismsCreated(),
		Organisms:             s.organismManager.CaptureOrganismRecords(),
		OrganismGrid:          s.organismManager.CaptureOrganismGrid(),
		CurrentPhMap:          currentPh,
		PreviousPhMap:         previousPh,
		FoodItems:             s.foodManager.CaptureFoodRecords(),
		Ancestors:             s.organismManager.CaptureAncestors(),
	}

	if err := s.recorder.WriteSnapshot(snap); err != nil {
		fmt.Printf("\nWarning: failed to write snapshot at cycle %d: %v", s.cycle, err)
	}
}

// RestoreDescendantTrees injects pre-built descendant trees into the organism manager.
func (s *Simulation) RestoreDescendantTrees(payload *checkpoint.DescendantTreesPayload) {
	s.organismManager.RestoreDescendantTrees(payload)
}

// RestoreHistory injects pre-built pH history into the organism manager.
func (s *Simulation) RestoreHistory(payload *checkpoint.HistoryPayload) {
	s.organismManager.RestoreHistory(payload)
}

// CloseRecorder writes the descendant trees and finalizes the checkpoint file.
func (s *Simulation) CloseRecorder() {
	if s.recorder != nil {
		// Write the full descendant trees as a final section
		treesPayload := s.organismManager.CaptureDescendantTrees()
		if err := s.recorder.WriteDescendantTrees(treesPayload); err != nil {
			fmt.Printf("\nWarning: failed to write descendant trees: %v", err)
		}
		// Write the full pH history as a final section
		histPayload := s.organismManager.CaptureHistory()
		if err := s.recorder.WriteHistory(histPayload); err != nil {
			fmt.Printf("\nWarning: failed to write history: %v", err)
		}
		if err := s.recorder.Close(); err != nil {
			fmt.Printf("\nWarning: failed to close checkpoint file: %v", err)
		}
		s.recorder = nil
	}
}

func (s *Simulation) updateEnvironment() {
	start := time.Now()
	s.environmentManager.Update()
	s.EnvironmentUpdateTime = time.Since(start)
}

func (s *Simulation) updateFood() {
	start := time.Now()
	s.foodManager.Update()
	s.FoodUpdateTime = time.Since(start)
}

func (s *Simulation) updateOrganisms() {
	start := time.Now()
	s.organismManager.Update()
	s.OrganismUpdateTime = time.Since(start)
	s.OrganismUpdateLoopTime = s.organismManager.UpdateDuration
	s.OrganismResolveLoopTime = s.organismManager.ResolveDuration
}

// TimingSummary returns a formatted breakdown of average time per cycle
// over the accumulated window, then resets the accumulators.
func (s *Simulation) TimingSummary() string {
	if s.accCycles == 0 {
		return ""
	}
	n := s.accCycles
	avg := func(d time.Duration) time.Duration { return d / time.Duration(n) }

	accDecide := s.accSort + s.accDecideStats + s.accDecideTree + s.accDecideRequest
	accResolve := s.accResolveHealth + s.accResolveAction + s.accResolveSpawn + s.accResolveDead
	accounted := s.accEnv + s.accFood + accDecide + accResolve + s.accHistory + s.accCheckpoint
	other := s.accTotal - accounted
	if other < 0 {
		other = 0
	}

	spawnsPerCycle := float64(s.accSpawnCount) / float64(n)
	avgSpawn := time.Duration(0)
	if s.accSpawnCount > 0 {
		avgSpawn = s.accResolveSpawn / time.Duration(s.accSpawnCount)
	}

	summary := fmt.Sprintf(
		"Avg/cycle over %d cycles (total %s), %d organisms:\n"+
			"  Environment:     %8s\n"+
			"  Decide phase:    %8s\n"+
			"    Sort IDs:      %8s\n"+
			"    UpdateStats:   %8s\n"+
			"    Tree eval:     %8s\n"+
			"    Request map:   %8s\n"+
			"  Resolve phase:   %8s\n"+
			"    Health calc:   %8s\n"+
			"    Actions:       %8s\n"+
			"    Spawn:         %8s  (%.1f/cycle, %s each)\n"+
			"    Dead removal:  %8s\n"+
			"  History:         %8s\n"+
			"  Checkpoint:      %8s\n"+
			"  Other:           %8s\n"+
			"  TOTAL:           %8s",
		n, s.accTotal.Round(time.Millisecond), s.GetNumOrganisms(),
		avg(s.accEnv),
		avg(accDecide),
		avg(s.accSort), avg(s.accDecideStats),
		avg(s.accDecideTree), avg(s.accDecideRequest),
		avg(accResolve),
		avg(s.accResolveHealth), avg(s.accResolveAction),
		avg(s.accResolveSpawn), spawnsPerCycle, avgSpawn,
		avg(s.accResolveDead),
		avg(s.accHistory), avg(s.accCheckpoint),
		avg(other), avg(s.accTotal))

	// Reset accumulators
	s.accEnv, s.accFood, s.accSort = 0, 0, 0
	s.accDecideStats, s.accDecideTree, s.accDecideRequest = 0, 0, 0
	s.accResolveHealth, s.accResolveAction, s.accResolveDead = 0, 0, 0
	s.accResolveSpawn = 0
	s.accHistory, s.accCheckpoint = 0, 0
	s.accTotal, s.accCycles, s.accSpawnCount = 0, 0, 0

	return summary
}

// IsDone returns true if end condition met
func (s *Simulation) IsDone() bool {
	if s.GetNumOrganisms() == 0 {
		fmt.Printf("\nSimulation ended on cycle %d with %d organisms alive.", s.cycle, s.GetNumOrganisms())
		return true
	}
	return false
}

// IsDebug returns true if debug flag set on run
func (s *Simulation) IsDebug() bool {
	return s.options.IsDebugging
}

// ToggleDebug returns true if debug flag set on run
func (s *Simulation) ToggleDebug() {
	s.options.IsDebugging = s.options.IsDebugging == false
}

// Cycle returns the current simulation cycle number
func (s *Simulation) Cycle() int {
	return s.cycle
}

// IsPaused returns whether the simulation is currently stopped
func (s *Simulation) IsPaused() bool {
	return s.isPaused
}

// Pause sets isPaused to the given boolean value
func (s *Simulation) Pause(pause bool) {
	s.isPaused = pause
}

// AddOrganismUpdate registers that the Organism at a point has changed in a noteworthy way
func (s *Simulation) AddOrganismUpdate(point utils.Point) {
	s.updateManager.AddOrganismUpdate(point)
}

// AddPhUpdate registers that a point's ph was changed by a noteworthy amount
func (s *Simulation) AddPhUpdate(point utils.Point) {
	s.updateManager.AddPhUpdate(point)
}

// AddFoodUpdate registers that a point's food value was changed by a noteworthy amount
func (s *Simulation) AddFoodUpdate(point utils.Point) {
	s.updateManager.AddFoodUpdate(point)
}

// GetUpdatedFoodPoints returns a map of all points recently updated by the
// foodManager
func (s *Simulation) GetUpdatedFoodPoints() map[utils.Point]bool {
	return s.updateManager.GetUpdatedFoodPoints()
}

// GetUpdatedOrganismPoints returns a map of all points recently updated by the
// organismManager
func (s *Simulation) GetUpdatedOrganismPoints() map[utils.Point]bool {
	return s.updateManager.GetUpdatedOrganismPoints()
}

// GetUpdatedPhPoints returns a map of all points recently updated by the
// environmentManager
func (s *Simulation) GetUpdatedPhPoints() map[utils.Point]bool {
	return s.updateManager.GetUpdatedPhPoints()
}

// ClearUpdatedPoints clears all updated points for all content managers
func (s *Simulation) ClearUpdatedPoints() {
	s.updateManager.ClearMaps()
}

// GetAllOrganismInfo returns a map of Info on all living organisms
func (s *Simulation) GetAllOrganismInfo() map[int]*organism.Info {
	return s.organismManager.GetAllOrganismInfo()
}

// GetOrganismInfoAtPoint returns the Organism at a given point (nil if none found)
func (s *Simulation) GetOrganismInfoAtPoint(point utils.Point) *organism.Info {
	return s.organismManager.GetOrganismInfoAtPoint(point)
}

// GetOrganismInfoByID returns the Organism Info for a given ID (nil if none)
func (s *Simulation) GetOrganismInfoByID(id int) *organism.Info {
	return s.organismManager.GetOrganismInfoByID(id)
}

// GetOrganismTraitsByID returns the Organism Traits for a given ID
func (s *Simulation) GetOrganismTraitsByID(id int) (organism.Traits, bool) {
	return s.organismManager.GetOrganismTraitsByID(id)
}

// GetOldestId returns the id of the oldest living organism
func (s *Simulation) GetOldestId() int {
	return s.organismManager.GetOldestId()
}

// GetMostChildrenId returns the id of the most traveled organism
func (s *Simulation) GetMostChildrenId() int {
	return s.organismManager.GetMostChildrenId()
}

// GetMostTraveledId returns the id of the most traveled organism
func (s *Simulation) GetMostTraveledId() int {
	return s.organismManager.GetMostTraveledId()
}

// GetOrganismDecisionTreeByID returns a copy of the currently-used decision tree of the
// given organism (nil if no organism found)
func (s *Simulation) GetOrganismDecisionTreeByID(id int) *d.Tree {
	return s.organismManager.GetOrganismDecisionTreeByID(id)
}

// GetHistory returns a history map by type. Caller must hold history read lock.
func (s *Simulation) GetHistory(histType manager.HistoryType) map[int]map[int]int32 {
	return s.organismManager.GetHistory(histType)
}

// GetAncestorColors returns a map of all ancestors with at least one descendant
// and the ancestor's color
func (s *Simulation) GetAncestorColors() map[int]color.Color {
	return s.organismManager.GetAncestorColors()
}

// LockHistoryForReading acquires a read lock on the history data.
func (s *Simulation) LockHistoryForReading() { s.organismManager.LockHistoryForReading() }

// UnlockHistoryForReading releases the read lock on the history data.
func (s *Simulation) UnlockHistoryForReading() { s.organismManager.UnlockHistoryForReading() }

// GetOrganismTreeNode returns the descendant tree node for the given organism ID.
func (s *Simulation) GetOrganismTreeNode(id int) *organism.DescendantNode {
	return s.organismManager.GetOrganismTreeNode(id)
}

// GetDescendantTrees returns the root node of each ancestor's family tree
func (s *Simulation) GetDescendantTrees() map[int]*organism.DescendantNode {
	return s.organismManager.GetDescendantTrees()
}

// GetAncestorsSorted returns a list of all original ancestor IDs in order
func (s *Simulation) GetAncestorsSorted() []int {
	ancestors := s.organismManager.GetAncestors()
	sort.Ints(ancestors)
	return ancestors
}

// GetNumOrganisms returns the total number of all living organisms in the simulation.
func (s *Simulation) GetNumOrganisms() int {
	return s.organismManager.OrganismCount()
}

// GetDeadCount returns the total number of organisms that have died in the simulation.
func (s *Simulation) GetDeadCount() int {
	return s.organismManager.DeadCount()
}

// GetFoodItems returns a map of all food items in the grid
func (s *Simulation) GetFoodItems() map[utils.Point]*food.Item {
	return s.foodManager.GetFoodItems()
}

// CheckOrganismAtPoint returns the result of running a check against any
// Organism object found at a given Point.
func (s *Simulation) CheckOrganismAtPoint(point utils.Point, checkFunc organism.OrgCheck) bool {
	return s.organismManager.CheckOrganismAtPoint(point, checkFunc)
}

// OrganismCount returns the current number of Organisms alive in the simulation
func (s *Simulation) OrganismCount() int {
	return s.organismManager.OrganismCount()
}

func (s *Simulation) AveragePh() float64 {
	return s.environmentManager.GetAveragePh()
}

// GetFoodAtPoint returns the value of any food at a given point and whether
// a food item actually exists there.
func (s *Simulation) GetFoodAtPoint(point utils.Point) (*food.Item, bool) {
	return s.foodManager.GetFoodAtPoint(point)
}

// CheckFoodAtPoint returns the result of running a check against any food Item
// object found at a given Point.
func (s *Simulation) CheckFoodAtPoint(point utils.Point, checkFunc organism.FoodCheck) bool {
	item, found := s.foodManager.GetFoodAtPoint(point)
	return checkFunc(item, found)
}

// AddFoodAtPoint attempts to add a food value to a given point and returns the actual
// amount of food added.
func (s *Simulation) AddFoodAtPoint(point utils.Point, value int) {
	s.foodManager.AddFoodAtPoint(point, value)
}

// RemoveFoodAtPoint attempts to add a food value to a given point and returns the actual
// amount of food added.
func (s *Simulation) RemoveFoodAtPoint(point utils.Point, value int) {
	s.foodManager.RemoveFoodAtPoint(point, value)
}

// Select sets the currently selected organism ID. -1 if none selected
func (s *Simulation) Select(id int) {
	s.selectedID = id
}

// GetSelected returns the currently selected organism ID. -1 if none selected
func (s *Simulation) GetSelected() int {
	return s.selectedID
}

// GetPhMap returns the full 2D map of all pH values in the environment
func (s *Simulation) GetPhMap() [][]float64 {
	return s.environmentManager.GetPhMap()
}

// GetWalls returns all points in the environment that contain a wall
func (s *Simulation) GetWalls() []utils.Point {
	return s.environmentManager.GetWalls()
}

// GetPhAtPoint returns the current Ph of the environment at a given location
func (s *Simulation) GetPhAtPoint(point utils.Point) float64 {
	return s.environmentManager.GetPhAtPoint(point)
}

// AddPhChangeAtPoint adds a given value to the environment's Ph at a given location
func (s *Simulation) AddPhChangeAtPoint(point utils.Point, change float64) {
	s.environmentManager.AddPhChangeAtPoint(point, change)
}
