package utils

import (
	"fmt"
	"math/rand"

	c "github.com/Zebbeni/protozoa/config"
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
func GetRandomPoint(width, height int) Point {
	return Point{
		X: rand.Intn(width),
		Y: rand.Intn(height),
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

// IsWall returns true if the given point is on a pool border
func (p *Point) IsWall() bool {
	return IsWall(p.X, p.Y)
}

// IsWall returns true if some given coordinates are on a pool border, making
// sure to allow movement through 'gates' in the center of each wall.
func IsWall(x, y int) bool {
	if c.UsePools() == false {
		return false
	}

	if x%c.PoolWidth() == 0 && (y+(c.PoolHeight()/2))%c.PoolHeight() != 0 {
		return true
	}
	if y%c.PoolHeight() == 0 && (x+(c.PoolWidth()/2))%c.PoolWidth() != 0 {
		return true
	}
	return false
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
