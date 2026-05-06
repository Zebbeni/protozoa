package replay

import (
	"github.com/Zebbeni/protozoa/checkpoint"
)

// stateRing is a fixed-capacity ring buffer of recent simulation
// snapshots, indexed by cycle number for O(1) prev-cycle restoration.
//
// As replay plays forward, each post-Update tick pushes a fresh
// snapshot onto the ring; the oldest entry rolls off when the buffer
// is full. Prev-cycle clicks within the buffer window restore the
// requested cycle directly from memory; clicks outside fall back to
// the file's snapshot index plus forward-sim.
//
// Snapshots are full SnapshotPayloads — the same shape persisted to
// disk — so the existing ResetFromSnapshot path can swap them in
// without any new restore code.
type stateRing struct {
	capacity int
	entries  []*checkpoint.SnapshotPayload // most recent at the end; oldest at the front
}

// newStateRing returns an empty ring with the given capacity.
func newStateRing(capacity int) *stateRing {
	if capacity < 1 {
		capacity = 1
	}
	return &stateRing{
		capacity: capacity,
		entries:  make([]*checkpoint.SnapshotPayload, 0, capacity),
	}
}

// Push appends a snapshot to the ring, evicting the oldest if at
// capacity. Snapshots are stored by reference; callers must not mutate
// after pushing.
func (r *stateRing) Push(snap *checkpoint.SnapshotPayload) {
	if snap == nil {
		return
	}
	if len(r.entries) >= r.capacity {
		// Drop oldest. Slice trick: shift left by 1.
		copy(r.entries, r.entries[1:])
		r.entries = r.entries[:len(r.entries)-1]
	}
	r.entries = append(r.entries, snap)
}

// Lookup returns the snapshot for the given cycle if present, else
// (nil, false).
func (r *stateRing) Lookup(cycle int) (*checkpoint.SnapshotPayload, bool) {
	// Most-recent-first scan — prev-cycle clicks usually want the
	// snapshot near the tail.
	for i := len(r.entries) - 1; i >= 0; i-- {
		if r.entries[i].Cycle == cycle {
			return r.entries[i], true
		}
	}
	return nil, false
}

// Clear drops all entries. Called on seek to invalidate stale "future"
// state captured during prior playback.
func (r *stateRing) Clear() {
	r.entries = r.entries[:0]
}

// Len returns the current number of entries (0 ≤ Len ≤ capacity).
func (r *stateRing) Len() int {
	return len(r.entries)
}
