package main

import (
	"fmt"
	"image/color"
	"log"
	"math/rand"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

const (
	screenWidth  = 800
	screenHeight = 600
	boxSize      = 5.0
)

var (
	boxX       = 400.0
	boxY       = 300.0
	boxColor   = color.RGBA{0x80, 0x80, 0x80, 0x80}
	fadeColor  = color.RGBA{0x00, 0x00, 0x00, 0x16}
	filter     = ebiten.FilterLinear
	prevScreen *ebiten.Image
)

func update(screen *ebiten.Image) error {
	if ebiten.IsRunningSlowly() {
		return nil
	}

	// draw previous image and draw rectangle over all (make old frames fade)
	screen.DrawImage(prevScreen, nil)
	ebitenutil.DrawRect(screen, 0, 0, screenWidth, screenHeight, fadeColor)

	// update box location
	boxX += float64(rand.Intn(3)*boxSize) - boxSize
	boxY += float64(rand.Intn(3)*boxSize) - boxSize
	// prevent going out of screen area
	boxX = float64(int(boxX+screenWidth) % screenWidth)
	boxY = float64(int(boxY+screenHeight) % screenHeight)
	// draw box
	ebitenutil.DrawRect(screen, boxX, boxY, boxSize, boxSize, boxColor)

	// draw current screen content to prevScreen to save for next update
	prevScreen.DrawImage(screen, nil)

	// write current FPS to screen
	ebitenutil.DebugPrint(screen, fmt.Sprintf("FPS: %0.2f", ebiten.CurrentFPS()))

	return nil
}

func main() {
	prevScreen, _ = ebiten.NewImage(screenWidth, screenHeight, filter)
	prevScreen.Fill(fadeColor)
	if err := ebiten.Run(update, screenWidth, screenHeight, 1, "Shapes (Ebiten Demo)"); err != nil {
		log.Fatal(err)
	}
}
