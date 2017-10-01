package simulation

import (
	"image/color"

	c "../constants"
	m "../models"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

var forceColor = color.RGBA{0x80, 0x80, 0xFF, 0xFF}
var particleColor = color.RGBA{0x80, 0xFF, 0x80, 0xFF}

// Simulation contains a list of forces, particles, and drawing settings
type Simulation struct {
	forces    [c.NumForces]m.Force
	particles [c.NumParticles]m.Particle
}

// NewSimulation returns a simulation with generated particles and forces
func NewSimulation() Simulation {
	var particles [c.NumParticles]m.Particle
	for i := 0; i < c.NumParticles; i++ {
		particles[i] = m.NewParticle()
	}
	var forces [c.NumForces]m.Force
	for i := 0; i < c.NumForces; i++ {
		forces[i] = m.NewForce()
	}
	simulation := Simulation{particles: particles, forces: forces}
	return simulation
}

// Update calls Update functions for all particles and forces in simulation
func (s *Simulation) Update() {
	for p := range s.particles {
		s.particles[p].Update(s.forces)
	}
	for f := range s.forces {
		s.forces[f].Update()
	}
}

// Render draws all particles and forces to the screen
func (s *Simulation) Render(screen *ebiten.Image) {
	for _, force := range s.forces {
		renderForce(force, screen)
	}
	for _, particle := range s.particles {
		renderParticle(particle, screen)
	}
}

// renderForce draws a force to the screen
func renderForce(f m.Force, screen *ebiten.Image) {
	size := c.MaxForceSize * (f.Force / c.MaxForce)
	ebitenutil.DrawRect(screen, f.X-(size/2.0), f.Y-(size/2.0), size, size, forceColor)
	ebitenutil.DrawLine(screen, f.X, f.Y, f.X+f.ForceX*5.0, f.Y+f.ForceY*5.0, forceColor)
}

// renderParticle draws a particle to the screen
func renderParticle(p m.Particle, screen *ebiten.Image) {
	size := c.ParticleSize
	ebitenutil.DrawRect(screen, p.X-(size/2.0), p.Y-(size/2.0), size, size, particleColor)
}
