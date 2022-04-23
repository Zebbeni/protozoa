package ux

import (
	"fmt"
	"runtime"
	"time"

	"github.com/Zebbeni/protozoa/simulation"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	debugWidth  = 250
	debugHeight = 300
)

type Debug struct {
	simulation *simulation.Simulation
	image      *ebiten.Image

	renderTime      time.Duration
	gridRenderTime  time.Duration
	panelRenderTime time.Duration
}

func NewDebug(sim *simulation.Simulation) *Debug {
	return &Debug{
		simulation: sim,
	}
}

func (d *Debug) render() *ebiten.Image {
	image := ebiten.NewImage(debugWidth, debugHeight)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// write info to screen
	info := fmt.Sprintf("FPS: %0.2f", ebiten.CurrentFPS())
	info = fmt.Sprintf("%s\nAlloc: %v", info, m.Alloc/1024)
	info = fmt.Sprintf("%s\nTotalAlloc: %v", info, m.TotalAlloc/1024)
	info = fmt.Sprintf("%s\nSys: %v", info, m.Sys/1024)
	info = fmt.Sprintf("%s\nNumGC: %v", info, m.NumGC/1024)
	info = fmt.Sprintf("%s\nFoodUpdate:     %10s", info, d.simulation.FoodUpdateTime)
	info = fmt.Sprintf("%s\nOrganismUpdate: %10s", info, d.simulation.OrganismUpdateTime)
	info = fmt.Sprintf("%s\nTotal Update:   %10s", info, d.simulation.UpdateTime)
	info = fmt.Sprintf("%s\nRender Grid:    %10s", info, d.gridRenderTime)
	info = fmt.Sprintf("%s\nRender Panel:   %10s", info, d.panelRenderTime)
	info = fmt.Sprintf("%s\nTotal Render:   %10s", info, d.renderTime)
	info = fmt.Sprintf("%s\nTotal:          %10s", info, d.renderTime+d.simulation.UpdateTime)
	ebitenutil.DebugPrint(image, info)

	return image
}
