package replay

import (
	"encoding/json"
	"fmt"
	"io"
	"math"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/checkpoint"
	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/neural"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simulation"
)

type Controller struct {
	reader    *checkpoint.Reader
	sim       *simulation.Simulation
	options   *config.Options
	snapshots []checkpoint.SnapshotEntry

	trees          *manager.DescendantTrees
	historyPayload *checkpoint.HistoryPayload

	// Animation + timing state shared with the renderer.
	AnimState *animation.State

	// wasPaused tracks pause transitions so we can reset the animation clock on unpause and avoid a burst of catch-up cycles.
	wasPaused bool

	Speed      float64 // playback multiplier; see animation package for semantics
	FinalCycle int     // last cycle in the file (from last snapshot)

	// AutoSpeed, when true, lets the viewer drive Speed from the camera's current zoom level via UpdateSpeedFromZoom.
	AutoSpeed bool

	// SeekCount increments on every successful snapshot seek.
	SeekCount int

	ring *stateRing

	// recorded is the configuration the simulation ran with.
	recorded config.Globals
	closed   bool
}

// stateRingCapacity is how many recent post-Update sim states the in-memory ring buffer keeps.
const stateRingCapacity = 100

// NewController opens a .pzr file and restores from the first snapshot.
func NewController(path string, options *config.Options) (*Controller, error) {
	reader, err := checkpoint.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open replay file: %w", err)
	}

	if reader.SnapshotCount() == 0 {
		reader.Close()
		return nil, fmt.Errorf("replay file has no snapshots")
	}

	// Apply config from the checkpoint file header
	recorded := recordedGlobals(reader.Header)
	globals := recorded
	config.SetGlobals(&globals)

	snap, err := reader.ReadSnapshot(0)
	if err != nil {
		reader.Close()
		return nil, fmt.Errorf("failed to read first snapshot: %w", err)
	}

	sim, err := simulation.RestoreFromSnapshot(snap, options)
	if err != nil {
		reader.Close()
		return nil, fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	lastSnap := reader.SnapshotIndex[reader.SnapshotCount()-1]

	ctrl := &Controller{
		reader:     reader,
		sim:        sim,
		options:    options,
		snapshots:  reader.SnapshotIndex,
		AnimState:  animation.NewState(),
		Speed:      1,
		FinalCycle: lastSnap.Cycle,
		AutoSpeed:  true,
		ring:       newStateRing(stateRingCapacity),
		recorded:   recorded,
	}

	// Scan for descendant trees and history sections
	ctrl.loadEndSections()

	return ctrl, nil
}

func recordedGlobals(header checkpoint.FileHeader) config.Globals {
	current := *config.GetCurrentGlobals()
	g := current
	if len(header.Config) == 0 || json.Unmarshal(header.Config, &g) != nil {
		g = current
		g.Seed = int(header.Seed)
	}
	g.GridUnitsWide = header.GridUnitsWide
	g.GridUnitsHigh = header.GridUnitsHigh
	g.ScreenWidth, g.ScreenHeight = current.ScreenWidth, current.ScreenHeight
	g.Theme, g.PhColorScheme = current.Theme, current.PhColorScheme
	g.Music = current.Music
	return g
}

// Globals returns a copy of the settings the replayed simulation ran with, safe for the caller to edit.
func (c *Controller) Path() string { return c.reader.Path() }

func (c *Controller) Globals() config.Globals {
	g := c.recorded
	g.InitialAbilityScores = append([]int(nil), g.InitialAbilityScores...)
	return g
}

func (c *Controller) Simulation() *simulation.Simulation {
	return c.sim
}

func (c *Controller) Cycle() int {
	return c.sim.Cycle()
}

func (c *Controller) Update() {
	paused := c.sim.IsPaused()
	if paused {
		c.wasPaused = true
		return
	}
	if c.wasPaused {
		// Just unpaused: re-anchor the clock so we don't fast-forward through cycles based on how long the pause lasted.
		c.AnimState.ResetClock()
		c.wasPaused = false
	}

	// Keep animation state in sync with the caller-owned Speed knob.
	c.AnimState.Speed = c.Speed

	for c.AnimState.ShouldAdvance() {
		if c.sim.Cycle() >= c.FinalCycle {
			c.sim.Pause(true)
			return
		}
		c.AnimState.BeforeUpdate(c.sim.GetAllOrganismInfo())
		c.sim.Update()
		c.AnimState.AfterUpdate(c.sim.GetAllOrganismInfo())
		c.pushRing()
	}
}

// pushRing captures the simulation's current post-Update state and stores it in the in-memory ring so a subsequent prev-cycle can restore it without going to disk.
func (c *Controller) pushRing() {
	if c.ring == nil {
		return
	}
	if snap := c.sim.CaptureSnapshot(); snap != nil {
		c.ring.Push(snap)
	}
}

// StepBackward rolls the playback back exactly one cycle.
//
// It restores the cycle BEFORE the target and plays that cycle, rather than
// restoring the target itself. A snapshot carries what the world IS, not what
// each brain was computing when it chose its action: a net's activations are
// display state filled in by Choose, so a state restored without playing a
// cycle leaves the panel's SIGNAL view with nothing flowing through the net
// and the health ledger with no cycle to report, until some later cycle fills
// them in. The replayed cycle is the one that was recorded, since the restore
// carries the rng with it.
func (c *Controller) StepBackward() bool {
	target := c.sim.Cycle() - 1
	if target < 0 {
		return false
	}
	if target > 0 {
		if snap, ok := c.ringLookup(target - 1); ok && c.restoreRingState(snap) {
			c.replayOneCycle()
			return true
		}
	}
	if snap, ok := c.ringLookup(target); ok && c.restoreRingState(snap) {
		return true
	}
	if err := c.SeekToCycle(target); err != nil {
		return false
	}
	return true
}

func (c *Controller) ringLookup(cycle int) (*checkpoint.SnapshotPayload, bool) {
	if c.ring == nil {
		return nil, false
	}
	return c.ring.Lookup(cycle)
}

// restoreRingState puts the simulation back to one of the ring's states and
// reinstalls the sections a restore drops.
func (c *Controller) restoreRingState(snap *checkpoint.SnapshotPayload) bool {
	if err := c.sim.ResetFromSnapshot(snap); err != nil {
		return false
	}
	if c.trees != nil {
		c.sim.InstallDescendantTrees(c.trees)
	}
	if c.historyPayload != nil {
		c.sim.RestoreHistory(c.historyPayload)
	}
	c.AnimState.Frames = map[int]animation.Frame{}
	c.AnimState.ResetClock()
	c.SeekCount++
	return true
}

// replayOneCycle plays the cycle the restored state stands in front of. No
// animation batch is captured: a step backward is not a transition to animate.
func (c *Controller) replayOneCycle() {
	wasPaused := c.sim.IsPaused()
	c.sim.Pause(false)
	c.sim.Update()
	c.sim.Pause(wasPaused)
	c.pushRing()
}

func (c *Controller) SeekToSnapshot(index int) error {
	snap, err := c.reader.ReadSnapshot(index)
	if err != nil {
		return fmt.Errorf("failed to read snapshot %d: %w", index, err)
	}

	if err := c.sim.ResetFromSnapshot(snap); err != nil {
		return fmt.Errorf("failed to restore snapshot %d: %w", index, err)
	}

	// Re-inject cached end-of-sim data into the new organism manager
	if c.trees != nil {
		c.sim.InstallDescendantTrees(c.trees)
	}
	if c.historyPayload != nil {
		c.sim.RestoreHistory(c.historyPayload)
	}
	c.AnimState.Frames = map[int]animation.Frame{}
	c.AnimState.ResetClock()
	if c.ring != nil {
		c.ring.Clear()
	}
	c.SeekCount++
	return nil
}

// SeekToCycle finds the nearest snapshot before the target cycle, restores from it, then simulates forward to the target.
func (c *Controller) SeekToCycle(target int) error {
	// Find the nearest snapshot at or before the target
	bestIdx := 0
	for i, entry := range c.snapshots {
		if entry.Cycle <= target {
			bestIdx = i
		} else {
			break
		}
	}
	// A snapshot landing exactly on the target would play no cycles at all,
	// which leaves every per-cycle reading empty — see StepBackward. Starting
	// one snapshot earlier costs an interval of forward play and is the only
	// way to arrive with the target cycle actually played.
	if bestIdx > 0 && c.snapshots[bestIdx].Cycle == target {
		bestIdx--
	}

	if err := c.SeekToSnapshot(bestIdx); err != nil {
		return err
	}

	// Temporarily unpause to simulate forward, since Update() checks isPaused
	wasPaused := c.sim.IsPaused()
	c.sim.Pause(false)
	for c.sim.Cycle() < target {
		c.sim.Update()
	}
	c.sim.Pause(wasPaused)
	return nil
}

// StepForward advances the simulation by one cycle regardless of pause state, capturing the animation batch so the renderer can animate the transition.
func (c *Controller) StepForward() {
	if c.sim.Cycle() >= c.FinalCycle {
		return
	}
	wasPaused := c.sim.IsPaused()
	c.sim.Pause(false)
	c.AnimState.BeforeUpdate(c.sim.GetAllOrganismInfo())
	c.sim.Update()
	c.AnimState.AfterUpdate(c.sim.GetAllOrganismInfo())
	c.pushRing()
	c.sim.Pause(wasPaused)
}

// SetSpeed sets the playback speed multiplier and disables AutoSpeed.
func (c *Controller) SetSpeed(speed float64) {
	c.AutoSpeed = false
	c.setSpeedInternal(speed)
}

func (c *Controller) setSpeedInternal(speed float64) {
	if speed < MinReplaySpeed {
		speed = MinReplaySpeed
	}
	if speed > MaxReplaySpeed {
		speed = MaxReplaySpeed
	}
	c.Speed = SnapToPowerOfTwo(speed)
	c.AnimState.Speed = c.Speed
}

// SnapToPowerOfTwo rounds a speed to the nearest power of two, in the log domain so 3x lands on 2x or 4x by ratio.
func SnapToPowerOfTwo(speed float64) float64 {
	if speed <= 0 {
		return MinReplaySpeed
	}
	return math.Pow(2, math.Round(math.Log2(speed)))
}

// MinReplaySpeed and MaxReplaySpeed bound the user-selectable playback speeds.
const (
	MinReplaySpeed = 0.25
	MaxReplaySpeed = 64
)

// SpeedForUnitSize returns the auto-speed for a given camera unit size.
func SpeedForUnitSize(unitSize int) float64 {
	switch {
	case unitSize >= 32:
		return 1
	case unitSize >= 16:
		return 2
	case unitSize >= 8:
		return 4
	default:
		return 8
	}
}

func (c *Controller) UpdateSpeedFromZoom(unitSize int) {
	if c.AutoSpeed {
		c.setSpeedInternal(SpeedForUnitSize(unitSize))
	}
}

// EnableAutoSpeed turns AutoSpeed on and immediately re-anchors Speed to the given unit size.
func (c *Controller) EnableAutoSpeed(unitSize int) {
	c.AutoSpeed = true
	c.setSpeedInternal(SpeedForUnitSize(unitSize))
}

// ReconstructionResult is the panel-relevant snapshot of an organism reconstructed from an earlier point in the recording.
type ReconstructionResult struct {
	Info        *organism.Info
	Traits      organism.Traits
	TraitsValid bool
	// Exactly one of DecisionTree and Net carries the organism's brain, the
	// same split the snapshot it came from holds.
	DecisionTree *d.Tree
	Net          *neural.Net
	Err          error
}

// ReconstructOrganismAt rebuilds simulation state at targetCycle from the nearest preceding snapshot, runs forward to that cycle.
func (c *Controller) ReconstructOrganismAt(targetCycle, orgID int) ReconstructionResult {
	// Find the nearest snapshot at or before targetCycle.
	bestIdx := -1
	for i, entry := range c.snapshots {
		if entry.Cycle <= targetCycle {
			bestIdx = i
		} else {
			break
		}
	}
	if bestIdx < 0 {
		return ReconstructionResult{Err: fmt.Errorf("no snapshot at or before cycle %d", targetCycle)}
	}

	reader, err := checkpoint.OpenReader(c.reader.Path())
	if err != nil {
		return ReconstructionResult{Err: fmt.Errorf("open reader: %w", err)}
	}
	defer reader.Close()

	snap, err := reader.ReadSnapshot(bestIdx)
	if err != nil {
		return ReconstructionResult{Err: fmt.Errorf("read snapshot %d: %w", bestIdx, err)}
	}

	// RestoreFromSnapshot builds a Simulation with no recorder, so forward-play here doesn't write to the .pzr file.
	sim, err := simulation.RestoreFromSnapshot(snap, c.options)
	if err != nil {
		return ReconstructionResult{Err: fmt.Errorf("restore snapshot: %w", err)}
	}

	sim.Pause(false)
	for sim.Cycle() < targetCycle {
		sim.Update()
	}

	res := ReconstructionResult{
		Info:         sim.GetOrganismInfoByID(orgID),
		DecisionTree: sim.GetOrganismDecisionTreeByID(orgID),
		Net:          sim.GetOrganismNeuralNetByID(orgID),
	}
	res.Traits, res.TraitsValid = sim.GetOrganismTraitsByID(orgID)
	return res
}

func (c *Controller) SnapshotCycles() []int {
	cycles := make([]int, len(c.snapshots))
	for i, s := range c.snapshots {
		cycles[i] = s.Cycle
	}
	return cycles
}

func (c *Controller) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.reader.Close()
}

// loadEndSections scans the file for the descendant trees and history sections (written at the end of the simulation), caches them.
func (c *Controller) loadEndSections() {
	freshReader, err := checkpoint.OpenReader(c.reader.Path())
	if err != nil {
		return
	}
	defer freshReader.Close()

	freshReader.SeekAfterHeader()
	for {
		sType, cycle, payload, err := freshReader.ReadNextSection()
		if err != nil {
			break
		}
		switch sType {
		case checkpoint.SectionDescendantTrees:
			if trees, ok := payload.(*checkpoint.DescendantTreesPayload); ok {
				c.trees = manager.BuildDescendantTrees(trees)
				c.sim.InstallDescendantTrees(c.trees)
			}
		case checkpoint.SectionHistory:
			if hist, ok := payload.(*checkpoint.HistoryPayload); ok {
				c.historyPayload = hist
				c.sim.RestoreHistory(hist)
			}
		}
		// End sections (descendant trees / history) are tagged with the simulation's true final cycle.
		if cycle > c.FinalCycle {
			c.FinalCycle = cycle
		}
	}
}

// ReadAllSections reads through the file to find the true final cycle (scanning delta sections past the last snapshot).
func (c *Controller) ReadAllSections() {
	c.reader.SeekAfterHeader()
	for {
		sType, cycle, _, err := c.reader.ReadNextSection()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		_ = sType
		if cycle > c.FinalCycle {
			c.FinalCycle = cycle
		}
	}
}
