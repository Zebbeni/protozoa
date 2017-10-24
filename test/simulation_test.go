package test

import (
	"fmt"
	"math/rand"
	"testing"

	s "../simulation"
)

func TestSimulateOneCycle(t *testing.T) {
	rand.Seed(3)
	simulation := s.NewSimulation(testSimulationConfig())
	for i := 0; i < 10; i++ {
		simulation.Update()
		fmt.Printf("simulation update: %d\n", i)
	}
}
