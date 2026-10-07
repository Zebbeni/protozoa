package ux

import (
	"fmt"
	d "github.com/Zebbeni/protozoa/decision"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/instrument"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
)

type mode int
type layerType int

// Render layers, stacked bottom-to-top in compose order.
const (
	layerPh layerType = iota
	// layerBuriedFood is drawn straight above the pH wash and below everything else.
	layerBuriedFood
	layerWalls
	layerFood
	layerOrganisms
	layerSelection
)

// orgColor mode controls how organisms are tinted on the grid.
const (
	orgColorTrue mode = iota
	orgColorPhEffect
	orgColorHealth
	// orgColorAbility tints each organism gray→green by its score in Grid.colorAbility.
	orgColorAbility
	// orgColorSuccess tints each organism gray→green by how much of the time left in the recorded run its line of descent survives.
	orgColorSuccess
	// orgColorTolerance tints each organism green→red by how well it tolerates the pH of the cell it's in.
	orgColorTolerance
	// orgColorFamily tints each organism by how it is related to the selected one.
	orgColorFamily
	// orgColorAge tints each organism gray→green by how far through its life it is, against max_lifespan.
	orgColorAge
	// orgColorSize tints each organism gray→green by its size against maximum_max_size.
	orgColorSize
	// orgColorAction tints by one action: brightest for an organism taking it right now, gray→mid green by that action's share of its decision tree otherwise.
	orgColorAction
)

var allOrgColorModes = []mode{
	orgColorTrue,
	orgColorPhEffect,
	orgColorHealth,
	orgColorAbility,
	orgColorSuccess,
	orgColorTolerance,
	orgColorFamily,
	orgColorAge,
	orgColorSize,
	orgColorAction,
}

const (
	selectOldest mode = iota
	selectMostChildren
	selectMostTraveled
	selectMostSuccessful
	selectMostAggressive
	selectManual
)

type Grid struct {
	simulation *simulation.Simulation
	Camera     *Camera
	animState  *animation.State

	layers map[layerType]*ebiten.Image

	mouseHoverLocation utils.Point
	mouseOnGrid        bool
	doRefresh          bool
	// showPh / showFood / showOrganisms / showWalls toggle their respective layers independently of the organism colour mode.
	showPh   bool
	showFood bool
	// showBuriedFood is off for a live run and ON for a replay, the one layer whose default depends on which it is.
	showBuriedFood bool
	showOrganisms  bool
	showWalls      bool
	orgColor       mode
	selectMode     mode
	// colorAbility is the ability orgColorAbility colours by.
	colorAbility physiology.Ability
	// colorAction is the action orgColorAction colours by, kept when the mode changes like colorAbility is.
	colorAction d.Action

	// familyTint answers the FAMILY colour mode's "how is this organism related to the selected one".
	familyTint *familyTinter

	// oldestAlive is the age of the oldest living organism, refreshed once per organism-layer pass.
	oldestAlive int
	// drawOrder is the per-frame organism draw order, reused to keep the sort out of the allocator.
	drawOrder []*organism.Info
	clearImg  *ebiten.Image
	// selectionBoxImg is the source bitmap stamped onto layerSelection for every highlighted organism.
	selectionBoxImg *ebiten.Image

	// phBuffer is the W × H source-of-truth for the pH layer: one pixel per cell, no border.
	phBuffer *ebiten.Image

	// phBordered is the (W+2) × (H+2) scratch image that backs the pH layer.
	phBordered *ebiten.Image

	// phLinear is phBordered upscaled with FilterLinear to the active sprite set's resolution (Camera.SpriteSize px per cell).
	phLinear     *ebiten.Image
	phLinearCell int

	// Per-phase render timings, refreshed each call to Render().
	timeWalls          time.Duration
	timePh             time.Duration
	timeFood           time.Duration
	timeOrganisms      time.Duration
	timeCompose        time.Duration
	timeSelectionBoxes time.Duration
}

// RenderTimings is the per-phase breakdown surfaced to the debug overlay.
type RenderTimings struct {
	Walls          time.Duration
	Ph             time.Duration
	Food           time.Duration
	Organisms      time.Duration
	Compose        time.Duration
	SelectionBoxes time.Duration
}

func (g *Grid) LastRenderTimings() RenderTimings {
	return RenderTimings{
		Walls:          g.timeWalls,
		Ph:             g.timePh,
		Food:           g.timeFood,
		Organisms:      g.timeOrganisms,
		Compose:        g.timeCompose,
		SelectionBoxes: g.timeSelectionBoxes,
	}
}

func NewGrid(sim *simulation.Simulation) *Grid {
	viewportW := (config.ScreenWidth() - panelWidth) / GridDisplayScale
	viewportH := config.ScreenHeight() / GridDisplayScale
	cam := NewCamera(viewportW, viewportH)

	g := &Grid{
		simulation:    sim,
		Camera:        cam,
		doRefresh:     true,
		showPh:        true,
		showFood:      true,
		showOrganisms: true,
		showWalls:     true,
		orgColor:      orgColorTrue,
		selectMode:    selectOldest,
	}
	g.initLayerImages()
	g.loadOrganismImages()
	g.buildClearImg()
	return g
}

func (g *Grid) initLayerImages() {
	g.layers = map[layerType]*ebiten.Image{
		layerBuriedFood: g.newBlankLayer(),
		layerPh:         g.newBlankLayer(),
		layerWalls:      g.newBlankLayer(),
		layerFood:       g.newBlankLayer(),
		layerOrganisms:  g.newBlankLayer(),
		layerSelection:  g.newBlankLayer(),
	}
	g.buildSelectionBoxImg()
}

func (g *Grid) loadOrganismImages() {
	resources.SelectZoom(g.Camera.SpriteSet())
}

func (g *Grid) newBlankLayer() *ebiten.Image {
	return ebiten.NewImage(g.Camera.WorldPixelWidth(), g.Camera.WorldPixelHeight())
}

func (g *Grid) unitSize() int {
	return g.Camera.GridUnitSize()
}

// SetAnimationState attaches the playback animation state the renderer should read from.
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

// Render paints each layer (pH/walls/food/organisms/selection) into its own off-screen image, then composes a viewport-sized result by tiling the layers as a wallpaper around the camera position.
func (g *Grid) Render() *ebiten.Image {
	if g.doRefresh {
		g.layers[layerPh] = g.newBlankLayer()
		g.layers[layerBuriedFood] = g.newBlankLayer()
		g.layers[layerWalls] = g.newBlankLayer()
		g.layers[layerFood] = g.newBlankLayer()
		g.layers[layerOrganisms] = g.newBlankLayer()
		g.layers[layerSelection] = g.newBlankLayer()
	}

	t := time.Now()
	g.renderWalls(g.layers[layerWalls], g.doRefresh)
	g.timeWalls = time.Since(t)

	t = time.Now()
	g.renderPh(g.layers[layerPh], g.doRefresh)
	g.timePh = time.Since(t)

	t = time.Now()
	g.renderFood(g.layers[layerFood], g.doRefresh)
	// Both food layers share the same dirty-cell set.
	g.renderBuriedFood(g.layers[layerBuriedFood], g.doRefresh)
	g.timeFood = time.Since(t)

	// Fetch the alive organism map once per render so renderOrganisms and the selection layer can share it.
	aliveInfos := g.simulation.GetAllOrganismInfo()

	t = time.Now()
	g.renderOrganisms(g.layers[layerOrganisms], g.doRefresh, aliveInfos)
	g.timeOrganisms = time.Since(t)

	// Populate the selection layer before compose so its tile-draw folds into the same wallpaper loop as the other layers.
	t = time.Now()
	g.populateSelectionLayer(aliveInfos)
	g.timeSelectionBoxes = time.Since(t)

	composeStart := time.Now()

	viewportImage := instrument.NewImage(g.Camera.ViewportW, g.Camera.ViewportH)

	us := g.unitSize()
	wpw := float64(g.Camera.WorldPixelWidth())
	wph := float64(g.Camera.WorldPixelHeight())
	vw := float64(g.Camera.ViewportW)
	vh := float64(g.Camera.ViewportH)

	// Camera position in pixels, normalized to [0, worldPixelSize).
	camPxX := g.Camera.NormalizedX() * float64(us)
	camPxY := g.Camera.NormalizedY() * float64(us)

	// firstOffset returns the leftmost / topmost tile origin so that the tile straddling viewport coord 0 is included.
	firstOffset := func(camPx, worldPx float64) float64 {
		// camPx is the in-world pixel where viewport (0,0) lands.
		off := -camPx
		for off > 0 {
			off -= worldPx
		}
		return off
	}

	xStart := firstOffset(camPxX, wpw)
	yStart := firstOffset(camPxY, wph)

	// drawLayer paints the given layer at every tile origin needed to cover the viewport.
	drawLayer := func(layer *ebiten.Image, scaleX, scaleY float64, filter ebiten.Filter) {
		for ox := xStart; ox < vw; ox += wpw {
			for oy := yStart; oy < vh; oy += wph {
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

	if g.showPh {
		// pH layer was upscaled from the bordered scratch into world- pixel size by renderPh.
		drawLayer(g.layers[layerPh], 1, 1, ebiten.FilterNearest)
	}
	if g.showBuriedFood {
		drawLayer(g.layers[layerBuriedFood], 1, 1, ebiten.FilterNearest)
	}
	if g.showWalls {
		drawLayer(g.layers[layerWalls], 1, 1, ebiten.FilterNearest)
	}
	if g.showFood {
		drawLayer(g.layers[layerFood], 1, 1, ebiten.FilterNearest)
	}
	if g.showOrganisms {
		drawLayer(g.layers[layerOrganisms], 1, 1, ebiten.FilterNearest)
	}
	// Hover cell first (so any selection box stamped over the same cell wins on top), then the selection layer wallpaper.
	if g.mouseOnGrid {
		g.renderHoverCellBox(viewportImage, themedForegroundDim())
	}
	drawLayer(g.layers[layerSelection], 1, 1, ebiten.FilterNearest)
	g.timeCompose = time.Since(composeStart)

	g.doRefresh = false
	return viewportImage
}

// RenderOverlayText draws the mode label and hover-info text directly onto the final screen.
func (g *Grid) RenderOverlayText(screen *ebiten.Image) {
	if !g.mouseOnGrid {
		return
	}

	// Zoom label at top-left of the grid area.
	xPadding := 10
	yPadding := 20
	fg := themedForeground()
	info := fmt.Sprintf("ZOOM: %dpx", g.Camera.GridUnitSize())
	text.Draw(screen, info, resources.FontSourceCodePro10, panelWidth+xPadding, yPadding, fg)

	foodItem, hasFood := g.simulation.GetFoodAtPoint(g.mouseHoverLocation)
	if !hasFood {
		foodItem = nil
	}
	infoText := hoverInfoText(
		g.simulation.GetPhAtPoint(g.mouseHoverLocation),
		g.simulation.GetOrganismInfoAtPoint(g.mouseHoverLocation),
		g.simulation.GetWallStrengthAtPoint(g.mouseHoverLocation),
		foodItem,
		g.simulation.GetBuriedFoodAtPoint(g.mouseHoverLocation),
		g.mouseHoverLocation,
	)

	mx, my := ebiten.CursorPosition()
	screenX := mx + 15
	screenY := my - 10
	bounds := boundString(resources.FontSourceCodePro10, infoText)
	if screenX+bounds.Dx() > config.ScreenWidth() {
		screenX = mx - bounds.Dx() - 15
	}
	text.Draw(screen, infoText, resources.FontSourceCodePro10, screenX, screenY, fg)
}

// ShowPh reports whether the pH layer is currently being drawn.
func (g *Grid) ShowPh() bool { return g.showPh }

// ShowFood reports whether the food layer is currently being drawn.
func (g *Grid) ShowFood() bool { return g.showFood }

func (g *Grid) applyReplayViewDefaults() {
	g.showBuriedFood = true
}

func (g *Grid) ColorAction() d.Action { return g.colorAction }

func (g *Grid) ShowBuriedFood() bool { return g.showBuriedFood }

// ShowOrganisms reports whether the organism layer is currently drawn.
func (g *Grid) ShowOrganisms() bool { return g.showOrganisms }

func (g *Grid) OrgColor() mode { return g.orgColor }

func (g *Grid) ColorAbility() physiology.Ability { return g.colorAbility }

func (g *Grid) SetManualSelection() {
	g.selectMode = selectManual
}

func (g *Grid) MouseHover(point utils.Point, onGrid bool) {
	g.mouseHoverLocation = point
	g.mouseOnGrid = onGrid
}

// drawStaticSprite draws a non-rotated sprite at cell (x, y).
func (g *Grid) drawStaticSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, col colorful.Color) {
	g.drawStaticSpriteAlpha(img, x, y, spriteImg, col, 1)
}

func (g *Grid) drawStaticSpriteAlpha(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, col colorful.Color, alpha float64) {
	if spriteImg == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	if s := g.Camera.SpriteScale(); s != 1 {
		op.Filter = spriteFilter(s)
		op.GeoM.Scale(s, s)
	}
	op.GeoM.Translate(x, y)
	a := float32(alpha)
	op.ColorScale.Scale(float32(col.R)*a, float32(col.G)*a, float32(col.B)*a, a)
	img.DrawImage(spriteImg, op)
}

func (g *Grid) buildClearImg() {
	us := g.unitSize()
	g.clearImg = ebiten.NewImage(us, us)
	g.clearImg.Fill(color.White)
}

func (g *Grid) clearSquare(img *ebiten.Image, x, y float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.CompositeMode = ebiten.CompositeModeDestinationOut
	img.DrawImage(g.clearImg, op)
}
