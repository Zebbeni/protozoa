package ux

import (
	"time"

	"github.com/Zebbeni/protozoa/simulation"
	"github.com/hajimehoshi/ebiten"
)

type Interface struct {
	simulation *simulation.Simulation

	grid  *Grid
	panel *Panel
	debug *Debug

	gridOptions  *ebiten.DrawImageOptions
	panelOptions *ebiten.DrawImageOptions
	debugOptions *ebiten.DrawImageOptions
}

func NewInterface(sim *simulation.Simulation) *Interface {
	i := &Interface{
		simulation:   sim,
		grid:         NewGrid(sim),
		panel:        NewPanel(sim),
		gridOptions:  &ebiten.DrawImageOptions{},
		panelOptions: &ebiten.DrawImageOptions{},
	}
	i.gridOptions.GeoM.Translate(panelWidth, 0)

	i.debug = NewDebug(sim)
	i.debugOptions = &ebiten.DrawImageOptions{}
	i.debugOptions.GeoM.Translate(panelWidth, 0)
	return i
}

func (i *Interface) Render(screen *ebiten.Image) {
	screen.Clear()

	start := time.Now()

	i.renderGrid(screen)
	i.renderPanel(screen)

	i.debug.renderTime = time.Since(start)
	if i.simulation.IsDebug() {
		debugImage := i.debug.render()
		screen.DrawImage(debugImage, i.debugOptions)
	}
}

func (i *Interface) renderGrid(screen *ebiten.Image) {
	start := time.Now()
	gridImage := i.grid.Render()
	screen.DrawImage(gridImage, i.gridOptions)
	i.debug.gridRenderTime = time.Since(start)
}

func (i *Interface) renderPanel(screen *ebiten.Image) {
	start := time.Now()
	panelImage := i.panel.Render()
	screen.DrawImage(panelImage, i.panelOptions)
	i.debug.panelRenderTime = time.Since(start)
}
