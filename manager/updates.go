package manager

import (
	"sync"

	"github.com/Zebbeni/protozoa/utils"
)

type UpdateManager struct {
	organismUpdates map[string]utils.Point
	phUpdates       map[string]utils.Point
	foodUpdates     map[string]utils.Point

	mutex sync.Mutex
}

func NewUpdateManager() *UpdateManager {
	m := &UpdateManager{}
	m.ClearMaps()
	return m
}

func (m *UpdateManager) ClearMaps() {
	m.mutex.Lock()
	m.organismUpdates = make(map[string]utils.Point)
	m.phUpdates = make(map[string]utils.Point)
	m.foodUpdates = make(map[string]utils.Point)
	m.mutex.Unlock()
}

func (m *UpdateManager) AddOrganismUpdate(p utils.Point) {
	m.mutex.Lock()
	m.organismUpdates[p.ToString()] = p
	m.mutex.Unlock()
}

// GetUpdatedOrganismPoints returns the full updated organism point map
// (We should do this in a way that avoids sharing the actual map)
func (m *UpdateManager) GetUpdatedOrganismPoints() map[string]utils.Point {
	return m.organismUpdates
}

func (m *UpdateManager) AddPhUpdate(p utils.Point) {
	m.mutex.Lock()
	m.phUpdates[p.ToString()] = p
	m.mutex.Unlock()
}

// GetUpdatedPhPoints returns the full updated ph point map
// (We should do this in a way that avoids sharing the actual map)
func (m *UpdateManager) GetUpdatedPhPoints() map[string]utils.Point {
	return m.phUpdates
}

func (m *UpdateManager) AddFoodUpdate(p utils.Point) {
	m.mutex.Lock()
	m.foodUpdates[p.ToString()] = p
	m.mutex.Unlock()
}

// GetUpdatedFoodPoints returns the full updated food point map
// (We should do this in a way that avoids sharing the actual map)
func (m *UpdateManager) GetUpdatedFoodPoints() map[string]utils.Point {
	return m.foodUpdates
}
