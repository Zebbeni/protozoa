package replay

import (
	"fmt"
	"io"

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

	// Playback state
	Speed      int // cycles per frame (1 = normal, 2 = 2x, etc.)
	FinalCycle int // last cycle in the file (from last snapshot)
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
		Speed:      1,
		FinalCycle: lastSnap.Cycle,
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

// Update advances the simulation if not paused. Called each frame.
func (c *Controller) Update() {
	if c.sim.IsPaused() {
		return
	}
	for i := 0; i < c.Speed; i++ {
		if c.sim.Cycle() >= c.FinalCycle {
			c.sim.Pause(true)
			return
		}
		c.sim.Update()
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

// StepForward advances the simulation by one cycle regardless of pause state.
func (c *Controller) StepForward() {
	if c.sim.Cycle() >= c.FinalCycle {
		return
	}
	wasPaused := c.sim.IsPaused()
	c.sim.Pause(false)
	c.sim.Update()
	c.sim.Pause(wasPaused)
}

// SetSpeed sets the playback speed (cycles per frame).
func (c *Controller) SetSpeed(speed int) {
	if speed < 1 {
		speed = 1
	}
	c.Speed = speed
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
