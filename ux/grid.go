package ux

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/instrument"
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

// minOrganismAnimationUnitSize is the smallest per-cell unit size at which
// organism sprite animations play. Below this (Zoom4), the renderer pins
// to frame 0 of the sheet — at tiny sizes per-frame differences are too
// small to read and the flicker adds more noise than animation.
const minOrganismAnimationUnitSize = 8

var (
	foodColor = colorful.HSLuv(120, 0.2, 0.25)
	wallColor = colorful.HSLuv(60, 0.25, 0.1)
)

type Grid struct {
	simulation *simulation.Simulation
	Camera     *Camera
	animState  *animation.State

	layers map[layerType]*ebiten.Image

	mouseHoverLocation utils.Point
	mouseOnGrid        bool
	doRefresh          bool
	// showPh / showFood / showOrganisms toggle their respective layers
	// independently of the organism colour mode. orgColor decides how
	// organisms are tinted when shown.
	showPh        bool
	showFood      bool
	showOrganisms bool
	orgColor      mode
	selectMode    mode
	clearImg   *ebiten.Image
	// selectionBoxImg is the source bitmap stamped onto layerSelection
	// for every highlighted organism. Authored white-on-transparent so
	// the per-stamp ColorScale can tint to any selection colour.
	// Rebuilt on zoom change.
	selectionBoxImg *ebiten.Image

	// Per-phase render timings, refreshed each call to Render(). Surfaced
	// to the debug overlay so a slow frame can be attributed to a
	// specific layer or compose pass.
	timeWalls       time.Duration
	timeEnv         time.Duration
	timeFood        time.Duration
	timeOrganisms   time.Duration
	timeCompose     time.Duration
	timeSelectionBoxes time.Duration

	// descHighlight caches the set of descendant IDs to highlight for
	// the currently-selected organism, valid for a window of cycles
	// around the playhead. The set is built once per (selection,
	// window) and consulted O(1) per living organism per render —
	// avoiding the per-frame subtree walk that got expensive when we
	// allowed dead-organism selections.
	descHighlight *descHighlightCache
}

// descHighlightCache is the precomputed answer to "which IDs are
// descendants of selID and alive somewhere in [fromCycle, toCycle]?"
// Rebuilt on selection change or when the playhead crosses the window.
type descHighlightCache struct {
	selID              int
	fromCycle, toCycle int
	ids                map[int]struct{}
}

// descHighlightBufferCycles is how far ahead of the playhead each
// rebuild looks. Bigger means fewer rebuilds but a larger walk each
// time; ~1000 cycles at default speed = a rebuild every several seconds
// of wall-clock playback, which is barely perceptible.
const descHighlightBufferCycles = 1000

// RenderTimings is the per-phase breakdown surfaced to the debug
// overlay. Returned by LastRenderTimings.
type RenderTimings struct {
	Walls       time.Duration
	Env         time.Duration
	Food        time.Duration
	Organisms   time.Duration
	Compose     time.Duration
	SelectionBoxes time.Duration
}

// LastRenderTimings returns the per-phase timings from the most recent
// Render() call. Used by the debug overlay; safe to call any time.
func (g *Grid) LastRenderTimings() RenderTimings {
	return RenderTimings{
		Walls:       g.timeWalls,
		Env:         g.timeEnv,
		Food:        g.timeFood,
		Organisms:   g.timeOrganisms,
		Compose:     g.timeCompose,
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
		layerEnv:       g.newEnvLayer(),
		layerWalls:     g.newBlankLayer(),
		layerFood:      g.newBlankLayer(),
		layerOrganisms: g.newBlankLayer(),
		layerSelection: g.newBlankLayer(),
	}
	g.buildSelectionBoxImg()
}

// buildSelectionBoxImg creates a white outlined-square image at the
// current unit size, used as the source for every per-organism
// selection box. Stamping a tinted copy of this image is one DrawImage
// per highlight; the previous version drew 4 lines × N visible tiles
// per box, which dominated frames with many highlights at low zoom.
//
// Uses 1px-tall / 1px-wide filled rects (not DrawLine) so each side
// lands on a whole-pixel row/column and the corners overlap cleanly.
// DrawLine's sub-pixel boundary places y=0 on the row above pixel 0
// and gets clipped, leaving visible gaps on the top and left.
func (g *Grid) buildSelectionBoxImg() {
	us := g.unitSize()
	img := ebiten.NewImage(us, us)
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	usf := float64(us)
	ebitenutil.DrawRect(img, 0, 0, usf, 1, white)     // top
	ebitenutil.DrawRect(img, 0, 0, 1, usf, white)     // left
	ebitenutil.DrawRect(img, 0, usf-1, usf, 1, white) // bottom
	ebitenutil.DrawRect(img, usf-1, 0, 1, usf, white) // right
	g.selectionBoxImg = img
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
		g.layers[layerSelection] = g.newBlankLayer()
	}

	t := time.Now()
	g.renderWalls(g.layers[layerWalls], g.doRefresh)
	g.timeWalls = time.Since(t)

	t = time.Now()
	g.renderEnvironment(g.layers[layerEnv], g.doRefresh)
	g.timeEnv = time.Since(t)

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
	// world-pixel size where applicable (e.g. the env layer is at
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
		// env layer is already at sprite-native resolution with
		// bilinear-interpolated colours baked in by renderPhValue.
		// Compose with FilterNearest — the final upscale to display
		// pixels is purely a sprite-style integer multiplication.
		envScale := g.Camera.SpriteScale()
		drawLayer(g.layers[layerEnv], envScale, envScale, ebiten.FilterNearest)
	}
	drawLayer(g.layers[layerWalls], 1, 1, ebiten.FilterNearest)
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
// (black under dark, white under light).
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
func (g *Grid) renderOrganisms(organismsImage *ebiten.Image, refresh bool, organismInfo map[int]*organism.Info) {
	organismsImage.Clear()

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
			}
			g.renderOrganism(synth, organismsImage)
		}
	}
}

// populateSelectionLayer redraws the selection-box layer in world
// coordinates. Each highlight is a single DrawImage of selectionBoxImg
// with a per-color tint; the surrounding compose loop handles wallpaper
// tiling so we don't re-stamp the same box at every visible tile.
//
// Tiered styling, brightest last so it overdraws the rest:
//   - In selectMostSuccessful mode, every currently-living organism on
//     the most-successful set gets a faded box.
//   - Otherwise, every living descendant of the selected organism
//     (resolved through the descendant-highlight cache) gets a faded
//     box.
//   - The selected organism itself gets the full themed foreground.
//
// The hover-cell box is drawn separately in screen space (see Render),
// since it shouldn't tile across the wallpaper.
func (g *Grid) populateSelectionLayer(aliveInfos map[int]*organism.Info) {
	layer := g.layers[layerSelection]
	layer.Clear()

	selID := g.simulation.GetSelected()

	if g.selectMode == selectMostSuccessful {
		successfulColor := fadedForeground(0x40)
		for _, id := range g.simulation.GetMostSuccessfulIds() {
			if id == selID {
				continue // drawn last in bright colour
			}
			if info, ok := aliveInfos[id]; ok {
				g.stampSelectionBox(layer, info.Location, successfulColor)
			}
		}
	} else if selID >= 0 {
		descColor := fadedForeground(0x40)
		currentCycle := g.simulation.Cycle()

		// Rebuild the descendant set when selection changed or the
		// playhead left the cached window.
		c := g.descHighlight
		if c == nil || c.selID != selID || currentCycle < c.fromCycle || currentCycle > c.toCycle {
			from := currentCycle
			to := currentCycle + descHighlightBufferCycles
			g.descHighlight = &descHighlightCache{
				selID:     selID,
				fromCycle: from,
				toCycle:   to,
				ids:       g.buildDescendantHighlightSet(selID, from, to),
			}
			c = g.descHighlight
		}

		// Iterate the smaller of the two sets so the per-render cost
		// is O(min(live, descendants)).
		if len(c.ids) > 0 {
			if len(c.ids) <= len(aliveInfos) {
				for id := range c.ids {
					if info, ok := aliveInfos[id]; ok {
						g.stampSelectionBox(layer, info.Location, descColor)
					}
				}
			} else {
				for id, info := range aliveInfos {
					if _, ok := c.ids[id]; ok {
						g.stampSelectionBox(layer, info.Location, descColor)
					}
				}
			}
		}
	}

	if selID >= 0 {
		if info, ok := aliveInfos[selID]; ok {
			g.stampSelectionBox(layer, info.Location, themedForeground())
		}
	}
}

// stampSelectionBox draws a single tinted copy of selectionBoxImg onto
// the selection layer at the given world cell. ColorScale handles the
// alpha tint, so the same source bitmap covers every selection
// variant.
func (g *Grid) stampSelectionBox(layer *ebiten.Image, point utils.Point, col color.Color) {
	us := g.unitSize()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(point.X*us), float64(point.Y*us))
	r, gv, b, a := col.RGBA()
	op.ColorScale.Scale(float32(r)/0xffff, float32(gv)/0xffff, float32(b)/0xffff, float32(a)/0xffff)
	layer.DrawImage(g.selectionBoxImg, op)
}

// buildDescendantHighlightSet walks selID's descendant subtree once
// and returns the set of node IDs that are alive at any cycle in
// [fromCycle, toCycle]. Used to populate descHighlight; callers don't
// hit this code on every render — only when the cache is invalid
// (selection change or playhead crossed the window boundary).
//
// Two prunes keep the walk bounded for very old selections:
//   - StartCycle > toCycle: subtree not yet born by window's end.
//     Children always have StartCycle >= parent.StartCycle, so the
//     entire subtree is irrelevant.
//   - AllBranchesDeadCycle != 0 && < fromCycle: every node in this
//     subtree died strictly before the window opens; no descendant
//     could be alive in the window.
//
// A node N is alive somewhere in [fromCycle, toCycle] iff
// N.StartCycle <= toCycle AND (N.EndCycle == 0 OR N.EndCycle >= fromCycle).
func (g *Grid) buildDescendantHighlightSet(selID, fromCycle, toCycle int) map[int]struct{} {
	set := make(map[int]struct{})
	root := g.simulation.GetTreeNodeByID(selID)
	if root == nil {
		return set
	}
	var walk func(n *organism.DescendantNode)
	walk = func(n *organism.DescendantNode) {
		n.ForEachChild(func(child *organism.DescendantNode) {
			if child.StartCycle > toCycle {
				return
			}
			if child.AllBranchesDeadCycle != 0 && child.AllBranchesDeadCycle < fromCycle {
				return
			}
			if child.EndCycle == 0 || child.EndCycle >= fromCycle {
				set[child.ID] = struct{}{}
			}
			walk(child)
		})
	}
	walk(root)
	return set
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

// renderHoverCellBox draws a single hover-cell outline at the cursor's
// actual viewport position. Unlike per-organism selection highlights,
// which live on a tiled world layer, the hover cursor is a UI element
// that should appear once where the cursor is. The cell origin in
// viewport coords accounts for the camera's sub-cell offset — cells
// in a tiled view don't generally align to viewport pixel 0.
func (g *Grid) renderHoverCellBox(img *ebiten.Image, col color.Color) {
	mx, my := ebiten.CursorPosition()
	// Convert from screen pixels to viewport pixels (the same image
	// space the rest of the grid renders into).
	vx := float64(mx-panelWidth) / float64(GridDisplayScale)
	vy := float64(my) / float64(GridDisplayScale)
	us := float64(g.unitSize())
	camPxX := g.Camera.NormalizedX() * us
	camPxY := g.Camera.NormalizedY() * us
	// World pixel coordinates align to the camera. The cell containing
	// the cursor has its origin at world-pixel floor((vx+camPx)/us)*us;
	// translating back into viewport coords subtracts camPx.
	x0 := math.Floor((vx+camPxX)/us)*us - camPxX
	y0 := math.Floor((vy+camPxY)/us)*us - camPxY
	x1 := x0 + us
	y1 := y0 + us
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
	switch g.orgColor {
	case orgColorPhEffect:
		organismColor = phEffectColor(info.PhPositive, info.PhNegative)
	case orgColorHealth:
		organismColor = healthColor(info.Health, info.Size)
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
		} else if g.animState.Speed >= 4 && g.Camera.SpriteFrameCount() > 1 {
			// At 4x+ speed with multi-frame sprites (currently only the
			// 16x16 set), the wall-clock window per cycle is too short
			// to play a real animation — pin every organism to frame 1
			// so the action reads as a recognisable mid-action pose
			// instead of either the start state (frame 0) or a frozen
			// end state.
			frameIdx = 1
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
	case animation.AnimMove, animation.AnimAttack, animation.AnimEat, animation.AnimEatFail:
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
