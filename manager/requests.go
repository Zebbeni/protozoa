package manager

import (
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/utils"
	"sync"
)

// RequestManager manages access maps that keep track of overlapping or
// conflicting requests placed by organisms due to concurrent action updates
type RequestManager struct {
	positionRequests     map[utils.Point]int       // the lowest id of an organism requesting to move or spawn at a point
	foodRequests         map[utils.Point]food.Item  // the amount of food eaten at a given point
	healthEffectRequests map[utils.Point]float64    // the total damage + healing effects at a given location

	mutex sync.Mutex
}

func (m *RequestManager) ClearMaps() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.positionRequests = make(map[utils.Point]int)
	m.foodRequests = make(map[utils.Point]food.Item)
	m.healthEffectRequests = make(map[utils.Point]float64)
}

func (m *RequestManager) GetPositionRequest(p utils.Point) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.positionRequests[p]
}

func (m *RequestManager) GetFoodRequests(p utils.Point) food.Item {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.foodRequests[p]
}

func (m *RequestManager) GetHealthEffects(p utils.Point) float64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.healthEffectRequests[p]
}

func (m *RequestManager) AddPositionRequest(p utils.Point, id int) {
	m.mutex.Lock()
	if id > m.positionRequests[p] {
		m.positionRequests[p] = id
	}
	m.mutex.Unlock()
}

func (m *RequestManager) AddFoodRequest(p utils.Point, value int) {
	m.mutex.Lock()
	if item, ok := m.foodRequests[p]; ok {
		value += item.Value
	}
	m.foodRequests[p] = food.Item{Point: p, Value: value}
	m.mutex.Unlock()
}

func (m *RequestManager) AddHealthEffectRequest(p utils.Point, v float64) {
	m.mutex.Lock()
	m.healthEffectRequests[p] += v
	m.mutex.Unlock()
}
