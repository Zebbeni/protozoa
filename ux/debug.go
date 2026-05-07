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
	debugHeight = 380
)

type Debug struct {
	simulation *simulation.Simulation
	image      *ebiten.Image

	renderTime      time.Duration
	gridRenderTime  time.Duration
	panelRenderTime time.Duration

	// Per-phase grid breakdown, set by Interface.renderGrid right after
	// the grid finishes drawing each frame.
	gridTimings RenderTimings
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
	info = fmt.Sprintf("%s\nEnvironmentUpdate: %7s", info, d.simulation.EnvironmentUpdateTime)
	info = fmt.Sprintf("%s\nFoodUpdate:     %10s", info, d.simulation.FoodUpdateTime)
	info = fmt.Sprintf("%s\nOrganismUpdate: %10s", info, d.simulation.OrganismUpdateTime)
	info = fmt.Sprintf("%s\n  OrganismUpdateLoop:  %10s", info, d.simulation.OrganismUpdateLoopTime)
	info = fmt.Sprintf("%s\n  OrganismResolveLoop: %10s", info, d.simulation.OrganismResolveLoopTime)
	info = fmt.Sprintf("%s\nTotal Update:   %10s", info, d.simulation.UpdateTime)
	info = fmt.Sprintf("%s\nRender Grid:    %10s", info, d.gridRenderTime)
	info = fmt.Sprintf("%s\n  Walls:        %10s", info, d.gridTimings.Walls)
	info = fmt.Sprintf("%s\n  Environment:  %10s", info, d.gridTimings.Env)
	info = fmt.Sprintf("%s\n  Food:         %10s", info, d.gridTimings.Food)
	info = fmt.Sprintf("%s\n  Organisms:    %10s", info, d.gridTimings.Organisms)
	info = fmt.Sprintf("%s\n  Compose:      %10s", info, d.gridTimings.Compose)
	info = fmt.Sprintf("%s\n  Selection:    %10s", info, d.gridTimings.SelectionBoxes)
	info = fmt.Sprintf("%s\nRender Panel:   %10s", info, d.panelRenderTime)
	info = fmt.Sprintf("%s\nTotal Render:   %10s", info, d.renderTime)
	info = fmt.Sprintf("%s\nTotal:          %10s", info, d.renderTime+d.simulation.UpdateTime)
	ebitenutil.DebugPrint(image, info)

	return image
}
