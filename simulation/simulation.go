package simulation

import (
	"encoding/json"
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
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	options *config.Options
	rng     *simrand.RNG

	cycle    int
	isPaused bool

	// minOrganismsArmed is set the first cycle the living population reaches twice config.MinOrganisms.
	minOrganismsArmed bool
	// endReported keeps IsDone from printing its end message on every call; callers poll it in loop conditions.
	endReported bool

	selectedID int

	organismManager    *manager.OrganismManager
	foodManager        *manager.FoodManager
	environmentManager *manager.EnvironmentManager
	wallManager        *manager.WallManager
	updateManager      *manager.UpdateManager

	// Checkpoint recording (nil if not recording)
	recorder *checkpoint.Writer

	UpdateTime, EnvironmentUpdateTime, FoodUpdateTime, OrganismUpdateTime time.Duration
	OrganismUpdateLoopTime, OrganismResolveLoopTime                       time.Duration

	accEnv, accFood, accSort, accHistory, accCheckpoint time.Duration
	accDecideStats, accDecideTree, accDecideRequest     time.Duration
	accResolveHealth, accResolveAction, accResolveDead  time.Duration
	accResolveSpawn                                     time.Duration
	accTotal                                            time.Duration
	accCycles, accSpawnCount                            int
}

// NewSimulation returns a simulation with generated world and organisms cycle increments at the beginning of Update() so start at -1 to ensure first actions are attributed to cycle 0. Seed precedence.
func NewSimulation(options *config.Options) *Simulation {
	seed := options.Seed
	if seed == 0 {
		seed = config.Seed()
	}
	if seed == 0 {
		// Both unspecified — pick a wall-clock-based seed so wasm builds (and any "leave it blank" CLI use) get fresh runs instead of reproducing the same simulation every launch.
		seed = int(time.Now().UnixNano())
	}
	rng := simrand.New(uint64(seed))
	sim := &Simulation{
		options:  options,
		rng:      rng,
		cycle:    -1,
		isPaused: false,
	}
	sim.updateManager = manager.NewUpdateManager()
	sim.wallManager = manager.NewWallManager(rng)
	sim.environmentManager = manager.NewEnvironmentManager(sim)
	sim.foodManager = manager.NewFoodManager(sim, rng)
	sim.organismManager = manager.NewOrganismManager(sim, rng)

	header := checkpoint.FileHeader{
		Seed:               uint64(seed),
		CheckpointInterval: options.CheckpointInterval,
		GridUnitsWide:      config.GridUnitsWide(),
		GridUnitsHigh:      config.GridUnitsHigh(),
		Config:             recordedConfig(seed),
	}
	w, err := checkpoint.NewWriter(options.CheckpointFile, header)
	if err != nil {
		fmt.Printf("\nWarning: failed to create checkpoint file: %v", err)
	} else {
		sim.recorder = w
	}

	return sim
}

func (s *Simulation) Update() {
	if s.isPaused {
		return
	}

	s.cycle++
	start := time.Now()

	s.updateEnvironment()
	s.updateFood()
	s.updateOrganisms()
	s.updateMinOrganismsArmed()

	s.UpdateTime = time.Since(start)

	checkpointStart := time.Now()
	if s.recorder != nil && s.cycle%s.options.CheckpointInterval == 0 {
		s.writeSnapshot()
	}
	checkpointTime := time.Since(checkpointStart)

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

// CaptureSnapshot deep-copies the current simulation state into a SnapshotPayload.
func (s *Simulation) CaptureSnapshot() *checkpoint.SnapshotPayload {
	rngState, err := s.rng.MarshalState()
	if err != nil {
		fmt.Printf("\nWarning: failed to marshal RNG state: %v", err)
		return nil
	}
	currentPh, previousPh := s.environmentManager.CapturePhMaps()
	return &checkpoint.SnapshotPayload{
		Cycle:                 s.cycle,
		RNGState:              rngState,
		TotalOrganismsCreated: s.organismManager.TotalOrganismsCreated(),
		MinOrganismsArmed:     s.minOrganismsArmed,
		Organisms:             s.organismManager.CaptureOrganismRecords(),
		OrganismGrid:          s.organismManager.CaptureOrganismGrid(),
		CurrentPhMap:          currentPh,
		PreviousPhMap:         previousPh,
		FoodItems:             s.foodManager.CaptureFoodRecords(),
		BuriedFood:            s.foodManager.CaptureBuriedFoodRecords(),
		Walls:                 captureWallRecords(s.wallManager),
		Ancestors:             s.organismManager.CaptureAncestors(),
	}
}

func (s *Simulation) writeSnapshot() {
	snap := s.CaptureSnapshot()
	if snap == nil {
		return
	}
	if err := s.recorder.WriteSnapshot(snap); err != nil {
		fmt.Printf("\nWarning: failed to write snapshot at cycle %d: %v", s.cycle, err)
	}
}

// InstallDescendantTrees points the organism manager at already-decoded trees, keeping node pointers stable across a seek.
func (s *Simulation) InstallDescendantTrees(d *manager.DescendantTrees) {
	s.organismManager.InstallDescendantTrees(d)
}

func (s *Simulation) TreesGeneration() int {
	return s.organismManager.TreesGeneration()
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
const (
	// Re-measured with SIZE_PROBE=1 after max_decision_tree_size doubled:
	// a descendant node carries its serialized tree, so a bigger tree limit
	// is a bigger node. 27.2-33.7 observed; 30 with the fixed overhead below
	// clears every sample, and it rounds UP on purpose because this drives a
	// cap — over-estimating ends a run slightly early, under-estimating
	// overruns the limit the user set.
	replayBytesPerNode  = 30
	replayCloseOverhead = 8 << 10
)

// EstimatedReplayBytes is how big the replay file is projected to be if the run ended now.
func (s *Simulation) EstimatedReplayBytes() int64 {
	if s.recorder == nil {
		return 0
	}
	nodes := int64(s.organismManager.TotalOrganismsCreated())
	return s.recorder.BytesWritten() + nodes*replayBytesPerNode + replayCloseOverhead
}

func (s *Simulation) CloseRecorder() {
	if s.recorder != nil {
		// Tag the end sections with the true final cycle so the replay viewer can advance past the last snapshot to wherever the simulation actually ended.
		treesPayload := s.organismManager.CaptureDescendantTrees()
		if err := s.recorder.WriteDescendantTrees(treesPayload, s.cycle); err != nil {
			fmt.Printf("\nWarning: failed to write descendant trees: %v", err)
		}
		histPayload := s.organismManager.CaptureHistory()
		if err := s.recorder.WriteHistory(histPayload, s.cycle); err != nil {
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
	s.foodManager.Update(s.Cycle())
	s.FoodUpdateTime = time.Since(start)
}

func (s *Simulation) updateOrganisms() {
	start := time.Now()
	s.organismManager.Update()
	s.OrganismUpdateTime = time.Since(start)
	s.OrganismUpdateLoopTime = s.organismManager.UpdateDuration
	s.OrganismResolveLoopTime = s.organismManager.ResolveDuration
}

// TimingSummary returns a formatted breakdown of average time per cycle over the accumulated window, then resets the accumulators.
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
			"    Parallel work: %8s\n"+
			"    Merge:         %8s\n"+
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
		avg(s.accDecideRequest),
		avg(accResolve),
		avg(s.accResolveHealth), avg(s.accResolveAction),
		avg(s.accResolveSpawn), spawnsPerCycle, avgSpawn,
		avg(s.accResolveDead),
		avg(s.accHistory), avg(s.accCheckpoint),
		avg(other), avg(s.accTotal))

	s.accEnv, s.accFood, s.accSort = 0, 0, 0
	s.accDecideStats, s.accDecideTree, s.accDecideRequest = 0, 0, 0
	s.accResolveHealth, s.accResolveAction, s.accResolveDead = 0, 0, 0
	s.accResolveSpawn = 0
	s.accHistory, s.accCheckpoint = 0, 0
	s.accTotal, s.accCycles, s.accSpawnCount = 0, 0, 0

	return summary
}

// updateMinOrganismsArmed arms the low-population end condition the first time the population reaches twice MinOrganisms.
func (s *Simulation) updateMinOrganismsArmed() {
	if s.minOrganismsArmed {
		return
	}
	if min := config.MinOrganisms(); min > 0 && s.GetNumOrganisms() >= 2*min {
		s.minOrganismsArmed = true
	}
}

// EndCondition is why a run stopped, or EndNone while it is still going.
type EndCondition int

const (
	// EndNone means the run is still going.
	EndNone EndCondition = iota
	EndExtinct
	// EndBelowMinimum: fewer than min_organisms are alive, after the population armed the condition by reaching twice that.
	EndBelowMinimum
	EndMaxCycles
	// EndMaxReplaySize: the replay file is projected to reach max_replay_size_mb.
	EndMaxReplaySize
)

// endCondition is the end-condition decision, kept free of simulation state so it can be tested directly.
func endCondition(alive, cycle int, armed bool, minOrganisms, maxCycles int,
	replayBytes, maxReplayBytes int64) EndCondition {
	if alive == 0 {
		return EndExtinct
	}
	if minOrganisms > 0 && armed && alive < minOrganisms {
		return EndBelowMinimum
	}
	if maxCycles > 0 && cycle >= maxCycles {
		return EndMaxCycles
	}
	if maxReplayBytes > 0 && replayBytes >= maxReplayBytes {
		return EndMaxReplaySize
	}
	return EndNone
}

// IsDone returns true if end condition met
func (s *Simulation) IsDone() bool {
	return s.EndCondition() != EndNone
}

func (s *Simulation) EndCondition() EndCondition {
	alive := s.GetNumOrganisms()
	end := endCondition(alive, s.cycle, s.minOrganismsArmed,
		config.MinOrganisms(), config.MaxCycles(),
		s.EstimatedReplayBytes(), int64(config.MaxReplaySizeMb())<<20)
	if end == EndNone {
		return EndNone
	}
	if !s.endReported {
		s.endReported = true
		if end == EndMaxReplaySize {
			fmt.Printf(maxReplayEndMessage, s.cycle, config.MaxReplaySizeMb())
		} else if end == EndMaxCycles {
			fmt.Printf(maxCyclesEndMessage, s.cycle, config.MaxCycles())
		} else if alive == 0 {
			fmt.Printf("\nSimulation ended on cycle %d: population went extinct.", s.cycle)
		} else {
			fmt.Printf("\nSimulation ended on cycle %d: %d organisms alive, below the minimum of %d.",
				s.cycle, alive, config.MinOrganisms())
		}
	}
	return end
}

// IsDebug returns true if debug flag set on run
func (s *Simulation) IsDebug() bool {
	return s.options.IsDebugging
}

// ToggleDebug returns true if debug flag set on run
func (s *Simulation) ToggleDebug() {
	s.options.IsDebugging = s.options.IsDebugging == false
}

func (s *Simulation) Cycle() int {
	return s.cycle
}

func (s *Simulation) IsPaused() bool {
	return s.isPaused
}

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

// AddWallUpdate registers that a point's wall strength was changed.
func (s *Simulation) AddWallUpdate(point utils.Point) {
	s.updateManager.AddWallUpdate(point)
}

// GetUpdatedWallPoints returns the set of wall cells changed since the last refresh.
func (s *Simulation) GetUpdatedWallPoints() map[utils.Point]bool {
	return s.updateManager.GetUpdatedWallPoints()
}

func (s *Simulation) GetUpdatedFoodPoints() map[utils.Point]bool {
	return s.updateManager.GetUpdatedFoodPoints()
}

func (s *Simulation) GetUpdatedOrganismPoints() map[utils.Point]bool {
	return s.updateManager.GetUpdatedOrganismPoints()
}

func (s *Simulation) GetUpdatedPhPoints() map[utils.Point]bool {
	return s.updateManager.GetUpdatedPhPoints()
}

func (s *Simulation) ClearUpdatedPoints() {
	s.updateManager.ClearMaps()
}

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

func (s *Simulation) GetOrganismTraitsByID(id int) (organism.Traits, bool) {
	return s.organismManager.GetOrganismTraitsByID(id)
}

func (s *Simulation) GetOldestId() int {
	return s.organismManager.GetOldestId()
}

func (s *Simulation) GetMostChildrenId() int {
	return s.organismManager.GetMostChildrenId()
}

func (s *Simulation) GetMostTraveledId() int {
	return s.organismManager.GetMostTraveledId()
}

func (s *Simulation) GetMostAggressiveId() int {
	return s.organismManager.GetMostAggressiveId()
}

// GetMostSuccessfulId returns the id of the oldest living organism whose descendant tree node meets the "most successful" criteria (AllBranchesDeadCycle == 0 or equal to the tree's max).
func (s *Simulation) GetMostSuccessfulId() int {
	return s.organismManager.GetMostSuccessfulId()
}

// GetMostSuccessfulIds returns the IDs of all currently-living organisms whose descendant tree node meets the "most successful" criteria.
func (s *Simulation) GetMostSuccessfulIds() []int {
	return s.organismManager.GetMostSuccessfulIds()
}

// IsMostSuccessful reports whether the given organism ID is on the most-successful lineage (alive or dead).
func (s *Simulation) IsMostSuccessful(id int) bool {
	return s.organismManager.IsMostSuccessful(id)
}

// GetOrganismDecisionTreeByID returns a copy of the currently-used decision tree of the given organism (nil if no organism found)
func (s *Simulation) GetOrganismDecisionTreeByID(id int) *d.Tree {
	return s.organismManager.GetOrganismDecisionTreeByID(id)
}

func (s *Simulation) GetHistory(histType manager.HistoryType) map[int]map[int]int32 {
	return s.organismManager.GetHistory(histType)
}

// GetAncestorColors returns a map of all ancestors with at least one descendant and the ancestor's color
func (s *Simulation) GetAncestorColors() map[int]color.Color {
	return s.organismManager.GetAncestorColors()
}

// LockHistoryForReading acquires a read lock on the history data.
func (s *Simulation) LockHistoryForReading() { s.organismManager.LockHistoryForReading() }

// UnlockHistoryForReading releases the read lock on the history data.
func (s *Simulation) UnlockHistoryForReading() { s.organismManager.UnlockHistoryForReading() }

func (s *Simulation) GetOrganismTreeNode(id int) *organism.DescendantNode {
	return s.organismManager.GetOrganismTreeNode(id)
}

func (s *Simulation) GetTreeNodeByID(id int) *organism.DescendantNode {
	return s.organismManager.GetTreeNodeByID(id)
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

func (s *Simulation) GetNumOrganisms() int {
	return s.organismManager.OrganismCount()
}

// GetDeadCount returns the total number of organisms that have died in the simulation.
func (s *Simulation) GetDeadCount() int {
	return s.organismManager.DeadCount()
}

func (s *Simulation) GetFoodItems() map[utils.Point]*food.Item {
	return s.foodManager.GetFoodItems()
}

// FoodCount returns the total number of food items currently on the grid.
func (s *Simulation) FoodCount() int {
	return s.foodManager.FoodCount()
}

// RecordedEndCycle returns the end of the recorded run being replayed, or 0 in a live run.
func (s *Simulation) RecordedEndCycle() int {
	return s.organismManager.RecordedEndCycle()
}

func (s *Simulation) WallCount() int {
	return s.wallManager.Count()
}

// CheckOrganismAtPoint returns the result of running a check against any Organism object found at a given Point.
func (s *Simulation) CheckOrganismAtPoint(point utils.Point, checkFunc organism.OrgCheck) bool {
	return s.organismManager.CheckOrganismAtPoint(point, checkFunc)
}

// OrganismCount returns the current number of Organisms alive in the simulation
func (s *Simulation) OrganismCount() int {
	return s.organismManager.OrganismCount()
}

func (s *Simulation) AverageAbilityScores() [physiology.AbilityCount]float64 {
	return s.organismManager.AverageAbilityScores()
}

func (s *Simulation) AveragePh() float64 {
	return s.environmentManager.GetAveragePh()
}

func (s *Simulation) PhRange() (float64, float64) {
	return s.environmentManager.GetPhRange()
}

// GetFoodAtPoint returns the value of any food at a given point and whether a food item actually exists there.
func (s *Simulation) GetFoodAtPoint(point utils.Point) (*food.Item, bool) {
	return s.foodManager.GetFoodAtPoint(point)
}

// CheckFoodAtPoint returns the result of running a check against any food Item object found at a given Point.
func (s *Simulation) CheckFoodAtPoint(point utils.Point, checkFunc organism.FoodCheck) bool {
	item, found := s.foodManager.GetFoodAtPoint(point)
	return checkFunc(item, found)
}

// AddFoodAtPoint attempts to add a food value to a given point and returns the actual amount of food added.
func (s *Simulation) AddFoodAtPoint(point utils.Point, value int) {
	s.foodManager.AddFoodAtPoint(point, value)
}

// RemoveFoodAtPoint attempts to add a food value to a given point and returns the actual amount of food added.
func (s *Simulation) RemoveFoodAtPoint(point utils.Point, value int) {
	s.foodManager.RemoveFoodAtPoint(point, value)
}

func (s *Simulation) GetBuriedFoodAtPoint(point utils.Point) int {
	return s.foodManager.GetBuriedFoodAtPoint(point)
}

// UnburyFoodAtPoint moves buried food back into reach, returning how much moved.
func (s *Simulation) UnburyFoodAtPoint(point utils.Point, value int) int {
	return s.foodManager.Unbury(point, value)
}

func (s *Simulation) BuriedFoodCount() int {
	return s.foodManager.BuriedFoodCount()
}

// GetBuriedFood is the whole buried layer, for the renderer's full refresh pass.
func (s *Simulation) GetBuriedFood() map[utils.Point]int {
	return s.foodManager.GetBuriedFood()
}

// BuryAllFoodAtPoint pushes a whole pile under, for where a wall goes up.
func (s *Simulation) BuryAllFoodAtPoint(point utils.Point) int {
	return s.foodManager.BuryAllAt(point)
}

func (s *Simulation) Select(id int) {
	s.selectedID = id
}

func (s *Simulation) GetSelected() int {
	return s.selectedID
}

func (s *Simulation) GetPhMap() [][]float64 {
	return s.environmentManager.GetPhMap()
}

// GetWalls returns the current map of wall locations to strengths.
func (s *Simulation) GetWalls() map[utils.Point]int {
	return s.wallManager.GetWalls()
}

func (s *Simulation) IsWallAtPoint(p utils.Point) bool {
	return s.wallManager.IsWallAtPoint(p)
}

func (s *Simulation) IsOrganismAtPoint(p utils.Point) bool {
	if s.organismManager == nil {
		return false
	}
	return s.organismManager.IsOrganismAtPoint(p)
}

// GetWallStrengthAtPoint returns the wall's strength at p, or 0 if no wall is present.
func (s *Simulation) GetWallStrengthAtPoint(p utils.Point) int {
	return s.wallManager.GetWallStrengthAtPoint(p)
}

// AddWallStrength adjusts the wall at p by delta, clamped to [0, MaxWallStrength].
func (s *Simulation) AddWallStrength(p utils.Point, delta int) int {
	return s.wallManager.AddWallStrength(p, delta)
}

// captureWallRecords converts the WallManager's wall map into the snapshot-friendly slice of WallRecords.
func captureWallRecords(wm *manager.WallManager) []checkpoint.WallRecord {
	walls := wm.GetWalls()
	out := make([]checkpoint.WallRecord, 0, len(walls))
	for p, s := range walls {
		out = append(out, checkpoint.WallRecord{
			X:        uint16(p.X),
			Y:        uint16(p.Y),
			Strength: uint8(s),
		})
	}
	return out
}

func (s *Simulation) GetPhAtPoint(point utils.Point) float64 {
	return s.environmentManager.GetPhAtPoint(point)
}

// AddPhChangeAtPoint adds a given value to the environment's Ph at a given location
func (s *Simulation) AddPhChangeAtPoint(point utils.Point, change float64) {
	s.environmentManager.AddPhChangeAtPoint(point, change)
}

func recordedConfig(seed int) []byte {
	g := *config.GetCurrentGlobals()
	g.Seed = seed
	data, err := json.Marshal(g)
	if err != nil {
		return nil
	}
	return data
}

// maxCyclesEndMessage is kept out of the branch above so the edit that added it did not have to touch the two messages already there.
const maxCyclesEndMessage = "\nSimulation ended on cycle %d: reached the maximum of %d cycles."

const maxReplayEndMessage = "\nSimulation ended on cycle %d: replay file reached the %dMB limit."
