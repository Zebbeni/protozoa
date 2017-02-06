package simulation

import (
	"math/rand"
	"models"
)

const NUM_PROTISTS = 10
const NUM_CYCLES = 100

func generateProtists(num_to_generate int, environment *models.Environment) []models.Protist {
	protists := make([]models.Protist, num_to_generate)
	for i := 0; i < num_to_generate; i++ {
		newPro := generateProtist(i, environment)
		protists[i] = newPro
	}
	return protists
}

func generateProtist(num int, environment *models.Environment) models.Protist {
	newProtist := models.Protist{
		ID:          num,
		Health:      100,
		Food:        100,
		Days_lived:  0,
		Covered:     false,
		Alive:       true,
		Environment: environment,
	}
	newProtist.GenerateSequence()
	newProtist.GenerateActionFromSequence()
	return newProtist
}

func generateEnvironment() *models.Environment {
	startingTemperature := rand.Intn(101)
	startingNumDead := 0
	environment := &models.Environment{
		Temperature: startingTemperature,
		GoodWeather: 0,
		BadWeather:  0,
		NumDead:     startingNumDead,
	}
	return environment
}

type Simulation struct{
	Protists 	[]models.Protist
	Environment 	*models.Environment
	NumCycles 	int
}

func NewSimulation() *Simulation {
	rand.Seed(12)
	environment := generateEnvironment()
	protists := generateProtists(NUM_PROTISTS, environment)
	simulation := &Simulation{
		Protists: protists,
		Environment: environment,
		NumCycles: NUM_CYCLES,
	}
	return simulation
}

func (s *Simulation) DoCycle() {

	s.Environment.UpdateEnvironment()
	for _, p := range s.Protists {
		p.DoCycle()
	}
	s.NumCycles++
}
