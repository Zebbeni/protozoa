package ux

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"

	r "github.com/Zebbeni/protozoa/resources"
	s "github.com/Zebbeni/protozoa/simulation"
)

const (
	padding     = 15
	panelWidth  = 400
	panelHeight = 1000

	titleXOffset = padding
	titleYOffset = padding

	statsXOffset = padding
	statsYOffset = 69

	graphXOffset = padding
	graphYOffset = 130
	graphWidth   = 370
	graphHeight  = 120
)

type Panel struct {
	MouseHandler
	simulation         *s.Simulation
	previousPanelImage *ebiten.Image
	graph              *Graph
}

func NewPanel(sim *s.Simulation) *Panel {
	return &Panel{
		simulation: sim,
		graph:      NewGraph(sim),
	}
}

func (p *Panel) Render() *ebiten.Image {
	panelImage := ebiten.NewImage(panelWidth, panelHeight)

	if p.shouldRefresh() {
		p.renderDividingLine(panelImage)
		p.renderTitle(panelImage)
		p.renderPlayPauseButton(panelImage)
		p.renderStats(panelImage)
		p.renderGraph(panelImage)

		p.previousPanelImage = ebiten.NewImage(panelWidth, panelHeight)
		p.previousPanelImage.DrawImage(panelImage, nil)
	} else {
		panelImage.DrawImage(p.previousPanelImage, nil)
	}

	return panelImage
}

func (p *Panel) shouldRefresh() bool {
	return true
}

func (p *Panel) renderDividingLine(panelImage *ebiten.Image) {
	ebitenutil.DrawRect(panelImage, float64(panelWidth)-1, 0, float64(panelWidth), float64(panelHeight), color.White)
}

func (p *Panel) renderTitle(panelImage *ebiten.Image) {
	bounds := text.BoundString(r.FontInversionz40, "protozoa")
	text.Draw(panelImage, "protozoa", r.FontInversionz40, titleXOffset, titleYOffset+bounds.Dy(), color.White)
}

func (p *Panel) renderPlayPauseButton(panelImage *ebiten.Image) {
	imageOptions := &ebiten.DrawImageOptions{}
	imageOptions.GeoM.Translate(float64(panelWidth)-float64(r.PauseButton.Bounds().Dx())-padding, padding-2)
	panelImage.DrawImage(r.PauseButton, imageOptions)
}

func (p *Panel) renderStats(panelImage *ebiten.Image) {
	statsString := fmt.Sprintf("CYCLE: %9d\nORGANISMS: %5d\nDEAD: %10d",
		p.simulation.Cycle(), p.simulation.OrganismCount(), p.simulation.GetDeadCount())
	text.Draw(panelImage, statsString, r.FontSourceCodePro12, statsXOffset, statsYOffset, color.White)
}

func (p *Panel) renderGraph(panelImage *ebiten.Image) {
	text.Draw(panelImage, "HISTORY", r.FontSourceCodePro12, graphXOffset, graphYOffset, color.White)
	graphImage := p.graph.Render()
	graphOptions := &ebiten.DrawImageOptions{}
	scaleX := float64(graphWidth) / float64(graphImage.Bounds().Dx())
	scaleY := float64(graphHeight) / float64(graphImage.Bounds().Dy())
	graphOptions.GeoM.Scale(scaleX, scaleY)
	graphOptions.GeoM.Translate(graphXOffset, graphYOffset+10)

	panelImage.DrawImage(graphImage, graphOptions)

	// draw border around graph
	left, top, right, bottom := float64(graphXOffset), float64(graphYOffset+10), float64(graphXOffset+graphWidth), float64(graphYOffset+graphHeight+10)
	ebitenutil.DrawLine(panelImage, left, top, right, top, color.White)
	ebitenutil.DrawLine(panelImage, right, top, right, bottom, color.White)
	ebitenutil.DrawLine(panelImage, left, bottom, right, bottom, color.White)
	ebitenutil.DrawLine(panelImage, left, top, left, bottom, color.White)
}
