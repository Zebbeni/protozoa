package manager

import (
	"math"
	"math/rand"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/food"
	u "github.com/Zebbeni/protozoa/utils"
)

// FoodManager contains 2D array of all food values
type FoodManager struct {
	*config.Protozoa

	api food.API

	Items map[string]*food.Item
}

// NewFoodManager initializes a new foodItem map of MinFood
func NewFoodManager(api food.API, protozoa *config.Protozoa) *FoodManager {
	m := &FoodManager{
		api:   api,
		Items: make(map[string]*food.Item),
	}
	m.Protozoa = protozoa
	return m
}

// Update is called on every cycle and adds new FoodItems at a constant rate
func (m *FoodManager) Update() {
	if rand.Float64() < m.ChanceToAddFoodItem {
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
	x := rand.Intn(m.GridUnitsWide)
	y := rand.Intn(m.GridUnitsHigh)
	value := rand.Intn(m.MaxFoodValue)
	point := u.Point{X: x, Y: y}
	if added := m.AddFoodAtPoint(point, value); added > 0 {
		m.api.AddGridPointToUpdate(point)
	}
}

// AddFoodAtPoint adds a foodItem with a given value at a given location if not
// occupied. Returns the value added
func (m *FoodManager) AddFoodAtPoint(point u.Point, value int) int {
	if value <= 0 {
		return 0
	}

	m.api.AddGridPointToUpdate(point)

	locationString := point.ToString()
	item, exists := m.Items[locationString]
	if !exists {
		value = int(math.Min(math.Max(0.0, float64(value)), float64(m.MaxFoodValue)))
		m.Items[locationString] = food.NewItem(point, value)
		return value
	}

	originalValue := item.Value
	item.Value += value
	if item.Value > m.MaxFoodValue {
		item.Value = m.MaxFoodValue
		return m.MaxFoodValue - originalValue
	}
	return value
}

// RemoveFoodAtPoint subtracts a given value from the Item at a given point.
// If value is more than the current food value, remove foodItem from the map
// Returns the actual amount of food removed.
func (m *FoodManager) RemoveFoodAtPoint(point u.Point, value int) int {
	if value <= 0 {
		return 0
	}

	locationString := point.ToString()
	item, exists := m.Items[locationString]
	if !exists {
		return 0
	}

	m.api.AddGridPointToUpdate(point)

	originalValue := item.Value
	item.Value -= value

	if item.Value < m.MinFoodValue {
		delete(m.Items, locationString)
	}

	if originalValue >= value {
		return value
	}

	return originalValue
}

// GetFoodAtPoint returns the FoodItem value at a given point (nil if none found)
func (m *FoodManager) GetFoodAtPoint(point u.Point) *food.Item {
	foodItem, _ := m.Items[point.ToString()]
	return foodItem
}

// GetFoodItems returns the current list of food items
func (m *FoodManager) GetFoodItems() map[string]*food.Item {
	return m.Items
}
