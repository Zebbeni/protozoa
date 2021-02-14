package food

import (
	"math"
	"math/rand"

	c "github.com/Zebbeni/protozoa/constants"
	u "github.com/Zebbeni/protozoa/utils"
)

// Item contains an x, y coordinate and a food value
type Item struct {
	point u.Point
	value int
}

// Point returns FoodItem's Point value
func (f *Item) Point() u.Point {
	return f.point
}

// Value returns FoodItem's current value
func (f *Item) Value() int {
	return f.value
}

// Manager contains 2D array of all food values
type Manager struct {
	Items map[string]*Item
}

// NewManager initializes a new foodItem map of MinFood
func NewManager() *Manager {
	return &Manager{Items: make(map[string]*Item)}
}

// Update is called on every cycle and adds new FoodItems at a constant rate
func (m *Manager) Update() {
	if rand.Float64() < c.ChanceToAddFoodItem {
		m.AddRandomFoodItem()
	}
}

// FoodCount returns a count of all food items in the FoodManager map
func (m *Manager) FoodCount() int {
	return len(m.Items)
}

// AddRandomFoodItem attempts to add a FoodItem object to a random location
// Gives up if first attempt to place food fails.
func (m *Manager) AddRandomFoodItem() {
	x := rand.Intn(c.GridWidth)
	y := rand.Intn(c.GridHeight)
	value := rand.Intn(c.MaxFoodValue)
	point := u.Point{X: x, Y: y}
	m.AddFoodAtPoint(point, value)
}

// AddFoodAtPoint adds a foodItem with a given value at a given location if not
// occupied. Returns the value added
func (m *Manager) AddFoodAtPoint(point u.Point, value int) int {
	if value < 0 {
		return 0
	}

	locationString := point.ToString()
	item, exists := m.Items[locationString]
	if !exists {
		value = int(math.Min(math.Max(0, float64(value)), c.MaxFoodValue))
		m.Items[locationString] = &Item{
			point: point,
			value: value,
		}
		return value
	}

	originalValue := item.value
	item.value += value
	if item.value > c.MaxFoodValue {
		item.value = c.MaxFoodValue
		return c.MaxFoodValue - originalValue
	}
	return value
}

// RemoveFoodAtPoint subtracts a given value from the Item at a given point.
// If value is more than the current food value, remove foodItem from the map
// Returns the actual amount of food removed.
func (m *Manager) RemoveFoodAtPoint(point u.Point, value int) int {
	if value < 0 {
		return 0
	}

	locationString := point.ToString()
	item, exists := m.Items[locationString]
	if !exists {
		return 0
	}

	originalValue := item.value
	item.value -= value

	if item.value < c.MinFoodValue {
		delete(m.Items, locationString)
	}

	if originalValue >= value {
		return value
	}

	return originalValue
}

// GetFoodAtPoint returns the FoodItem value at a given point (nil if none found)
func (m *Manager) GetFoodAtPoint(point u.Point) *Item {
	foodItem, _ := m.Items[point.ToString()]
	return foodItem
}

// GetFoodItems returns the current list of food items
func (m *Manager) GetFoodItems() map[string]*Item {
	return m.Items
}
