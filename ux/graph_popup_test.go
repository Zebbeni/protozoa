package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// popupTestPanel is a Panel with only what the popup reads.
func popupTestPanel(t *testing.T) *Panel {
	t.Helper()
	loadKeyGlobals(t)
	return &Panel{}
}

// TestGraphPopupGeometryFitsTheScreen: the popup is laid out from the
// screen size, so a small window must not produce a graph area with no
// room in it — or, worse, a negative one, which is a crash when the
// image is scaled into it.
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
		// And it has to be bigger than the panel graph it expands, or
		// there is no point opening it.
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

// TestGraphPopupIsModal: while it is open it consumes the frame's mouse,
// so a click can't fall through to the grid behind it.
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

// TestExpandRequestIsTakenOnce: the panel raises a flag and the runner
// takes it, so one click opens the popup exactly once rather than
// re-opening it every frame the button stays pressed.
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

// TestExpandButtonOnlyExistsOnHover: it sits over the graph, and a
// control permanently covering the corner of a plot is in the way of
// the thing it is meant to help you read. No hitbox means no click.
func TestExpandButtonOnlyExistsOnHover(t *testing.T) {
	p := popupTestPanel(t)

	p.drawExpandButton(nil, 0, 0, graphWidth, false)
	if p.graphExpandRect != nil {
		t.Error("the expand button is clickable without hovering the graph")
	}
}
