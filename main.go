package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"time"

	c "github.com/Zebbeni/protozoa/constants"
	r "github.com/Zebbeni/protozoa/resources"
	s "github.com/Zebbeni/protozoa/simulation"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

var (
	filter     = ebiten.FilterLinear
	simulation s.Simulation

	isDebug bool
)

func update(screen *ebiten.Image) error {
	// update simulation every time. Only re-render if not running slowly
	simulation.Update()
	if ebiten.IsRunningSlowly() {
		return nil
	}
	simulation.Render(screen)

	if isDebug {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		// write info to screen
		infoString := fmt.Sprintf("FPS: %0.2f\nAlloc = %v\nTotalAlloc = %v\nSys = %v\nNumGC = %v\nOrganisms: %d\nFood: %d\ntotalDuration: %10s\nupdateDuration: %10s\norganismUpdate: %10s\norganismResolve: %10s\nrenderDuration: %10s",
			ebiten.CurrentFPS(), m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC, simulation.GetNumOrganisms(), simulation.GetFoodCount(), simulation.TotalDuration(), simulation.TotalUpdateDuration(), simulation.OrganismUpdateDuration(), simulation.OrganismResolveDuration(), simulation.TotalRenderDuration())
		ebitenutil.DebugPrint(screen, infoString)
	}
	return nil
}

func main() {
	r.Init()
	rand.Seed(0)

	isHeadless := flag.Bool("headless", false, "Run simulation without visualization")
	isDebugging := flag.Bool("debug", false, "Run simulation and display debug statistics")
	trials := flag.Int("trials", 1, "Number of trials to run")
	flag.Parse()

	numTrials := *trials
	isDebug = *isDebugging

	if *isHeadless {
		sumAllCycles := 0
		for count := 0; count < numTrials; count++ {
			simulation = s.NewSimulation()
			start := time.Now()
			for !simulation.IsDone() {
				simulation.Update()
			}
			sumAllCycles += simulation.NumCycles()
			elapsed := time.Since(start)
			fmt.Printf("\nTotal runtime for simulation %d: %s, cycles: %d\n", count, elapsed, simulation.NumCycles())
		}
		avgCycles := sumAllCycles / numTrials
		fmt.Printf("\nAverage number of cycles to reach 5000: %d\n", avgCycles)
	} else {
		simulation = s.NewSimulation()
		if err := ebiten.Run(update, c.ScreenWidth, c.ScreenHeight, 1, "Protozoa"); err != nil {
			log.Fatal(err)
		}
	}
}
