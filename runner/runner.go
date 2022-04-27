package runner

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/ux"
	"github.com/hajimehoshi/ebiten/v2"
)

type Runner struct {
	sim *simulation.Simulation
	ui  *ux.Interface

	pressedKeys map[ebiten.Key]bool
}

func (r *Runner) Update() error {
	r.handleUserInput()
	r.sim.Update()
	return nil
}

func (r *Runner) handleUserInput() {
	r.ui.HandleUserInput()
}

func (r *Runner) Draw(screen *ebiten.Image) {
	r.ui.Render(screen)
	r.sim.ClearUpdatedPoints()
}

func (r *Runner) Layout(_, _ int) (int, int) {
	return c.ScreenWidth(), c.ScreenHeight()
}

func RunSimulation(opts *c.Options) {
	resources.Init()
	rand.Seed(5)

	if opts.IsHeadless {
		sumAllCycles := 0
		for count := 0; count < opts.TrialCount; count++ {
			sim := simulation.NewSimulation(opts)
			start := time.Now()
			for !sim.IsDone() {
				sim.Update()
				if sim.Cycle()%100 == 0 {
					fmt.Println("cycle:", sim.Cycle(), "organisms:", sim.OrganismCount())
				}
			}
			sumAllCycles += sim.Cycle()
			elapsed := time.Since(start)
			fmt.Printf("\nTotal runtime for simulation %d: %s, cycles: %d\n", count, elapsed, sim.Cycle())
		}
		avgCycles := sumAllCycles / opts.TrialCount
		fmt.Printf("\nAverage number of cycles to reach 5000: %d\n", avgCycles)
	} else {
		sim := simulation.NewSimulation(opts)
		//for sim.Cycle() < -1 {
		//	sim.Update()
		//	fmt.Printf("\nCycle: %5d, organisms: %5d", sim.Cycle(), sim.OrganismCount())
		//}

		ui := ux.NewInterface(sim)
		gameRunner := &Runner{
			sim:         sim,
			ui:          ui,
			pressedKeys: map[ebiten.Key]bool{},
		}

		ebiten.SetWindowResizable(true)
		ebiten.SetScreenClearedEveryFrame(false)
		if err := ebiten.RunGame(gameRunner); err != nil {
			log.Fatal(err)
		}
	}
}
