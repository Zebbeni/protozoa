package manager

import (
	"sync"

	"github.com/Zebbeni/protozoa/utils"
)

// Wall-strength bounds. Walls are added by ActBurrow (positive
// delta), removed by ActDig (negative delta); a 0-strength wall is
// removed from the map entirely so IsWallAtPoint returns false.
// The cap at 7 keeps the strength visualisable as a small set of
// alpha tiers without making walls indestructibly expensive to dig.
const (
	MinWallStrength = 1
	MaxWallStrength = 7
)

// WallManager owns the stateful wall grid that replaced the
// pool-bordered pure-function walls. Walls are a sparse map keyed by
// point — most of the grid is empty, and feature-driven actions
// (dig/burrow) add or remove walls dynamically over the sim's life.
// State changes are guarded by an RWMutex so concurrent readers
// (renderer, conditions) don't race the writer.
type WallManager struct {
	walls map[utils.Point]int
	mu    sync.RWMutex
}

// NewWallManager returns an empty wall grid. Genesis sims start with
// no walls — they only appear via burrow actions once a lineage
// evolves Tusks.
func NewWallManager() *WallManager {
	return &WallManager{
		walls: make(map[utils.Point]int),
	}
}

// IsWallAtPoint reports whether a wall is present (strength > 0) at p.
func (m *WallManager) IsWallAtPoint(p utils.Point) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.walls[p] > 0
}

// GetWallStrengthAtPoint returns the strength of the wall at p, or 0
// if there's no wall. Callers can branch on >0 instead of IsWall when
// they need the strength value too (e.g. damage-per-action math).
func (m *WallManager) GetWallStrengthAtPoint(p utils.Point) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.walls[p]
}

// AddWallStrength adjusts the wall at p by delta, clamped to
// [0, MaxWallStrength]. A resulting strength of 0 removes the entry
// from the map so IsWallAtPoint becomes false. Returns the new
// strength. Positive delta is burrow, negative is dig.
func (m *WallManager) AddWallStrength(p utils.Point, delta int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := m.walls[p]
	next := cur + delta
	if next > MaxWallStrength {
		next = MaxWallStrength
	}
	if next <= 0 {
		delete(m.walls, p)
		return 0
	}
	m.walls[p] = next
	return next
}

// GetWalls returns a copy of the current wall map. Used by the
// renderer (every refresh) and snapshot capture; returning a copy
// keeps callers from racing the writer through the underlying map.
func (m *WallManager) GetWalls() map[utils.Point]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[utils.Point]int, len(m.walls))
	for p, s := range m.walls {
		out[p] = s
	}
	return out
}

// Restore replaces the wall state, used by checkpoint restore. The
// caller hands ownership of walls to the manager; do not mutate the
// passed map after restoring.
func (m *WallManager) Restore(walls map[utils.Point]int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.walls = walls
}

// Count returns the number of cells currently containing a wall.
// Useful for diagnostics and tests.
func (m *WallManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.walls)
}
