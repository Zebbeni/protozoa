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

	// Dying organisms are gone from GetAllOrganismInfo but their frames
	// linger in animState.Frames for one cycle so we can play AnimDie.
	// Synthesise an Info from the frame and feed it through the normal
	// renderer so the sprite pipeline (role selection, rotation, colour
	// tint) stays unified between live and dying organisms.
	if g.animState != nil {
		for id, frame := range g.animState.Frames {
			if _, alive := organismInfo[id]; alive {
				continue
			}
			if !frame.Dying {
				continue
			}
			synth := &organism.Info{
				ID:        id,
				Location:  frame.FromLocation,
				Direction: frame.Direction,
				Size:      frame.Size,
				Action:    frame.Action,
				Color:     frame.Color,
				PhEffect:  frame.PhEffect,
			}
			g.renderOrganism(synth, organismsImage)
		}
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

	// Size-to-role mapping: quartiles of MaximumMaxSize.
	//   tiny   — below 25%
	//   small  — 25% to below 50%
	//   medium — 50% to below 75%
	//   large  — 75% and above
	maxSize := config.MaximumMaxSize()
	var role resources.ImageRole
	switch {
	case info.Size < maxSize*0.25:
		role = resources.RoleOrganismTiny
	case info.Size < maxSize*0.5:
		role = resources.RoleOrganismSmall
	case info.Size < maxSize*0.75:
		role = resources.RoleOrganismMedium
	default:
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

	sprite := resources.Sprite(role, anim, frameIdx)
	g.drawOrganismSprite(img, gridX*us, gridY*us, sprite, direction, organismColor)
}

// animatedCellPosition returns the organism's grid-unit position for the
// current render frame, given its animation Frame and the cycle progress.
//
// Move and attack are pinned at FromLocation: both use 2-cell spritesheets
// that paint the full journey (move: left→right travel; attack: lunge into
// extending cell and return), so interpolating the draw position on top
// would double the motion. Everything else lerps linearly from FromLocation
// to ToLocation. animate==false collapses to the settled ToLocation (very
// high playback speeds that outrun the animation window).
func animatedCellPosition(f animation.Frame, progress float64, animate bool) (float64, float64) {
	if !animate {
		return float64(f.ToLocation.X), float64(f.ToLocation.Y)
	}

	if f.Action == decision.ActMove || f.Action == decision.ActAttack {
		return float64(f.FromLocation.X), float64(f.FromLocation.Y)
	}

	return lerp(float64(f.FromLocation.X), float64(f.ToLocation.X), progress),
		lerp(float64(f.FromLocation.Y), float64(f.ToLocation.Y), progress)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// drawStaticSprite draws a non-rotated sprite at cell (x, y). Used for
// food, walls, and other layers that don't animate.
//
// Colorization is multiplicative via ColorScale: sprite pixels are
// authored white-on-transparent (optionally with grayscale shading), and
// each channel gets scaled by the target colour. A white pixel becomes
// the full target colour; a 50% grey pixel becomes a 50%-intensity
// version of the same hue; black stays black. Preserves internal
// brightness variation of the source art.
func (g *Grid) drawStaticSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, col colorful.Color) {
	if spriteImg == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	if s := g.Camera.SpriteScale(); s != 1 {
		op.GeoM.Scale(s, s)
	}
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(float32(col.R), float32(col.G), float32(col.B), 1)
	img.DrawImage(spriteImg, op)
}

// drawOrganismSprite draws a sprite rotated to match `direction`, anchored
// to the organism's base cell, using the grid's current camera zoom.
func (g *Grid) drawOrganismSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, direction utils.Point, col colorful.Color) {
	cellSize := float64(zoomSpriteSizes[g.Camera.SpriteSet()])
	drawAnimatedSprite(img, x, y, spriteImg, direction, col, cellSize, g.Camera.SpriteScale())
}

// drawAnimatedSprite draws a sprite rotated to face `direction`, anchored to
// its base cell.
//
// Sprites are authored facing -Y (up) and white-on-transparent (optionally
// with grayscale shading). Colorization is multiplicative via ColorScale:
// each channel of each pixel is multiplied by the target colour. White
// pixels become the full target colour; grey pixels become a dimmer
// version of the same hue (brightness variation in the sheet is
// preserved); black stays black.
//
// Single-cell sprites fill a cellSize x cellSize canvas. Multi-cell
// sprites use a cellSize x (N*cellSize) canvas where the base cell is the
// BOTTOM cellSize-tall region and the extending cell(s) sit above it — so
// the organism travels from bottom to top within the sprite at its
// default (up-facing) orientation.
//
// Positive rotation turns clockwise in ebiten's coordinate system. Up-
// facing (0, -1) maps to 0 rotation; east (1, 0) to +π/2; south (0, 1) to
// π; west (-1, 0) to -π/2. Rotation is centred on the base cell (whose
// centre in sprite-local coords is (cellSize/2, spriteH - cellSize/2)), so
// the base cell stays pinned at (x, y) and any extending cells swing to
// align with the facing direction regardless of how tall the sprite is.
//
// Shared between the main grid renderer and the standalone animation-test
// screen so they paint identically.
func drawAnimatedSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, direction utils.Point, col colorful.Color, cellSize, scale float64) {
	if spriteImg == nil {
		return
	}
	b := spriteImg.Bounds()
	spriteH := float64(b.Dy())

	// Base cell sits at the BOTTOM cellSize x cellSize region for multi-
	// cell (vertical-extending) sprites; for single-cell sprites it's the
	// whole image. The formula below reduces to (cellSize/2, cellSize/2)
	// in the single-cell case.
	anchorX := cellSize / 2
	anchorY := spriteH - cellSize/2

	// Compensate the final translate so the anchor (base-cell centre) lands
	// at world (x + cellSize*scale/2, y + cellSize*scale/2) regardless of
	// sprite height — keeping (x, y) = base-cell top-left.
	compensateY := (cellSize/2 - anchorY) * scale

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-anchorX, -anchorY)
	op.GeoM.Rotate(directionAngle(direction))
	op.GeoM.Translate(anchorX, anchorY)
	if scale != 1 {
		op.GeoM.Scale(scale, scale)
	}
	op.GeoM.Translate(x, y+compensateY)
	op.ColorScale.Scale(float32(col.R), float32(col.G), float32(col.B), 1)
	img.DrawImage(spriteImg, op)
}

// directionAngle converts a cardinal utils.Point direction into the
// rotation angle to apply to an up-facing (-Y) sprite.
//
// atan2(y, x) treats +X as 0; sprites are authored facing -Y, so we offset
// by +π/2 to make (0, -1) = no rotation, (1, 0) = +π/2 CW, etc. Non-
// cardinal points still produce a sensible angle through atan2.
func directionAngle(d utils.Point) float64 {
	if d.X == 0 && d.Y == 0 {
		return 0
	}
	return math.Atan2(float64(d.Y), float64(d.X)) + math.Pi/2
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
