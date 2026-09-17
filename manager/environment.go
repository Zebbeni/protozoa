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
	// minPh / maxPh are the extremes across the grid, tracked in the same
	// pass that averages it: an average alone hides a world that is half
	// acid and half base.
	minPh, maxPh float64

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
			m.previousPhMap[x][y] = c.InitialPh()
			m.currentPhMap[x][y] = c.InitialPh()
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

// GetPhRange returns the lowest and highest pH anywhere on the grid.
func (m *EnvironmentManager) GetPhRange() (float64, float64) {
	return m.minPh, m.maxPh
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

// neighbourOffsets is the fixed iteration order for a cell's four
// cardinal neighbours, as (dx, dy) unit steps. Order is fixed (not map
// iteration) because the neighbour sum below is float arithmetic:
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
func (m *EnvironmentManager) diffusePhLevels() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	diffFactor := c.PhDiffuseFactor()

	adjPh := func(x, y int) (float64, bool) {
		return m.previousPhMap[x][y], !m.api.IsWallAtPoint(utils.Point{X: x, Y: y})
	}

	// return the mean of all diffuse-able adjacent points.
	avgAdjPh := func(x, y int) float64 {
		neighbours := 0
		total := 0.0
		for _, off := range neighbourOffsets {
			nx := (x + off.X + gridW) % gridW
			ny := (y + off.Y + gridH) % gridH
			ph, ok := adjPh(nx, ny)
			if !ok {
				continue
			}
			total += ph
			neighbours++
		}
		if neighbours == 0 {
			// Completely walled in — keep current value untouched.
			return m.previousPhMap[x][y]
		}
		return total / float64(neighbours)
	}

	// Water stats skip wall cells: a wall holds whatever pH it was built
	// in for as long as it stands, so counting them would report the
	// world's history rather than the water its organisms live in.
	totalPh := 0.0
	pointCount := 0.0
	minPh, maxPh := math.Inf(1), math.Inf(-1)
	// set each value in the current phMap to its value in the previous phMap, plus
	// the average difference between itself and its N,S,E,W neighbors (times the
	// diffusion factor provided by the config)
	for x := 0; x < gridW; x++ {
		for y := 0; y < gridH; y++ {
			prevVal := m.previousPhMap[x][y]

			// Wall cells freeze their pH at the value they had when
			// the wall appeared — propagate the prevMap value
			// verbatim. Removal of the wall lets it rejoin diffusion
			// naturally the next cycle.
			if m.api.IsWallAtPoint(utils.Point{X: x, Y: y}) {
				m.setPhAtPoint(utils.Point{X: x, Y: y}, prevVal)
				continue
			}

			totalPh += prevVal
			pointCount++
			minPh, maxPh = math.Min(minPh, prevVal), math.Max(maxPh, prevVal)

			avgAdjacentPh := avgAdjPh(x, y)
			change := (avgAdjacentPh - prevVal) * diffFactor
			m.setPhAtPoint(utils.Point{X: x, Y: y}, prevVal+change)
		}
	}

	if pointCount == 0 {
		// Every cell is a wall: no water to report, so leave the last
		// figures standing rather than dividing by zero.
		return
	}
	m.averagePh = totalPh / pointCount
	m.minPh, m.maxPh = minPh, maxPh
}
