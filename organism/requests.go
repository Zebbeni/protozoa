package organism

import (
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/utils"
	"sync"
)

// RequestManager manages access maps that keep track of overlapping or
// conflicting requests placed by organisms due to concurrent action updates
type RequestManager struct {
	positionRequests     map[string]int       // the number of times a location received a move or spawn request
	foodRequests         map[string]food.Item // the amount of food eaten at a given point
	healthEffectRequests map[string]float64   // the total damage + healing effects at a given location

	mutex sync.Mutex
}

func (m *RequestManager) ClearRequestMaps() {
	m.mutex.Lock()
	m.positionRequests = make(map[string]int)
	m.foodRequests = make(map[string]food.Item)
	m.healthEffectRequests = make(map[string]float64)
	m.mutex.Unlock()
}

func (m *RequestManager) GetPositionRequestsAt(p utils.Point) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.positionRequests[p.ToString()]
}

func (m *RequestManager) GetFoodRequestsAt(p utils.Point) food.Item {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.foodRequests[p.ToString()]
}

func (m *RequestManager) GetHealthEffectRequestsAt(p utils.Point) float64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.healthEffectRequests[p.ToString()]
}

func (m *RequestManager) AddPositionRequest(p utils.Point) {
	pString := p.ToString()
	m.mutex.Lock()
	m.positionRequests[pString]++
	m.mutex.Unlock()
}

func (m *RequestManager) AddFoodRequest(p utils.Point, value int) {
	pString := p.ToString()
	m.mutex.Lock()
	if item, ok := m.foodRequests[pString]; ok {
		value += item.Value
	}
	m.foodRequests[pString] = food.Item{Point: p, Value: value}
	m.mutex.Unlock()
}

func (m *RequestManager) AddHealthEffectRequest(p utils.Point, v float64) {
	pString := p.ToString()
	m.mutex.Lock()
	m.healthEffectRequests[pString] += v
	m.mutex.Unlock()
}
