package ux

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/instrument"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
)

type mode int
type layerType int

// Render layers, stacked bottom-to-top in compose order. Each layer's
// rendering lives in its own file (grid_ph.go, grid_walls.go, etc.);
// this file only owns the layer-image map, the compose loop, and the
// helpers shared across layers.
const (
	layerPh layerType = iota
	layerWalls
	layerFood
	layerOrganisms
	layerSelection
)

// orgColor mode controls how organisms are tinted on the grid. The pH
// and food layers are independent toggles; this mode only affects the
// organism layer.
const (
	orgColorTrue mode = iota
	orgColorPhEffect
	orgColorHealth
)

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
	// showPh / showFood / showOrganisms / showWalls toggle their respective layers
	// independently of the organism colour mode. orgColor decides how
	// organisms are tinted when shown.
	showPh        bool
	showFood      bool
	showOrganisms bool
	showWalls     bool
	orgColor      mode
	selectMode    mode
	clearImg      *ebiten.Image
	// selectionBoxImg is the source bitmap stamped onto layerSelection
	// for every highlighted organism. Authored white-on-transparent so
	// the per-stamp ColorScale can tint to any selection colour.
	// Rebuilt on zoom change.
	selectionBoxImg *ebiten.Image

	// phBuffer is the W × H source-of-truth for the pH layer: one
	// pixel per cell, no border. Updated in place via Set on
	// incremental changes or WritePixels on full refresh. Acts as the
	// source for the sub-image draws that build phBordered, so we
	// never draw an image to itself.
	phBuffer *ebiten.Image

	// phBordered is the (W+2) × (H+2) scratch image that backs the pH
	// layer: phBuffer's content plus a 1-pixel wrap-aware border on
	// every side, stamped in via sub-image draws from phBuffer. Its
	// border lets the gradient blend across world wrap edges so the
	// wallpaper-tile seam disappears. Size is independent of zoom.
	phBordered *ebiten.Image

	// phLinear is the intermediate for the pH layer's two-pass
	// upscale: phBordered scaled phLinearUpscale× with FilterLinear.
	// A second FilterNearest pass blows this the rest of the way up
	// into layers[layerPh], so the smooth gradient reads as chunky
	// pixel blocks. Size is independent of zoom.
	phLinear *ebiten.Image

	// Per-phase render timings, refreshed each call to Render(). Surfaced
	// to the debug overlay so a slow frame can be attributed to a
	// specific layer or compose pass.
	timeWalls          time.Duration
	timePh             time.Duration
	timeFood           time.Duration
	timeOrganisms      time.Duration
	timeCompose        time.Duration
	timeSelectionBoxes time.Duration

	// descHighlight caches the set of descendant IDs to highlight for
	// the currently-selected organism, valid for a window of cycles
	// around the playhead. The set is built once per (selection,
	// window) and consulted O(1) per living organism per render —
	// avoiding the per-frame subtree walk that got expensive when we
	// allowed dead-organism selections.
	descHighlight *descHighlightCache
}

// RenderTimings is the per-phase breakdown surfaced to the debug
// overlay. Returned by LastRenderTimings.
type RenderTimings struct {
	Walls          time.Duration
	Ph             time.Duration
	Food           time.Duration
	Organisms      time.Duration
	Compose        time.Duration
	SelectionBoxes time.Duration
}

// LastRenderTimings returns the per-phase timings from the most recent
// Render() call. Used by the debug overlay; safe to call any time.
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
		layerPh:        g.newBlankLayer(),
		layerWalls:     g.newBlankLayer(),
		layerFood:      g.newBlankLayer(),
		layerOrganisms: g.newBlankLayer(),
		layerSelection: g.newBlankLayer(),
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

// Render paints each layer (pH/walls/food/organisms/selection) into its
// own off-screen image, then composes a viewport-sized result by tiling
// the layers as a wallpaper around the camera position. The per-layer
// rendering logic lives in grid_ph.go, grid_walls.go, grid_food.go,
// grid_organisms.go, and grid_selection.go — this method is purely the
// orchestrator.
func (g *Grid) Render() *ebiten.Image {
	if g.doRefresh {
		g.layers[layerPh] = g.newBlankLayer()
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
	g.timeFood = time.Since(t)

	// Fetch the alive organism map once per render so renderOrganisms
	// and the selection layer can share it. Each call rebuilds a fresh
	// map and allocates an Info per organism — sharing halves that cost
	// when a selection is active.
	aliveInfos := g.simulation.GetAllOrganismInfo()

	t = time.Now()
	g.renderOrganisms(g.layers[layerOrganisms], g.doRefresh, aliveInfos)
	g.timeOrganisms = time.Since(t)

	// Populate the selection layer before compose so its tile-draw
	// folds into the same wallpaper loop as the other layers — one
	// DrawImage per highlight (instead of 4 line draws × N visible
	// tiles per highlight, which dominated frames at low zoom with a
	// sizeable descendant set).
	t = time.Now()
	g.populateSelectionLayer(aliveInfos)
	g.timeSelectionBoxes = time.Since(t)

	composeStart := time.Now()

	// Compose visible portion into viewport-sized image. The world is
	// rendered as a tiled wallpaper of copies — the simulation grid
	// wraps mathematically (utils.Point.Wrap), so each tile is a valid
	// view of the same world. Camera position picks which slice of the
	// wallpaper sits at viewport (0,0); zooming out far enough to see
	// the whole grid still tiles the surrounding viewport area.
	viewportImage := instrument.NewImage(g.Camera.ViewportW, g.Camera.ViewportH)

	us := g.unitSize()
	wpw := float64(g.Camera.WorldPixelWidth())
	wph := float64(g.Camera.WorldPixelHeight())
	vw := float64(g.Camera.ViewportW)
	vh := float64(g.Camera.ViewportH)

	// Camera position in pixels, normalized to [0, worldPixelSize).
	camPxX := g.Camera.NormalizedX() * float64(us)
	camPxY := g.Camera.NormalizedY() * float64(us)

	// firstOffset returns the leftmost / topmost tile origin so that
	// the tile straddling viewport coord 0 is included. Subsequent
	// tiles step by world-pixel size until viewport is fully covered.
	firstOffset := func(camPx, worldPx float64) float64 {
		// camPx is the in-world pixel where viewport (0,0) lands.
		// Tile origin = -camPx, then walk back by worldPx until the
		// origin is <= 0.
		off := -camPx
		for off > 0 {
			off -= worldPx
		}
		return off
	}

	xStart := firstOffset(camPxX, wpw)
	yStart := firstOffset(camPxY, wph)

	// drawLayer paints the given layer at every tile origin needed to
	// cover the viewport. scaleX/scaleY scale the layer image up to
	// world-pixel size where applicable (e.g. the pH layer is at
	// sprite-native resolution). filter picks the ebiten sampling mode.
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
		// pH layer was upscaled from the bordered scratch into world-
		// pixel size by renderPh, so it tiles 1:1 here like every other
		// layer.
		drawLayer(g.layers[layerPh], 1, 1, ebiten.FilterNearest)
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
	// Hover cell first (so any selection box stamped over the same
	// cell wins on top), then the selection layer wallpaper.
	if g.mouseOnGrid {
		g.renderHoverCellBox(viewportImage, themedForegroundDim())
	}
	drawLayer(g.layers[layerSelection], 1, 1, ebiten.FilterNearest)
	g.timeCompose = time.Since(composeStart)

	// Overlay text is drawn on the final screen by the caller (see
	// RenderOverlayText) so it isn't multiplied by GridDisplayScale.

	g.doRefresh = false
	return viewportImage
}

// RenderOverlayText draws the mode label and hover-info text directly onto
// the final screen. Called by the Interface after compositing the scaled
// grid, so the text isn't multiplied by GridDisplayScale.
func (g *Grid) RenderOverlayText(screen *ebiten.Image) {
	if !g.mouseOnGrid {
		return
	}

	// Zoom label at top-left of the grid area. Shows the on-screen
	// unit size — what the camera actually renders at, pre-GridDisplayScale.
	xPadding := 10
	yPadding := 20
	fg := themedForeground()
	info := fmt.Sprintf("ZOOM: %dpx", g.Camera.GridUnitSize())
	text.Draw(screen, info, resources.FontSourceCodePro10, panelWidth+xPadding, yPadding, fg)

	// Hover info text near the cursor.
	infoText := fmt.Sprintf("PH: %2.1f", g.simulation.GetPhAtPoint(g.mouseHoverLocation))
	if info := g.simulation.GetOrganismInfoAtPoint(g.mouseHoverLocation); info != nil {
		infoText += fmt.Sprintf("\nORG: %d", info.ID)
		infoText += fmt.Sprintf("\nSIZE: %.0f", info.Size)
	} else if strength := g.simulation.GetWallStrengthAtPoint(g.mouseHoverLocation); strength > 0 {
		infoText += fmt.Sprintf("\nWALL: %d", strength)
	} else if foodItem, exists := g.simulation.GetFoodAtPoint(g.mouseHoverLocation); exists {
		infoText += fmt.Sprintf("\nFOOD: %d", foodItem.Value)
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

// ShowPh reports whether the pH layer is currently being drawn.
func (g *Grid) ShowPh() bool { return g.showPh }

// ShowFood reports whether the food layer is currently being drawn.
func (g *Grid) ShowFood() bool { return g.showFood }

// ShowOrganisms reports whether the organism layer is currently drawn.
func (g *Grid) ShowOrganisms() bool { return g.showOrganisms }

// OrgColor reports the active organism colour mode.
func (g *Grid) OrgColor() mode { return g.orgColor }

func (g *Grid) SetManualSelection() {
	g.selectMode = selectManual
}

func (g *Grid) MouseHover(point utils.Point, onGrid bool) {
	g.mouseHoverLocation = point
	g.mouseOnGrid = onGrid
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
