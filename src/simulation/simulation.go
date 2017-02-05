package simulation

import (
	"fmt"
	"math/rand"
	"models"
	"net/http"
)

const NUM_PROTISTS = 100
const NUM_CYCLES = 1000

var protists [NUM_PROTISTS]*models.Protist

func generateProtist(num int, environment *models.Environment) *models.Protist {
	newProtist := &models.Protist{
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

func RunSimulation(w http.ResponseWriter) {
	rand.Seed(12)

	environment := generateEnvironment()

	// create a ton of random protists
	for i := 0; i < NUM_PROTISTS; i++ {
		newPro := generateProtist(i, environment)
		protists[i] = newPro
	}

	for cycle := 0; cycle < NUM_CYCLES; cycle++ {
		fmt.Println("\n\nDay", cycle+1)
		environment.UpdateEnvironment()
		for _, p := range protists {
			p.DoCycle()
		}
		fmt.Println("\nStill alive: ")
		for _, p := range protists {
			if p.Alive {
				fmt.Print(" models.Protist ", p.ID, ", ")
			}
		}
		if environment.NumDead >= NUM_PROTISTS {
			cycle = NUM_CYCLES
		}
	}
	fmt.Println("\nDays of Bad weather: ", environment.BadWeather)
	fmt.Println("Days of Good weather: ", environment.GoodWeather)
	for i, p := range protists {
		fmt.Fprintf(w, "<p>Protist %d\tDays lived: %d\t\tSequence: %v</p>", i, p.Days_lived, p.Sequence)
	}

	// print details of last surviving protists
}
