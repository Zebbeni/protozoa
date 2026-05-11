package manager

import (
	"math"
	"sync"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// FoodManager contains 2D array of all food values
type FoodManager struct {
	api           food.API
	rng           *simrand.RNG
	Items         map[utils.Point]*food.Item
	isInitialized bool

	mutex sync.RWMutex
}

// NewFoodManager initializes a new foodItem map of MinFood
func NewFoodManager(api food.API, rng *simrand.RNG) *FoodManager {
	m := &FoodManager{
		api:           api,
		rng:           rng,
		Items:         make(map[utils.Point]*food.Item),
		isInitialized: false,
	}
	m.InitializeFood(config.InitialFood())
	return m
}

func (m *FoodManager) InitializeFood(count int) {
	for i := 0; i < count; i++ {
		m.AddRandomFoodItem()
	}
}

// Update is called on every cycle and adds new FoodItems at a constant rate
func (m *FoodManager) Update() {
	if m.rng.Float64() < config.ChanceToAddFoodItem() {
		m.AddRandomFoodItem()
	}
}

// FoodCount returns a count of all food items in the FoodManager map
func (m *FoodManager) FoodCount() int {
	return len(m.Items)
}

// AddRandomFoodItem attempts to add a FoodItem object to a random location
// Gives up if first attempt to place food fails.
func (m *FoodManager) AddRandomFoodItem() {
	x := m.rng.Intn(config.GridUnitsWide())
	y := m.rng.Intn(config.GridUnitsHigh())
	value := m.rng.Intn(config.MaxFoodValue())
	point := utils.Point{X: x, Y: y}
	m.addFood(point, value)
}

// AddFoodAtPoint adds a foodItem with a given value at a given location if not
// occupied, or adds food to the existing food item there (up to maximum allowed)
func (m *FoodManager) AddFoodAtPoint(point utils.Point, value int) {
	m.addFood(point, value)
}

// RemoveFoodAtPoint subtracts a given value from the Item at a given point.
// If value is more than the current food value, remove foodItem from the map
func (m *FoodManager) RemoveFoodAtPoint(point utils.Point, value int) {
	m.removeFood(point, value)
}

// GetFoodAtPoint returns the FoodItem value at a given point (nil if none found)
func (m *FoodManager) GetFoodAtPoint(point utils.Point) (*food.Item, bool) {
	return m.getFood(point)
}

// GetFoodItems returns the current list of food items
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

func (m *FoodManager) addFood(point utils.Point, value int) {
	if value <= 0 || m.api.IsWallAtPoint(point) {
		return
	}

	m.mutex.Lock()
	item, exists := m.Items[point]
	if exists {
		value += item.Value
	}
	value = int(math.Min(math.Max(0.0, float64(value)), float64(config.MaxFoodValue())))
	m.Items[point] = food.NewItem(point, value)
	m.mutex.Unlock()

	m.addUpdatedPoint(point)
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
