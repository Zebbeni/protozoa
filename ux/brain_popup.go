package ux

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	r "github.com/Zebbeni/protozoa/resources"
)

// BrainPopup is the net drawn big, opened from the expand control in the BRAIN
// tab, in the same box the expanded graph opens in.
//
// It redraws the net rather than scaling the panel's picture of it, which is
// what the graph popup does: a graph is an image, so scaling it is the same
// graph bigger, while the net is a diagram of labelled rows, and scaling that
// would blow its 8-point labels up into blur. Redrawn, the room goes into the
// labels and the spread between the rows instead.
type BrainPopup struct {
	panel *Panel
	open  bool
	// controls is where this view drew its own SIGNAL / WEIGHTS / NUMBERS
	// buttons, in screen coordinates. Its own, not the panel's: the two are
	// drawn in different spaces and a click outside this box belongs to the
	// panel.
	controls brainControls
	// nodeHits is where the dots went last frame, and focus is the one a
	// click picked out. Only this view has them: the tab draws the same net
	// into 73px, where two columns of dots are a few pixels apart.
	nodeHits []brainNodeHit
	focus    *brainNode
}

func NewBrainPopup(p *Panel) *BrainPopup { return &BrainPopup{panel: p} }

func (b *BrainPopup) IsOpen() bool { return b.open }

func (b *BrainPopup) Open() { b.open = true }

// Close puts the whole net back, so the next opening is not still filtered to
// a node the reader has long since stopped thinking about.
func (b *BrainPopup) Close() {
	b.open = false
	b.focus = nil
}

const (
	brainPopupPad    = 16
	brainPopupTitleH = 34
)

// brainPopupHeaderH is the title plus the view buttons under it, which is
// where the diagram starts. The buttons sit above the diagram rather than
// under it so they are in the same place as the tab's, which has no bottom to
// put them at — the panel scrolls and a tall net runs off it.
func brainPopupHeaderH() int { return brainPopupTitleH + brainViewButtonsH() }

func (b *BrainPopup) rect() popupRectT { return popupBoxRect() }

// diagramRect is what is left of the box under the title: the whole of it,
// since the diagram carries its own labels and needs no controls.
func (b *BrainPopup) diagramRect() popupRectT {
	rect := b.rect()
	return newRect(
		rect.Min.X+brainPopupPad, rect.Min.Y+brainPopupHeaderH(),
		rect.Dx()-2*brainPopupPad, rect.Dy()-brainPopupHeaderH()-brainPopupPad,
	)
}

func (b *BrainPopup) closeRect() popupRectT {
	rect := b.rect()
	return newRect(rect.Max.X-brainPopupPad-graphPopupCloseW, rect.Min.Y+4,
		graphPopupCloseW, graphPopupCloseW)
}

// Update handles the popup's input and reports whether it consumed this
// frame's mouse.
//
// It is NOT modal, which is the one place it parts company with the expanded
// graph. Clicking away used to close it, so stepping to the next cycle with it
// open both shut it and lost the click — and watching one organism's net
// change cycle by cycle is the reason to have it open at all. It keeps the
// mouse only while the cursor is over its box, so the panel underneath does
// not react to a click that landed on the diagram, and everything outside is
// the panel's.
func (b *BrainPopup) Update() bool {
	if !b.open {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		b.Close()
		return true
	}
	mx, my := ebiten.CursorPosition()
	if !b.takesMouseAt(mx, my) {
		return false
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		b.clickAt(mx, my)
	}
	return true
}

// takesMouseAt is whether this frame's mouse belongs to the expanded net.
func (b *BrainPopup) takesMouseAt(mx, my int) bool {
	return b.open && hitRect(mx, my, b.rect())
}

// clickAt acts on a click already known to be inside the box. The close
// control is the only thing that shuts it: a click anywhere else on the
// diagram does nothing, rather than closing a view the user is reading.
func (b *BrainPopup) clickAt(mx, my int) {
	if hitRect(mx, my, b.closeRect()) {
		b.Close()
		return
	}
	if b.panel.takeBrainControlClick(b.controls, mx, my) {
		return
	}
	b.pickNode(mx, my)
}

// pickNode picks out the dot under the click, or clears what was picked out
// when the click lands on neither a dot nor a control — clicking the same dot
// again is how a reader puts the whole net back.
func (b *BrainPopup) pickNode(mx, my int) {
	for _, hit := range b.nodeHits {
		if !hit.contains(mx, my) {
			continue
		}
		if b.focus != nil && *b.focus == hit.brainNode {
			b.focus = nil
			return
		}
		node := hit.brainNode
		b.focus = &node
		return
	}
	b.focus = nil
}

func (b *BrainPopup) Draw(screen *ebiten.Image) {
	if !b.open {
		return
	}
	rect := b.rect()
	fillRect(screen, rect, themeBackgroundColor())
	drawModalBorder(screen, rect)

	c := b.closeRect()
	drawGraphButton(screen, c.Min.X, c.Min.Y, c.Dx(), c.Dy(), "X", false)

	dr := b.diagramRect()
	net, acting, dim, ok := b.panel.ExpandedBrain()
	if !ok {
		text.Draw(screen, "BRAIN", r.FontSourceCodePro12,
			rect.Min.X+brainPopupPad, rect.Min.Y+18, themedForeground())
		text.Draw(screen, "(no brain to show)", r.FontSourceCodePro10,
			dr.Min.X, dr.Min.Y, themedForegroundDim())
		b.controls, b.nodeHits, b.focus = brainControls{}, nil, nil
		return
	}

	title := fmt.Sprintf("BRAIN: ORG %d   %s", b.panel.simulation.GetSelected(), brainSummary(net))
	text.Draw(screen, title, r.FontSourceCodePro12,
		rect.Min.X+brainPopupPad, rect.Min.Y+18, brainInk(dim, true))

	// The same view buttons the tab draws, writing back to the same panel,
	// plus the NUMBERS toggle — this is the only view wide enough to label a
	// connection, so it is the only one that offers the control.
	b.controls = b.panel.drawBrainControls(screen, dr.Min.X, rect.Min.Y+brainPopupTitleH, dr.Dx(), true)

	// Clipped to its own area, so a net taller than the box is cut off at the
	// edge rather than drawn over the border and the title.
	clipped := screen.SubImage(image.Rect(dr.Min.X, dr.Min.Y, dr.Max.X, dr.Max.Y)).(*ebiten.Image)
	// The selection moves and a lineage mutates, so the node a click picked
	// out may not be in this net at all.
	if b.focus != nil && !brainHasNode(net, *b.focus) {
		b.focus = nil
	}
	shown := brainShown{view: b.panel.brainView, focus: b.focus, numbersHidden: b.panel.brainNumbersHidden}
	b.nodeHits = drawBrainExpanded(clipped, net, acting, dim, shown, dr.Min.X, dr.Min.Y, dr.Dx(), dr.Dy())
}
