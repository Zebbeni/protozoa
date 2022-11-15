package ux

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
)

type Interface struct {
	simulation *simulation.Simulation
	selection  *organism.Info

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

func (i *Interface) HandleUserInput() {
	i.handleKeyboard()
	i.handleMouse()
}

func (i *Interface) handleKeyboard() {
	if inpututil.IsKeyJustReleased(ebiten.KeySpace) {
		i.simulation.Pause(!i.simulation.IsPaused())
	}
	if inpututil.IsKeyJustReleased(ebiten.KeyM) {
		i.grid.ChangeViewMode()
	}
	if inpututil.IsKeyJustReleased(ebiten.KeyO) {
		i.grid.UpdateAutoSelect()
	}
}

func (i *Interface) UpdateSelected() {
	id := -1
	switch i.grid.selectMode {
	case selectOldest:
		id = i.simulation.GetOldestId()
	case selectMostChildren:
		id = i.simulation.GetMostChildrenId()
	case selectMostTraveled:
		id = i.simulation.GetMostTraveledId()
	default:
		return
	}
	i.simulation.Select(id)
}

// eventually let's implement a more comprehensive event handler system
// but for right now, when the grid is the only thing we're using with mouse
// events, I think this is fine.
func (i *Interface) handleMouse() {
	i.handleMouseHover()

	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		i.handleLeftClick()
	}
}

func (i *Interface) handleMouseHover() {
	gridLocation, onGrid := i.getMouseGridLocation()
	i.grid.MouseHover(gridLocation, onGrid)
}

func (i *Interface) handleLeftClick() {
	if selectedPoint, onGrid := i.getMouseGridLocation(); onGrid {
		i.grid.SetManualSelection()
		if info := i.simulation.GetOrganismInfoAtPoint(selectedPoint); info != nil {
			i.simulation.Select(info.ID)
		} else {
			i.simulation.Select(-1)
		}
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

// getMouseGridLocation returns the mouse's point on the grid along with a
// boolean telling us if the point is within the grid bounds
func (i *Interface) getMouseGridLocation() (utils.Point, bool) {
	mouseX, mouseY := ebiten.CursorPosition()
	relativeGridX := mouseX - panelWidth
	relativeGridY := mouseY
	gridX := relativeGridX / config.GridUnitSize()
	gridY := relativeGridY / config.GridUnitSize()
	gridW := config.GridUnitsWide()
	gridH := config.GridUnitsHigh()
	onGrid := gridX >= 0 && gridY >= 0 && gridX < gridW && gridY < gridH
	return utils.Point{X: gridX, Y: gridY}, onGrid
}
