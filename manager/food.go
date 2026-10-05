package manager

import (
	"math"
	"sync"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

type FoodManager struct {
	api           food.API
	rng           *simrand.RNG
	Items         map[utils.Point]*food.Item
	isInitialized bool

	// Buried is the food that has settled out of reach, by cell.
	Buried map[utils.Point]int

	mutex sync.RWMutex
}

func NewFoodManager(api food.API, rng *simrand.RNG) *FoodManager {
	m := &FoodManager{
		api:           api,
		rng:           rng,
		Items:         make(map[utils.Point]*food.Item),
		Buried:        make(map[utils.Point]int),
		isInitialized: false,
	}
	m.InitializeFood(config.InitialFood())
	m.InitializeBuriedFood(config.InitialBuriedFood())
	return m
}

// InitializeBuriedFood seeds the buried layer so digging has something to find before burial has stocked the ground.
func (m *FoodManager) InitializeBuriedFood(count int) {
	if count <= 0 {
		return
	}
	lo, hi := config.MinInitialBuriedValue(), config.MaxInitialBuriedValue()
	if hi < lo {
		lo, hi = hi, lo
	}
	if hi <= 0 {
		return
	}
	if lo < 1 {
		lo = 1
	}
	span := hi - lo + 1
	for i := 0; i < count; i++ {
		point := utils.Point{
			X: m.rng.Intn(config.GridUnitsWide()),
			Y: m.rng.Intn(config.GridUnitsHigh()),
		}
		value := lo + m.rng.Intn(span)
		if m.api.IsWallAtPoint(point) {
			// Buried under a wall is unreachable: nothing can stand there to dig it.
			continue
		}
		m.mutex.Lock()
		m.Buried[point] += value
		if maxV := config.MaxBuriedFoodValue(); m.Buried[point] > maxV {
			m.Buried[point] = maxV
		}
		m.mutex.Unlock()
		m.addUpdatedPoint(point)
	}
}

func (m *FoodManager) InitializeFood(count int) {
	for i := 0; i < count; i++ {
		m.AddRandomFoodItem()
	}
}

func (m *FoodManager) Update(cycle int) {
	if m.rng.Float64() < config.ChanceToAddFoodItem() {
		m.AddRandomFoodItem()
	}
	interval := config.BurialInterval()
	if interval > 0 && cycle%interval == 0 {
		m.BuryFood(config.BurialAmount())
	}
}

// BuryFood moves up to amount out of every food pile into the buried layer beneath it.
func (m *FoodManager) BuryFood(amount int) {
	if amount <= 0 {
		return
	}
	maxBuried := config.MaxBuriedFoodValue()

	m.mutex.Lock()
	buried := make([]utils.Point, 0, len(m.Items))
	for point, item := range m.Items {
		room := maxBuried - m.Buried[point]
		if room <= 0 {
			continue
		}
		moved := amount
		if item.Value < moved {
			moved = item.Value
		}
		if room < moved {
			moved = room
		}
		if moved <= 0 {
			continue
		}
		item.Value -= moved
		m.Buried[point] += moved
		if item.Value <= 0 {
			delete(m.Items, point)
		}
		buried = append(buried, point)
	}
	m.mutex.Unlock()

	for _, point := range buried {
		m.addUpdatedPoint(point)
	}
}

// Unbury moves up to amount from the buried layer at a point back into the food layer and returns how much actually moved.
func (m *FoodManager) Unbury(point utils.Point, amount int) int {
	if amount <= 0 {
		return 0
	}
	m.mutex.RLock()
	available := m.Buried[point]
	m.mutex.RUnlock()
	if available <= 0 {
		return 0
	}
	if amount > available {
		amount = available
	}

	moved := m.placeFood(point, amount)
	if moved <= 0 {
		return 0
	}

	m.mutex.Lock()
	m.Buried[point] -= moved
	if m.Buried[point] <= 0 {
		delete(m.Buried, point)
	}
	m.mutex.Unlock()
	return moved
}

// BuryAllAt pushes every unit of food at a point into the buried layer and returns how much moved.
func (m *FoodManager) BuryAllAt(point utils.Point) int {
	m.mutex.Lock()
	item, exists := m.Items[point]
	if !exists || item.Value <= 0 {
		m.mutex.Unlock()
		return 0
	}
	moved := item.Value
	delete(m.Items, point)
	m.Buried[point] += moved
	m.mutex.Unlock()

	m.addUpdatedPoint(point)
	return moved
}

func (m *FoodManager) GetBuriedFoodAtPoint(point utils.Point) int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.Buried[point]
}

func (m *FoodManager) GetBuriedFood() map[utils.Point]int {
	return m.Buried
}

func (m *FoodManager) BuriedFoodCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.Buried)
}

func (m *FoodManager) FoodCount() int {
	return len(m.Items)
}

// AddRandomFoodItem attempts to add a FoodItem object to a random location Gives up if first attempt to place food fails.
func (m *FoodManager) AddRandomFoodItem() {
	x := m.rng.Intn(config.GridUnitsWide())
	y := m.rng.Intn(config.GridUnitsHigh())
	value := m.rng.Intn(config.MaxFoodValue())
	point := utils.Point{X: x, Y: y}
	m.addFood(point, value)
}

// AddFoodAtPoint adds a foodItem with a given value at a given location if it is free, or adds to the food already there (up to the maximum allowed).
func (m *FoodManager) AddFoodAtPoint(point utils.Point, value int) {
	m.addFood(point, value)
}

// RemoveFoodAtPoint subtracts a given value from the Item at a given point.
func (m *FoodManager) RemoveFoodAtPoint(point utils.Point, value int) {
	m.removeFood(point, value)
}

// GetFoodAtPoint returns the FoodItem value at a given point (nil if none found)
func (m *FoodManager) GetFoodAtPoint(point utils.Point) (*food.Item, bool) {
	return m.getFood(point)
}

func (m *FoodManager) GetFoodItems() map[utils.Point]*food.Item {
	return m.Items
}

func (m *FoodManager) removeFood(point utils.Point, value int) {
	if value <= 0 || m.api.IsWallAtPoint(point) {
		return
	}

	m.mutex.RLock()
	item, exists := m.Items[point]
	m.mutex.RUnlock()

	if !exists {
		return
	}

	item.Value -= value
	if item.Value <= config.MinFoodValue() {
		m.mutex.Lock()
		delete(m.Items, point)
		m.mutex.Unlock()
	}

	m.addUpdatedPoint(point)
}

// addFood is the one place food enters the world, so it is where the no-food-under-an-organism rule is enforced.
func (m *FoodManager) addFood(point utils.Point, value int) {
	if m.api.IsOrganismAtPoint(point) {
		return
	}
	m.placeFood(point, value)
}

// placeFood is the insert itself, without the no-food-under-an-organism guard.
func (m *FoodManager) placeFood(point utils.Point, value int) int {
	if value <= 0 || m.api.IsWallAtPoint(point) {
		return 0
	}

	m.mutex.Lock()
	before := 0
	if item, exists := m.Items[point]; exists {
		before = item.Value
	}
	total := int(math.Min(math.Max(0.0, float64(before+value)), float64(config.MaxFoodValue())))
	m.Items[point] = food.NewItem(point, total)
	m.mutex.Unlock()

	m.addUpdatedPoint(point)
	return total - before
}

func (m *FoodManager) getFood(point utils.Point) (*food.Item, bool) {
	m.mutex.RLock()
	item, found := m.Items[point]
	m.mutex.RUnlock()

	return item, found
}

func (m *FoodManager) addUpdatedPoint(point utils.Point) {
	m.api.AddFoodUpdate(point)
}
