package runner

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten"

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
	// update simulation every time. Only re-render if not running slowly
	sim.Update()
	if ebiten.IsDrawingSkipped() {
		return nil
	}
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
		sim = simulation.NewSimulation(opts)
		ui = ux.NewInterface(sim)
		if err := ebiten.Run(update, c.ScreenWidth(), c.ScreenHeight(), 1, "Protozoa"); err != nil {
			log.Fatal(err)
		}
	}
}
