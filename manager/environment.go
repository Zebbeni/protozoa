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
			xFactor := (1.0 + math.Sin(float64(x)/(float64(c.GridUnitsWide())/(2*math.Pi)))) / 2.0
			yFactor := (1.0 + math.Sin(float64(y)/(float64(c.GridUnitsHigh())/(2*math.Pi)))) / 2.0
			factor := (xFactor + yFactor) / 2.0
			val := factor*(c.MaxInitialPh()-c.MinInitialPh()) + c.MinInitialPh()
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

func (m *EnvironmentManager) GetWalls() []utils.Point {
	if c.UsePools() == false {
		return []utils.Point{}
	}

	max := (c.GridUnitsWide() / c.PoolWidth()) * (c.GridUnitsHigh() / c.PoolHeight())
	points := make([]utils.Point, 0, max)
	for x := 0; x < c.GridUnitsWide(); x++ {
		for y := 0; y < c.GridUnitsHigh(); y++ {
			if utils.IsWall(x, y) {
				points = append(points, utils.Point{X: x, Y: y})
			}
		}
	}
	return points
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
// Also, while iterating, calculates average ph in environment
func (m *EnvironmentManager) diffusePhLevels() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	diffFactor := c.PhDiffuseFactor()

	adjPh := func(x, y int) (float64, bool) {
		return m.previousPhMap[x][y], !utils.IsWall(x, y)
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
		return avgPh / float64(neighbors)
	}

	// return average ph of all adjacent points (even if in walls)
	avgAdjPhAll := func(x, y int) float64 {
		avgPh := 0.0
		ph, _ := adjPh(x, (y+1)%gridH)
		avgPh += ph
		ph, _ = adjPh(x, (y+gridH-1)%gridH)
		avgPh += ph
		ph, _ = adjPh((x+1)%gridW, y)
		avgPh += ph
		ph, _ = adjPh((x+gridW-1)%gridW, y)
		avgPh += ph
		return avgPh / 4.0
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

			// Just set wall ph to the average of its neighbors
			// (doesn't really affect anything but appearance, since we don't
			// diffuse this value back to the rest of the environment
			if utils.IsWall(x, y) {
				m.setPhAtPoint(utils.Point{X: x, Y: y}, avgAdjPhAll(x, y))
				continue
			}

			avgAdjacentPh := avgAdjPh(x, y)
			change := (avgAdjacentPh - prevVal) * diffFactor
			m.setPhAtPoint(utils.Point{X: x, Y: y}, prevVal+change)
		}
	}

	m.averagePh = totalPh / pointCount
}
