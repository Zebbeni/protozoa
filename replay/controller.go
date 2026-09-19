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
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simulation"
)

// Controller manages playback of a .pzr checkpoint file.
// It holds the checkpoint reader and a live simulation that can be
// advanced cycle-by-cycle or jumped to any snapshot.
type Controller struct {
	reader    *checkpoint.Reader
	sim       *simulation.Simulation
	options   *config.Options
	snapshots []checkpoint.SnapshotEntry

	// Pre-loaded data that survives seeks
	trees          *manager.DescendantTrees
	historyPayload *checkpoint.HistoryPayload

	// Animation + timing state shared with the renderer. Owned here so the
	// same clock drives both "when to advance the sim" and "how far through
	// the current cycle's animation are we." See animation package docs.
	AnimState *animation.State

	// wasPaused tracks pause transitions so we can reset the animation clock
	// on unpause and avoid a burst of catch-up cycles.
	wasPaused bool

	// Playback state
	Speed      float64 // playback multiplier; see animation package for semantics
	FinalCycle int     // last cycle in the file (from last snapshot)

	// AutoSpeed, when true, lets the viewer drive Speed from the camera's
	// current zoom level via UpdateSpeedFromZoom — zoomed out views play
	// faster so you can see large-scale behaviour without the sim
	// crawling. A manual SetSpeed call turns this off. The UI exposes a
	// toggle that turns it back on and re-anchors speed to the zoom.
	AutoSpeed bool

	// SeekCount increments on every successful snapshot seek. UI
	// elements that cache sim state (e.g. the minimap) compare it
	// against their last-seen value to invalidate when the playhead
	// jumps — a pure observer signal with no callback plumbing.
	SeekCount int

	// ring holds recent post-Update simulation states. Step-back
	// within the ring window restores from memory in O(1); outside
	// the window we fall back to the snapshot+forward-sim path.
	ring *stateRing

	// recorded is the configuration the simulation ran with.
	recorded config.Globals
	closed   bool
}

// stateRingCapacity is how many recent post-Update sim states the
// in-memory ring buffer keeps. Each entry is a full SnapshotPayload
// (~hundreds of KB on typical sims), so the ring trades memory for
// instant prev-cycle response. 100 covers ~8 seconds of normal-speed
// playback — beyond that, prev-cycle drops back to the snapshot path.
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

	// Restore from the first snapshot
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

// recordedGlobals returns the settings a replay file was recorded with.
// Files from before settings were recorded fall back to the current
// settings with the recorded grid size and seed. Display preferences
// (window size, theme, pH colours) always stay the viewer's own.
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
	return g
}

// Globals returns a copy of the settings the replayed simulation ran
// with, safe for the caller to edit.
func (c *Controller) Globals() config.Globals {
	g := c.recorded
	g.InitialAbilityScores = append([]int(nil), g.InitialAbilityScores...)
	return g
}

// Simulation returns the current simulation state for rendering.
func (c *Controller) Simulation() *simulation.Simulation {
	return c.sim
}

// Cycle returns the current cycle.
func (c *Controller) Cycle() int {
	return c.sim.Cycle()
}

// Update is called each ebiten tick. It advances the simulation based on
// wall-clock time rather than ebiten's tick rate, so playback speed is
// decoupled from the render rate and the animation window stays constant.
//
// The loop keeps advancing cycles while the animation clock says the
// current cycle's window has fully elapsed — at normal speeds that's at
// most one cycle per tick; at very high speeds (Speed > BaseFramesPerCycle)
// multiple cycles can collapse into a single tick.
func (c *Controller) Update() {
	paused := c.sim.IsPaused()
	if paused {
		c.wasPaused = true
		return
	}
	if c.wasPaused {
		// Just unpaused: re-anchor the clock so we don't fast-forward
		// through cycles based on how long the pause lasted.
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

// pushRing captures the simulation's current post-Update state and
// stores it in the in-memory ring so a subsequent prev-cycle can
// restore it without going to disk.
func (c *Controller) pushRing() {
	if c.ring == nil {
		return
	}
	if snap := c.sim.CaptureSnapshot(); snap != nil {
		c.ring.Push(snap)
	}
}

// StepBackward rolls the playback back exactly one cycle. Returns true
// on success, false if no prior state is available (cycle 0 or below).
//
// Fast path: if the in-memory ring contains a snapshot for the target
// cycle, restore it directly — O(1), no disk read, no forward-sim.
//
// Fallback: SeekToCycle does the full snapshot-rewind + forward-sim
// path. The ring fills up as the user plays forward, so the fast
// path covers the common case (scrubbing the recent past).
func (c *Controller) StepBackward() bool {
	target := c.sim.Cycle() - 1
	if target < 0 {
		return false
	}
	if snap, ok := c.ring.Lookup(target); ok {
		if err := c.sim.ResetFromSnapshot(snap); err == nil {
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
	}
	if err := c.SeekToCycle(target); err != nil {
		return false
	}
	return true
}

// SeekToSnapshot restores from a specific snapshot index.
// The existing simulation pointer is updated in-place so all UI references
// remain valid.
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
	// Clear animation state: pre-seek frames no longer describe the current
	// organism set, and the clock must start fresh at the new position.
	c.AnimState.Frames = map[int]animation.Frame{}
	c.AnimState.ResetClock()
	// Drop the in-memory state ring — its entries refer to a different
	// chain of cycles and would mislead future StepBackward calls. The
	// ring will refill as forward-play resumes.
	if c.ring != nil {
		c.ring.Clear()
	}
	c.SeekCount++
	return nil
}

// SeekToCycle finds the nearest snapshot before the target cycle,
// restores from it, then simulates forward to the target.
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

// StepForward advances the simulation by one cycle regardless of pause state,
// capturing the animation batch so the renderer can animate the transition.
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

// SetSpeed sets the playback speed multiplier and disables AutoSpeed —
// the user is explicitly overriding the auto-from-zoom behaviour.
func (c *Controller) SetSpeed(speed float64) {
	c.AutoSpeed = false
	c.setSpeedInternal(speed)
}

// setSpeedInternal updates Speed without touching AutoSpeed. Used by the
// auto-sync path so re-anchoring from zoom doesn't toggle the user's
// preference off. Clamps only to the global Min/MaxReplaySpeed bounds —
// zoom level never limits how fast the user can play — and snaps to a
// power of two.
//
// Snapping rather than trusting the callers: the buttons halve and
// double, and the bounds are powers of two, so every speed *should* be
// one already — but the auto-speed table returned 6 at the smallest
// zoom, and one halving from there put the user on 3x, then 1.5x, then
// 0.75x, with the label falling back to printing "1.5x". Making it a
// property of the setter means no future caller can reintroduce that.
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

// SnapToPowerOfTwo rounds a speed to the nearest power of two, in the
// log domain so 3x lands on 2x or 4x by ratio rather than by distance.
// Speeds below the minimum or above the maximum are left to the caller's
// clamp; this only picks the step.
func SnapToPowerOfTwo(speed float64) float64 {
	if speed <= 0 {
		return MinReplaySpeed
	}
	return math.Pow(2, math.Round(math.Log2(speed)))
}

// MinReplaySpeed and MaxReplaySpeed bound the user-selectable playback
// speeds. Slower than 0.25x crawls so far it stops being useful; faster
// than 64x outpaces the renderer's catch-up loop.
const (
	MinReplaySpeed = 0.25
	MaxReplaySpeed = 64
)

// SpeedForUnitSize returns the auto-speed for a given camera unit size.
// Larger zoom (bigger unit sizes) → 1x; smaller zoom → progressively
// faster so large-scale behaviour plays in a reasonable amount of time.
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

// UpdateSpeedFromZoom re-anchors the speed to the camera zoom when
// AutoSpeed is on. A manually chosen speed is left exactly as it is —
// zooming in never slows playback down.
func (c *Controller) UpdateSpeedFromZoom(unitSize int) {
	if c.AutoSpeed {
		c.setSpeedInternal(SpeedForUnitSize(unitSize))
	}
}

// EnableAutoSpeed turns AutoSpeed on and immediately re-anchors Speed to
// the given unit size. Pass the current camera unit size at the call
// site.
func (c *Controller) EnableAutoSpeed(unitSize int) {
	c.AutoSpeed = true
	c.setSpeedInternal(SpeedForUnitSize(unitSize))
}

// ReconstructionResult is the panel-relevant snapshot of an organism
// reconstructed from an earlier point in the recording. Any of Info /
// TraitsValid / DecisionTree may be zero-valued if the organism didn't
// exist at the requested cycle. Err carries any failure that prevented
// reconstruction (file I/O, missing snapshot, etc).
type ReconstructionResult struct {
	Info         *organism.Info
	Traits       organism.Traits
	TraitsValid  bool
	DecisionTree *d.Tree
	Err          error
}

// ReconstructOrganismAt rebuilds simulation state at targetCycle from
// the nearest preceding snapshot, runs forward to that cycle, and
// returns the named organism's stats. Heavy: forward-plays up to one
// CheckpointInterval, so call from a goroutine. Uses a fresh checkpoint
// reader so the controller's main reader isn't disturbed.
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

	// RestoreFromSnapshot builds a Simulation with no recorder, so
	// forward-play here doesn't write to the .pzr file.
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
	}
	res.Traits, res.TraitsValid = sim.GetOrganismTraitsByID(orgID)
	return res
}

// SnapshotCycles returns the cycle numbers of all snapshots.
func (c *Controller) SnapshotCycles() []int {
	cycles := make([]int, len(c.snapshots))
	for i, s := range c.snapshots {
		cycles[i] = s.Cycle
	}
	return cycles
}

// Close closes the underlying reader.
// Safe to call more than once.
func (c *Controller) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.reader.Close()
}

// loadEndSections scans the file for the descendant trees and history sections
// (written at the end of the simulation), caches them, and injects into the sim.
func (c *Controller) loadEndSections() {
	// Open a fresh reader to scan, since the original file handle's read
	// position may be unreliable after gob decoder buffering.
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
		// End sections (descendant trees / history) are tagged with the
		// simulation's true final cycle. Snapshots fire on a fixed
		// interval, so the last snapshot is almost always before the
		// true end. Lift FinalCycle to whichever cycle the file
		// actually ran to so the viewer can replay the trailing
		// post-snapshot tail. Older files wrote 0 here and degrade
		// gracefully — FinalCycle stays at the last-snapshot value.
		if cycle > c.FinalCycle {
			c.FinalCycle = cycle
		}
	}
}

// ReadAllSections reads through the file to find the true final cycle
// (scanning delta sections past the last snapshot).
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
