package models

import (
	"math"
	"math/rand"

	c "../constants"
)

// Force contains a location, angle and magnitude (including x and y magnitude)
type Force struct {
	X, Y, angle, Force, ForceX, ForceY float64
}

// Update changes angle by a small amount and calculates force magnitude's
// x and y components
func (f *Force) Update() {
	f.angle += rand.Float64()
	f.Force = math.Max(0, math.Min(f.Force+rand.Float64()-0.5, c.MaxForce))
	f.ForceX = math.Cos(f.angle) * f.Force
	f.ForceY = math.Sin(f.angle) * f.Force
}

// NewForce returns a new Force object randomly placed anywhere on the screen
// with random angle and force magnitude
func NewForce() Force {
	x := rand.Float64() * c.ScreenWidth
	y := rand.Float64() * c.ScreenHeight
	angle := rand.Float64() * 2 * math.Pi
	force := rand.Float64() * c.MaxForce
	newForce := Force{X: x, Y: y, angle: angle, Force: force}
	newForce.Update()
	return newForce
}
