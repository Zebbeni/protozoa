package manager

import (
	"github.com/Zebbeni/protozoa/utils"
)

// RequestManager manages access maps that keep track of overlapping or conflicting requests placed by organisms due to concurrent action updates
type RequestManager struct {
	// positionRequests is the winning claim per contested cell.
	positionRequests     map[utils.Point]PositionClaim
	healthEffectRequests map[utils.Point][]HealthEffect
}

// HealthEffect is one pending health change aimed at a cell, tagged with its source so the receiving organism can respond to the originator.
type HealthEffect struct {
	Amount   float64
	SourceID int
}

func (m *RequestManager) ClearMaps() {
	m.positionRequests = make(map[utils.Point]PositionClaim)
	m.healthEffectRequests = make(map[utils.Point][]HealthEffect)
}

// PositionClaim is a bid for a cell: who made it and how big they are.
type PositionClaim struct {
	ID   int
	Size float64
}

// GetPositionRequest returns the ID of whoever won the cell, or -1 if nobody claimed it.
func (m *RequestManager) GetPositionRequest(p utils.Point) int {
	claim, ok := m.positionRequests[p]
	if !ok {
		return -1
	}
	return claim.ID
}

// HasPositionRequest reports whether anyone claimed this cell during the decide phase.
func (m *RequestManager) HasPositionRequest(p utils.Point) bool {
	_, ok := m.positionRequests[p]
	return ok
}

// GetHealthEffects returns every pending effect aimed at p, each still carrying its source.
func (m *RequestManager) GetHealthEffects(p utils.Point) []HealthEffect {
	return m.healthEffectRequests[p]
}

// TotalHealthEffect sums the pending effects at p, for callers that only need the net change.
func (m *RequestManager) TotalHealthEffect(p utils.Point) float64 {
	total := 0.0
	for _, e := range m.healthEffectRequests[p] {
		total += e.Amount
	}
	return total
}

// AddPositionRequest claims a cell for an organism of a given size.
func (m *RequestManager) AddPositionRequest(p utils.Point, id int, size float64) {
	if cur, ok := m.positionRequests[p]; ok && !beats(size, id, cur.Size, cur.ID) {
		return
	}
	m.positionRequests[p] = PositionClaim{ID: id, Size: size}
}

func beats(size float64, id int, otherSize float64, otherID int) bool {
	if size != otherSize {
		return size > otherSize
	}
	return id > otherID
}

// AddAttackRequest queues attack damage against whatever is in the cell, carrying the attacker's ID so the defender's thorns know who hit it.
func (m *RequestManager) AddAttackRequest(p utils.Point, v float64, sourceID int) {
	m.healthEffectRequests[p] = append(m.healthEffectRequests[p],
		HealthEffect{Amount: v, SourceID: sourceID})
}

// MergeFrom merges another RequestManager's data into this one using the same aggregation rules (largest-wins for position, concatenation for health effects so each keeps its source).
func (m *RequestManager) MergeFrom(other *RequestManager) {
	for p, claim := range other.positionRequests {
		m.AddPositionRequest(p, claim.ID, claim.Size)
	}
	for p, effects := range other.healthEffectRequests {
		m.healthEffectRequests[p] = append(m.healthEffectRequests[p], effects...)
	}
}
