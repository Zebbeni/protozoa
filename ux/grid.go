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

// minOrganismAnimationUnitSize is the smallest per-cell unit size at which
// organism sprite animations play. Below this (Zoom4), the renderer pins
// to frame 0 of the sheet — at tiny sizes per-frame differences are too
// small to read and the flicker adds more noise than animation.
const minOrganismAnimationUnitSize = 8

var (
	foodColor = colorful.HSLuv(120, 0.2, 0.25)
	wallColor = colorful.HSLuv(60, 0.25, 0.1)
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

// newEnvLayer creates the environment image at sprite-native resolution
// per grid cell (4 or 16 px/cell depending on the active sprite set).
// Each pixel's colour is bilinearly interpolated CPU-side from the four
// nearest cells' pH values when the cell changes, with wrap-aware
// neighbour sampling. The viewport compose step uses FilterNearest, so
// no GPU bilinear pass blurs the result — keeping the gradients smooth
// without the seam artifact that FilterLinear leaves at the world wrap
// edge.
func (g *Grid) newEnvLayer() *ebiten.Image {
	cell := zoomSpriteSizes[g.Camera.SpriteSet()]
	return ebiten.NewImage(config.GridUnitsWide()*cell, config.GridUnitsHigh()*cell)
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
	// scaleX/scaleY scale the layer image up (e.g., env layer is sized at
	// sprite-native resolution per cell and scaled by the sprite scale).
	// Offsets are always in viewport pixel space. filter picks the ebiten
	// sampling mode — nearest for sprite layers (no blur), linear for the
	// env layer (smooths cell-boundary transitions).
	drawLayer := func(layer *ebiten.Image, scaleX, scaleY float64, filter ebiten.Filter) {
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
				op.Filter = filter
				if scaleX != 1 || scaleY != 1 {
					op.GeoM.Scale(scaleX, scaleY)
				}
				op.GeoM.Translate(ox, oy)
				viewportImage.DrawImage(layer, op)
			}
		}
	}

	if g.viewMode == orgsPhMode || g.viewMode == phOnlyMode {
		// env layer is already at sprite-native resolution with
		// bilinear-interpolated colours baked in by renderPhValue.
		// Compose with FilterNearest — the final upscale to display
		// pixels is purely a sprite-style integer multiplication.
		envScale := g.Camera.SpriteScale()
		drawLayer(g.layers[layerEnv], envScale, envScale, ebiten.FilterNearest)
	}
	drawLayer(g.layers[layerWalls], 1, 1, ebiten.FilterNearest)
	if g.viewMode != phOnlyMode {
		drawLayer(g.layers[layerFood], 1, 1, ebiten.FilterNearest)
		drawLayer(g.layers[layerOrganisms], 1, 1, ebiten.FilterNearest)
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
		g.rebuildEnvLayer(envImage)
		return
	}
	updatedPoints := g.simulation.GetUpdatedPhPoints()
	for point := range updatedPoints {
		phVal := g.simulation.GetPhAtPoint(point)
		g.renderPhValue(envImage, point.X, point.Y, phVal)
	}
}

// rebuildEnvLayer redraws every pixel of the env layer in one CPU pass,
// used on full refreshes (initial render, theme change, zoom change).
// Each pixel is bilinearly interpolated from the four nearest cells'
// colours with wrap-aware sampling, then uploaded via WritePixels.
func (g *Grid) rebuildEnvLayer(envImage *ebiten.Image) {
	N := zoomSpriteSizes[g.Camera.SpriteSet()]
	W := config.GridUnitsWide()
	H := config.GridUnitsHigh()
	imgW := W * N
	imgH := H * N

	phMap := g.simulation.GetPhMap()
	cellColors := make([]colorful.Color, W*H)
	for x := 0; x < W; x++ {
		for y := 0; y < H; y++ {
			cellColors[x*H+y] = g.phToColor(phMap[x][y])
		}
	}

	buf := make([]byte, 4*imgW*imgH)
	invN := 1.0 / float64(N)
	for py := 0; py < imgH; py++ {
		v := (float64(py)+0.5)*invN - 0.5
		cy := int(math.Floor(v))
		fy := v - float64(cy)
		cy0 := ((cy%H)+H)%H
		cy1 := ((cy+1)%H+H)%H
		for px := 0; px < imgW; px++ {
			u := (float64(px)+0.5)*invN - 0.5
			cx := int(math.Floor(u))
			fx := u - float64(cx)
			cx0 := ((cx%W)+W)%W
			cx1 := ((cx+1)%W+W)%W

			c00 := cellColors[cx0*H+cy0]
			c10 := cellColors[cx1*H+cy0]
			c01 := cellColors[cx0*H+cy1]
			c11 := cellColors[cx1*H+cy1]

			r := (1-fy)*((1-fx)*c00.R+fx*c10.R) + fy*((1-fx)*c01.R+fx*c11.R)
			gv := (1-fy)*((1-fx)*c00.G+fx*c10.G) + fy*((1-fx)*c01.G+fx*c11.G)
			b := (1-fy)*((1-fx)*c00.B+fx*c10.B) + fy*((1-fx)*c01.B+fx*c11.B)

			i := (py*imgW + px) * 4
			buf[i] = uint8(r * 255)
			buf[i+1] = uint8(gv * 255)
			buf[i+2] = uint8(b * 255)
			buf[i+3] = 255
		}
	}
	envImage.WritePixels(buf)
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

// renderPhValue updates the 2N×2N pixel region of envImage that depends
// on cell (gridX, gridY)'s pH value. Pixels are bilinear-interpolated
// from the cell and its 8 neighbours (wrap-aware), so each cell is
// flat-coloured at its centre and smoothly transitions to neighbours
// without any GPU FilterLinear pass — eliminating the seam at world
// wrap edges.
//
// Hue runs acid→base across the range. Saturation grows with distance
// from neutral pH (mid-pH is grey, extremes are colourful). Lightness
// flips with the theme so extremes stay high-contrast against the
// window fill, and the low-sat end blends towards the active theme's
// background so neutral cells visually disappear into the grid fill
// (black under dark, white under light, light-blue under light_blue).
func (g *Grid) renderPhValue(envImage *ebiten.Image, gridX, gridY int, phVal float64) {
	N := zoomSpriteSizes[g.Camera.SpriteSet()]
	halfN := N / 2
	W := config.GridUnitsWide()
	H := config.GridUnitsHigh()
	imgW := W * N
	imgH := H * N

	var col [3][3]colorful.Color
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			var ph float64
			if dx == 0 && dy == 0 {
				ph = phVal
			} else {
				wx := ((gridX+dx)%W + W) % W
				wy := ((gridY+dy)%H + H) % H
				ph = g.simulation.GetPhAtPoint(utils.Point{X: wx, Y: wy})
			}
			col[dy+1][dx+1] = g.phToColor(ph)
		}
	}

	startPxX := gridX*N + halfN - N
	startPxY := gridY*N + halfN - N
	invN := 1.0 / float64(N)

	for dpy := 0; dpy < 2*N; dpy++ {
		py := startPxY + dpy
		v := (float64(py)+0.5)*invN - 0.5
		cy := int(math.Floor(v))
		fy := v - float64(cy)
		iy := cy - (gridY - 1)
		for dpx := 0; dpx < 2*N; dpx++ {
			px := startPxX + dpx
			u := (float64(px)+0.5)*invN - 0.5
			cx := int(math.Floor(u))
			fx := u - float64(cx)
			ix := cx - (gridX - 1)

			c00 := col[iy][ix]
			c10 := col[iy][ix+1]
			c01 := col[iy+1][ix]
			c11 := col[iy+1][ix+1]

			r := (1-fy)*((1-fx)*c00.R+fx*c10.R) + fy*((1-fx)*c01.R+fx*c11.R)
			gv := (1-fy)*((1-fx)*c00.G+fx*c10.G) + fy*((1-fx)*c01.G+fx*c11.G)
			b := (1-fy)*((1-fx)*c00.B+fx*c10.B) + fy*((1-fx)*c01.B+fx*c11.B)

			wpx := ((px % imgW) + imgW) % imgW
			wpy := ((py % imgH) + imgH) % imgH
			envImage.Set(wpx, wpy, color.RGBA{
				R: uint8(r * 255), G: uint8(gv * 255), B: uint8(b * 255), A: 255,
			})
		}
	}
}

// phToColor maps a pH value to its display colour. Extracted from the
// per-pixel render loop so renderPhValue and rebuildEnvLayer can share
// the same blend formula.
func (g *Grid) phToColor(phVal float64) colorful.Color {
	neutral := (config.MaxPh() + config.MinPh()) / 2.0
	halfRange := (config.MaxPh() - config.MinPh()) / 2.0
	weight := 0.0
	if halfRange > 0 {
		weight = math.Abs(phVal-neutral) / halfRange
		if weight > 1 {
			weight = 1
		}
	}
	bgR, bgG, bgB := config.ThemeBackgroundRGB()
	tgtR, tgtG, tgtB := config.PhTargetColorRGB(phVal)
	bg := colorful.Color{R: bgR, G: bgG, B: bgB}
	target := colorful.Color{R: tgtR, G: tgtG, B: tgtB}
	return bg.BlendRgb(target, weight).Clamped()
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
		g.renderSelectionBox(g.mouseHoverLocation, viewportImage, themedForegroundDim())
	}
	if info := g.simulation.GetOrganismInfoByID(g.simulation.GetSelected()); info != nil {
		g.renderSelectionBox(info.Location, viewportImage, themedForeground())
	}
}

// RenderOverlayText draws the mode label and hover-info text directly onto
// the final screen. Called by the Interface after compositing the scaled
// grid, so the text isn't multiplied by GridDisplayScale.
func (g *Grid) RenderOverlayText(screen *ebiten.Image) {
	if !g.mouseOnGrid {
		return
	}

	// View mode / selection / zoom label at top-left of the grid area.
	// Zoom label shows the on-screen unit size, e.g. "ZOOM: 16px", which
	// is what the camera actually renders at (pre-GridDisplayScale).
	xPadding := 10
	yPadding := 20
	fg := themedForeground()
	info := fmt.Sprintf("VIEW MODE: %s\nSELECTED: %s\nZOOM: %dpx", viewModeNames[g.viewMode], selectModeNames[g.selectMode], g.Camera.GridUnitSize())
	text.Draw(screen, info, resources.FontSourceCodePro10, panelWidth+xPadding, yPadding, fg)

	// Hover info text near the cursor.
	infoText := fmt.Sprintf("PH: %2.1f", g.simulation.GetPhAtPoint(g.mouseHoverLocation))
	if info := g.simulation.GetOrganismInfoAtPoint(g.mouseHoverLocation); info != nil {
		infoText += fmt.Sprintf("\nORG: %d", info.ID)
		infoText += fmt.Sprintf("\nSIZE: %.0f", info.Size)
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
	text.Draw(screen, infoText, resources.FontSourceCodePro10, screenX, screenY, fg)
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

func (g *Grid) renderSelectionBox(point utils.Point, img *ebiten.Image, col color.Color) {
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

	// Box sits exactly on the selected cell's outer boundary. Previously
	// an us/8 outset was added, which scaled with zoom and visibly spilled
	// into adjacent cells at high zoom levels.
	x0, y0 := vx, vy
	x1, y1 := vx+us, vy+us
	ebitenutil.DrawLine(img, x0, y0, x1, y0, col)
	ebitenutil.DrawLine(img, x0, y0, x0, y1, col)
	ebitenutil.DrawLine(img, x0, y1, x1, y1, col)
	ebitenutil.DrawLine(img, x1, y0, x1, y1, col)
}

func (g *Grid) renderFoodItem(item *food.Item, img *ebiten.Image) {
	us := g.unitSize()
	x := float64(item.Point.X) * float64(us)
	y := float64(item.Point.Y) * float64(us)
	sprite := resources.Sprite(foodRoleForValue(item.Value), animation.AnimIdle, 0)
	g.drawStaticSprite(img, x, y, sprite, foodColor)
}

// foodRoleForValue maps a food item's value to one of the three food
// size-tier sprites. Thirds of MaxFoodValue, matching the organism
// size-tier split:
//
//	value < MaxFoodValue/3     → small
//	value < 2*MaxFoodValue/3   → medium
//	else                       → large
func foodRoleForValue(value int) resources.ImageRole {
	maxVal := config.MaxFoodValue()
	if maxVal <= 0 {
		return resources.RoleFoodMedium
	}
	v := float64(value)
	third := float64(maxVal) / 3.0
	switch {
	case v < third:
		return resources.RoleFoodSmall
	case v < 2*third:
		return resources.RoleFoodMedium
	default:
		return resources.RoleFoodLarge
	}
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

	// Size-to-role mapping: thirds of MaximumMaxSize.
	//   small  — below 33%
	//   medium — 33% to below 66%
	//   large  — 66% and above
	maxSize := config.MaximumMaxSize()
	var role resources.ImageRole
	switch {
	case info.Size < maxSize*(1.0/3.0):
		role = resources.RoleOrganismSmall
	case info.Size < maxSize*(2.0/3.0):
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
		// Animation is entirely sprite-based — the spritesheet paints any
		// motion between cells. We only gate sprite-frame advancement on
		// playback speed (skip above 2x) and on-screen unit size (skip
		// when sprites are too small to read per-frame differences). When
		// either says "don't animate" we snap to a single frame instead
		// of cycling.
		animate := g.animState.AnimatesPosition() && g.unitSize() >= minOrganismAnimationUnitSize
		if frame, ok := g.animState.Frames[info.ID]; ok {
			gridX, gridY = animatedCellPosition(frame)
			direction = frame.Direction
			// ForFrame, not ForAction, so a move whose position didn't
			// change falls through to AnimBlocked instead of drawing the
			// 2-cell travel sprite in place.
			anim = animation.ForFrame(frame)
		}
		if animate {
			frameIdx = g.animState.SpriteFrameIndex(g.Camera.SpriteFrameCount())
		} else if isMultiCellAnim(anim) {
			// Multi-cell (_xl) sprites depict per-frame detail that
			// matters for visual consistency at cycle boundaries —
			// move/attack's travel path, eat's crumbs in the adjacent
			// cell, etc. Without frame advancement we'd stay on frame 0
			// for the whole cycle, which typically depicts a mid-action
			// state and snaps backwards when the next cycle begins.
			// Snap to the last frame instead — the authored end state.
			frameIdx = g.Camera.SpriteFrameCount() - 1
			if frameIdx < 0 {
				frameIdx = 0
			}
		}
	}

	sprite := resources.Sprite(role, anim, frameIdx)
	g.drawOrganismSprite(img, gridX*us, gridY*us, sprite, direction, organismColor)
}

// animatedCellPosition returns the organism's grid-unit anchor for the
// current render frame. All motion is painted by the spritesheet itself,
// so we just anchor at FromLocation — the sprite's base-cell origin.
//
// For 2-cell actions (move, attack, eat) this puts the base cell at the
// source and the extending cell in the direction the organism is
// facing; single-cell actions have FromLocation == ToLocation so the
// anchor choice doesn't matter.
func animatedCellPosition(f animation.Frame) (float64, float64) {
	return float64(f.FromLocation.X), float64(f.FromLocation.Y)
}

// isMultiCellAnim reports whether the animation uses a 2-cell (_xl)
// spritesheet that extends into the cell ahead of the organism. Multi-
// cell sprites need frame-index handling that differs from 1-cell
// sprites: without frame advancement we snap to the last frame so the
// authored end state (shape at top of the 2-cell canvas) is what shows
// for the whole cycle, not the pre-action start state.
func isMultiCellAnim(a animation.Animation) bool {
	switch a {
	case animation.AnimMove, animation.AnimAttack, animation.AnimEat:
		return true
	}
	return false
}

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
