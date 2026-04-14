package runner

import (
	"fmt"
	"log"
	"time"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/ux"
	"github.com/hajimehoshi/ebiten/v2"
)

type runnerState int

const (
	stateConfigScreen runnerState = iota
	stateRunning
)

type Runner struct {
	opts         *c.Options
	state        runnerState
	configScreen *ux.ConfigScreen
	sim          *simulation.Simulation
	ui           *ux.Interface
	pressedKeys  map[ebiten.Key]bool
}

func (r *Runner) Update() error {
	switch r.state {
	case stateConfigScreen:
		if r.configScreen.Update() {
			// User accepted — apply config and start simulation
			globals := r.configScreen.Globals()
			c.SetGlobals(globals)
			resources.Init()
			ebiten.SetScreenClearedEveryFrame(false)
			r.startSimulation()
		}
	case stateRunning:
		r.ui.HandleUserInput()
		r.sim.Update()
		r.ui.UpdateSelected()
	}
	return nil
}

func (r *Runner) Draw(screen *ebiten.Image) {
	switch r.state {
	case stateConfigScreen:
		r.configScreen.Draw(screen)
	case stateRunning:
		r.ui.Render(screen)
		r.sim.ClearUpdatedPoints()
	}
}

func (r *Runner) Layout(outsideWidth, outsideHeight int) (int, int) {
	if outsideWidth != c.ScreenWidth() || outsideHeight != c.ScreenHeight() {
		globals := c.GetCurrentGlobals()
		globals.ScreenWidth = outsideWidth
		globals.ScreenHeight = outsideHeight
		c.SetGlobals(globals)
		if r.ui != nil {
			r.ui.OnResize()
		}
	}
	return outsideWidth, outsideHeight
}

func (r *Runner) startSimulation() {
	r.sim = simulation.NewSimulation(r.opts)
	r.ui = ux.NewInterface(r.sim)
	r.state = stateRunning
}

func RunSimulation(opts *c.Options) {
	resources.Init()

	if opts.IsHeadless {
		sumAllCycles := 0
		for count := 0; count < opts.TrialCount; count++ {
			sim := simulation.NewSimulation(opts)
			start := time.Now()
			for !sim.IsDone() {
				sim.Update()
				if sim.Cycle()%100 == 0 {
					fmt.Printf("\nCycle: %6d   Organisms: %d   AvgPh: %2.2f", sim.Cycle(), sim.OrganismCount(), sim.AveragePh())
				}
			}
			sumAllCycles += sim.Cycle()
			elapsed := time.Since(start)
			fmt.Printf("\nTotal runtime for simulation %d: %s, cycles: %d\n", count, elapsed, sim.Cycle())
		}
		avgCycles := sumAllCycles / opts.TrialCount
		fmt.Printf("\nAverage number of cycles to reach 5000: %d\n", avgCycles)
	} else {
		// If a config file was specified, skip the config screen
		if opts.ConfigFile != "" {
			gameRunner := &Runner{
				opts:        opts,
				state:       stateRunning,
				pressedKeys: map[ebiten.Key]bool{},
			}
			gameRunner.sim = simulation.NewSimulation(opts)
			gameRunner.ui = ux.NewInterface(gameRunner.sim)

			ebiten.SetWindowResizable(true)
			ebiten.SetScreenClearedEveryFrame(false)
			if err := ebiten.RunGame(gameRunner); err != nil {
				log.Fatal(err)
			}
		} else {
			// Show config screen first
			globals := c.GetDefaultGlobals()
			gameRunner := &Runner{
				opts:         opts,
				state:        stateConfigScreen,
				configScreen: ux.NewConfigScreen(&globals),
				pressedKeys:  map[ebiten.Key]bool{},
			}

			ebiten.SetWindowResizable(true)
			ebiten.SetScreenClearedEveryFrame(true)
			if err := ebiten.RunGame(gameRunner); err != nil {
				log.Fatal(err)
			}
		}
	}
}
