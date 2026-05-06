package manager

import (
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/utils"
)

// RequestManager manages access maps that keep track of overlapping or
// conflicting requests placed by organisms due to concurrent action updates
type RequestManager struct {
	positionRequests     map[utils.Point]int
	foodRequests         map[utils.Point]food.Item
	healthEffectRequests map[utils.Point]float64
}

func (m *RequestManager) ClearMaps() {
	m.positionRequests = make(map[utils.Point]int)
	m.foodRequests = make(map[utils.Point]food.Item)
	m.healthEffectRequests = make(map[utils.Point]float64)
}

func (m *RequestManager) GetPositionRequest(p utils.Point) int {
	return m.positionRequests[p]
}

func (m *RequestManager) GetFoodRequests(p utils.Point) food.Item {
	return m.foodRequests[p]
}

func (m *RequestManager) GetHealthEffects(p utils.Point) float64 {
	return m.healthEffectRequests[p]
}

func (m *RequestManager) AddPositionRequest(p utils.Point, id int) {
	if id > m.positionRequests[p] {
		m.positionRequests[p] = id
	}
}

func (m *RequestManager) AddFoodRequest(p utils.Point, value int) {
	if item, ok := m.foodRequests[p]; ok {
		value += item.Value
	}
	m.foodRequests[p] = food.Item{Point: p, Value: value}
}

func (m *RequestManager) AddHealthEffectRequest(p utils.Point, v float64) {
	m.healthEffectRequests[p] += v
}

// MergeFrom merges another RequestManager's data into this one using the
// same aggregation rules (max for position, sum for food and health).
func (m *RequestManager) MergeFrom(other *RequestManager) {
	for p, id := range other.positionRequests {
		if id > m.positionRequests[p] {
			m.positionRequests[p] = id
		}
	}
	for p, item := range other.foodRequests {
		if existing, ok := m.foodRequests[p]; ok {
			item.Value += existing.Value
		}
		m.foodRequests[p] = item
	}
	for p, v := range other.healthEffectRequests {
		m.healthEffectRequests[p] += v
	}
}
