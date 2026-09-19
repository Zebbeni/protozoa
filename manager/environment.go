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
// Walls slow diffusion rather than stopping it, in proportion to their
// strength: a cell's permeability runs from 1 (open water) down to 0 at
// MaxWallStrength. That governs both directions — how fast the cell's
// own pH follows its surroundings, and how much it contributes to its
// neighbours' averages — so a flimsy wall is very nearly water and only
// a full-strength one seals completely.
//
// A wall used to be excluded outright, which made every wall in the
// world an absolute barrier whatever it was made of, and meant digging
// one down changed nothing at all until the last point came off. With
// permeability the wall gets weaker as a barrier as it is worn away,
// and a destroyed one rejoins diffusion with no special unfreeze step.
func (m *EnvironmentManager) diffusePhLevels() {
	gridW, gridH := c.GridUnitsWide(), c.GridUnitsHigh()
	diffFactor := c.PhDiffuseFactor()

	// permeabilityAt is how freely pH moves through a cell: 1 in open
	// water, falling linearly to 0 at MaxWallStrength.
	permeabilityAt := func(x, y int) float64 {
		strength := m.api.GetWallStrengthAtPoint(utils.Point{X: x, Y: y})
		if strength <= 0 {
			return 1
		}
		return math.Max(0, 1-float64(strength)/float64(MaxWallStrength))
	}

	// Mean of the adjacent points, each weighted by how freely pH gets
	// through it. Open water weighs 1, so a cell with no walls around
	// it gets exactly the plain mean it always did.
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

			// Wall cells stay out of the water stats whatever their
			// strength: organisms can't be in one, so counting them
			// would report pH nothing lives in. They still diffuse.
			isWall := m.api.IsWallAtPoint(utils.Point{X: x, Y: y})
			if !isWall {
				totalPh += prevVal
				pointCount++
				minPh, maxPh = math.Min(minPh, prevVal), math.Max(maxPh, prevVal)
			}

			// A cell follows its surroundings at the diffusion rate scaled
			// by its own permeability, so a strong wall barely moves and
			// open water moves at the full rate.
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
		// Every cell is a wall: no water to report, so leave the last
		// figures standing rather than dividing by zero.
		return
	}
	m.averagePh = totalPh / pointCount
	m.minPh, m.maxPh = minPh, maxPh
}
