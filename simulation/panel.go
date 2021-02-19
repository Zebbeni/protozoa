package simulation

import (
	"image/color"

	c "github.com/Zebbeni/protozoa/constants"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	"github.com/hajimehoshi/ebiten/text"
)

const (
	padding = 15
)

func (s *Simulation) renderPanel(screen *ebiten.Image) {
	panelImage, _ := ebiten.NewImage(c.PanelWidth, c.PanelHeight, ebiten.FilterDefault)

	if s.shouldRefreshPanel() {
		// draw dividing line on right side of panel
		ebitenutil.DrawRect(panelImage, float64(c.PanelWidth)-1, 0, float64(c.PanelWidth), float64(c.PanelHeight), color.White)

		s.renderTitle(panelImage)
		s.renderPlayPauseButton(panelImage)

		s.previousPanelFrame, _ = ebiten.NewImage(c.PanelWidth, c.PanelHeight, ebiten.FilterDefault)
		s.previousPanelFrame.DrawImage(panelImage, nil)
	} else {
		panelImage.DrawImage(s.previousPanelFrame, nil)
	}

	screen.DrawImage(panelImage, nil)
}

func (s *Simulation) shouldRefreshPanel() bool {
	return true
}

func (s *Simulation) renderTitle(panelImage *ebiten.Image) {
	xOffset, yOffset := padding, padding
	bounds := text.BoundString(r.FontInversionz40, "protozoa")
	text.Draw(panelImage, "protozoa", r.FontInversionz40, xOffset, yOffset+bounds.Dy(), color.White)
}

func (s *Simulation) renderPlayPauseButton(panelImage *ebiten.Image) {
	drawOptions := &ebiten.DrawImageOptions{}
	drawOptions.GeoM.Translate(float64(c.PanelWidth)-float64(r.PauseButton.Bounds().Dx())-padding, padding-2)
	panelImage.DrawImage(r.PauseButton, drawOptions)
}
