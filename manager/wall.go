package manager

import (
	"sync"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

const (
	MinWallStrength = 1
	MaxWallStrength = 100
)

// WallManager owns the stateful wall grid that replaced the pool-bordered pure-function walls.
type WallManager struct {
	rng   *simrand.RNG
	walls map[utils.Point]int
	mu    sync.RWMutex
}

// NewWallManager returns a wall grid seeded with config.InitialWalls() randomly placed walls of random strength in [MinWallStrength, MaxWallStrength].
func NewWallManager(rng *simrand.RNG) *WallManager {
	m := &WallManager{
		rng:   rng,
		walls: make(map[utils.Point]int),
	}
	m.InitializeWalls(config.InitialWalls())
	return m
}

func (m *WallManager) InitializeWalls(n int) {
	if n <= 0 {
		return
	}
	w := config.GridUnitsWide()
	h := config.GridUnitsHigh()
	strengthRange := MaxWallStrength - MinWallStrength + 1
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := 0; i < n; i++ {
		p := utils.Point{X: m.rng.Intn(w), Y: m.rng.Intn(h)}
		m.walls[p] = MinWallStrength + m.rng.Intn(strengthRange)
	}
}

func (m *WallManager) IsWallAtPoint(p utils.Point) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.walls[p] > 0
}

// GetWallStrengthAtPoint returns the strength of the wall at p, or 0 if there's no wall.
func (m *WallManager) GetWallStrengthAtPoint(p utils.Point) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.walls[p]
}

// AddWallStrength adjusts the wall at p by delta, clamped to [0, MaxWallStrength].
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

func (m *WallManager) GetWalls() map[utils.Point]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[utils.Point]int, len(m.walls))
	for p, s := range m.walls {
		out[p] = s
	}
	return out
}

func (m *WallManager) Restore(walls map[utils.Point]int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.walls = walls
}

func (m *WallManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.walls)
}
