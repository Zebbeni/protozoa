package ux

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"

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
	attackColor        = colorful.HSLuv(0.0, 255.0, 1.0)
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

	layers map[layerType]*ebiten.Image

	mouseHoverLocation utils.Point
	mouseOnGrid        bool
	doRefresh          bool
	viewMode           mode
	selectMode         mode
	clearImg           *ebiten.Image
}

func NewGrid(sim *simulation.Simulation) *Grid {
	viewportW := config.ScreenWidth() - panelWidth
	viewportH := config.ScreenHeight()
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
	resources.SelectZoom(int(g.Camera.Zoom))
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
	envImage := g.newEnvLayer()
	wallsImage := g.newBlankLayer()
	foodImage := g.newBlankLayer()
	orgsImage := g.newBlankLayer()
	selImage := g.newBlankLayer()

	g.renderWalls(wallsImage, g.doRefresh)
	g.renderEnvironment(envImage, g.doRefresh)
	g.renderFood(foodImage, g.doRefresh)
	g.renderOrganisms(orgsImage, g.doRefresh)
	g.renderSelectionBoxes(selImage)

	g.layers[layerEnv] = envImage
	g.layers[layerWalls] = wallsImage
	g.layers[layerFood] = foodImage
	g.layers[layerOrganisms] = orgsImage

	// Compose visible portion into viewport-sized image.
	// When zoomed out fully, scale the world to fit the viewport and center it.
	viewportImage := ebiten.NewImage(g.Camera.ViewportW, g.Camera.ViewportH)
	fitScale, offsetX, offsetY := g.Camera.FitScaleAndOffset()

	visRect := g.Camera.VisibleRect()
	drawOp := &ebiten.DrawImageOptions{}
	drawOp.GeoM.Translate(float64(-visRect.Min.X), float64(-visRect.Min.Y))
	if fitScale != 1.0 {
		drawOp.GeoM.Scale(fitScale, fitScale)
		drawOp.Filter = ebiten.FilterLinear // smooth scaling when fitting world to viewport
	}
	drawOp.GeoM.Translate(float64(offsetX), float64(offsetY))

	if g.viewMode == orgsPhMode || g.viewMode == phOnlyMode {
		// Environment is 1px-per-cell; scale up to world-pixel size with linear filtering
		envOp := &ebiten.DrawImageOptions{}
		us := float64(g.unitSize())
		envOp.GeoM.Scale(us, us)
		envOp.GeoM.Translate(float64(-visRect.Min.X), float64(-visRect.Min.Y))
		if fitScale != 1.0 {
			envOp.GeoM.Scale(fitScale, fitScale)
		}
		envOp.GeoM.Translate(float64(offsetX), float64(offsetY))
		envOp.Filter = ebiten.FilterLinear
		viewportImage.DrawImage(envImage, envOp)
	}
	viewportImage.DrawImage(wallsImage, drawOp)
	if g.viewMode != phOnlyMode {
		viewportImage.DrawImage(foodImage, drawOp)
		viewportImage.DrawImage(orgsImage, drawOp)
	}
	viewportImage.DrawImage(selImage, drawOp)

	// Draw text overlays at screen resolution (after scaling)
	g.renderOverlayText(viewportImage)

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
		envImage.DrawImage(g.layers[layerEnv], nil)
		updatedPoints := g.simulation.GetUpdatedPhPoints()
		for _, point := range updatedPoints {
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
	} else {
		wallsImage.DrawImage(g.layers[layerWalls], nil)
	}
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
		foodImage.DrawImage(g.layers[layerFood], nil)
		updatedPoints := g.simulation.GetUpdatedFoodPoints()
		for _, point := range updatedPoints {
			us := g.unitSize()
			x, y := point.X*us, point.Y*us
			g.clearSquare(foodImage, float64(x), float64(y))
			if item, exists := g.simulation.GetFoodAtPoint(point); exists {
				g.renderFoodItem(item, foodImage)
			}
		}
	}
}

func (g *Grid) renderOrganisms(organismsImage *ebiten.Image, refresh bool) {
	if refresh {
		organismInfo := g.simulation.GetAllOrganismInfo()
		for _, info := range organismInfo {
			g.renderOrganism(info, organismsImage)
		}
	} else {
		organismsImage.DrawImage(g.layers[layerOrganisms], nil)
		updatedPoints := g.simulation.GetUpdatedOrganismPoints()
		for _, point := range updatedPoints {
			us := g.unitSize()
			x, y := point.X*us, point.Y*us
			g.clearSquare(organismsImage, float64(x), float64(y))
			if info := g.simulation.GetOrganismInfoAtPoint(point); info != nil {
				g.renderOrganism(info, organismsImage)
			}
		}
	}
}

// renderSelectionBoxes draws selection box outlines in world-space (gets scaled with the world).
func (g *Grid) renderSelectionBoxes(selectionsImage *ebiten.Image) {
	if g.mouseOnGrid {
		g.renderSelectionBox(g.mouseHoverLocation, selectionsImage, hoverColor)
	}
	if info := g.simulation.GetOrganismInfoByID(g.simulation.GetSelected()); info != nil {
		g.renderSelectionBox(info.Location, selectionsImage, selectColor)
	}
}

// renderOverlayText draws text overlays at screen resolution on the viewport image.
func (g *Grid) renderOverlayText(viewportImage *ebiten.Image) {
	// View mode name at top-left
	if g.mouseOnGrid {
		xPadding := 10
		yPadding := 20
		info := fmt.Sprintf("VIEW MODE: %s\nSELECTED: %s", viewModeNames[g.viewMode], selectModeNames[g.selectMode])
		text.Draw(viewportImage, info, resources.FontSourceCodePro10, xPadding, yPadding, selectionInfoColor)
	}

	// Hover info text near the cursor
	if g.mouseOnGrid {
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

		// Position text near the hovered grid cell in screen coordinates
		mx, my := ebiten.CursorPosition()
		screenX := mx - panelWidth + 15
		screenY := my - 10
		bounds := boundString(resources.FontSourceCodePro10, infoText)
		if screenX+bounds.Dx() > g.Camera.ViewportW {
			screenX = mx - panelWidth - bounds.Dx() - 15
		}
		_ = infoColor
		text.Draw(viewportImage, infoText, resources.FontSourceCodePro10, screenX, screenY, selectionInfoColor)
	}
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
	x, y := float64(point.X)*us, float64(point.Y)*us
	ebitenutil.DrawLine(img, x-2, y-2, x+us+3, y-2, col)
	ebitenutil.DrawLine(img, x-2, y-2, x-2, y+us+3, col)
	ebitenutil.DrawLine(img, x-2, y+us+3, x+us+3, y+us+3, col)
	ebitenutil.DrawLine(img, x+us+3, y-2, x+us+3, y+us+3, col)
}

func (g *Grid) renderFoodItem(item *food.Item, img *ebiten.Image) {
	us := g.unitSize()
	x := float64(item.Point.X) * float64(us)
	y := float64(item.Point.Y) * float64(us)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.ColorM.Translate(foodColor.R, foodColor.G, foodColor.B, 0)
	img.DrawImage(resources.Images[resources.RoleFood], op)
}

func (g *Grid) renderWall(wallsImage *ebiten.Image, point utils.Point) {
	us := g.unitSize()
	x := float64(point.X) * float64(us)
	y := float64(point.Y) * float64(us)
	g.drawSprite(wallsImage, x, y, resources.RoleBox, wallColor)
}

func (g *Grid) renderOrganism(info *organism.Info, img *ebiten.Image) {
	us := g.unitSize()
	point := info.Location.Times(us)
	x, y := float64(point.X), float64(point.Y)

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

	if info.Action == decision.ActAttack {
		organismColor = attackColor
	}

	g.drawSprite(img, x, y, role, organismColor)
}

func (g *Grid) drawSprite(img *ebiten.Image, x, y float64, role resources.ImageRole, col colorful.Color) {
	spriteImg := resources.Images[role]
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.ColorM.Translate(col.R, col.G, col.B, 0)
	img.DrawImage(spriteImg, op)
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
