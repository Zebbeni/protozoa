package ux

import (
	s "github.com/Zebbeni/protozoa/simulation"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	"image/color"

	r "github.com/Zebbeni/protozoa/resources"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/text"
)

const (
	padding = 15
	panelWidth    = 400
	panelHeight   = 1000
)

type Panel struct {
	MouseHandler
	simulation *s.Simulation
	previousPanelImage *ebiten.Image
}

func NewPanel(sim *s.Simulation) *Panel {
	return &Panel{
		simulation: sim,
	}
}

func (p *Panel) Render() *ebiten.Image {
	panelImage, _ := ebiten.NewImage(panelWidth, panelHeight, ebiten.FilterDefault)

	if p.shouldRefresh() {
		p.renderDividingLine(panelImage)
		p.renderTitle(panelImage)
		p.renderPlayPauseButton(panelImage)

		p.previousPanelImage, _ = ebiten.NewImage(panelWidth, panelHeight, ebiten.FilterDefault)
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
	xOffset, yOffset := padding, padding
	bounds := text.BoundString(r.FontInversionz40, "protozoa")
	text.Draw(panelImage, "protozoa", r.FontInversionz40, xOffset, yOffset+bounds.Dy(), color.White)
}

func (p *Panel) renderPlayPauseButton(panelImage *ebiten.Image) {
	drawOptions := &ebiten.DrawImageOptions{}
	drawOptions.GeoM.Translate(float64(panelWidth)-float64(r.PauseButton.Bounds().Dx())-padding, padding-2)
	panelImage.DrawImage(r.PauseButton, drawOptions)
}
