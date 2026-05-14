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
	for x := 0; x < gridW; x++ {
		m.previousPhMap[x] = make([]float64, gridH)
		m.currentPhMap[x] = make([]float64, gridH)
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
	m.diffusePhLevels()
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
func (m *EnvironmentManager) diffusePhLevels() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	diffFactor := c.PhDiffuseFactor()

	adjPh := func(x, y int) (float64, bool) {
		return m.previousPhMap[x][y], !m.api.IsWallAtPoint(utils.Point{X: x, Y: y})
	}

	// return average of all diffuse-able adjacent points
	avgAdjPh := func(x, y int) float64 {
		neighbors := 0
		avgPh := 0.0
		if ph, ok := adjPh(x, (y+1)%gridH); ok {
			avgPh += ph
			neighbors++
		}
		if ph, ok := adjPh(x, (y+gridH-1)%gridH); ok {
			avgPh += ph
			neighbors++
		}
		if ph, ok := adjPh((x+1)%gridW, y); ok {
			avgPh += ph
			neighbors++
		}
		if ph, ok := adjPh((x+gridW-1)%gridW, y); ok {
			avgPh += ph
			neighbors++
		}
		if neighbors == 0 {
			// Completely walled in — keep current value untouched.
			return m.previousPhMap[x][y]
		}
		return avgPh / float64(neighbors)
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
