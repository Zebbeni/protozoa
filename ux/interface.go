package ux

import (
	"image"
	"log"
	"math"
	"runtime/debug"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/replay"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
)

const (
	dragThreshold = 3 // pixels before a click becomes a drag
	panSpeed      = 2 // grid units per frame for arrow key pan
)

type Interface struct {
	simulation *simulation.Simulation
	selection  *organism.Info

	grid       *Grid
	panel      *Panel
	minimap    *Minimap
	debug      *Debug
	replayCtrl *replay.Controller

	gridOptions  *ebiten.DrawImageOptions
	panelOptions *ebiten.DrawImageOptions
	debugOptions *ebiten.DrawImageOptions

	// Drag detection
	mouseDownPos image.Point
	mouseDown    bool
	isDragging   bool
	lastDragPos  image.Point
}

func NewInterface(sim *simulation.Simulation) *Interface {
	grid := NewGrid(sim)
	i := &Interface{
		simulation:   sim,
		grid:         grid,
		panel:        NewPanel(sim, grid),
		minimap:      NewMinimap(sim, grid),
		gridOptions:  &ebiten.DrawImageOptions{},
		panelOptions: &ebiten.DrawImageOptions{},
	}
	i.gridOptions.GeoM.Scale(GridDisplayScale, GridDisplayScale)
	i.gridOptions.GeoM.Translate(panelWidth, 0)

	i.debug = NewDebug(sim)
	i.debugOptions = &ebiten.DrawImageOptions{}
	i.debugOptions.GeoM.Translate(panelWidth, 0)
	return i
}

// SetReplayController enables replay controls in the panel and hands the
// grid the controller's animation state so sprite animation stays in sync
// with cycle advancement.
func (i *Interface) SetReplayController(ctrl *replay.Controller) {
	i.replayCtrl = ctrl
	i.panel.SetReplayController(ctrl)
	i.grid.SetAnimationState(ctrl.AnimState)
	i.minimap.SetReplayController(ctrl)
	// Initial speed sync — if AutoSpeed is on (default), anchor Speed to
	// whatever zoom the camera is at.
	i.syncReplaySpeedToZoom()
	// If the replay opens with exactly one organism alive (the typical
	// "single ancestor" config), centre the camera on it so the user
	// doesn't have to hunt for the founder pixel on a wide grid. Larger
	// initial populations are left at the default world-centre framing.
	if infos := i.simulation.GetAllOrganismInfo(); len(infos) == 1 {
		for _, info := range infos {
			i.grid.Camera.CenterOn(info.Location.X, info.Location.Y)
			break
		}
	}
}

// syncReplaySpeedToZoom pushes the current camera unit size through the
// replay controller's auto-speed handler. No-op if no replay controller
// is attached or AutoSpeed is off.
func (i *Interface) syncReplaySpeedToZoom() {
	if i.replayCtrl == nil {
		return
	}
	i.replayCtrl.UpdateSpeedFromZoom(i.grid.Camera.GridUnitSize())
}

// OnResize updates viewport dimensions when the window is resized.
func (i *Interface) OnResize() {
	viewportW := (config.ScreenWidth() - panelWidth) / GridDisplayScale
	viewportH := config.ScreenHeight() / GridDisplayScale
	i.grid.Camera.ViewportW = viewportW
	i.grid.Camera.ViewportH = viewportH
	i.grid.doRefresh = true
}

func (i *Interface) Render(screen *ebiten.Image) {
	// Catch any panic inside the draw stack so the DXGI DEVICE_REMOVED
	// crashes leave a useful trace. Without this the runtime panic
	// happens deep inside ebiten and the cycle/speed context is lost.
	defer func() {
		if rec := recover(); rec != nil {
			cycle := i.simulation.Cycle()
			speed := 0.0
			if i.replayCtrl != nil {
				speed = i.replayCtrl.Speed
			}
			log.Printf("Render panic at cycle=%d speed=%v viewMode=%v: %v\n%s",
				cycle, speed, i.grid.ViewMode(), rec, debug.Stack())
			panic(rec)
		}
	}()
	fillThemeBackground(screen)

	start := time.Now()

	i.renderGrid(screen)
	i.minimap.Draw(screen)
	i.renderPanel(screen)

	i.debug.renderTime = time.Since(start)
	if i.simulation.IsDebug() {
		debugImage := i.debug.render()
		screen.DrawImage(debugImage, i.debugOptions)
	}
}

func (i *Interface) HandleUserInput() {
	i.handleKeyboard()
	i.panel.HandleScroll()
	i.handleMouse()
	i.minimap.Update()
}

func (i *Interface) handleKeyboard() {
	if inpututil.IsKeyJustReleased(ebiten.KeySpace) {
		i.simulation.Pause(!i.simulation.IsPaused())
	}
	if inpututil.IsKeyJustReleased(ebiten.KeyD) {
		i.simulation.ToggleDebug()
	}
	if inpututil.IsKeyJustReleased(ebiten.KeyT) {
		cycleTheme()
		i.grid.doRefresh = true
	}

	// Zoom via keyboard
	mx, my := ebiten.CursorPosition()
	pivotX, pivotY := (mx-panelWidth)/GridDisplayScale, my/GridDisplayScale
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) { // + key
		i.grid.SetZoom(i.grid.Camera.Zoom+1, pivotX, pivotY)
		i.syncReplaySpeedToZoom()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		i.grid.SetZoom(i.grid.Camera.Zoom-1, pivotX, pivotY)
		i.syncReplaySpeedToZoom()
	}

	// Pan via arrow keys (continuous while held)
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		i.grid.Camera.Pan(-panSpeed, 0)
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		i.grid.Camera.Pan(panSpeed, 0)
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		i.grid.Camera.Pan(0, -panSpeed)
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		i.grid.Camera.Pan(0, panSpeed)
	}
}

func (i *Interface) handleMouse() {
	// Zoom via mouse wheel (only when cursor is over the grid area)
	_, wy := ebiten.Wheel()
	if wy != 0 {
		mx, my := ebiten.CursorPosition()
		if mx >= panelWidth {
			pivotX, pivotY := (mx-panelWidth)/GridDisplayScale, my/GridDisplayScale
			if wy > 0 {
				i.grid.SetZoom(i.grid.Camera.Zoom+1, pivotX, pivotY)
			} else {
				i.grid.SetZoom(i.grid.Camera.Zoom-1, pivotX, pivotY)
			}
			i.syncReplaySpeedToZoom()
		}
	}

	// Drag / click handling
	mx, my := ebiten.CursorPosition()

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Check replay controls first (panel area)
		if mx < panelWidth && i.panel.HandleReplayClick(mx, my) {
			return
		}
		// Check minimap click
		if i.minimap.HandleClick(mx, my) {
			return
		}
		i.mouseDownPos = image.Pt(mx, my)
		i.lastDragPos = i.mouseDownPos
		i.mouseDown = true
		i.isDragging = false
	}

	if i.mouseDown && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		dx := mx - i.mouseDownPos.X
		dy := my - i.mouseDownPos.Y
		dist := math.Sqrt(float64(dx*dx + dy*dy))

		if !i.isDragging && dist > dragThreshold {
			i.isDragging = true
		}

		if i.isDragging {
			// Pan by the delta since last frame. Screen pixels cover GridDisplayScale×
			// more area than the internal viewport, so factor that into the conversion.
			unitPixels := float64(i.grid.Camera.GridUnitSize() * GridDisplayScale)
			frameDX := float64(mx-i.lastDragPos.X) / unitPixels
			frameDY := float64(my-i.lastDragPos.Y) / unitPixels
			i.grid.Camera.Pan(-frameDX, -frameDY)
			i.lastDragPos = image.Pt(mx, my)
		}
	}

	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		if i.mouseDown && !i.isDragging {
			i.handleLeftClick()
		}
		i.mouseDown = false
		i.isDragging = false
	}

	// Hover (always, for tooltip)
	i.handleMouseHover()
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
	case selectMostSuccessful:
		id = i.simulation.GetMostSuccessfulId()
	default:
		return
	}
	i.simulation.Select(id)
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
	// Overlay text drawn after scaling so the font stays crisp at its
	// authored size instead of being multiplied by GridDisplayScale.
	i.grid.RenderOverlayText(screen)
	i.debug.gridRenderTime = time.Since(start)
}

func (i *Interface) renderPanel(screen *ebiten.Image) {
	start := time.Now()
	panelImage := i.panel.Render()
	screen.DrawImage(panelImage, i.panelOptions)
	i.debug.panelRenderTime = time.Since(start)
}

// getMouseGridLocation converts the cursor position to world grid coordinates
// using the camera's offset and zoom level. Cursor positions inside the
// panel return onGrid=false directly — without this guard a wrapping
// world would normalise the negative screenX back into [0, w) and the
// hover overlay would keep drawing while the cursor is over the panel.
func (i *Interface) getMouseGridLocation() (utils.Point, bool) {
	mouseX, mouseY := ebiten.CursorPosition()
	if mouseX < panelWidth {
		return utils.Point{}, false
	}
	screenX := (mouseX - panelWidth) / GridDisplayScale
	screenY := mouseY / GridDisplayScale
	gridX, gridY, onGrid := i.grid.Camera.ScreenToGrid(screenX, screenY)
	return utils.Point{X: gridX, Y: gridY}, onGrid
}
