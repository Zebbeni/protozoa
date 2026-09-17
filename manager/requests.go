package manager

import (
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/utils"
)

// RequestManager manages access maps that keep track of overlapping or
// conflicting requests placed by organisms due to concurrent action updates
type RequestManager struct {
	positionRequests map[utils.Point]int
	foodRequests     map[utils.Point]food.Item
	// healthEffectRequests keeps each incoming effect separate, with
	// the ID of whoever caused it, rather than summing to a single
	// number per cell. Damage reflection needs to know who to send the
	// thorns back to, and a summed scalar has already thrown that away.
	healthEffectRequests map[utils.Point][]HealthEffect
}

// HealthEffect is one pending health change aimed at a cell, tagged
// with its source so the receiving organism can respond to the
// originator — which thorns need, to know who to hurt back. SourceID is
// -1 for effects with no organism behind them.
type HealthEffect struct {
	Amount   float64
	SourceID int
}

func (m *RequestManager) ClearMaps() {
	m.positionRequests = make(map[utils.Point]int)
	m.foodRequests = make(map[utils.Point]food.Item)
	m.healthEffectRequests = make(map[utils.Point][]HealthEffect)
}

func (m *RequestManager) GetPositionRequest(p utils.Point) int {
	return m.positionRequests[p]
}

// HasPositionRequest reports whether anyone claimed this cell during the
// decide phase. A claim is only staked on a cell that was empty then, so
// no claim at all means the cell was already blocked — which is what
// separates walking into a wall from losing a race for open water.
func (m *RequestManager) HasPositionRequest(p utils.Point) bool {
	_, ok := m.positionRequests[p]
	return ok
}

func (m *RequestManager) GetFoodRequests(p utils.Point) food.Item {
	return m.foodRequests[p]
}

// GetHealthEffects returns every pending effect aimed at p, each still
// carrying its source.
func (m *RequestManager) GetHealthEffects(p utils.Point) []HealthEffect {
	return m.healthEffectRequests[p]
}

// TotalHealthEffect sums the pending effects at p, for callers that
// only need the net change.
func (m *RequestManager) TotalHealthEffect(p utils.Point) float64 {
	total := 0.0
	for _, e := range m.healthEffectRequests[p] {
		total += e.Amount
	}
	return total
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

// AddAttackRequest queues attack damage against whatever is in the cell,
// carrying the attacker's ID so the defender's thorns know who hit it.
// Attacks are the only source of queued health effects.
func (m *RequestManager) AddAttackRequest(p utils.Point, v float64, sourceID int) {
	m.healthEffectRequests[p] = append(m.healthEffectRequests[p],
		HealthEffect{Amount: v, SourceID: sourceID})
}

// MergeFrom merges another RequestManager's data into this one using the
// same aggregation rules (max for position, sum for food, concatenation
// for health effects so each keeps its source).
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
	for p, effects := range other.healthEffectRequests {
		m.healthEffectRequests[p] = append(m.healthEffectRequests[p], effects...)
	}
}
