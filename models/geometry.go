package models

import "fmt"

// Point contains simple X and Y coordinates for a point in space
type Point struct {
	X, Y int
}

func (p *Point) toString() string {
	return fmt.Sprintf("%d,%d", p.X, p.Y)
}
