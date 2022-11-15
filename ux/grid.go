package ux

import (
	"fmt"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"
	"math"
)

type size int
type mode int

const (
	sizeSmall size = iota
	sizeMedium
	sizeLarge
	sizeFill
	sizeBox
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

const (
	phMaxHue = 120.0
)

var (
	squareImgSmall, squareImgMedium, squareImgLarge, squareImgFill, squareImgBox *ebiten.Image

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

	previousEnvImage   *ebiten.Image
	previousWallsImage *ebiten.Image
	previousFoodImage  *ebiten.Image
	previousOrgsImage  *ebiten.Image

	mouseHoverLocation utils.Point
	mouseOnGrid        bool
	doRefresh          bool
	viewMode           mode
	selectMode         mode
}

func NewGrid(simulation *simulation.Simulation) *Grid {
	g := &Grid{
		simulation:         simulation,
		previousWallsImage: newBlankLayer(),
		previousEnvImage:   newBlankLayer(),
		previousFoodImage:  newBlankLayer(),
		previousOrgsImage:  newBlankLayer(),
		doRefresh:          true,
		viewMode:           orgsPhMode,
	}
	loadOrganismImages()
	return g
}

func loadOrganismImages() {
	squareImgSmall = resources.SquareSmall
	squareImgMedium = resources.SquareMedium
	squareImgLarge = resources.SquareLarge
	squareImgFill = resources.SquareFill
	squareImgBox = resources.SquareBox
}

// Render draws all organisms and food on the simulation grid
func (g *Grid) Render() *ebiten.Image {
	envImage := newBlankLayer()
	wallsImage := newBlankLayer()
	foodImage := newBlankLayer()
	orgsImage := newBlankLayer()
	selImage := newBlankLayer()
	gridImage := newBlankLayer()

	g.renderWalls(wallsImage, g.doRefresh)
	g.renderEnvironment(envImage, g.doRefresh)
	g.renderFood(foodImage, g.doRefresh)
	g.renderOrganisms(orgsImage, g.doRefresh)
	g.renderSelections(selImage)

	g.previousWallsImage = wallsImage
	g.previousEnvImage = envImage
	g.previousFoodImage = foodImage
	g.previousOrgsImage = orgsImage

	if g.viewMode == orgsPhMode || g.viewMode == phOnlyMode {
		gridImage.DrawImage(envImage, nil)
	}
	gridImage.DrawImage(wallsImage, nil)
	gridImage.DrawImage(foodImage, nil)

	if g.viewMode != phOnlyMode {
		gridImage.DrawImage(orgsImage, nil)
	}

	gridImage.DrawImage(selImage, nil)

	g.doRefresh = false

	return gridImage
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
		envImage.DrawImage(g.previousEnvImage, nil)
		updatedPoints := g.simulation.GetUpdatedPhPoints()
		for _, point := range updatedPoints {
			// clear square to be updated
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
		wallsImage.DrawImage(g.previousWallsImage, nil)
	}
}

func (g *Grid) renderPhValue(envImage *ebiten.Image, gridX, gridY int, phVal float64) {
	x := float64(gridX) * float64(config.GridUnitSize())
	y := float64(gridY) * float64(config.GridUnitSize())
	hue := (phVal / config.MaxPh()) * phMaxHue
	sat := math.Abs(phVal-((config.MaxPh()+config.MinPh())/2.0)) / (config.MaxPh() - config.MinPh())
	light := 0.5 + (0.5 * math.Sin(math.Pi*(sat-0.5)))
	col := colorful.HSLuv(hue, sat, light)
	g.drawSquare(envImage, x, y, sizeFill, col)
}

func (g *Grid) renderFood(foodImage *ebiten.Image, refresh bool) {
	if refresh {
		items := g.simulation.GetFoodItems()
		for _, item := range items {
			g.renderFoodItem(item, foodImage)
		}
	} else {
		foodImage.DrawImage(g.previousFoodImage, nil)
		updatedPoints := g.simulation.GetUpdatedFoodPoints()
		for _, point := range updatedPoints {
			// clear square to be updated
			x, y := point.X*config.GridUnitSize(), point.Y*config.GridUnitSize()
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
		organismsImage.DrawImage(g.previousOrgsImage, nil)
		updatedPoints := g.simulation.GetUpdatedOrganismPoints()
		for _, point := range updatedPoints {
			// clear square to be updated
			x, y := point.X*config.GridUnitSize(), point.Y*config.GridUnitSize()
			g.clearSquare(organismsImage, float64(x), float64(y))

			if info := g.simulation.GetOrganismInfoAtPoint(point); info != nil {
				g.renderOrganism(info, organismsImage)
			}
		}
	}
}

func (g *Grid) renderSelections(selectionsImage *ebiten.Image) {
	if g.mouseOnGrid {
		infoColor := hoverColor
		infoText := fmt.Sprintf("PH: %2.1f", g.simulation.GetPhAtPoint(g.mouseHoverLocation))
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

		g.renderSelection(g.mouseHoverLocation, selectionsImage, infoColor)
		g.renderSelectionText(g.mouseHoverLocation, selectionsImage, infoText, selectionInfoColor)
		g.renderViewModeName(selectionsImage)
	}

	if info := g.simulation.GetOrganismInfoByID(g.simulation.GetSelected()); info != nil {
		g.renderSelection(info.Location, selectionsImage, selectColor)
	}
}

func newBlankLayer() *ebiten.Image {
	return ebiten.NewImage(config.GridWidth(), config.GridHeight())
}

// ChangeViewMode switches to the next mode listed in viewModes
func (g *Grid) ChangeViewMode() {
	g.viewMode = viewModes[(int(g.viewMode)+1)%len(viewModes)]
	g.doRefresh = true
}

// UpdateAutoSelect switches to the next auto select mode listed in selectModes
func (g *Grid) UpdateAutoSelect() {
	// cycle among all but the last selectMode, which is manual.
	// Manual selection his is switched to by clicking an organism
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

// renderSelection draws a square around a single item on the grid
func (g *Grid) renderSelection(point utils.Point, img *ebiten.Image, col colorful.Color) {
	x, y := float64(point.X*config.GridUnitSize()), float64(point.Y*config.GridUnitSize())
	ebitenutil.DrawLine(img, x-2, y-2, x+float64(config.GridUnitSize())+3, y-2, col)                                                               // top
	ebitenutil.DrawLine(img, x-2, y-2, x-2, y+float64(config.GridUnitSize())+3, col)                                                               // left
	ebitenutil.DrawLine(img, x-2, y+float64(config.GridUnitSize())+3, x+float64(config.GridUnitSize())+3, y+float64(config.GridUnitSize())+3, col) // bottom
	ebitenutil.DrawLine(img, x+float64(config.GridUnitSize())+3, y-2, x+float64(config.GridUnitSize())+3, y+float64(config.GridUnitSize())+3, col) // right
}

func (g *Grid) renderSelectionText(point utils.Point, img *ebiten.Image, message string, col colorful.Color) {
	xPadding := 10
	bounds := text.BoundString(resources.FontSourceCodePro10, message)
	x := xPadding + config.GridUnitSize() + (point.X * config.GridUnitSize())
	y := point.Y * config.GridUnitSize()
	if x+bounds.Dx() > config.GridWidth() {
		x = (point.X * config.GridUnitSize()) - xPadding - bounds.Dx()
	}
	text.Draw(img, message, resources.FontSourceCodePro10, x, y, col)
}

func (g *Grid) renderViewModeName(img *ebiten.Image) {
	xPadding := 10
	yPadding := 20
	x := xPadding
	y := yPadding
	info := fmt.Sprintf("VIEW MODE: %s\nSELECTED: %s", viewModeNames[g.viewMode], selectModeNames[g.selectMode])
	text.Draw(img, info, resources.FontSourceCodePro10, x, y, selectionInfoColor)
}

// renderFoodItem draws a food item to the given image
func (g *Grid) renderFoodItem(item *food.Item, img *ebiten.Image) {
	x := float64(item.Point.X) * float64(config.GridUnitSize())
	y := float64(item.Point.Y) * float64(config.GridUnitSize())

	value := float64(item.Value)
	foodSize := sizeSmall
	if value < float64(config.MaxFoodValue())*0.4375 {
		foodSize = sizeSmall
	} else if value < float64(config.MaxFoodValue())*0.8125 {
		foodSize = sizeMedium
	} else {
		foodSize = sizeLarge
	}

	g.drawSquare(img, x, y, foodSize, foodColor)
}

// renderWall draws a wall icon to the given image
func (g *Grid) renderWall(wallsImage *ebiten.Image, point utils.Point) {
	x := float64(point.X) * float64(config.GridUnitSize())
	y := float64(point.Y) * float64(config.GridUnitSize())

	g.drawSquare(wallsImage, x, y, sizeBox, wallColor)
}

// renderOrganism draws an organism to the given image
func (g *Grid) renderOrganism(info *organism.Info, img *ebiten.Image) {
	point := info.Location.Times(config.GridUnitSize())
	x, y := float64(point.X), float64(point.Y)

	organismSize := sizeSmall
	if info.Size < config.MaximumMaxSize()*0.4375 {
		organismSize = sizeSmall
	} else if info.Size < config.MaximumMaxSize()*0.8125 {
		organismSize = sizeMedium
	} else {
		organismSize = sizeLarge
	}

	organismColor := info.Color

	if g.viewMode == phEffectsOnlyMode {
		maxEffect := config.MaxOrganismPhGrowthEffect() * info.Size
		spectrumValue := (info.Size*info.PhEffect + maxEffect) / (2 * maxEffect)
		hue := phMaxHue * spectrumValue
		sat := 0.5 + math.Abs(spectrumValue-0.5)
		light := 0.25 + math.Abs(spectrumValue-0.5)
		organismColor = colorful.HSLuv(hue, sat, light)
	}

	if info.Action == decision.ActAttack {
		organismColor = attackColor
	}

	g.drawSquare(img, x, y, organismSize, organismColor)
}

func (g *Grid) drawSquare(img *ebiten.Image, x, y float64, sz size, col colorful.Color) {
	var squareImg *ebiten.Image
	switch sz {
	case sizeSmall:
		squareImg = squareImgSmall
		break
	case sizeMedium:
		squareImg = squareImgMedium
		break
	case sizeLarge:
		squareImg = squareImgLarge
		break
	case sizeFill:
		squareImg = squareImgFill
		break
	case sizeBox:
		squareImg = squareImgBox
		break
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.ColorM.Translate(col.R, col.G, col.B, 0)
	img.DrawImage(squareImg, op)
}

func (g *Grid) clearSquare(img *ebiten.Image, x, y float64) {
	squareImg := squareImgFill
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.ColorM.Translate(0, 0, 0, 1.0)
	op.CompositeMode = ebiten.CompositeModeDestinationOut

	img.DrawImage(squareImg, op)
}
