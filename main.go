package main

import (
	"fmt"
	"image/color"
	"log"
	"math/rand"

	"./utils"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

const (
	screenWidth  = 800
	screenHeight = 600
	boxSize      = 10.0
)

var (
	boxX       = 400.0
	boxY       = 300.0
	boxColor   = color.RGBA{0x80, 0x80, 0x80, 0x80}
	fadeColor  = color.RGBA{0x00, 0x00, 0x00, 0x20}
	frames     = 0
	filter     = ebiten.FilterLinear
	prevScreen *ebiten.Image
)

func update(screen *ebiten.Image) error {
	if ebiten.IsRunningSlowly() {
		return nil
	}

	// draw previous image first
	if frames > 0 {
		screen.DrawImage(prevScreen, nil)
		ebitenutil.DrawRect(screen, 0, 0, screenWidth, screenHeight, fadeColor)
	}
	boxX += float64(rand.Intn(3)*boxSize) - boxSize
	boxY += float64(rand.Intn(3)*boxSize) - boxSize
	if boxX < 0 || boxX > screenWidth || boxY < 0 || boxY > screenHeight {
		boxX = screenWidth / 2.0
		boxY = screenHeight / 2.0
	}
	ebitenutil.DrawRect(screen, boxX, boxY, boxSize, boxSize, boxColor)

	prevScreen, _ = ebiten.NewImageFromImage(screen, filter)

	utils.IncIntPtr(&frames)
	fmt.Printf("frame %d\n", frames)
	ebitenutil.DebugPrint(screen, fmt.Sprintf("FPS: %0.2f", ebiten.CurrentFPS()))
	return nil
}

func main() {
	if err := ebiten.Run(update, screenWidth, screenHeight, 1, "Shapes (Ebiten Demo)"); err != nil {
		log.Fatal(err)
	}
}
