package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"time"

	c "./constants"
	s "./simulation"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

var (
	fadeColor  = color.RGBA{0, 0, 0, 100}
	filter     = ebiten.FilterLinear
	prevScreen *ebiten.Image
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

	return nil
}

// initializePreviousScreen creates and fills an image the same size as screen
func initializePreviousScreen() {
	prevScreen, _ = ebiten.NewImage(c.ScreenWidth, c.ScreenHeight, filter)
	prevScreen.Fill(fadeColor)
}

func main() {
	isHeadless := flag.Bool("headless", false, "Run simulation without visualising")
	flag.Parse()

	if *isHeadless {
		numTrials := 5
		sumCycles := 0
		for count := 0; count < numTrials; count++ {
			simulation = s.NewSimulation()
			start := time.Now()
			cycles := 0
			for !simulation.IsDone() {
				simulation.Update()
				cycles++
			}
			sumCycles += cycles
			elapsed := time.Since(start)
			fmt.Printf("\n\nSimulation #%d Complete:\n%d cycles for an organism to live to %d.", count, cycles, c.OrganismAgeToEndSimulation)
			fmt.Printf("\nTotal runtime: %s\n", elapsed)
		}
		avgCycles := sumCycles / numTrials
		fmt.Printf("\nAverage number of cycles to reach 10000: %d\n", avgCycles)
	} else {
		simulation = s.NewSimulation()
		initializePreviousScreen()
		if err := ebiten.Run(update, c.ScreenWidth, c.ScreenHeight, 1, "Shapes (Ebiten Demo)"); err != nil {
			log.Fatal(err)
		}
	}
}
