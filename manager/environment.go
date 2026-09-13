package manager

import (
	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/environment"
	"github.com/Zebbeni/protozoa/utils"
	"math"
	"sync"
)

// EnvironmentManager contains an image
type EnvironmentManager struct {
	api environment.API

	currentPhMap  [][]float64
	previousPhMap [][]float64

	// flowMap is the per-cell current ("flow field"). A cell's vector
	// biases which neighbours its pH mixes with during diffusion: the
	// upstream neighbour is weighted up and the downstream one down,
	// so a pH pattern travels along the flow. Still cells (the zero
	// vector) keep the original isotropic behaviour exactly.
	//
	// Fimbriae organisms write into this map via CirculateFlowAtPoint;
	// every cell decays back toward still each cycle, so a current
	// only persists while something keeps stirring it.
	flowMap [][]utils.Vector

	averagePh float64

	mutex sync.Mutex
}

func NewEnvironmentManager(api environment.API) *EnvironmentManager {
	manager := &EnvironmentManager{
		api: api,
	}

	manager.initializePhMap()

	return manager
}

func (m *EnvironmentManager) initializePhMap() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	m.previousPhMap = make([][]float64, gridW)
	m.currentPhMap = make([][]float64, gridW)
	m.flowMap = make([][]utils.Vector, gridW)
	for x := 0; x < gridW; x++ {
		m.previousPhMap[x] = make([]float64, gridH)
		m.currentPhMap[x] = make([]float64, gridH)
		// Zero vectors: the environment starts as still water and
		// diffuses isotropically until something circulates it.
		m.flowMap[x] = make([]utils.Vector, gridH)
		for y := 0; y < gridH; y++ {
			// Start all locations at neutral ph. Walls don't exist
			// at sim start (they only appear when an organism
			// performs ActDig and places side walls); the pH map is
			// uniform until that happens.
			val := (c.MaxInitialPh() + c.MinInitialPh()) / 2.0
			m.previousPhMap[x][y] = val
			m.currentPhMap[x][y] = val
		}
	}
}

func (m *EnvironmentManager) Update() {
	m.updatePrevCurrentPhMaps()
	m.decayFlow()
	m.diffusePhLevels()
}

// GetFlowAtPoint returns the current flow vector at a point. Zero
// means still water.
func (m *EnvironmentManager) GetFlowAtPoint(point utils.Point) utils.Vector {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.flowMap[point.X][point.Y]
}

// GetFlowMap returns a direct reference to the flow field. Used by the
// snapshot capture path and any renderer that wants to visualise the
// current; callers must not mutate it.
func (m *EnvironmentManager) GetFlowMap() [][]utils.Vector {
	return m.flowMap
}

// CirculateFlowAtPoint nudges the flow at a point toward dir, scaled by
// strength, and clamps the result to unit magnitude. Additive rather
// than assign-based on purpose: two organisms facing the same way
// reinforce each other's current, and two facing opposite ways cancel,
// so a colony's aggregate facing is what shapes the field.
//
// Marks the cell dirty so the pH renderer repaints it — the flow
// affects how the cell will look once it starts mixing differently.
func (m *EnvironmentManager) CirculateFlowAtPoint(point utils.Point, dir utils.Point, strength float64) {
	push := utils.VectorFromPoint(dir).Scale(strength)

	m.mutex.Lock()
	m.flowMap[point.X][point.Y] = m.flowMap[point.X][point.Y].Add(push).ClampUnit()
	m.mutex.Unlock()

	m.addUpdatedPoint(point)
}

// decayFlow pulls every cell's current back toward still by
// FlowDecayFactor. Without it a single circulate would steer a cell
// forever; with it, a Fimbriae colony has to keep working to hold a
// current open, and an abandoned one silts up over a predictable
// number of cycles.
//
// Walls are hard-zeroed rather than decayed: solid rock carries no
// current, so a cell that becomes a wall loses its flow immediately
// and a cell that stops being one starts still.
func (m *EnvironmentManager) decayFlow() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	decay := c.FlowDecayFactor()

	m.mutex.Lock()
	defer m.mutex.Unlock()
	for x := 0; x < gridW; x++ {
		for y := 0; y < gridH; y++ {
			v := m.flowMap[x][y]
			if v.IsZero() {
				continue
			}
			if m.api.IsWallAtPoint(utils.Point{X: x, Y: y}) {
				m.flowMap[x][y] = utils.Vector{}
				continue
			}
			v = v.Scale(decay)
			// Snap to still below the threshold so cells don't carry
			// denormal-ish residue forever and IsZero keeps paying off
			// as a fast path.
			if v.Length() < flowRestThreshold {
				v = utils.Vector{}
			}
			m.flowMap[x][y] = v
		}
	}
}

func (m *EnvironmentManager) GetPhMap() [][]float64 {
	return m.currentPhMap
}

// GetPhAtPoint returns the current pH level of the environment at a given point
func (m *EnvironmentManager) GetPhAtPoint(point utils.Point) float64 {
	return m.getCurrentPh(point)
}

func (m *EnvironmentManager) GetAveragePh() float64 {
	return m.averagePh
}

// AddPhChangeAtPoint adds a positive or negative value to pH, bounded by the
// minimum and maximum pH values provided by the config
func (m *EnvironmentManager) AddPhChangeAtPoint(point utils.Point, change float64) {
	value := change + m.getCurrentPh(point)
	m.setPhAtPoint(point, value)
}

func (m *EnvironmentManager) setPhAtPoint(point utils.Point, val float64) {
	prevPh := m.getPreviousPh(point)
	newPh := math.Max(math.Min(val, c.MaxPh()), c.MinPh())

	// only flag a worthwhile update if change is passed the threshold to update
	incrementToDisplay := c.PhIncrementToDisplay()
	if int(prevPh/incrementToDisplay) != int(newPh/incrementToDisplay) {
		m.addUpdatedPoint(point)
	}

	m.setCurrentPh(point, newPh)
}

// setCurrentPh sets the current pH level of the environment at a given point
func (m *EnvironmentManager) setCurrentPh(point utils.Point, ph float64) {
	m.mutex.Lock()
	m.currentPhMap[point.X][point.Y] = ph
	m.mutex.Unlock()
}

// getCurrentPh returns the current pH level of the environment at a given point
func (m *EnvironmentManager) getCurrentPh(point utils.Point) float64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.currentPhMap[point.X][point.Y]
}

// getPreviousPh returns the previous pH level of the environment at a given point
func (m *EnvironmentManager) getPreviousPh(point utils.Point) float64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.previousPhMap[point.X][point.Y]
}

func (m *EnvironmentManager) addUpdatedPoint(point utils.Point) {
	m.api.AddPhUpdate(point)
}

// Between cycles, swap which phMap we're treating as the 'previous' ph values
// and which 'current' ph map we will be updating
func (m *EnvironmentManager) updatePrevCurrentPhMaps() {
	prevPhMap := m.previousPhMap
	m.previousPhMap = m.currentPhMap
	m.currentPhMap = prevPhMap
}

// flowRestThreshold is the magnitude below which a decaying flow
// vector snaps to exactly still. Keeps IsZero() a meaningful fast
// path instead of letting cells carry vanishing residue indefinitely.
const flowRestThreshold = 1e-4

// neighbourOffsets is the fixed iteration order for a cell's four
// cardinal neighbours, as (dx, dy) unit steps. Order is fixed (not
// map iteration) because the weighted sum below is float arithmetic:
// changing the summation order would change the low bits and break
// replay determinism.
var neighbourOffsets = [4]utils.Point{
	{X: 0, Y: 1},  // down (+y)
	{X: 0, Y: -1}, // up
	{X: 1, Y: 0},  // right (+x)
	{X: -1, Y: 0}, // left
}

// simulate diffusion of ph across the environment by adjusting each
// ph value toward its neighbors' values.
// Also, while iterating, calculates average ph in environment.
//
// Walls trap the pH value at their cell: the wall's value is held
// constant (carried forward from previousPhMap) and the cell's pH is
// excluded from the neighbour average of its non-wall neighbours.
// When the wall is later destroyed (strength → 0 via ActDig), the
// trapped value just re-enters the diffusion average naturally on
// the next cycle. No special unfreeze step needed.
//
// Currents (the flowMap) make the neighbour average anisotropic. A
// cell with flow v weights each neighbour by
//
//	w = 1 - FlowBias * dot(v, dirToNeighbour)
//
// which weights the UPSTREAM neighbour (the one the flow is coming
// from, where dot is negative) above the downstream one. The cell
// therefore adopts more of what is upstream of it, and a pH pattern
// travels along v rather than against it — the sign here is the easy
// thing to get backwards. FlowBias < 1 keeps every weight positive,
// so this stays a genuine weighted average: it never creates or
// destroys pH, it only changes which direction mixes fastest. Still
// cells (v == 0) give every neighbour w == 1, which is exactly the
// original unweighted mean.
func (m *EnvironmentManager) diffusePhLevels() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	diffFactor := c.PhDiffuseFactor()
	flowBias := c.FlowBias()

	adjPh := func(x, y int) (float64, bool) {
		return m.previousPhMap[x][y], !m.api.IsWallAtPoint(utils.Point{X: x, Y: y})
	}

	// return weighted average of all diffuse-able adjacent points,
	// skewed by this cell's flow vector (see the doc comment above).
	avgAdjPh := func(x, y int) float64 {
		flow := m.flowMap[x][y]
		still := flow.IsZero() || flowBias == 0

		totalWeight := 0.0
		weightedPh := 0.0
		for _, off := range neighbourOffsets {
			nx := (x + off.X + gridW) % gridW
			ny := (y + off.Y + gridH) % gridH
			ph, ok := adjPh(nx, ny)
			if !ok {
				continue
			}
			weight := 1.0
			if !still {
				// dot < 0 for the upstream neighbour, so subtracting
				// raises its weight above 1 and lowers the downstream
				// one below it. |flow| <= 1 and flowBias < 1 keep the
				// result in (0, 2).
				weight = 1 - flowBias*flow.Dot(utils.VectorFromPoint(off))
			}
			weightedPh += ph * weight
			totalWeight += weight
		}
		if totalWeight == 0 {
			// Completely walled in — keep current value untouched.
			return m.previousPhMap[x][y]
		}
		return weightedPh / totalWeight
	}

	totalPh := 0.0
	pointCount := float64(gridW * gridH)
	// set each value in the current phMap to its value in the previous phMap, plus
	// the average difference between itself and its N,S,E,W neighbors (times the
	// diffusion factor provided by the config)
	for x := 0; x < gridW; x++ {
		for y := 0; y < gridH; y++ {
			prevVal := m.previousPhMap[x][y]
			totalPh += prevVal

			// Wall cells freeze their pH at the value they had when
			// the wall appeared — propagate the prevMap value
			// verbatim. Removal of the wall lets it rejoin diffusion
			// naturally the next cycle.
			if m.api.IsWallAtPoint(utils.Point{X: x, Y: y}) {
				m.setPhAtPoint(utils.Point{X: x, Y: y}, prevVal)
				continue
			}

			avgAdjacentPh := avgAdjPh(x, y)
			change := (avgAdjacentPh - prevVal) * diffFactor
			m.setPhAtPoint(utils.Point{X: x, Y: y}, prevVal+change)
		}
	}

	m.averagePh = totalPh / pointCount
}
