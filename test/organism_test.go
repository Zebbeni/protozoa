package test

import (
	"fmt"
	"math/rand"
	"testing"

	s "github.com/Zebbeni/protozoa/simulation"
)

func TestOrganismDecisionTree(t *testing.T) {
	rand.Seed(3)
	simulation := s.NewSimulation(testSimulationConfig())
	for i := 0; i < 1; i++ {
		simulation.Update()
		fmt.Printf("simulation update: %d\n", i)
	}
}
