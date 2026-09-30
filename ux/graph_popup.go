package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

type GraphPopup struct {
	panel *Panel
	open  bool
}

func NewGraphPopup(p *Panel) *GraphPopup { return &GraphPopup{panel: p} }

func (g *GraphPopup) IsOpen() bool { return g.open }

func (g *GraphPopup) Open()  { g.open = true }
func (g *GraphPopup) Close() { g.open = false }

// Popup geometry: a margin off every edge, with the graph filling what is left above a row of controls.
const (
	graphPopupMargin  = 48
	graphPopupPad     = 16
	graphPopupTitleH  = 26
	graphPopupCloseW  = 24
	graphPopupFooterH = 3*graphButtonPitch + graphPopupPad
	graphPopupScale   = 0.75
)

func (g *GraphPopup) rect() popupRectT {
	// Scaled off the old near-fullscreen box and centred, rather than by growing the margin.
	w := int(float64(config.ScreenWidth()-2*graphPopupMargin) * graphPopupScale)
	h := int(float64(config.ScreenHeight()-2*graphPopupMargin) * graphPopupScale)
	return newRect((config.ScreenWidth()-w)/2, (config.ScreenHeight()-h)/2, w, h)
}

func (g *GraphPopup) graphRect() popupRectT {
	r := g.rect()
	return newRect(
		r.Min.X+graphPopupPad, r.Min.Y+graphPopupTitleH,
		r.Dx()-2*graphPopupPad, r.Dy()-graphPopupTitleH-graphPopupFooterH,
	)
}

func (g *GraphPopup) closeRect() popupRectT {
	r := g.rect()
	return newRect(r.Max.X-graphPopupPad-graphPopupCloseW, r.Min.Y+4,
		graphPopupCloseW, graphPopupCloseW)
}

// Update handles the popup's input and reports whether it consumed this frame's mouse.
func (g *GraphPopup) Update() bool {
	if !g.open {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Close()
		return true
	}
	mx, my := ebiten.CursorPosition()

	// The graph's own controls first, at this popup's geometry.
	gr := g.graphRect()
	if g.panel.graphMouseAt(mx, my, gr.Min.X, gr.Min.Y, gr.Dx(), gr.Dy()) {
		return true
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	if hitRect(mx, my, g.closeRect()) {
		g.Close()
		return true
	}
	if !hitRect(mx, my, g.rect()) {
		// Clicking away closes it, the same as every other modal here.
		g.Close()
		return true
	}
	// The buttons were drawn in screen coordinates.
	g.panel.handleGraphButtonClick(mx, my)
	return true
}

func (g *GraphPopup) Draw(screen *ebiten.Image) {
	if !g.open {
		return
	}
	rect := g.rect()
	fillRect(screen, rect, themeBackgroundColor())
	drawModalBorder(screen, rect)

	title := graphModeLabel(g.panel.graphMode, g.panel.graphShowSelected)
	text.Draw(screen, title, r.FontSourceCodePro12,
		rect.Min.X+graphPopupPad, rect.Min.Y+18, themedForeground())

	c := g.closeRect()
	drawGraphButton(screen, c.Min.X, c.Min.Y, c.Dx(), c.Dy(), "X", false)

	gr := g.graphRect()
	if img := g.panel.lastGraphImage; img != nil {
		g.panel.drawGraphInto(screen, img, gr.Min.X, gr.Min.Y, gr.Dx(), gr.Dy())
	} else if fraction, show := g.panel.graph.RenderProgress(); show {
		drawGraphProgress(screen, gr.Min.X, gr.Min.Y, gr.Dx(), gr.Dy(), fraction)
	}

	// The same buttons the panel draws, at this width.
	g.panel.renderGraphButtons(screen, gr.Min.X, gr.Max.Y+graphPopupPad, gr.Dx())
}
