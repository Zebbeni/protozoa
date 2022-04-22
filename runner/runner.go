package runner

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/ux"
)

var (
	sim *simulation.Simulation
	ui  *ux.Interface
)

func update(screen *ebiten.Image) error {
	sim.Update()
	ui.Render(screen)
	sim.UpdateCycle()
	return nil
}

func RunSimulation(opts *c.Options) {
	r.Init()
	rand.Seed(1)

	if opts.IsHeadless {
		sumAllCycles := 0
		for count := 0; count < opts.TrialCount; count++ {
			sim = simulation.NewSimulation(opts)
			start := time.Now()
			for !sim.IsDone() {
				sim.Update()
				if sim.Cycle()%100 == 0 {
					fmt.Println("cycle:", sim.Cycle(), "organisms:", sim.OrganismCount())
				}
				sim.UpdateCycle()
			}
			sumAllCycles += sim.Cycle()
			elapsed := time.Since(start)
			fmt.Printf("\nTotal runtime for simulation %d: %s, cycles: %d\n", count, elapsed, sim.Cycle())
		}
		avgCycles := sumAllCycles / opts.TrialCount
		fmt.Printf("\nAverage number of cycles to reach 5000: %d\n", avgCycles)
	} else {
		// We need to define a game object that satisfies the ebiten 'Game' interface, with the
		// update as its Update function and
		// and then call RunGame on that game object
		// game :=

		sim = simulation.NewSimulation(opts)
		ui = ux.NewInterface(sim)
		//if err := ebiten.Run(update, c.ScreenWidth(), c.ScreenHeight(), 1, "Protozoa"); err != nil {
		//	log.Fatal(err)
		//}
		if err := ebiten.RunGame(update, c.ScreenWidth(), c.ScreenHeight(), 1, "Protozoa"); err != nil {
			log.Fatal(err)
		}
	}
}
