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
	// menu is the replay menu, nil outside replay mode.
	menu *ReplayMenu
	// zoomWheel turns a wheel reading into at most one zoom step; see wheel_step.go.
	zoomWheel *wheelStepper

	// graphPopup is the enlarged graph, opened from the expand control in the panel graph's corner.
	graphPopup *GraphPopup

	gridOptions  *ebiten.DrawImageOptions
	panelOptions *ebiten.DrawImageOptions
	debugOptions *ebiten.DrawImageOptions

	mouseDownPos image.Point
	mouseDown    bool
	isDragging   bool
	lastDragPos  image.Point

	// lastAutoSelectedID tracks the most recent auto-pick so we only trigger a smooth pan when Find Most actually changes selection, not on every frame the same organism keeps winning.
	lastAutoSelectedID int
}

// autoPanDuration is how long the smooth-pan transition takes when the camera follows a newly-selected organism.
const autoPanDuration = time.Second

func NewInterface(sim *simulation.Simulation) *Interface {
	grid := NewGrid(sim)
	i := &Interface{
		simulation:         sim,
		grid:               grid,
		panel:              NewPanel(sim, grid),
		minimap:            NewMinimap(sim, grid),
		gridOptions:        &ebiten.DrawImageOptions{},
		panelOptions:       &ebiten.DrawImageOptions{},
		lastAutoSelectedID: -1,
	}
	i.gridOptions.GeoM.Scale(GridDisplayScale, GridDisplayScale)
	i.gridOptions.GeoM.Translate(panelWidth, 0)

	i.graphPopup = NewGraphPopup(i.panel)
	i.debug = NewDebug(sim)
	i.debugOptions = &ebiten.DrawImageOptions{}
	i.debugOptions.GeoM.Translate(panelWidth, 0)
	return i
}

// SetReplayController enables replay controls in the panel and hands the grid the controller's animation state so sprite animation stays in sync with cycle advancement.
func (i *Interface) SetReplayController(ctrl *replay.Controller) {
	i.replayCtrl = ctrl
	i.menu = NewReplayMenu(ctrl.Globals(), ctrl.Path())
	i.panel.SetReplayController(ctrl)
	i.grid.SetAnimationState(ctrl.AnimState)
	i.grid.applyReplayViewDefaults()
	i.minimap.SetReplayController(ctrl)
	// Initial speed sync — if AutoSpeed is on (default), anchor Speed to whatever zoom the camera is at.
	i.syncReplaySpeedToZoom()
	// If the replay opens with exactly one organism alive (the typical "single ancestor" config), centre the camera on it so the user doesn't have to hunt for the founder pixel on a wide grid.
	if infos := i.simulation.GetAllOrganismInfo(); len(infos) == 1 {
		for _, info := range infos {
			i.grid.Camera.CenterOn(info.Location.X, info.Location.Y)
			break
		}
	}
}

// syncReplaySpeedToZoom pushes the current camera unit size through the replay controller's auto-speed handler.
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
	// Catch any panic inside the draw stack so the DXGI DEVICE_REMOVED crashes leave a useful trace.
	defer func() {
		if rec := recover(); rec != nil {
			cycle := i.simulation.Cycle()
			speed := 0.0
			if i.replayCtrl != nil {
				speed = i.replayCtrl.Speed
			}
			log.Printf("Render panic at cycle=%d speed=%v showPh=%v orgColor=%v: %v\n%s",
				cycle, speed, i.grid.ShowPh(), i.grid.OrgColor(), rec, debug.Stack())
			panic(rec)
		}
	}()
	fillThemeBackground(screen)

	start := time.Now()

	i.renderGrid(screen)
	i.minimap.Draw(screen)
	i.renderColorKey(screen)
	i.renderPanel(screen)
	// Over the panel, since it is modal and the panel is what it covers.
	i.graphPopup.Draw(screen)
	if i.menu != nil {
		i.menu.Draw(screen)
	}

	i.debug.renderTime = time.Since(start)
	if i.simulation.IsDebug() {
		debugImage := i.debug.render()
		screen.DrawImage(debugImage, i.debugOptions)
	}
}

// TakeMenuChoice returns and clears the replay menu choice the runner has to act on, if any.
func (i *Interface) TakeMenuChoice() ReplayMenuChoice {
	if i.menu == nil {
		return ReplayMenuNone
	}
	return i.menu.Take()
}

func (i *Interface) HandleUserInput() {
	// The open replay menu takes all input.
	if i.menu != nil && i.menu.IsOpen() {
		i.menu.Update()
		return
	}
	if i.graphPopup.Update() {
		return
	}
	// Advance any in-flight smooth-pan animation before reading input.
	i.grid.Camera.UpdatePan()
	i.handleKeyboard()
	// Graph zoom/pan gets first claim on the mouse.
	if i.panel.TakeGraphExpandRequest() {
		i.graphPopup.Open()
	}
	if !i.panel.HandleGraphInput() {
		i.panel.HandleScroll()
		i.handleMouse()
	}
	i.minimap.Update()
	if i.menu != nil && (i.panel.TakeMenuRequest() || inpututil.IsKeyJustPressed(ebiten.KeyEscape)) {
		i.menu.Open()
	}
}

func (i *Interface) handleKeyboard() {
	if inpututil.IsKeyJustReleased(ebiten.KeySpace) {
		i.simulation.Pause(!i.simulation.IsPaused())
	}
	if inpututil.IsKeyJustReleased(ebiten.KeyD) {
		i.simulation.ToggleDebug()
	}

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

	// Vertical pan stays on up/down (continuous while held).
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		i.grid.Camera.Pan(0, -panSpeed)
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		i.grid.Camera.Pan(0, panSpeed)
	}

	// In replay mode, left / right step the timeline (one cycle per press).
	if i.replayCtrl != nil {
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			i.replayCtrl.StepBackward()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			i.replayCtrl.StepForward()
		}
	}
}

func (i *Interface) handleMouse() {
	// Zoom via mouse wheel (only when cursor is over the grid area)
	_, wy := ebiten.Wheel()
	if i.zoomWheel == nil {
		i.zoomWheel = newWheelStepper()
	}
	if step := i.zoomWheel.step(wy); step != 0 {
		mx, my := ebiten.CursorPosition()
		if mx >= panelWidth {
			pivotX, pivotY := (mx-panelWidth)/GridDisplayScale, my/GridDisplayScale
			i.grid.SetZoom(i.grid.Camera.Zoom+ZoomLevel(step), pivotX, pivotY)
			i.syncReplaySpeedToZoom()
		}
	}

	mx, my := ebiten.CursorPosition()

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mx < panelWidth && i.panel.HandleReplayClick(mx, my) {
			return
		}
		if i.minimap.HandleClick(mx, my) {
			return
		}
		if i.colorKeyAbsorbsClick(mx, my) {
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
			// Pan by the delta since last frame.
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
	case selectMostAggressive:
		id = i.simulation.GetMostAggressiveId()
	default:
		// Manual mode (or anything that isn't a Find Most pick).
		i.lastAutoSelectedID = -1
		return
	}
	i.simulation.Select(id)

	if id != i.lastAutoSelectedID {
		i.lastAutoSelectedID = id
		if id >= 0 {
			if info := i.simulation.GetOrganismInfoByID(id); info != nil {
				i.grid.Camera.PanTo(info.Location.X, info.Location.Y, autoPanDuration)
			}
		}
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
	// Overlay text drawn after scaling so the font stays crisp at its authored size instead of being multiplied by GridDisplayScale.
	i.grid.RenderOverlayText(screen)
	i.debug.gridRenderTime = time.Since(start)
	i.debug.gridTimings = i.grid.LastRenderTimings()
}

// renderColorKey draws the legend for the organism colour mode in the bottom-right corner, stacked above the minimap.
func (i *Interface) renderColorKey(screen *ebiten.Image) {
	k, width, bottom, ok := i.colorKeyPlacement()
	if !ok {
		return
	}
	drawColorKey(screen, k, width, bottom)
}

// colorKeyPlacement is the key on show and where its bottom edge goes, or ok=false when there is none.
func (i *Interface) colorKeyPlacement() (k colorKey, width, bottom int, ok bool) {
	if !i.grid.ShowOrganisms() {
		return colorKey{}, 0, 0, false
	}
	k = colorKeyFor(i.grid.OrgColor(), i.grid.ColorAbility(), i.grid.ColorAction())
	if k.empty() {
		return colorKey{}, 0, 0, false
	}
	bottom = config.ScreenHeight() - minimapPadding
	if i.minimap.Visible() {
		bottom = i.minimap.Top() - minimapPadding
	}
	return k, i.minimap.Width(), bottom, true
}

func (i *Interface) colorKeyAbsorbsClick(x, y int) bool {
	k, width, bottom, ok := i.colorKeyPlacement()
	if !ok {
		return false
	}
	return image.Pt(x, y).In(colorKeyRect(k, width, bottom))
}

func (i *Interface) renderPanel(screen *ebiten.Image) {
	start := time.Now()
	panelImage := i.panel.Render()
	screen.DrawImage(panelImage, i.panelOptions)
	i.debug.panelRenderTime = time.Since(start)
}

// getMouseGridLocation converts the cursor position to world grid coordinates using the camera's offset and zoom level.
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
