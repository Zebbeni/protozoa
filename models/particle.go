package models

import (
	"math"
	"math/rand"

	c "../constants"
)

// Particle contains a location, previous location, and current velocity
type Particle struct {
	X, Y, PrevX, PrevY, vX, vY float64
}

// Update calcuates particle velocity and location based on surrounding forces
func (p *Particle) Update(forces [c.NumForces]Force) {
	// if particle if offscreen, re-center and muffle current velocity
	if p.X < 0 || p.Y < 0 || p.X > c.ScreenWidth || p.Y > c.ScreenHeight {
		p.X = c.ScreenWidth / 2.0
		p.Y = c.ScreenHeight / 2.0
		p.vX *= 0.1
		p.vY *= 0.1
	}
	// update velocity
	for _, force := range forces {
		xDist := force.X - p.X
		yDist := force.Y - p.Y
		forceDist := math.Sqrt(math.Pow(xDist, 2) + math.Pow(yDist, 2))
		// give nearby forces more influence on velocity
		p.vX += force.ForceX / forceDist
		p.vY += force.ForceY / forceDist
	}
	p.PrevX = p.X
	p.PrevY = p.Y
	p.X += p.vX
	p.Y += p.vY
}

// NewParticle returns a new Particle object randomly placed within 1.0 of the
// center of the screen
func NewParticle() Particle {
	x := (rand.Float64() * c.ScreenWidth) + rand.Float64()
	y := (rand.Float64() * c.ScreenHeight) + rand.Float64()
	newParticle := Particle{X: x, Y: y, PrevX: x, PrevY: y, vX: 0.0, vY: 0.0}
	return newParticle
}
