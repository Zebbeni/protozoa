package ux

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
)

type mode int
type layerType int

const (
	layerEnv layerType = iota
	layerWalls
	layerFood
	layerOrganisms
)

// use separate constant group to ensure orgsPhMode starts at 0
const (
	orgsPhMode mode = iota
	organismsOnlyMode
	phEffectsOnlyMode
	phOnlyMode
)

const (
	selectOldest mode = iota
	selectMostChildren
	selectMostTraveled
	selectManual
)

var (
	foodColor          = colorful.HSLuv(120, 0.2, 0.25)
	wallColor          = colorful.HSLuv(60, 0.25, 0.1)
	selectColor        = colorful.HSLuv(0.0, 255.0, 1.0)
	hoverColor         = colorful.HSLuv(0.0, 0, 0.7)
	selectionInfoColor = colorful.HSLuv(0.0, 0, 1.0)
	viewModes          = []mode{orgsPhMode, organismsOnlyMode, phEffectsOnlyMode, phOnlyMode}
	selectModes        = []mode{selectOldest, selectMostChildren, selectMostTraveled, selectManual}
	viewModeNames      = map[mode]string{
		orgsPhMode:        "ORGANISMS & PH",
		organismsOnlyMode: "ORGANISMS ONLY",
		phEffectsOnlyMode: "ORGANISM PH EFFECTS",
		phOnlyMode:        "PH ONLY",
	}
	selectModeNames = map[mode]string{
		selectOldest:       "OLDEST",
		selectMostChildren: "MOST CHILDREN",
		selectMostTraveled: "MOST TRAVELED",
		selectManual:       "MANUAL SELECT",
	}
)

type Grid struct {
	simulation *simulation.Simulation
	Camera     *Camera
	animState  *animation.State

	layers map[layerType]*ebiten.Image

	mouseHoverLocation utils.Point
	mouseOnGrid        bool
	doRefresh          bool
	viewMode           mode
	selectMode         mode
	clearImg           *ebiten.Image
}

func NewGrid(sim *simulation.Simulation) *Grid {
	viewportW := (config.ScreenWidth() - panelWidth) / GridDisplayScale
	viewportH := config.ScreenHeight() / GridDisplayScale
	cam := NewCamera(viewportW, viewportH)

	g := &Grid{
		simulation: sim,
		Camera:     cam,
		doRefresh:  true,
		viewMode:   organismsOnlyMode,
		selectMode: selectOldest,
	}
	g.initLayerImages()
	g.loadOrganismImages()
	g.buildClearImg()
	return g
}

func (g *Grid) initLayerImages() {
	g.layers = map[layerType]*ebiten.Image{
		layerEnv:       g.newEnvLayer(),
		layerWalls:     g.newBlankLayer(),
		layerFood:      g.newBlankLayer(),
		layerOrganisms: g.newBlankLayer(),
	}
}

func (g *Grid) loadOrganismImages() {
	resources.SelectZoom(g.Camera.SpriteSet())
}

func (g *Grid) newBlankLayer() *ebiten.Image {
	return ebiten.NewImage(g.Camera.WorldPixelWidth(), g.Camera.WorldPixelHeight())
}

// newEnvLayer creates a 1-pixel-per-grid-cell image for the environment layer.
func (g *Grid) newEnvLayer() *ebiten.Image {
	return ebiten.NewImage(config.GridUnitsWide(), config.GridUnitsHigh())
}

func (g *Grid) unitSize() int {
	return g.Camera.GridUnitSize()
}

// SetAnimationState attaches the playback animation state the renderer
// should read from. Without it the grid falls back to drawing organisms at
// their current (non-interpolated) locations.
func (g *Grid) SetAnimationState(s *animation.State) {
	g.animState = s
}

// SetZoom changes zoom level, recreates layer caches, and forces a full refresh.
func (g *Grid) SetZoom(level ZoomLevel, pivotScreenX, pivotScreenY int) {
	if level == g.Camera.Zoom {
		return
	}
	g.Camera.SetZoom(level, pivotScreenX, pivotScreenY)
	g.loadOrganismImages()
	g.buildClearImg()
	g.initLayerImages()
	g.doRefresh = true
}

// Render draws all layers and returns a viewport-sized image.
func (g *Grid) Render() *ebiten.Image {
	if g.doRefresh {
		g.layers[layerEnv] = g.newEnvLayer()
		g.layers[layerWalls] = g.newBlankLayer()
		g.layers[layerFood] = g.newBlankLayer()
		g.layers[layerOrganisms] = g.newBlankLayer()
	}

	g.renderWalls(g.layers[layerWalls], g.doRefresh)
	g.renderEnvironment(g.layers[layerEnv], g.doRefresh)
	g.renderFood(g.layers[layerFood], g.doRefresh)
	g.renderOrganisms(g.layers[layerOrganisms], g.doRefresh)

	// Compose visible portion into viewport-sized image with wrapping support.
	viewportImage := ebiten.NewImage(g.Camera.ViewportW, g.Camera.ViewportH)
	centerOX, centerOY := g.Camera.CenterOffset()

	us := g.unitSize()
	wpw := float64(g.Camera.WorldPixelWidth())
	wph := float64(g.Camera.WorldPixelHeight())

	// Camera position in pixels, normalized to [0, worldPixelSize)
	camPxX := g.Camera.NormalizedX() * float64(us)
	camPxY := g.Camera.NormalizedY() * float64(us)

	// Draw world layer at tiled offsets to cover the viewport when wrapping.
	// On non-wrapping axes, just use the center offset.
	// scaleX/scaleY scale the layer image up (e.g., env layer is 1px/cell, scaled by unitSize).
	// Offsets are always in viewport pixel space.
	drawLayer := func(layer *ebiten.Image, scaleX, scaleY float64) {
		xOffsets := []float64{-camPxX}
		if g.Camera.WrapsX() {
			xOffsets = append(xOffsets, -camPxX+wpw)
		} else {
			xOffsets = []float64{float64(centerOX)}
		}
		yOffsets := []float64{-camPxY}
		if g.Camera.WrapsY() {
			yOffsets = append(yOffsets, -camPxY+wph)
		} else {
			yOffsets = []float64{float64(centerOY)}
		}

		for _, ox := range xOffsets {
			for _, oy := range yOffsets {
				op := &ebiten.DrawImageOptions{}
				if scaleX != 1 || scaleY != 1 {
					op.GeoM.Scale(scaleX, scaleY)
				}
				op.GeoM.Translate(ox, oy)
				viewportImage.DrawImage(layer, op)
			}
		}
	}

	if g.viewMode == orgsPhMode || g.viewMode == phOnlyMode {
		drawLayer(g.layers[layerEnv], float64(us), float64(us))
	}
	drawLayer(g.layers[layerWalls], 1, 1)
	if g.viewMode != phOnlyMode {
		drawLayer(g.layers[layerFood], 1, 1)
		drawLayer(g.layers[layerOrganisms], 1, 1)
	}

	// Draw selection boxes directly on viewport in screen coordinates
	g.renderSelectionBoxes(viewportImage)

	// Overlay text is drawn on the final screen by the caller (see
	// RenderOverlayText) so it isn't multiplied by GridDisplayScale.

	g.doRefresh = false
	return viewportImage
}

func (g *Grid) renderEnvironment(envImage *ebiten.Image, refresh bool) {
	if refresh {
		phMap := g.simulation.GetPhMap()
		for x := range phMap {
			for y := range phMap[x] {
				g.renderPhValue(envImage, x, y, phMap[x][y])
			}
		}
	} else {
		updatedPoints := g.simulation.GetUpdatedPhPoints()
		for point := range updatedPoints {
			phVal := g.simulation.GetPhAtPoint(point)
			g.renderPhValue(envImage, point.X, point.Y, phVal)
		}
	}
}

func (g *Grid) renderWalls(wallsImage *ebiten.Image, refresh bool) {
	if refresh {
		wallPoints := g.simulation.GetWalls()
		for _, wallPoint := range wallPoints {
			g.renderWall(wallsImage, wallPoint)
		}
	}
	// Walls are static — nothing to do on incremental frames
}

func (g *Grid) renderPhValue(envImage *ebiten.Image, gridX, gridY int, phVal float64) {
	// Environment image is 1px per cell — set the pixel directly
	hue := phMaxHue - (phMaxHue * phVal / config.MaxPh())
	sat := math.Abs(phVal-((config.MaxPh()+config.MinPh())/2.0)) / (config.MaxPh() - config.MinPh())
	light := 0.5 + (0.5 * math.Sin(math.Pi*(sat-0.5)))
	col := colorful.HSLuv(hue, sat, light)
	r, g2, b, _ := col.RGBA()
	envImage.Set(gridX, gridY, color.RGBA{
		R: uint8(r >> 8), G: uint8(g2 >> 8), B: uint8(b >> 8), A: 255,
	})
}

func (g *Grid) renderFood(foodImage *ebiten.Image, refresh bool) {
	if refresh {
		items := g.simulation.GetFoodItems()
		for _, item := range items {
			g.renderFoodItem(item, foodImage)
		}
	} else {
		updatedPoints := g.simulation.GetUpdatedFoodPoints()
		for point := range updatedPoints {
			us := g.unitSize()
			x, y := point.X*us, point.Y*us
			g.clearSquare(foodImage, float64(x), float64(y))
			if item, exists := g.simulation.GetFoodAtPoint(point); exists {
				g.renderFoodItem(item, foodImage)
			}
		}
	}
}

// renderOrganisms fully clears and redraws the organism layer every call.
//
// Unlike the other layers (walls, food, env), organisms are animated: we
// need a fresh draw every render tick so sprites at interpolated positions
// don't leave trails and so the animation frame index can advance. Clearing
// unconditionally also means "attack/move animations passing through a
// neighbour cell" requires no special bookkeeping — both cells are empty
// on the organism layer each frame, and food/walls below come from their
// own layers.
func (g *Grid) renderOrganisms(organismsImage *ebiten.Image, refresh bool) {
	organismsImage.Clear()

	organismInfo := g.simulation.GetAllOrganismInfo()
	for _, info := range organismInfo {
		g.renderOrganism(info, organismsImage)
	}
}

// renderSelectionBoxes draws selection box outlines in viewport coordinates.
func (g *Grid) renderSelectionBoxes(viewportImage *ebiten.Image) {
	if g.mouseOnGrid {
		g.renderSelectionBox(g.mouseHoverLocation, viewportImage, hoverColor)
	}
	if info := g.simulation.GetOrganismInfoByID(g.simulation.GetSelected()); info != nil {
		g.renderSelectionBox(info.Location, viewportImage, selectColor)
	}
}

// RenderOverlayText draws the mode label and hover-info text directly onto
// the final screen. Called by the Interface after compositing the scaled
// grid, so the text isn't multiplied by GridDisplayScale.
func (g *Grid) RenderOverlayText(screen *ebiten.Image) {
	if !g.mouseOnGrid {
		return
	}

	// View mode / selection label at top-left of the grid area.
	xPadding := 10
	yPadding := 20
	info := fmt.Sprintf("VIEW MODE: %s\nSELECTED: %s", viewModeNames[g.viewMode], selectModeNames[g.selectMode])
	text.Draw(screen, info, resources.FontSourceCodePro10, panelWidth+xPadding, yPadding, selectionInfoColor)

	// Hover info text near the cursor.
	infoText := fmt.Sprintf("PH: %2.1f", g.simulation.GetPhAtPoint(g.mouseHoverLocation))
	infoColor := hoverColor
	if info := g.simulation.GetOrganismInfoAtPoint(g.mouseHoverLocation); info != nil {
		infoText += fmt.Sprintf("\nORG: %d", info.ID)
		infoText += fmt.Sprintf("\nSIZE: %.0f", info.Size)
		infoColor = info.Color
	} else {
		if foodItem, exists := g.simulation.GetFoodAtPoint(g.mouseHoverLocation); exists {
			infoText += fmt.Sprintf("\nFOOD: %d", foodItem.Value)
		}
	}
	infoText += fmt.Sprintf("\nPOINT: %v", g.mouseHoverLocation)

	mx, my := ebiten.CursorPosition()
	screenX := mx + 15
	screenY := my - 10
	bounds := boundString(resources.FontSourceCodePro10, infoText)
	if screenX+bounds.Dx() > config.ScreenWidth() {
		screenX = mx - bounds.Dx() - 15
	}
	_ = infoColor
	text.Draw(screen, infoText, resources.FontSourceCodePro10, screenX, screenY, selectionInfoColor)
}

// ViewMode returns the current grid view mode
func (g *Grid) ViewMode() mode {
	return g.viewMode
}

// ChangeViewMode switches to the next mode listed in viewModes
func (g *Grid) ChangeViewMode() {
	g.viewMode = viewModes[(int(g.viewMode)+1)%len(viewModes)]
	g.doRefresh = true
}

// UpdateAutoSelect switches to the next auto select mode listed in selectModes
func (g *Grid) UpdateAutoSelect() {
	g.selectMode = selectModes[(int(g.selectMode)+1)%(len(selectModes)-1)]
	g.doRefresh = true
}

func (g *Grid) SetManualSelection() {
	g.selectMode = selectManual
}

func (g *Grid) MouseHover(point utils.Point, onGrid bool) {
	g.mouseHoverLocation = point
	g.mouseOnGrid = onGrid
}

func (g *Grid) renderSelectionBox(point utils.Point, img *ebiten.Image, col colorful.Color) {
	us := float64(g.unitSize())
	centerOX, centerOY := g.Camera.CenterOffset()
	wpw := float64(g.Camera.WorldPixelWidth())
	wph := float64(g.Camera.WorldPixelHeight())
	camPxX := g.Camera.NormalizedX() * us
	camPxY := g.Camera.NormalizedY() * us

	// Grid position in world pixels
	worldX := float64(point.X) * us
	worldY := float64(point.Y) * us

	// Convert to viewport coordinates (handle wrapping)
	var vx, vy float64
	if g.Camera.WrapsX() {
		vx = math.Mod(worldX-camPxX+wpw, wpw)
	} else {
		vx = worldX + float64(centerOX)
	}
	if g.Camera.WrapsY() {
		vy = math.Mod(worldY-camPxY+wph, wph)
	} else {
		vy = worldY + float64(centerOY)
	}

	p := math.Round(us / 8.0)
	x0, y0 := vx-p, vy-p
	x1, y1 := vx+us+p, vy+us+p
	ebitenutil.DrawLine(img, x0, y0, x1, y0, col)
	ebitenutil.DrawLine(img, x0, y0, x0, y1, col)
	ebitenutil.DrawLine(img, x0, y1, x1, y1, col)
	ebitenutil.DrawLine(img, x1, y0, x1, y1, col)
}

func (g *Grid) renderFoodItem(item *food.Item, img *ebiten.Image) {
	us := g.unitSize()
	x := float64(item.Point.X) * float64(us)
	y := float64(item.Point.Y) * float64(us)
	sprite := resources.Sprite(resources.RoleFood, animation.AnimIdle, 0)
	g.drawStaticSprite(img, x, y, sprite, foodColor)
}

func (g *Grid) renderWall(wallsImage *ebiten.Image, point utils.Point) {
	us := g.unitSize()
	x := float64(point.X) * float64(us)
	y := float64(point.Y) * float64(us)
	sprite := resources.Sprite(resources.RoleBox, animation.AnimIdle, 0)
	g.drawStaticSprite(wallsImage, x, y, sprite, wallColor)
}

// renderOrganism draws an organism at its animation-interpolated position
// using the action-specific sprite frame, rotated to face its direction.
//
// If we have an animation.Frame for this organism we use its FromLocation →
// ToLocation pair plus the current Progress() to animate. Without one (newly
// born organism, fresh seek, first frame before any cycle advance) we fall
// back to the organism's static Location.
func (g *Grid) renderOrganism(info *organism.Info, img *ebiten.Image) {
	us := float64(g.unitSize())

	role := resources.RoleOrganismSmall
	if info.Size < config.MaximumMaxSize()*0.4375 {
		role = resources.RoleOrganismSmall
	} else if info.Size < config.MaximumMaxSize()*0.8125 {
		role = resources.RoleOrganismMedium
	} else {
		role = resources.RoleOrganismLarge
	}

	organismColor := info.Color
	if g.viewMode == phEffectsOnlyMode {
		organismColor = PhEffectColor(organism.PhEffectSpectrumValue(info.PhEffect, config.MaxOrganismPhGrowthEffect()))
	}

	// Defaults used when animation state is unavailable or the organism has
	// no frame yet: render statically at the current location facing its
	// current direction.
	gridX := float64(info.Location.X)
	gridY := float64(info.Location.Y)
	direction := info.Direction
	anim := animation.ForAction(info.Action)
	frameIdx := 0

	if g.animState != nil {
		if frame, ok := g.animState.Frames[info.ID]; ok {
			gridX, gridY = animatedCellPosition(frame, g.animState.Progress(), g.animState.AnimatesPosition())
			direction = frame.Direction
			// ForFrame, not ForAction, so a move whose position didn't
			// change falls through to AnimBlocked instead of drawing the
			// 2-cell travel sprite in place.
			anim = animation.ForFrame(frame)
		}
		frameIdx = g.animState.FrameIndex()
	}

	// Desync chemo frames by organism ID so a field of chemosynthesising
	// organisms wobbles out of phase rather than all moving in lockstep.
	// The loop still reads correctly across consecutive chemo cycles
	// because the offset is fixed per organism.
	if anim == animation.AnimChemo {
		frameIdx += info.ID % animation.BaseFramesPerCycle
	}
	sprite := resources.Sprite(role, anim, frameIdx)
	g.drawOrganismSprite(img, gridX*us, gridY*us, sprite, direction, organismColor)
}

// animatedCellPosition returns the organism's grid-unit position for the
// current render frame, given its animation Frame and the cycle progress.
//
// Move is pinned at FromLocation: the move spritesheet is 2 cells wide and
// depicts the journey from origin to destination within its own pixels, so
// interpolating the draw position on top of that would double the motion.
// Attack interpolates as a bounce: sprite lunges into the cell it's facing
// and returns, passing through (FromLocation + Direction) at progress 0.5.
// Everything else lerps linearly from FromLocation to ToLocation.
// animate==false collapses to the settled ToLocation (very high playback
// speeds that outrun the animation window).
func animatedCellPosition(f animation.Frame, progress float64, animate bool) (float64, float64) {
	if !animate {
		return float64(f.ToLocation.X), float64(f.ToLocation.Y)
	}

	if f.Action == decision.ActMove {
		return float64(f.FromLocation.X), float64(f.FromLocation.Y)
	}

	if f.Action == decision.ActAttack {
		// Lunge to (FromLocation + Direction) at the midpoint, return by end.
		targetX := float64(f.FromLocation.X + f.Direction.X)
		targetY := float64(f.FromLocation.Y + f.Direction.Y)
		fromX := float64(f.FromLocation.X)
		fromY := float64(f.FromLocation.Y)
		if progress < 0.5 {
			t := progress * 2
			return lerp(fromX, targetX, t), lerp(fromY, targetY, t)
		}
		t := (progress - 0.5) * 2
		return lerp(targetX, fromX, t), lerp(targetY, fromY, t)
	}

	return lerp(float64(f.FromLocation.X), float64(f.ToLocation.X), progress),
		lerp(float64(f.FromLocation.Y), float64(f.ToLocation.Y), progress)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// drawStaticSprite draws a non-rotated sprite at cell (x, y). Used for
// food, walls, and other layers that don't animate.
func (g *Grid) drawStaticSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, col colorful.Color) {
	if spriteImg == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	if s := g.Camera.SpriteScale(); s != 1 {
		op.GeoM.Scale(s, s)
	}
	op.GeoM.Translate(x, y)
	op.ColorM.Translate(col.R, col.G, col.B, 0)
	img.DrawImage(spriteImg, op)
}

// drawOrganismSprite draws a sprite rotated to match `direction`, anchored
// to the organism's base cell. Sprites are authored facing +X (right), with
// the base cell occupying the left-most 16x16 (or generally cellSize x
// cellSize) region of the sprite; multi-cell sprites like the 32x16 move
// sheet extend rightward from there.
//
// Positive rotation turns clockwise in ebiten's coordinate system (Y grows
// downward), so (0, +1) maps to +π/2, etc. We rotate around the base cell's
// center — (cellSize/2, cellSize/2) in sprite-local coords — rather than the
// geometric center of the sprite. That keeps the base cell pinned at (x, y)
// through the rotation, and any extending cells swing to line up with the
// organism's facing direction regardless of how wide the sprite is.
func (g *Grid) drawOrganismSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, direction utils.Point, col colorful.Color) {
	if spriteImg == nil {
		return
	}
	scale := g.Camera.SpriteScale()
	cellSize := float64(zoomSpriteSizes[g.Camera.SpriteSet()])
	anchorX := cellSize / 2
	anchorY := cellSize / 2

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-anchorX, -anchorY)
	op.GeoM.Rotate(directionAngle(direction))
	op.GeoM.Translate(anchorX, anchorY)
	if scale != 1 {
		op.GeoM.Scale(scale, scale)
	}
	op.GeoM.Translate(x, y)
	op.ColorM.Translate(col.R, col.G, col.B, 0)
	img.DrawImage(spriteImg, op)
}

// directionAngle converts a cardinal utils.Point direction into the
// rotation angle to apply to a +X-facing sprite. Organisms only face
// 4 cardinal directions in the current simulation; non-cardinal points
// fall through atan2 and still produce a sensible angle.
func directionAngle(d utils.Point) float64 {
	if d.X == 0 && d.Y == 0 {
		return 0
	}
	return math.Atan2(float64(d.Y), float64(d.X))
}

func (g *Grid) buildClearImg() {
	us := g.unitSize()
	g.clearImg = ebiten.NewImage(us, us)
	g.clearImg.Fill(color.White)
}

func (g *Grid) drawFill(img *ebiten.Image, x, y float64, col colorful.Color) {
	us := g.unitSize()
	ebitenutil.DrawRect(img, x, y, float64(us), float64(us), color.RGBA{
		R: uint8(col.R * 255), G: uint8(col.G * 255), B: uint8(col.B * 255), A: 255,
	})
}

func (g *Grid) clearSquare(img *ebiten.Image, x, y float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.CompositeMode = ebiten.CompositeModeDestinationOut
	img.DrawImage(g.clearImg, op)
}
