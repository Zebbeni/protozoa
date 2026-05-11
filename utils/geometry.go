package utils

import (
	"fmt"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

// Point contains simple X and Y coordinates for a point in space
// also usable with addition / subtraction as a directional unit vector
type Point struct {
	X, Y int
}

func (p Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
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
func GetRandomPoint(rng *simrand.RNG, width, height int) Point {
	return Point{
		X: rng.Intn(width),
		Y: rng.Intn(height),
	}
}

// GetRandomDirection returns a point representing a random direction
func GetRandomDirection(rng *simrand.RNG) Point {
	return Directions[rng.Intn(len(Directions))]
}

// Add add a given Point and returns the result
func (p Point) Add(toAdd Point) Point {
	return Point{X: p.X + toAdd.X, Y: p.Y + toAdd.Y}.Wrap()
}

// Sub subtracts a given Point and returns the wrapped result.
func (p Point) Sub(toSub Point) Point {
	return Point{X: p.X - toSub.X, Y: p.Y - toSub.Y}.Wrap()
}

// Times multiplies a given value and returns the result
func (p *Point) Times(toMultiply int) Point {
	return Point{
		X: p.X * toMultiply,
		Y: p.Y * toMultiply,
	}
}

// Wrap returns a point value after wrapping it around the grid
func (p Point) Wrap() Point {
	return Point{
		X: (p.X + c.GridUnitsWide()) % c.GridUnitsWide(),
		Y: (p.Y + c.GridUnitsHigh()) % c.GridUnitsHigh(),
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
