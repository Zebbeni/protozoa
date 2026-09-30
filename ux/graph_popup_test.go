package ux

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/config"
)

func popupTestPanel(t *testing.T) *Panel {
	t.Helper()
	loadKeyGlobals(t)
	return &Panel{}
}

func TestGraphPopupGeometryFitsTheScreen(t *testing.T) {
	p := popupTestPanel(t)
	g := NewGraphPopup(p)

	for _, size := range [][2]int{{1400, 800}, {1920, 1057}, {900, 600}} {
		config.GetCurrentGlobals().ScreenWidth = size[0]
		config.GetCurrentGlobals().ScreenHeight = size[1]

		outer := g.rect()
		gr := g.graphRect()
		if gr.Dx() <= 0 || gr.Dy() <= 0 {
			t.Errorf("at %dx%d the graph area is %dx%d", size[0], size[1], gr.Dx(), gr.Dy())
			continue
		}
		if gr.Dx() <= graphWidth || gr.Dy() <= graphHeight {
			t.Errorf("at %dx%d the expanded graph is %dx%d, no bigger than the panel's %dx%d",
				size[0], size[1], gr.Dx(), gr.Dy(), graphWidth, graphHeight)
		}
		// The footer must leave room for the buttons under the graph.
		if gr.Max.Y+graphPopupPad+3*graphButtonPitch > outer.Max.Y {
			t.Errorf("at %dx%d the buttons run past the bottom of the popup", size[0], size[1])
		}
		if !hitRect(g.closeRect().Min.X+1, g.closeRect().Min.Y+1, outer) {
			t.Errorf("at %dx%d the close control is outside the popup", size[0], size[1])
		}
	}
}

func TestGraphPopupIsModal(t *testing.T) {
	g := NewGraphPopup(popupTestPanel(t))

	if g.Update() {
		t.Error("a closed popup consumed input")
	}
	g.Open()
	if !g.IsOpen() {
		t.Fatal("Open didn't open it")
	}
	if !g.Update() {
		t.Error("an open popup let input through to the grid behind it")
	}
	g.Close()
	if g.IsOpen() || g.Update() {
		t.Error("Close didn't close it")
	}
}

func TestExpandRequestIsTakenOnce(t *testing.T) {
	p := popupTestPanel(t)

	if p.TakeGraphExpandRequest() {
		t.Error("an untouched panel reported an expand request")
	}
	p.graphExpandRequested = true
	if !p.TakeGraphExpandRequest() {
		t.Error("the request wasn't reported")
	}
	if p.TakeGraphExpandRequest() {
		t.Error("the request was reported twice")
	}
}

func TestExpandButtonOnlyExistsOnHover(t *testing.T) {
	p := popupTestPanel(t)

	p.drawExpandButton(nil, 0, 0, graphWidth, false)
	if p.graphExpandRect != nil {
		t.Error("the expand button is clickable without hovering the graph")
	}
}

func TestGraphPopupIsSmallerThanTheScreen(t *testing.T) {
	loadKeyGlobals(t)
	p := &GraphPopup{}
	rect := p.rect()

	fullW := config.ScreenWidth() - 2*graphPopupMargin
	fullH := config.ScreenHeight() - 2*graphPopupMargin
	if got, want := rect.Dx(), int(float64(fullW)*graphPopupScale); got != want {
		t.Errorf("popup is %dpx wide, want %dpx (%.0f%% of %d)", got, want, graphPopupScale*100, fullW)
	}
	if got, want := rect.Dy(), int(float64(fullH)*graphPopupScale); got != want {
		t.Errorf("popup is %dpx tall, want %dpx", got, want)
	}

	// Centred, so the margins either side match to within rounding.
	left, right := rect.Min.X, config.ScreenWidth()-rect.Max.X
	if left-right > 1 || right-left > 1 {
		t.Errorf("popup is not centred: %dpx left, %dpx right", left, right)
	}
	top, bottom := rect.Min.Y, config.ScreenHeight()-rect.Max.Y
	if top-bottom > 1 || bottom-top > 1 {
		t.Errorf("popup is not centred: %dpx top, %dpx bottom", top, bottom)
	}
	// Still bigger than the panel's own graph, which is the whole point.
	if rect.Dx() <= graphWidth {
		t.Errorf("popup graph is %dpx wide against the panel's %dpx; it is meant to be the bigger view",
			rect.Dx(), graphWidth)
	}
}

func TestGraphMouseFollowsTheRectItIsGiven(t *testing.T) {
	loadKeyGlobals(t)
	panel := &Panel{}
	popup := NewGraphPopup(panel)
	gr := popup.graphRect()
	cx, cy := gr.Min.X+gr.Dx()/2, gr.Min.Y+gr.Dy()/2

	panel.graphCursor = ebiten.CursorShapeDefault
	panel.graphMouseAt(cx, cy, gr.Min.X, gr.Min.Y, gr.Dx(), gr.Dy())
	overPopup := panel.graphCursor
	if overPopup == ebiten.CursorShapeDefault {
		t.Fatal("a cursor in the middle of the popup's graph did not read as over it")
	}

	// The same cursor, told the graph is a small box far away: not over it.
	panel.graphCursor = overPopup
	panel.graphMouseAt(cx, cy, 0, 0, 4, 4)
	if panel.graphCursor != ebiten.CursorShapeDefault {
		t.Errorf("a cursor outside the given rect still read as over the graph (%v); "+
			"the handler is not using the rect it was passed", panel.graphCursor)
	}
}

func TestGraphPopupHasNoZoomStateOfItsOwn(t *testing.T) {
	loadKeyGlobals(t)
	panel := &Panel{}
	popup := NewGraphPopup(panel)

	panel.graphView.reset()
	panel.graphView.zoomAt(0.5, 4) // factor > 1 narrows the span
	if !panel.graphView.zoomed() {
		t.Fatal("zoomAt did not zoom the panel's view; this test cannot say anything")
	}
	// The popup reads through the panel, so it sees that window.
	if !popup.panel.graphView.zoomed() {
		t.Error("the popup does not see the panel's zoom window")
	}
}
