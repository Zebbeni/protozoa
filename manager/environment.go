package manager

import (
	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/environment"
	"github.com/Zebbeni/protozoa/utils"
	"math"
	"sync"
)

type EnvironmentManager struct {
	api environment.API

	currentPhMap  [][]float64
	previousPhMap [][]float64

	averagePh float64
	// minPh / maxPh are the extremes across the grid, tracked in the same pass that averages it.
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

func (m *EnvironmentManager) GetPhAtPoint(point utils.Point) float64 {
	return m.getCurrentPh(point)
}

func (m *EnvironmentManager) GetAveragePh() float64 {
	return m.averagePh
}

func (m *EnvironmentManager) GetPhRange() (float64, float64) {
	return m.minPh, m.maxPh
}

// AddPhChangeAtPoint adds a positive or negative value to pH, bounded by the minimum and maximum pH values provided by the config
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

func (m *EnvironmentManager) getCurrentPh(point utils.Point) float64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.currentPhMap[point.X][point.Y]
}

func (m *EnvironmentManager) getPreviousPh(point utils.Point) float64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.previousPhMap[point.X][point.Y]
}

func (m *EnvironmentManager) addUpdatedPoint(point utils.Point) {
	m.api.AddPhUpdate(point)
}

// Between cycles, swap which phMap we're treating as the 'previous' ph values and which 'current' ph map we will be updating
func (m *EnvironmentManager) updatePrevCurrentPhMaps() {
	prevPhMap := m.previousPhMap
	m.previousPhMap = m.currentPhMap
	m.currentPhMap = prevPhMap
}

// neighbourOffsets is the fixed iteration order for a cell's four cardinal neighbours, as (dx, dy) unit steps.
var neighbourOffsets = [4]utils.Point{
	{X: 0, Y: 1},  // down (+y)
	{X: 0, Y: -1}, // up
	{X: 1, Y: 0},  // right (+x)
	{X: -1, Y: 0}, // left
}

// WallPermeability is how freely pH moves through a cell.
func WallPermeability(g *c.Globals, strength int) float64 {
	if strength <= 0 {
		return 1
	}
	fraction := float64(strength) / float64(MaxWallStrength)
	if g.WallPhBlockCurve != 1 {
		fraction = math.Pow(fraction, g.WallPhBlockCurve)
	}
	return math.Max(0, 1-g.WallPhBlockAtMax*fraction)
}

// simulate diffusion of ph across the environment by adjusting each ph value toward its neighbors' values.
func (m *EnvironmentManager) diffusePhLevels() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	diffFactor := c.PhDiffuseFactor()
	g := c.GetCurrentGlobals()

	// permeabilityAt is how freely pH moves through the cell at x,y.
	permeabilityAt := func(x, y int) float64 {
		return WallPermeability(g, m.api.GetWallStrengthAtPoint(utils.Point{X: x, Y: y}))
	}

	// Mean of the adjacent points, each weighted by how freely pH gets through it.
	avgAdjPh := func(x, y int) float64 {
		weight := 0.0
		total := 0.0
		for _, off := range neighbourOffsets {
			nx := (x + off.X + gridW) % gridW
			ny := (y + off.Y + gridH) % gridH
			w := permeabilityAt(nx, ny)
			if w <= 0 {
				continue
			}
			total += m.previousPhMap[nx][ny] * w
			weight += w
		}
		if weight == 0 {
			// Sealed in on every side — keep current value untouched.
			return m.previousPhMap[x][y]
		}
		return total / weight
	}

	totalPh := 0.0
	pointCount := 0.0
	minPh, maxPh := math.Inf(1), math.Inf(-1)
	// set each value in the current phMap to its value in the previous phMap, plus the average difference between itself and its N,S,E,W neighbors (times the diffusion factor provided by the config)
	for x := 0; x < gridW; x++ {
		for y := 0; y < gridH; y++ {
			prevVal := m.previousPhMap[x][y]

			// Wall cells stay out of the water stats whatever their strength.
			isWall := m.api.IsWallAtPoint(utils.Point{X: x, Y: y})
			if !isWall {
				totalPh += prevVal
				pointCount++
				minPh, maxPh = math.Min(minPh, prevVal), math.Max(maxPh, prevVal)
			}

			// A cell follows its surroundings at the diffusion rate scaled by its own permeability.
			self := permeabilityAt(x, y)
			if self <= 0 {
				m.setPhAtPoint(utils.Point{X: x, Y: y}, prevVal)
				continue
			}
			avgAdjacentPh := avgAdjPh(x, y)
			change := (avgAdjacentPh - prevVal) * diffFactor * self
			m.setPhAtPoint(utils.Point{X: x, Y: y}, prevVal+change)
		}
	}

	if pointCount == 0 {
		// Every cell is a wall: no water to report, so leave the last figures standing rather than dividing by zero.
		return
	}
	m.averagePh = totalPh / pointCount
	m.minPh, m.maxPh = minPh, maxPh
}
