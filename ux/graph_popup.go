package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// GraphPopup is the graph again, bigger. Opened from the expand control
// in the panel graph's corner.
//
// It owns no graph state of its own: the mode buttons it draws are the
// panel's, drawn at this size, and they write straight back to the panel
// — so the popup and the panel can never disagree about what is being
// shown, and closing it leaves the panel on whatever was picked here.
//
// The image is the one the panel already rendered this frame, not a
// second render: the graph is drawn at a fixed resolution and scaled to
// fit wherever it lands, so there is nothing to redraw for a bigger box.
type GraphPopup struct {
	panel *Panel
	open  bool
}

func NewGraphPopup(p *Panel) *GraphPopup { return &GraphPopup{panel: p} }

func (g *GraphPopup) IsOpen() bool { return g.open }

func (g *GraphPopup) Open()  { g.open = true }
func (g *GraphPopup) Close() { g.open = false }

// Popup geometry: a margin off every edge, with the graph filling what
// is left above a row of controls.
const (
	graphPopupMargin  = 48
	graphPopupPad     = 16
	graphPopupTitleH  = 26
	graphPopupCloseW  = 24
	graphPopupFooterH = 3*graphButtonPitch + graphPopupPad
)

// rect is the popup's outer bounds. popupRectT rather than
// image.Rectangle so it uses the same helpers as every other modal here.
func (g *GraphPopup) rect() popupRectT {
	return newRect(graphPopupMargin, graphPopupMargin,
		config.ScreenWidth()-2*graphPopupMargin, config.ScreenHeight()-2*graphPopupMargin)
}

// graphRect is where the enlarged graph itself goes.
func (g *GraphPopup) graphRect() popupRectT {
	r := g.rect()
	return newRect(
		r.Min.X+graphPopupPad, r.Min.Y+graphPopupTitleH,
		r.Dx()-2*graphPopupPad, r.Dy()-graphPopupTitleH-graphPopupFooterH,
	)
}

// closeRect is the close control in the title bar.
func (g *GraphPopup) closeRect() popupRectT {
	r := g.rect()
	return newRect(r.Max.X-graphPopupPad-graphPopupCloseW, r.Min.Y+4,
		graphPopupCloseW, graphPopupCloseW)
}

// Update handles the popup's input and reports whether it consumed this
// frame's mouse. Everything is consumed while it is open: it is modal,
// and a click that fell through would act on the grid behind it.
func (g *GraphPopup) Update() bool {
	if !g.open {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Close()
		return true
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}

	mx, my := ebiten.CursorPosition()
	if hitRect(mx, my, g.closeRect()) {
		g.Close()
		return true
	}
	if !hitRect(mx, my, g.rect()) {
		// Clicking away closes it, the same as every other modal here.
		g.Close()
		return true
	}
	// The buttons were drawn in screen coordinates, so the cursor needs
	// no adjusting — see renderGraphButtons on why sharing the hitboxes
	// with the panel is safe. Nothing has to restore the panel's own
	// hitboxes on close either: this Update consumes the closing click,
	// and the panel redraws its buttons before any input reaches it.
	g.panel.handleGraphButtonClick(mx, my)
	return true
}

// Draw paints the popup over everything else.
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

	// The same buttons the panel draws, at this width. They write back
	// to the panel, so what is picked here is what the panel shows once
	// this closes.
	g.panel.renderGraphButtons(screen, gr.Min.X, gr.Max.Y+graphPopupPad, gr.Dx())
}
