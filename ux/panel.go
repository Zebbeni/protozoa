package ux

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
	s "github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/ux/graph"
)

const (
	padding     = 15
	panelWidth  = 400
	panelInnerH = 2000 // tall enough for all content

	titleXOffset = padding
	titleYOffset = padding
	playXOffset  = padding
	playYOffset  = 0

	statsXOffset = padding
	statsYOffset = 69

	selectedXOffset = padding
	selectedYOffset = 300

	graphXOffset = padding
	graphYOffset = 130
	graphWidth   = 370
	graphHeight  = 120
)

type Panel struct {
	simulation         *s.Simulation
	grid               *Grid
	previousPanelImage *ebiten.Image
	graph              *graph.Graph
	scrollY            float64
	contentHeight      int // actual height of rendered content
}

func NewPanel(sim *s.Simulation, grid *Grid) *Panel {
	return &Panel{
		simulation: sim,
		grid:       grid,
		graph:      graph.NewGraph(sim),
	}
}

// HandleScroll processes mouse wheel input when the cursor is over the panel.
func (p *Panel) HandleScroll() {
	mx, _ := ebiten.CursorPosition()
	if mx >= 0 && mx < panelWidth {
		_, wy := ebiten.Wheel()
		p.scrollY -= wy * 20
		p.clampScroll()
	}
}

func (p *Panel) clampScroll() {
	screenH := config.ScreenHeight()
	maxScroll := float64(p.contentHeight - screenH)
	p.scrollY = max(0, min(p.scrollY, maxScroll))
}

func (p *Panel) Render() *ebiten.Image {
	screenH := config.ScreenHeight()

	// Render all content onto a tall inner image
	innerImage := ebiten.NewImage(panelWidth, panelInnerH)

	p.renderDividingLine(innerImage, screenH)
	p.renderTitle(innerImage)
	p.renderKeyBindingText(innerImage)
	p.renderStats(innerImage)
	p.renderGraph(innerImage)
	contentBottom := p.renderSelected(innerImage)
	p.contentHeight = contentBottom + padding

	// Extract visible portion based on scroll
	p.clampScroll()
	panelImage := ebiten.NewImage(panelWidth, screenH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, -p.scrollY)
	panelImage.DrawImage(innerImage, op)

	// Draw dividing line on the output (not scrolled)
	ebitenutil.DrawRect(panelImage, float64(panelWidth)-1, 0, 1, float64(screenH), color.White)

	// Draw scrollbar if content overflows
	if p.contentHeight > screenH {
		p.drawScrollbar(panelImage, screenH)
	}

	return panelImage
}

func (p *Panel) drawScrollbar(panelImage *ebiten.Image, screenH int) {
	scrollbarWidth := 4.0
	scrollbarX := float64(panelWidth) - scrollbarWidth - 3
	trackH := float64(screenH)

	// Thumb size proportional to visible fraction
	visibleFraction := float64(screenH) / float64(p.contentHeight)
	thumbH := max(20, trackH*visibleFraction)

	// Thumb position proportional to scroll position
	maxScroll := float64(p.contentHeight - screenH)
	scrollFraction := 0.0
	if maxScroll > 0 {
		scrollFraction = p.scrollY / maxScroll
	}
	thumbY := scrollFraction * (trackH - thumbH)

	// Track
	ebitenutil.DrawRect(panelImage, scrollbarX, 0, scrollbarWidth, trackH,
		color.RGBA{R: 40, G: 40, B: 40, A: 150})
	// Thumb
	ebitenutil.DrawRect(panelImage, scrollbarX, thumbY, scrollbarWidth, thumbH,
		color.RGBA{R: 120, G: 120, B: 120, A: 200})
}

func (p *Panel) renderDividingLine(panelImage *ebiten.Image, screenH int) {
	// Drawn on the final output instead, see Render()
}

func (p *Panel) renderTitle(panelImage *ebiten.Image) {
	bounds := boundString(r.FontInversionz40, "protozoa")
	text.Draw(panelImage, "protozoa", r.FontInversionz40, titleXOffset, titleYOffset+bounds.Dy(), color.White)
}

func (p *Panel) renderKeyBindingText(panelImage *ebiten.Image) {
	var lines []string
	if p.simulation.IsPaused() {
		lines = []string{"[Space] to Resume", "[M] to Change Mode"}
	} else {
		lines = []string{"[Space] to Pause", "[M] to Change Mode", "[O] to Auto Select"}
	}

	lineHeight := r.FontSourceCodePro10.Metrics().Height.Round()
	y := titleYOffset + lineHeight
	for _, line := range lines {
		bounds := boundString(r.FontSourceCodePro10, line)
		x := panelWidth - playXOffset - bounds.Dx()
		text.Draw(panelImage, line, r.FontSourceCodePro10, x, y, color.White)
		y += lineHeight
	}
}

func (p *Panel) renderStats(panelImage *ebiten.Image) {
	statsString := fmt.Sprintf("CYCLE: %9d\nORGANISMS: %5d\nDEAD: %10d",
		p.simulation.Cycle(), p.simulation.OrganismCount(), p.simulation.GetDeadCount())
	text.Draw(panelImage, statsString, r.FontSourceCodePro12, statsXOffset, statsYOffset, color.White)
}

func (p *Panel) renderGraph(panelImage *ebiten.Image) {
	// Sync graph mode with grid view mode
	var graphMode graph.Mode
	var label string
	switch p.grid.ViewMode() {
	case organismsOnlyMode:
		graphMode = graph.ModePopulation
		label = "POPULATION HISTORY"
	case phEffectsOnlyMode:
		graphMode = graph.ModePopulationPhEffect
		label = "PH EFFECT POPULATION"
	case phOnlyMode:
		graphMode = graph.ModePh
		label = "PH DISTRIBUTION"
	default:
		graphMode = graph.ModePhEffect
		label = "PH EFFECT HISTORY"
	}
	p.graph.SetMode(graphMode)

	// Append selected organism ID to population graph titles
	if p.graph.HasSelection() && (graphMode == graph.ModePopulation || graphMode == graph.ModePopulationPhEffect) {
		label = fmt.Sprintf("%s (ORG ID: %d)", label, p.simulation.GetSelected())
	}

	text.Draw(panelImage, label, r.FontSourceCodePro12, graphXOffset, graphYOffset, color.White)
	graphImage := p.graph.Render()
	if graphImage == nil {
		return
	}
	graphOptions := &ebiten.DrawImageOptions{}
	scaleX := float64(graphWidth) / float64(graphImage.Bounds().Dx())
	scaleY := float64(graphHeight) / float64(graphImage.Bounds().Dy())
	graphOptions.GeoM.Scale(scaleX, scaleY)
	graphOptions.GeoM.Translate(graphXOffset, graphYOffset+10)

	panelImage.DrawImage(graphImage, graphOptions)

	// Draw avg pH label on the pH graph at panel resolution
	if graphMode == graph.ModePh {
		avgPh := p.graph.LastAvgPh()
		if avgPh >= 0 {
			phLabel := fmt.Sprintf("avg: %.1f", avgPh)
			// Map pH to Y within the graph area: MaxPh=top, MinPh=bottom
			phRange := config.MaxPh() - config.MinPh()
			lineY := float64(graphYOffset+10) + float64(graphHeight)*(1.0-(avgPh-config.MinPh())/phRange)
			bounds := boundString(r.FontSourceCodePro8, phLabel)
			textX := graphXOffset + graphWidth - bounds.Dx() - 2
			textY := int(lineY) - 2
			if textY < graphYOffset+10+bounds.Dy() {
				textY = graphYOffset + 10 + bounds.Dy()
			}
			text.Draw(panelImage, phLabel, r.FontSourceCodePro8, textX, textY, color.White)
		}
	}

	// Draw start cycle label for selected sub-tree graphs
	if p.graph.HasSelection() && (graphMode == graph.ModePopulation || graphMode == graph.ModePopulationPhEffect) {
		startCycle := p.graph.SelectedStartCycle()
		if startCycle >= 0 {
			cycleLabel := fmt.Sprintf("cycle %d", startCycle)
			text.Draw(panelImage, cycleLabel, r.FontSourceCodePro8, graphXOffset+2, graphYOffset+10+8, color.White)
		}
	}

	// draw border around graph
	left, top, right, bottom := float64(graphXOffset), float64(graphYOffset+10), float64(graphXOffset+graphWidth), float64(graphYOffset+graphHeight+10)
	ebitenutil.DrawLine(panelImage, left, top, right, top, color.White)
	ebitenutil.DrawLine(panelImage, right, top, right, bottom, color.White)
	ebitenutil.DrawLine(panelImage, left, bottom, right, bottom, color.White)
	ebitenutil.DrawLine(panelImage, left, top, left, bottom, color.White)
}

// renderSelected draws the selected organism info and decision tree.
// Returns the Y position after the last line of content.
func (p *Panel) renderSelected(panelImage *ebiten.Image) int {
	id := p.simulation.GetSelected()
	info := p.simulation.GetOrganismInfoByID(id)
	traits, found := p.simulation.GetOrganismTraitsByID(id)

	decisionTree := p.simulation.GetOrganismDecisionTreeByID(id)
	if info == nil || decisionTree == nil || found == false {
		return selectedYOffset
	}
	infoString := fmt.Sprintf("ORGANISM ID:    %7d       HEALTH:       %[4]*.[3]*[2]f", info.ID, info.Health, 2, 5)
	infoString += fmt.Sprintf("\nANCESTOR ID:    %7d       SIZE:         %5.2f", info.AncestorID, info.Size)
	infoString += fmt.Sprintf("\nAGE:            %7d       CHILDREN:   %7d", info.Age, info.Children)
	infoString += fmt.Sprintf("\nMUTATE CHANCE:     %3.0f%%       SPAWN HEALTH: %[4]*.[3]*[2]f", traits.ChanceToMutateDecisionTree*100.0, traits.MinHealthToSpawn, 2, 5)
	infoString += fmt.Sprintf("\nPH TOLERANCE:   %1.1f-%1.1f       PH EFFECT: %+1.5f", traits.IdealPh-traits.PhTolerance, traits.IdealPh+traits.PhTolerance, traits.PhGrowthEffect)
	infoLineCount := strings.Count(infoString, "\n") + 1
	infoHeight := infoLineCount * r.FontSourceCodePro12.Metrics().Height.Round()
	offsetY := selectedYOffset + infoHeight + padding

	text.Draw(panelImage, infoString, r.FontSourceCodePro12, selectedXOffset, selectedYOffset, color.White)

	// Render decision tree with dim color for untravelled nodes
	text.Draw(panelImage, "DECISION TREE:", r.FontSourceCodePro10, selectedXOffset, offsetY, color.White)
	lineHeight := r.FontSourceCodePro10.Metrics().Height.Round()
	offsetY += lineHeight
	dimColor := color.RGBA{R: 80, G: 80, B: 80, A: 255}
	for _, line := range decisionTree.PrintLines() {
		clr := dimColor
		if line.WasTravelled {
			clr = color.RGBA{R: 255, G: 255, B: 255, A: 255}
		}
		text.Draw(panelImage, line.Text, r.FontSourceCodePro10, selectedXOffset, offsetY, clr)
		offsetY += lineHeight
	}
	return offsetY
}
