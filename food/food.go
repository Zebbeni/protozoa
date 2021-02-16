package food

import (
	u "github.com/Zebbeni/protozoa/utils"
)

// Item contains an x, y coordinate and a food value
type Item struct {
	Point u.Point
	Value int
}

// NewItem creates a new food Item with a given point and value
func NewItem(point u.Point, value int) *Item {
	return &Item{
		Point: point,
		Value: value,
	}
}
