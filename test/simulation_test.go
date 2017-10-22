package test

import (
	"testing"

	s "../simulation"
)

// return a simple simulation with a small number of organisms,
// on a small grid with very little food
func newTestSimulation() *s.Simulation {
	simulation := s.NewSimulation()
	return &simulation
}

func TestSimulateOneCycle(t *testing.T) {
	simulation := newTestSimulation()
}
