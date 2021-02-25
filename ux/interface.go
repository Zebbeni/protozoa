package ux

import (
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/hajimehoshi/ebiten"
)

type Interface struct {
	grid  *Grid
	panel *Panel

	gridOptions  *ebiten.DrawImageOptions
	panelOptions *ebiten.DrawImageOptions
}

func NewInterface(s *simulation.Simulation) *Interface {
	i := &Interface{
		grid:         NewGrid(s),
		panel:        NewPanel(s),
		gridOptions:  &ebiten.DrawImageOptions{},
		panelOptions: &ebiten.DrawImageOptions{},
	}
	i.gridOptions.GeoM.Translate(panelWidth, 0)
	return i
}

func (i *Interface) Render(screen *ebiten.Image) {
	screen.Clear()

	gridImage := i.grid.Render()
	panelImage := i.panel.Render()

	screen.DrawImage(gridImage, i.gridOptions)
	screen.DrawImage(panelImage, i.panelOptions)
}
