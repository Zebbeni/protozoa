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

	return &Controller{
		reader:     reader,
		sim:        sim,
		options:    options,
		snapshots:  reader.SnapshotIndex,
		Speed:      1,
		FinalCycle: lastSnap.Cycle,
	}, nil
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
func (c *Controller) SeekToSnapshot(index int) error {
	snap, err := c.reader.ReadSnapshot(index)
	if err != nil {
		return fmt.Errorf("failed to read snapshot %d: %w", index, err)
	}

	sim, err := simulation.RestoreFromSnapshot(snap, c.options)
	if err != nil {
		return fmt.Errorf("failed to restore snapshot %d: %w", index, err)
	}

	c.sim = sim
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

	// Simulate forward to the target cycle
	for c.sim.Cycle() < target {
		c.sim.Update()
	}
	return nil
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
