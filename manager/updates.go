package manager

import (
	"sync"

	"github.com/Zebbeni/protozoa/utils"
)

// UpdateType identifies which layer needs re-rendering.
type UpdateType int

const (
	UpdateOrganism UpdateType = iota
	UpdatePh
	UpdateFood
)

type UpdateManager struct {
	updates map[UpdateType]map[utils.Point]bool
	mutex   sync.Mutex
}

func NewUpdateManager() *UpdateManager {
	m := &UpdateManager{}
	m.ClearMaps()
	return m
}

func (m *UpdateManager) ClearMaps() {
	m.mutex.Lock()
	m.updates = map[UpdateType]map[utils.Point]bool{
		UpdateOrganism: make(map[utils.Point]bool),
		UpdatePh:       make(map[utils.Point]bool),
		UpdateFood:     make(map[utils.Point]bool),
	}
	m.mutex.Unlock()
}

func (m *UpdateManager) AddUpdate(t UpdateType, p utils.Point) {
	m.mutex.Lock()
	m.updates[t][p] = true
	m.mutex.Unlock()
}

func (m *UpdateManager) GetUpdatedPoints(t UpdateType) map[utils.Point]bool {
	return m.updates[t]
}

// Convenience methods to preserve existing API interface contracts
func (m *UpdateManager) AddOrganismUpdate(p utils.Point) { m.AddUpdate(UpdateOrganism, p) }
func (m *UpdateManager) AddPhUpdate(p utils.Point)       { m.AddUpdate(UpdatePh, p) }
func (m *UpdateManager) AddFoodUpdate(p utils.Point)     { m.AddUpdate(UpdateFood, p) }

func (m *UpdateManager) GetUpdatedOrganismPoints() map[utils.Point]bool {
	return m.GetUpdatedPoints(UpdateOrganism)
}
func (m *UpdateManager) GetUpdatedPhPoints() map[utils.Point]bool {
	return m.GetUpdatedPoints(UpdatePh)
}
func (m *UpdateManager) GetUpdatedFoodPoints() map[utils.Point]bool {
	return m.GetUpdatedPoints(UpdateFood)
}
