package main

import (
	"fmt"
	"image/color"
	"log"

	c "./constants"
	s "./simulation"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

var (
	fadeColor  = color.RGBA{0, 0, 0, 5}
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
	initializePreviousScreen()
	simulation = s.NewSimulation()
	if err := ebiten.Run(update, c.ScreenWidth, c.ScreenHeight, 1, "Shapes (Ebiten Demo)"); err != nil {
		log.Fatal(err)
	}
}
