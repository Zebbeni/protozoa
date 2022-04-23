package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// MouseState describes what the mouse is doing relative to a given MouseHandler
type MouseState int

const (
	// MouseOver (aka. hovering)
	MouseOver MouseState = iota
	// MouseOff (aka. Not hovering)
	MouseOff
	// MouseDown occurs when any button is pressed while on a MouseHandler object
	MouseDown
)

// MouseHandler provides an interface for UX elements that listen to mouse
// position and events
type MouseHandler interface {
	// Handle should attempt to process an x, y event and return true if it is able to handle it
	Handle(x, y int) bool
	RegisterMouseHandler(*MouseHandler)

	OnMouseOver(x, y int)
	OnMouseOff()
	OnMouseDown(x, y int)
	OnMouseUp(x, y int)
}

// Renderable provides an interface for UX elements that should be displayed to the screen
type Renderable interface {
	Render() *ebiten.Image
}

type UIComponent interface {
	MouseHandler
	Renderable
}
