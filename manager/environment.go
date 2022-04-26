package manager

import (
	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/environment"
	"github.com/Zebbeni/protozoa/utils"
	"math"
	"math/rand"
)

// EnvironmentManager contains an image
type EnvironmentManager struct {
	api           environment.API
	phMap         [][][]float64
	updatedPoints map[string]utils.Point
}

func NewEnvironmentManager(api environment.API) *EnvironmentManager {
	manager := &EnvironmentManager{
		api:           api,
		updatedPoints: make(map[string]utils.Point),
	}

	manager.initializePhMap()

	return manager
}

func (m *EnvironmentManager) initializePhMap() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	m.phMap = [][][]float64{make([][]float64, gridW), make([][]float64, gridW)}
	for x := 0; x < gridW; x++ {
		m.phMap[0][x] = make([]float64, gridH)
		m.phMap[1][x] = make([]float64, gridH)
		for y := 0; y < gridH; y++ {
			// initialize with random values
			val := rand.Float64()*(c.MaxInitialPh()-c.MinInitialPh()) + c.MinInitialPh()
			m.phMap[0][x][y] = val
			m.phMap[1][x][y] = val
		}
	}
}

func (m *EnvironmentManager) Update() {
	m.diffusePhLevels()
}

func (m *EnvironmentManager) GetPhMap() [][]float64 {
	return m.phMap[m.getCurrentIndex()]
}

func (m *EnvironmentManager) ClearPhMap() {
	m.updatedPoints = make(map[string]utils.Point)
}

// GetPhAtPoint returns the current pH level of the environment at a given point
func (m *EnvironmentManager) GetPhAtPoint(point utils.Point) float64 {
	return m.phMap[m.getCurrentIndex()][point.X][point.Y]
}

func (m *EnvironmentManager) GetUpdatedPoints() map[string]utils.Point {
	return m.updatedPoints
}

func (m *EnvironmentManager) ClearUpdatedPoints() {
	m.updatedPoints = make(map[string]utils.Point)
}

// AddPhChangeAtPoint adds a positive or negative value to pH, bounded by the
// minimum and maximum pH values provided by the config
func (m *EnvironmentManager) AddPhChangeAtPoint(point utils.Point, change float64) {
	m.setPhAtPoint(point, change+m.phMap[m.getCurrentIndex()][point.X][point.Y])
}

func (m *EnvironmentManager) setPhAtPoint(point utils.Point, val float64) {
	prevPh := m.phMap[m.getPreviousIndex()][point.X][point.Y]
	newPh := math.Max(math.Min(val, c.MaxPh()), c.MinPh())

	m.phMap[m.getCurrentIndex()][point.X][point.Y] = newPh

	// only flag a worthwhile update if change is passed the difference threshhold
	if int(prevPh/c.PhIncrementToDisplay()) != int(newPh/c.PhIncrementToDisplay()) {
		m.addUpdatedPoint(point)
	}
}

// We update our phMap in place to allow diffusion between cycles without copying
// our ph values into new slice
func (m *EnvironmentManager) getCurrentIndex() int {
	return m.api.Cycle() % 2
}

func (m *EnvironmentManager) getPreviousIndex() int {
	return 1 - (m.api.Cycle() % 2)
}

func (m *EnvironmentManager) addUpdatedPoint(point utils.Point) {
	m.updatedPoints[point.ToString()] = point
}

// simulate diffusion of ph across the environment by adjusting each
// ph value toward its neighbors' values
func (m *EnvironmentManager) diffusePhLevels() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	prev := m.getPreviousIndex()
	diffFactor := c.PhDiffuseFactor()
	// set each value in the current phMap to its value in the previous phMap, plus
	// the average difference between itself and its N,S,E,W neighbors (times the
	// diffusion factor provided by the config)
	for x := 0; x < gridW; x++ {
		for y := 0; y < gridH; y++ {
			prevVal := m.phMap[prev][x][y]

			nVal := m.phMap[prev][x][(y+1)%gridH]
			sVal := m.phMap[prev][x][(y+gridH-1)%gridH]
			eVal := m.phMap[prev][(x+1)%gridW][y]
			wVal := m.phMap[prev][(x+gridW-1)%gridW][y]
			change := (((nVal + sVal + eVal + wVal) / 4) - prevVal) * diffFactor

			m.setPhAtPoint(utils.Point{X: x, Y: y}, prevVal+change)
		}
	}
}
