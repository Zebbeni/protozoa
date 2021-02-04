package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"runtime"
	"time"

	c "github.com/Zebbeni/protozoa/constants"
	s "github.com/Zebbeni/protozoa/simulation"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

var (
	black      = color.RGBA{0, 0, 0, 255}
	filter     = ebiten.FilterLinear
	config     s.Config
	simulation s.Simulation
)

func update(screen *ebiten.Image) error {
	// update simulation every time. Only re-render if not running slowly
	simulation.Update()

	if ebiten.IsRunningSlowly() {
		return nil
	}
	screen.Clear()
	ebitenutil.DrawRect(screen, 0, 0, c.ScreenWidth, c.ScreenHeight, black)
	simulation.Render(screen)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// write info to screen
	infoString := fmt.Sprintf("FPS: %0.2f\nAlloc = %v\nTotalAlloc = %v\nSys = %v\nNumGC = %v\nOrganisms: %d",
		ebiten.CurrentFPS(), m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC, simulation.GetNumOrganisms())
	ebitenutil.DebugPrint(screen, infoString)
	return nil
}

func main() {
	rand.Seed(1)

	isHeadless := flag.Bool("headless", false, "Run simulation without visualization")
	// sharedTrees := flag.Bool("shared", true, "All organisms share library of decision trees")
	trials := flag.Int("trials", 1, "Number of trials to run")
	flag.Parse()

	numTrials := *trials

	if *isHeadless {
		sumCycles := 0
		for count := 0; count < numTrials; count++ {
			config := s.DefaultConfig()
			simulation = s.NewSimulation(config)
			start := time.Now()
			cycles := 0
			for !simulation.IsDone() {
				simulation.Update()
				cycles++
			}
			sumCycles += cycles
			elapsed := time.Since(start)
			fmt.Printf("\nTotal runtime for simulation %d: %s, cycles: %d\n", count, elapsed, cycles)
		}
		avgCycles := sumCycles / numTrials
		fmt.Printf("\nAverage number of cycles to reach 5000: %d\n", avgCycles)
	} else {
		config := s.DefaultConfig()
		simulation = s.NewSimulation(config)
		if err := ebiten.Run(update, c.ScreenWidth, c.ScreenHeight, 1, "Shapes (Ebiten Demo)"); err != nil {
			log.Fatal(err)
		}
	}
}
