package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"runtime"
	"time"

	c "./constants"
	s "./simulation"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

var (
	fadeColor  = color.RGBA{0, 0, 0, 255}
	filter     = ebiten.FilterLinear
	prevScreen *ebiten.Image
	config     s.SimulationConfig
	simulation s.Simulation
)

func update(screen *ebiten.Image) error {
	if ebiten.IsRunningSlowly() {
		return nil
	}
	// draw previous image and draw rectangle over all (make old frames fade)
	screen.DrawImage(prevScreen, nil)
	ebitenutil.DrawRect(screen, 0, 0, c.ScreenWidth, c.ScreenHeight, fadeColor)

	simulation.Update()
	simulation.Render(screen)

	// draw current screen content to prevScreen to save for next update
	prevScreen.DrawImage(screen, nil)
	// write current FPS to screen
	ebitenutil.DebugPrint(screen, fmt.Sprintf("FPS: %0.2f", ebiten.CurrentFPS()))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	ebitenutil.DebugPrint(screen, fmt.Sprintf("\nAlloc = %v\nTotalAlloc = %v\nSys = %v\nNumGC = %v\n\n", m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC))

	if simulation.IsDone() {
		return errors.New("Simulation complete")
	}
	return nil
}

// initializePreviousScreen creates and fills an image the same size as screen
func initializePreviousScreen() {
	prevScreen, _ = ebiten.NewImage(c.ScreenWidth, c.ScreenHeight, filter)
	prevScreen.Fill(fadeColor)
}

func main() {
	rand.Seed(1)

	isHeadless := flag.Bool("headless", false, "Run simulation without visualising")
	flag.Parse()

	if *isHeadless {
		numTrials := 5
		sumCycles := 0
		for count := 0; count < numTrials; count++ {
			config := s.DefaultSimulationConfig()
			simulation = s.NewSimulation(config)
			start := time.Now()
			cycles := 0
			for !simulation.IsDone() {
				simulation.Update()
				cycles++
			}
			sumCycles += cycles
			elapsed := time.Since(start)
			fmt.Printf("\nTotal runtime for simulation %d: %s\n", count, elapsed)
		}
		avgCycles := sumCycles / numTrials
		fmt.Printf("\nAverage number of cycles to reach 10000: %d\n", avgCycles)
	} else {
		config := s.DefaultSimulationConfig()
		simulation = s.NewSimulation(config)
		initializePreviousScreen()
		if err := ebiten.Run(update, c.ScreenWidth, c.ScreenHeight, 1, "Shapes (Ebiten Demo)"); err != nil {
			log.Fatal(err)
		}
	}
}
