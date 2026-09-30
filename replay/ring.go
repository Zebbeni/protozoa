package replay

import (
	"github.com/Zebbeni/protozoa/checkpoint"
)

// stateRing is a fixed-capacity ring buffer of recent simulation snapshots, indexed by cycle number for O(1) prev-cycle restoration.
type stateRing struct {
	capacity int
	entries  []*checkpoint.SnapshotPayload // most recent at the end; oldest at the front
}

func newStateRing(capacity int) *stateRing {
	if capacity < 1 {
		capacity = 1
	}
	return &stateRing{
		capacity: capacity,
		entries:  make([]*checkpoint.SnapshotPayload, 0, capacity),
	}
}

// Push appends a snapshot to the ring, evicting the oldest if at capacity.
func (r *stateRing) Push(snap *checkpoint.SnapshotPayload) {
	if snap == nil {
		return
	}
	if len(r.entries) >= r.capacity {
		copy(r.entries, r.entries[1:])
		r.entries = r.entries[:len(r.entries)-1]
	}
	r.entries = append(r.entries, snap)
}

// Lookup returns the snapshot for the given cycle if present, else (nil, false).
func (r *stateRing) Lookup(cycle int) (*checkpoint.SnapshotPayload, bool) {
	// Most-recent-first scan — prev-cycle clicks usually want the snapshot near the tail.
	for i := len(r.entries) - 1; i >= 0; i-- {
		if r.entries[i].Cycle == cycle {
			return r.entries[i], true
		}
	}
	return nil, false
}

func (r *stateRing) Clear() {
	r.entries = r.entries[:0]
}

func (r *stateRing) Len() int {
	return len(r.entries)
}
