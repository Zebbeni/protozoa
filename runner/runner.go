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
	if ebiten.IsRunningSlowly() {
		return nil
	}
	ui.Render(screen)
	sim.UpdateCycle()

	return nil
	//if isDebug {
	//	var m runtime.MemStats
	//	runtime.ReadMemStats(&m)
	//	// write info to screen
	//	infoString := fmt.Sprintf("FPS: %0.2f\nAlloc = %v\nTotalAlloc = %v\nSys = %v\nNumGC = %v\nOrganisms: %d\nFood: %d\ntotalDuration: %10s\nupdateDuration: %10s\norganismUpdate: %10s\norganismResolve: %10s\nrenderDuration: %10s",
	//		ebiten.CurrentFPS(), m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC, simulation.GetNumOrganisms(), simulation.GetFoodCount(), simulation.TotalDuration(), simulation.TotalUpdateDuration(), simulation.OrganismUpdateDuration(), simulation.OrganismResolveDuration(), simulation.TotalRenderDuration())
	//	ebitenutil.DebugPrint(screen, infoString)
	//}
}

func RunSimulation(opts *c.Options) {
	r.Init()
	rand.Seed(0)

	if opts.IsHeadless {
		sumAllCycles := 0
		for count := 0; count < opts.TrialCount; count++ {
			sim = simulation.NewSimulation()
			start := time.Now()
			for !sim.IsDone() {
				sim.Update()
			}
			sumAllCycles += sim.Cycle()
			elapsed := time.Since(start)
			fmt.Printf("\nTotal runtime for simulation %d: %s, cycles: %d\n", count, elapsed, sim.Cycle())
		}
		avgCycles := sumAllCycles / opts.TrialCount
		fmt.Printf("\nAverage number of cycles to reach 5000: %d\n", avgCycles)
	} else {
		sim = simulation.NewSimulation()
		ui = ux.NewInterface(sim)
		if err := ebiten.Run(update, c.ScreenWidth(), c.ScreenHeight(), 1, "Globals"); err != nil {
			log.Fatal(err)
		}
	}
}
