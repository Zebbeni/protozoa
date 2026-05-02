package replay

import (
	"fmt"
	"io"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/checkpoint"
	"github.com/Zebbeni/protozoa/config"
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
	treesPayload   *checkpoint.DescendantTreesPayload
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
}

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
	globals := config.GetCurrentGlobals()
	globals.GridUnitsWide = reader.Header.GridUnitsWide
	globals.GridUnitsHigh = reader.Header.GridUnitsHigh
	config.SetGlobals(globals)

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
	}

	// Scan for descendant trees and history sections
	ctrl.loadEndSections()

	return ctrl, nil
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
	}
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
	if c.treesPayload != nil {
		c.sim.RestoreDescendantTrees(c.treesPayload)
	}
	if c.historyPayload != nil {
		c.sim.RestoreHistory(c.historyPayload)
	}
	// Clear animation state: pre-seek frames no longer describe the current
	// organism set, and the clock must start fresh at the new position.
	c.AnimState.Frames = map[int]animation.Frame{}
	c.AnimState.ResetClock()
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
// preference off.
func (c *Controller) setSpeedInternal(speed float64) {
	if speed < MinReplaySpeed {
		speed = MinReplaySpeed
	}
	c.Speed = speed
	c.AnimState.Speed = speed
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
		return 6
	}
}

// UpdateSpeedFromZoom re-anchors Speed to the given unit size IF
// AutoSpeed is enabled. No-op otherwise. Call this from the viewer when
// the camera zoom changes.
func (c *Controller) UpdateSpeedFromZoom(unitSize int) {
	if !c.AutoSpeed {
		return
	}
	c.setSpeedInternal(SpeedForUnitSize(unitSize))
}

// EnableAutoSpeed turns AutoSpeed on and immediately re-anchors Speed to
// the given unit size. Pass the current camera unit size at the call
// site.
func (c *Controller) EnableAutoSpeed(unitSize int) {
	c.AutoSpeed = true
	c.setSpeedInternal(SpeedForUnitSize(unitSize))
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
func (c *Controller) Close() error {
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
		sType, _, payload, err := freshReader.ReadNextSection()
		if err != nil {
			break
		}
		switch sType {
		case checkpoint.SectionDescendantTrees:
			if trees, ok := payload.(*checkpoint.DescendantTreesPayload); ok {
				c.treesPayload = trees
				c.sim.RestoreDescendantTrees(trees)
			}
		case checkpoint.SectionHistory:
			if hist, ok := payload.(*checkpoint.HistoryPayload); ok {
				c.historyPayload = hist
				c.sim.RestoreHistory(hist)
			}
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
