package utils

import (
	"fmt"
	"math/rand"

	"github.com/Zebbeni/protozoa/constants"
)

// Point contains simple X and Y coordinates for a point in space
// also usable with addition / subtraction as a directional unit vector
type Point struct {
	X, Y int
}

var (
	directionUp    = Point{X: 0, Y: -1}
	directionRight = Point{X: +1, Y: 0}
	directionDown  = Point{X: 0, Y: +1}
	directionLeft  = Point{X: -1, Y: 0}
	// Directions is a list of all possible directions
	// to travel on the simulation grid
	Directions = [...]Point{
		directionUp,
		directionRight,
		directionDown,
		directionLeft,
	}
)

// GetRandomPoint returns a random point somewhere on the simulation grid
func GetRandomPoint() Point {
	return Point{
		X: rand.Intn(constants.GridWidth),
		Y: rand.Intn(constants.GridHeight),
	}
}

// GetRandomDirection returns a point representing a random direction
func GetRandomDirection() Point {
	return Directions[rand.Intn(len(Directions))]
}

// Add add a given Point and returns the result
func (p Point) Add(toAdd Point) Point {
	return Point{X: p.X + toAdd.X, Y: p.Y + toAdd.Y}.Wrap()
}

// Times multiplies a given value and returns the result
func (p *Point) Times(toMultiply int) Point {
	return Point{
		X: (p.X * toMultiply),
		Y: (p.Y * toMultiply),
	}
}

// Wrap returns a point value after wrapping it around the grid
func (p Point) Wrap() Point {
	return Point{
		X: (p.X + constants.GridWidth) % constants.GridWidth,
		Y: (p.Y + constants.GridHeight) % constants.GridHeight,
	}
}

// ToString returns a Point's values as the string, "<x>, <y>"
func (p *Point) ToString() string {
	return fmt.Sprintf("%d,%d", p.X, p.Y)
}

// Right returns the direction to the right of the current direction d
func (p Point) Right() (right Point) {
	switch p {
	case directionUp:
		right = directionRight
		break
	case directionRight:
		right = directionDown
		break
	case directionDown:
		right = directionLeft
		break
	case directionLeft:
		right = directionUp
		break
	}
	return
}

// Left returns the direction to the right of the current direction d
func (p Point) Left() (left Point) {
	switch p {
	case directionUp:
		left = directionLeft
		break
	case directionRight:
		left = directionUp
		break
	case directionDown:
		left = directionRight
		break
	case directionLeft:
		left = directionDown
		break
	}
	return
}
