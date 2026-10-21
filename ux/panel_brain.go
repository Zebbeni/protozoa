package ux

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/Zebbeni/protozoa/config"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"

	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/neural"
	r "github.com/Zebbeni/protozoa/resources"
	"golang.org/x/image/font"
)

// The BRAIN tab: what an organism running on a neural net reads, what it can
// choose, and what it chose this cycle.
//
// A decision tree prints as text because its structure IS the explanation —
// the branch taken is a sentence. A net has no such reading: every input
// reaches every output through weights that mean nothing on their own. So the
// tab answers the same three questions a different way, by drawing the net
// against what is flowing through it right now — the conditions reading true
// are lit, the connections carrying a signal are thick, and the action with
// the highest score is lit at the other end.

const (
	brainNodeRadius = 2.5
	brainRowGap     = 4
	// The least height the panel will lay a diagram out in, for a window too
	// short for the rule below to leave anything useful.
	brainMinPanelHeight = 180
	// The row pitch the connection widths were chosen at, and the most they
	// are scaled up by. A diagram drawn at four times the pitch would draw a
	// weak connection as a bar otherwise.
	brainEdgeRefRowH  = 22
	brainMaxEdgeScale = 3.0
	// The label columns either side of the diagram, which gets what is left.
	// Sixteen characters and ten, at the 5px a character this face is set at,
	// which is what the longest condition and action names need spelled out.
	// That is the trade the diagram pays for: it keeps the 76px between them.
	brainInputLabelW  = 80
	brainOutputLabelW = 50
	brainValueW       = 40
	brainLabelGap     = 4
	// A connection is drawn between these widths, by how much of the net's
	// strongest signal it is carrying. The thinnest is still visible, so the
	// shape of the net can be read on a cycle when almost nothing is flowing
	// through it.
	//
	// The minimum is a whole pixel because the strokes are not antialiased:
	// measured, a 0.5px HORIZONTAL line at a whole-numbered y writes **no
	// pixels at all**, which is exactly the line a one-unit layer feeding
	// another one-unit layer draws. Diagonals survive it, so the loss was
	// invisible in a grown net and total in a small one.
	brainMinEdgeW = 1.0
	brainMaxEdgeW = 3.0
	// The share of full ink a connection carrying nothing is drawn at.
	brainFaintestEdge = 0.14
	// And a connection outside a picked-out node's own, which has to sit
	// below anything the diagram is still saying something with.
	brainMutedEdge = 0.05
	// The half-width of a node's click target. The dots are a few pixels
	// across, so the target is what makes them aimable.
	brainNodeHitR = 8.0
	// What the output value column is measured against. An output has no
	// ceiling, so the column is sized for a figure past anything seen rather
	// than for this cycle's values, which would change its width every cycle.
	brainValueSample = "-9999.99"
	// The least a diagram may be squeezed to before a bigger face is taking
	// more than it gives.
	brainMinDiagramW = 60
)

// brainView is what the thickness and ink of a connection are measuring. The
// two answer different questions about the same net and neither replaces the
// other: SIGNAL is what the organism is doing with its brain on the cycle
// being shown, and WEIGHTS is what the brain IS, which the cycle cannot
// change. An edge out of a condition reading false carries nothing however
// heavy it is, so a lineage's strongest connection can be invisible in SIGNAL
// — and a net acting on one branch says nothing in SIGNAL about the rest of
// what it has evolved.
type brainView int

const (
	brainViewSignal brainView = iota
	brainViewWeights
)

// allBrainViews is the list the buttons are built from, in the order they are
// drawn. SIGNAL first, since it is the view that answers "why this action".
var allBrainViews = []brainView{brainViewSignal, brainViewWeights}

// brainViewLabels names each view on its button. SIGNAL rather than CURRENT or
// CALCULATED, because both of those read as adjectives with nothing after them
// and neither says what the width of a line means — the pair here is two
// nouns, each the quantity its view draws.
var brainViewLabels = map[brainView]string{
	brainViewSignal:  "SIGNAL",
	brainViewWeights: "WEIGHTS",
}

// brainEdgeValue is the quantity one connection is drawn at in this view: the
// weight times the value of the unit feeding it, or the weight alone.
func brainEdgeValue(net *neural.Net, view brainView, layer, unit, prev int) float64 {
	if view == brainViewWeights {
		return net.WeightAt(layer, unit, prev)
	}
	return net.Transmitted(layer, unit, prev)
}

// brainEdgeReference is what those values are measured against, so a line's
// width is its share of the biggest thing in the picture it is part of.
// Neither quantity has a fixed ceiling — a hidden unit is a ReLU of a sum of
// weights a lineage has drifted to — so there is nothing else to normalise by.
func brainEdgeReference(net *neural.Net, view brainView) float64 {
	if view == brainViewWeights {
		return net.StrongestWeight()
	}
	return net.StrongestTransmission()
}

// brainNode identifies one dot in the diagram: its column and its row within
// that column. Column 0 is the inputs and column c after it is net layer c-1.
type brainNode struct{ col, row int }

func (n brainNode) is(col, row int) bool { return n.col == col && n.row == row }

// touches is whether a node is the picked-out one or ONE STEP from it.
//
// One step, not the whole path through the net: every unit of a layer is
// wired to every unit of the next, so transitively a grown net is a single
// connected mass and picking out a node would light the whole diagram. One
// step is also the question a click is asking — what feeds this, and what
// does it feed.
func (n brainNode) touches(col, row int) bool {
	if n.col == col {
		return n.row == row
	}
	return col == n.col-1 || col == n.col+1
}

// brainNodeHit is where one dot was drawn, for the click that comes a frame
// later. Square rather than round: the dots are a few pixels across and the
// corners of a box that size are not somewhere a user can tell they missed.
type brainNodeHit struct {
	brainNode
	x, y, r float64
}

func (h brainNodeHit) contains(mx, my int) bool {
	return math.Abs(float64(mx)-h.x) <= h.r && math.Abs(float64(my)-h.y) <= h.r
}

// brainNumbersShown reads the panel's toggle the way round it is spoken
// about. The field is stored inverted so its zero value draws the numbers.
func (p *Panel) brainNumbersShown() bool { return !p.brainNumbersHidden }

// brainShown is what the diagram draws beyond its layout: which quantity the
// connections are measured at, which node the reader has picked out, and
// whether the connections carry their numbers.
type brainShown struct {
	view  brainView
	focus *brainNode
	// numbersHidden is the NUMBERS button in the expanded view, stored the
	// same way round as the panel's own field so the zero value here draws
	// the numbers too — a brainShown built for its focus alone should not
	// silently turn the annotation off. The tab passes it as well, so the two
	// cannot disagree, though its 73px diagram labels nothing at any net size
	// whatever this says.
	numbersHidden bool
}

// lit is whether a node is part of what the picked-out node touches. Nothing
// picked out means everything is lit, which is the ordinary diagram.
func (s brainShown) lit(col, row int) bool {
	return s.focus == nil || s.focus.touches(col, row)
}

// edgeLit is whether a connection is one of the picked-out node's own, which
// is narrower than both its ends being lit: two nodes either side of the
// focus are each lit, and the connection between them is not its business.
func (s brainShown) edgeLit(fromCol, from, to int) bool {
	return s.focus == nil || s.focus.is(fromCol, from) || s.focus.is(fromCol+1, to)
}

// brainHasNode is whether a net still holds the node a click picked out. The
// panel hands the expanded view a different organism whenever the selection
// moves, and a net grows and loses units as a lineage mutates.
func brainHasNode(net *neural.Net, n brainNode) bool {
	cols := append([]int{len(net.Inputs())}, net.LayerSizes()...)
	return n.col >= 0 && n.col < len(cols) && n.row >= 0 && n.row < cols[n.col]
}

// brainLayout is where one drawing of the net puts its parts: the face its
// labels are set in, the width each column takes, and the pitch between rows.
// The panel draws at a fixed layout; the expanded view picks one to fill the
// box it is given.
type brainLayout struct {
	face         font.Face
	inputLabelW  int
	outputLabelW int
	valueW       int
	nodeRadius   float64
	rowH         int
	// edgeScale multiplies the connection widths, so a diagram drawn at twice
	// the size is not drawn in the same hairlines.
	edgeScale float64
}

// panelBrainColumns is the tab's fixed geometry: the columns the short labels
// were written to fit. The row pitch is not part of it, since that follows the
// height the diagram is given rather than the width.
func panelBrainColumns() brainLayout {
	return brainLayout{
		face:         r.FontSourceCodePro8,
		inputLabelW:  brainInputLabelW,
		outputLabelW: brainOutputLabelW,
		valueW:       brainValueW,
		nodeRadius:   brainNodeRadius,
		edgeScale:    1,
	}
}

// panelBrainLayout is that geometry with its rows spread into the box given.
func panelBrainLayout(net *neural.Net, width, available int) brainLayout {
	lay := panelBrainColumns()
	lay.rowH = brainRowHeight(available, brainTallestColumn(net), brainLayerGap(net, lay, width), lay.face)
	lay.edgeScale = brainEdgeScale(lay.rowH)
	return lay
}

// brainRowHeight is the pitch between nodes in a column, which is also the
// pitch between their labels: the height available divided by the longest
// column, so the diagram uses the space it is given instead of sitting in a
// band at a fixed pitch with the rest empty under it.
//
// Every column takes the same pitch, so the layers stay level with each other
// — a short layer is centred against a long one rather than being spread to
// the same height at a pitch of its own, which would draw two layers whose
// rows line up nowhere.
//
// Held to the distance BETWEEN the layers, because height alone is the wrong
// thing to maximise: the panel column leaves 76px for the diagram, and a net
// given a screen of height there was drawn as a vertical ribbon whose
// connections all ran steeper than the eye could follow left to right. The cap
// only ever bites where the diagram is narrow, which is the panel; in the
// expanded view the layers are hundreds of pixels apart and it never applies.
//
// Floored at the label height, since two labels on one line is worse than a
// diagram that runs past the bottom: the panel scrolls, and the popup clips.
func brainRowHeight(available, tallest, layerGap int, face font.Face) int {
	pitch := available / max(tallest, 1)
	if layerGap > 0 {
		pitch = min(pitch, layerGap)
	}
	return max(face.Metrics().Height.Round(), pitch)
}

// brainLayerGap is the horizontal distance between one column of nodes and the
// next. 0 for a net with a single column, which has no connections to draw and
// so nothing to keep in proportion.
func brainLayerGap(net *neural.Net, lay brainLayout, width int) int {
	cols := 1 + len(net.LayerSizes())
	if cols < 2 {
		return 0
	}
	return lay.diagramW(width) / (cols - 1)
}

// brainEdgeScale keeps the connection widths in proportion to the pitch, so a
// diagram drawn at twice the size is not drawn in the same hairlines.
func brainEdgeScale(rowH int) float64 {
	return min(max(float64(rowH)/brainEdgeRefRowH, 1), brainMaxEdgeScale)
}

// panelBrainHeight is the height the panel lays the diagram out in: the rest
// of the screen below the top of the diagram. The selected organism is the
// last section in the panel, so that is genuinely what is free — and the panel
// scrolls, so a net too tall for one screen is reached by scrolling rather
// than by packing its rows together.
func panelBrainHeight(topY int) int {
	return max(config.ScreenHeight()-topY, brainMinPanelHeight)
}

// The node columns start a dot's radius further in than the gap alone, because
// a dot is drawn CENTRED on the column and so reaches back into the gap: at
// the expanded view's radius that left the input names touching the dots they
// name. Only the left edge moves — the right-hand gap is the wider one, being
// what is left over after three gaps are taken out of the width.
func (l brainLayout) nodePad() int { return int(math.Ceil(l.nodeRadius)) }

func (l brainLayout) diagramX(x int) int {
	return x + l.inputLabelW + brainLabelGap + l.nodePad()
}

func (l brainLayout) diagramW(width int) int {
	return width - l.inputLabelW - l.outputLabelW - l.valueW - 3*brainLabelGap - l.nodePad()
}

// brainDiagramX and brainDiagramW are where the node columns sit in the panel:
// between the two label columns.
func brainDiagramX(x int) int     { return panelBrainColumns().diagramX(x) }
func brainDiagramW(width int) int { return panelBrainColumns().diagramW(width) }

// drawBrainTab renders the net into the content column and returns the bottom
// of what it drew.
func (p *Panel) drawBrainTab(img *ebiten.Image, net *neural.Net, acting d.Action, dim bool, x, topY, width int) int {
	titleFace := r.FontSourceCodePro10

	y := topY + titleFace.Metrics().Height.Round()
	text.Draw(img, brainSummary(net), titleFace, x, y, brainInk(dim, true))
	y += brainRowGap

	// No NUMBERS button here: the tab's diagram labels nothing at any net
	// size, so the control would be one a reader could press and not see act.
	p.brainControls = p.drawBrainControls(img, x, y, width, false)
	y += brainViewButtonsH()

	lay := panelBrainLayout(net, width, panelBrainHeight(y))
	// No focus in the tab either, for the same reason: at 73px the dots of
	// two columns are a few pixels apart and nothing could be aimed at.
	shown := brainShown{view: p.brainView, numbersHidden: p.brainNumbersHidden}
	bottom, _ := drawBrainDiagram(img, net, acting, dim, shown, lay, x, y, width)

	// Stashed for the expanded view, which redraws the same net bigger.
	p.lastBrainNet, p.lastBrainActing, p.lastBrainDim = net, acting, dim
	p.drawBrainExpandControl(img, x, topY, width, bottom)
	return bottom
}

// drawBrainExpandControl puts the expand control in the content column's
// top-right corner while the cursor is over the tab, the same arrangement the
// graph has. It is drawn over the summary line rather than beside it, since
// the summary is as long as the net's shape makes it.
func (p *Panel) drawBrainExpandControl(img *ebiten.Image, x, top, width, bottom int) {
	mx, my := ebiten.CursorPosition()
	inner := my + int(p.scrollY)
	if mx < x || mx >= x+width || inner < top || inner >= bottom {
		p.brainExpandRect = nil
		return
	}
	rect := drawExpandControl(img, x, top, width)
	p.brainExpandRect = &rect
}

// drawBrainExpanded draws the net to fill the box given, centred in it, at the
// largest layout that fits.
func drawBrainExpanded(img *ebiten.Image, net *neural.Net, acting d.Action, dim bool, shown brainShown, x, y, width, height int) []brainNodeHit {
	lay := expandedBrainLayout(net, width, height)
	drawn := max(brainTallestColumn(net), 1) * lay.rowH
	top := y + max(0, (height-drawn)/2)
	_, hits := drawBrainDiagram(img, net, acting, dim, shown, lay, x, top, width)
	return hits
}

// expandedBrainLayout is the largest drawing of this net that fits the box: the
// biggest face whose rows fit the height and whose labels still leave a
// diagram between them, with the rows spread into the height available rather
// than packed to the label.
//
// The label columns are measured against the labels THIS net draws rather than
// against the whole vocabulary — a net reading four conditions should not
// reserve the width the longest condition name needs.
func expandedBrainLayout(net *neural.Net, width, height int) brainLayout {
	tallest := max(brainTallestColumn(net), 1)
	faces := []font.Face{r.FontSourceCodePro12, r.FontSourceCodePro10, r.FontSourceCodePro8}

	chosen := measuredBrainLayout(net, faces[len(faces)-1])
	for _, face := range faces {
		lay := measuredBrainLayout(net, face)
		if tallest*face.Metrics().Height.Round() <= height && lay.diagramW(width) >= brainMinDiagramW {
			chosen = lay
			break
		}
	}

	chosen.rowH = brainRowHeight(height, tallest, brainLayerGap(net, chosen, width), chosen.face)
	chosen.edgeScale = brainEdgeScale(chosen.rowH)
	return chosen
}

// measuredBrainLayout sizes the label columns to the labels this net draws, in
// the face given.
func measuredBrainLayout(net *neural.Net, face font.Face) brainLayout {
	inputW := 0
	for _, c := range net.Inputs() {
		inputW = max(inputW, textAdvance(face, brainConditionLabel(c)))
	}
	outputW := 0
	for _, a := range net.Outputs() {
		outputW = max(outputW, textAdvance(face, brainActionLabel(a)))
	}
	return brainLayout{
		face:         face,
		inputLabelW:  inputW,
		outputLabelW: outputW,
		valueW:       textAdvance(face, brainValueSample),
		// Set here rather than after the columns are measured, so the width
		// the diagram is judged to have is the width it is drawn at.
		nodeRadius: max(brainNodeRadius, float64(face.Metrics().Height.Round())/4),
		edgeScale:  1,
	}
}

// brainTallestColumn is the longest column in the diagram, which is what its
// height follows.
func brainTallestColumn(net *neural.Net) int {
	tallest := len(net.Inputs())
	for _, n := range net.LayerSizes() {
		tallest = max(tallest, n)
	}
	return tallest
}

// brainSummary is the net's shape in one line: how many conditions it reads,
// the units in each layer, and the node count the upkeep is charged on.
func brainSummary(net *neural.Net) string {
	shape := fmt.Sprintf("%d", len(net.Inputs()))
	for _, n := range net.LayerSizes() {
		shape += fmt.Sprintf(">%d", n)
	}
	return fmt.Sprintf("%s  (%d nodes)", shape, net.Nodes())
}

// brainLitPlatePad is the room a lit label's plate leaves either side of its
// text.
const brainLitPlatePad = 2

// drawBrainLabel draws one name in the input or output column: reversed out
// of a filled plate when the cycle is acting on it — a condition that read
// true, the action being taken — and in the usual ink otherwise.
//
// The lit and quiet states were two levels of one ink, which is the weakest
// distinction the panel has, on the two facts the tab exists to answer. A
// plate is read before any of the text is.
func drawBrainLabel(img *ebiten.Image, s string, face font.Face, x, baseline int, dim, lit, inFocus bool) {
	if !brainLabelIsPlated(lit, inFocus) {
		text.Draw(img, s, face, x, baseline, brainLabelInk(dim, lit, inFocus))
		return
	}
	m := face.Metrics()
	ebitenutil.DrawRect(img,
		float64(x-brainLitPlatePad), float64(baseline-m.Ascent.Round()),
		float64(textAdvance(face, s)+2*brainLitPlatePad),
		float64(m.Ascent.Round()+m.Descent.Round()),
		brainLitPlateFill(dim))
	text.Draw(img, s, face, x, baseline, brainLitPlateInk(dim))
}

// brainLabelIsPlated: the plate marks a state, so it goes only on a label the
// cycle is acting on and never on one faded back behind a picked-out node.
func brainLabelIsPlated(lit, inFocus bool) bool { return lit && inFocus }

// brainLitPlateFill is a lit label's plate and brainLitPlateInk the text over
// it, so the pair reads as black on white in the dark theme and white on near
// black in the light one. Themed rather than literally white: the light
// theme's fill is 240-grey, where a white plate is no plate at all.
func brainLitPlateFill(dim bool) color.RGBA {
	fill := chrome(color.RGBA{R: 255, G: 255, B: 255, A: 255}, color.RGBA{R: 20, G: 20, B: 25, A: 255})
	if dim {
		return withAlpha(fill, 0x90)
	}
	return fill
}

func brainLitPlateInk(dim bool) color.RGBA {
	ink := chrome(color.RGBA{R: 20, G: 20, B: 25, A: 255}, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if dim {
		return withAlpha(ink, 0xC0)
	}
	return ink
}

// brainLabelInk is a label's colour with a node picked out: a label outside
// what that node touches is faded back, whether or not its own condition is
// true, so the column reads as part of the same answer the diagram is giving.
func brainLabelInk(dim, lit, inFocus bool) color.Color {
	if !inFocus {
		if dim {
			return fadedForeground(0x28)
		}
		return fadedForeground(0x40)
	}
	return brainInk(dim, lit)
}

// brainInk is a label's colour: lit for a condition reading true or the action
// being taken, quiet for the rest.
func brainInk(dim, lit bool) color.Color {
	if dim {
		if lit {
			return themedForegroundDim()
		}
		return themedMuted()
	}
	if lit {
		return themedForeground()
	}
	return themedMuted()
}

// drawBrainDiagram draws one column of dots per layer with the connections
// between them, the input conditions named down the left and the actions down
// the right with their scores.
//
// Column 0 is the inputs and column c after it is net layer c-1, so a net that
// senses nothing leaves its first column empty rather than quietly renumbering
// the rest.
func drawBrainDiagram(img *ebiten.Image, net *neural.Net, acting d.Action, dim bool, shown brainShown, lay brainLayout, x, top, width int) (int, []brainNodeHit) {
	cols := append([]int{len(net.Inputs())}, net.LayerSizes()...)
	height := max(brainTallestColumn(net), 1) * lay.rowH

	diagramX, diagramW := lay.diagramX(x), lay.diagramW(width)
	centres := make([]float64, len(cols))
	units := make([][]float64, len(cols))
	for i, n := range cols {
		cx := float64(diagramX)
		if len(cols) > 1 {
			cx += float64(i) * float64(diagramW-1) / float64(len(cols)-1)
		}
		centres[i] = cx
		units[i] = spreadColumn(top, height, n, lay.rowH)
	}

	// Connections first, so a node sits on top of what reaches it.
	strongest := brainEdgeReference(net, shown.view)
	for c := 1; c < len(cols); c++ {
		for j, cy := range units[c] {
			for i, py := range units[c-1] {
				flow := brainEdgeValue(net, shown.view, c-1, j, i)
				ink := edgeColor(flow, strongest, dim)
				if net.WeightAt(c-1, j, i) == 0 {
					// A connection the lineage has made nothing of yet. It
					// carries nothing, so it is already at the thinnest; grey
					// is what separates it from a weighted connection whose
					// unit happens to read 0 this cycle.
					ink = unweightedEdgeColor(dim)
				}
				if !shown.edgeLit(c-1, i, j) {
					ink = mutedEdgeColor(dim)
				}
				vector.StrokeLine(img,
					float32(centres[c-1]), float32(py), float32(centres[c]), float32(cy),
					float32(edgeWidth(flow, strongest)*lay.edgeScale), ink, false)
			}
		}
	}

	// A click lands on the dot a frame after it was drawn, so where each one
	// went is kept. The target is bigger than the dot: these are a few pixels
	// across and the difference between hitting and missing one should not be
	// a pixel.
	hits := brainNodeHits(centres, units, max(lay.nodeRadius*2, brainNodeHitR))
	for i := range cols {
		for row, cy := range units[i] {
			ebitenutil.DrawRect(img,
				centres[i]-lay.nodeRadius, cy-lay.nodeRadius,
				lay.nodeRadius*2, lay.nodeRadius*2, brainNodeColor(dim, shown, i, row))
		}
	}

	// The numbers over the lines they belong to, last so nothing covers them.
	drawBrainEdgeLabels(img, net, dim, shown, lay, cols, centres, units)
	drawBrainNodeNumbers(img, net, dim, shown, lay, cols, centres, units)

	drawBrainInputLabels(img, net, dim, shown, lay, x, units[0])
	drawBrainOutputLabels(img, net, acting, dim, shown, lay, x, width, units[len(units)-1])

	return top + height, hits
}

// brainNodeHits is a click target for every dot in the diagram, so none of
// them is a button the reader can see and not press.
func brainNodeHits(centres []float64, units [][]float64, r float64) []brainNodeHit {
	hits := make([]brainNodeHit, 0, len(units))
	for col := range centres {
		for row, cy := range units[col] {
			hits = append(hits, brainNodeHit{
				brainNode: brainNode{col: col, row: row},
				x:         centres[col], y: cy, r: r,
			})
		}
	}
	return hits
}

// brainNodeColor is a dot's ink: the one picked out is the brightest thing in
// the diagram, what it touches is drawn as usual, and the rest is faded out of
// the way rather than removed — the shape of the net still has to read.
func brainNodeColor(dim bool, shown brainShown, col, row int) color.Color {
	switch {
	case shown.focus != nil && shown.focus.is(col, row):
		if dim {
			return fadedForeground(0x90)
		}
		return fadedForeground(0xFF)
	case !shown.lit(col, row):
		if dim {
			return fadedForeground(0x18)
		}
		return fadedForeground(0x28)
	case dim:
		return fadedForeground(0x60)
	default:
		return fadedForeground(0xC0)
	}
}

// drawBrainInputLabels names each condition to the left of its node, right
// aligned so the names run up against their own column.
func drawBrainInputLabels(img *ebiten.Image, net *neural.Net, dim bool, shown brainShown, lay brainLayout, x int, ys []float64) {
	values := net.Activations(0)
	right := x + lay.inputLabelW
	for i, c := range net.Inputs() {
		if i >= len(ys) {
			break
		}
		// An input reads 1 when its condition was true on the cycle shown.
		lit := i < len(values) && values[i] != 0
		label := fitLabel(brainConditionLabel(c), lay.inputLabelW, lay.face)
		drawBrainLabel(img, label, lay.face, right-textAdvance(lay.face, label),
			labelBaseline(ys[i], lay.face), dim, lit, shown.lit(0, i))
	}
}

// drawBrainOutputLabels names each action to the right of its node with the
// score it reached, and lights the one being taken.
func drawBrainOutputLabels(img *ebiten.Image, net *neural.Net, acting d.Action, dim bool, shown brainShown, lay brainLayout, x, width int, ys []float64) {
	values := net.LastOutputs()
	labelX := x + width - lay.outputLabelW - lay.valueW
	valueRight := x + width
	outCol := len(net.LayerSizes())
	for i, a := range net.Outputs() {
		if i >= len(ys) {
			break
		}
		lit, inFocus := a == acting, shown.lit(outCol, i)
		baseline := labelBaseline(ys[i], lay.face)
		drawBrainLabel(img, fitLabel(brainActionLabel(a), lay.outputLabelW, lay.face),
			lay.face, labelX, baseline, dim, lit, inFocus)
		if i < len(values) {
			value := fmt.Sprintf("%.2f", values[i])
			drawBrainLabel(img, value, lay.face,
				valueRight-textAdvance(lay.face, value), baseline, dim, lit, inFocus)
		}
	}
}

// drawBrainEdgeLabels writes each connection's number on the line it belongs
// to: the same quantity the line's width and ink are drawn at, so the picture
// and the numbers on it are one reading. In SIGNAL that is what the connection
// is carrying on the cycle shown — the weight times the value of the unit
// feeding it — and in WEIGHTS the weight alone. Labelling both with the weight
// made the numbers the one part of the diagram the buttons did not change.
//
// Every line out of one node is labelled at the same fraction of its length,
// which is what keeps the numbers apart: the lines leave a single point and
// fan out, so a fraction t along them they stand t * rowH apart. That is also
// what bounds how small t may be, and why the fractions are spread over a band
// inside (0, 1) rather than over the whole of it — a label a tenth of the way
// along a nine-way fan sits on the other eight.
//
// A column whose lines cannot hold their numbers apart is left unlabelled
// rather than labelled illegibly. In the panel that is every column, since the
// diagram there is 76px wide for as many as nine columns of nodes.
func drawBrainEdgeLabels(img *ebiten.Image, net *neural.Net, dim bool, shown brainShown, lay brainLayout, cols []int, centres []float64, units [][]float64) {
	face := brainEdgeFace()
	slot := brainEdgeLabelW(face)
	gap := brainNarrowestGap(centres)
	// ONE decision for the whole diagram. It was taken per layer, on the size
	// of the column feeding it, which meant a net could carry numbers between
	// two of its layers and not the third — reported as the numbers appearing
	// on some organisms and not others, and on the hidden lines only. Whether
	// a diagram is annotated is now a property of the diagram.
	if !brainDiagramLabelled(shown, cols, lay.rowH, gap, slot, face) {
		return
	}
	for c := 1; c < len(cols); c++ {
		for i, py := range units[c-1] {
			t := brainEdgeLabelFraction(i, cols[c-1], lay.rowH, gap, slot, face)
			if shown.focus != nil {
				t = brainFocusLabelFraction
			}
			lx := centres[c-1] + t*(centres[c]-centres[c-1])
			for j, cy := range units[c] {
				if !shown.edgeLit(c-1, i, j) {
					// Faded to nearly nothing, so a number over it would be
					// the one legible thing left on a line being set aside.
					continue
				}
				value := brainEdgeValue(net, shown.view, c-1, j, i)
				ly := py + t*(cy-py)
				label := brainEdgeNumber(value)
				lw := textAdvance(face, label)
				drawBrainEdgeNumber(img, label, face,
					int(lx)-lw/2, labelBaseline(ly, face), dim)
			}
		}
	}
}

// brainNodeNumberGap is the room a node's number leaves above its dot.
const brainNodeNumberGap = 2

// drawBrainNodeNumbers writes a number at each node, which is the one part of
// a unit no line can carry.
//
// A unit is relu(bias + what reached it), and NOTHING CONNECTS TO IT to
// deliver the bias — so a net whose inputs all read 0 looks like it should be
// inert and is not. That was reported twice before anything was drawn for it,
// on a 1>3>4>3 and on a 1>2>2>2, each time as hidden units carrying a signal
// out of connections carrying nothing in.
//
// Each view writes the quantity it is drawn at, the same rule the numbers on
// the lines follow: in WEIGHTS the bias added at the node, signed, since it is
// an addend rather than a reading; in SIGNAL what the unit is holding on the
// cycle shown. An input has a value but no bias of its own, so its column
// carries a number in SIGNAL and none in WEIGHTS.
func drawBrainNodeNumbers(img *ebiten.Image, net *neural.Net, dim bool, shown brainShown, lay brainLayout, cols []int, centres []float64, units [][]float64) {
	face := brainEdgeFace()
	if shown.numbersHidden || !brainNodeNumbersFit(lay, brainNarrowestGap(centres), face) {
		return
	}
	for c := range cols {
		for row, cy := range units[c] {
			if !shown.lit(c, row) {
				// Set aside with everything else behind a picked-out node.
				continue
			}
			label, ok := brainNodeNumber(net, shown.view, c, row)
			if !ok {
				continue
			}
			drawBrainEdgeNumber(img, label, face,
				int(centres[c])-textAdvance(face, label)/2,
				int(cy-lay.nodeRadius)-brainNodeNumberGap-face.Metrics().Descent.Round(), dim)
		}
	}
}

// brainNodeNumber is what one node is annotated with in this view, and
// whether it has anything to say at all.
func brainNodeNumber(net *neural.Net, view brainView, col, row int) (string, bool) {
	if view == brainViewWeights {
		if col == 0 {
			return "", false
		}
		return brainBiasNumber(net.BiasAt(col-1, row)), true
	}
	values := net.Activations(col)
	if row >= len(values) {
		return "", false
	}
	return brainEdgeNumber(values[row]), true
}

// brainBiasNumber writes a bias with its sign always, so it reads as the term
// added at the node rather than as another reading taken there.
func brainBiasNumber(v float64) string {
	n := brainEdgeNumber(v)
	if strings.HasPrefix(n, "-") {
		return n
	}
	return "+" + n
}

// brainNodeNumbersFit: the number sits above its own dot, so a row has to hold
// two lines of text — the label on the dot's own line and the number over it —
// and the columns have to stand a number apart or one would be drawn across
// the next column of dots. The panel's columns are 24px apart against a 30px
// number, so these are an expanded-view reading like the numbers on the lines.
func brainNodeNumbersFit(lay brainLayout, gap int, face font.Face) bool {
	return gap >= brainEdgeLabelW(face) && lay.rowH >= 2*face.Metrics().Height.Round()
}

// drawBrainEdgeNumber writes one number over its line, in the brightest ink
// the theme has and bolded by being drawn a second time a pixel to the right —
// the same faux bold the descendant tab uses to mark a most-successful
// lineage, since the project ships only the regular cut of Source Code Pro.
//
// It used to sit on a plate in the window fill, which kept it legible over a
// thick line and cost a filled box per connection: a net with a number on
// every line was a field of boxes with a diagram somewhere behind it, and the
// boxes hid the lines they did not belong to. Bold carries it over a line on
// its own, so nothing is covered.
func drawBrainEdgeNumber(img *ebiten.Image, label string, face font.Face, x, baseline int, dim bool) {
	ink := brainNumberInk(dim)
	text.Draw(img, label, face, x, baseline, ink)
	text.Draw(img, label, face, x+1, baseline, ink)
}

// brainEdgeNumber is a connection's value as it is written on its line.
//
// A value too small to show at two decimals falls back to one significant
// figure rather than printing as 0.00, because 0.00 is a real answer here and
// has to keep meaning what it says: in WEIGHTS it is a connection the lineage
// has made nothing of, drawn grey, and in SIGNAL it is one carrying nothing
// this cycle. A weight of 0.001 reading 0.00 over a coloured line would be
// claiming to be the first of those.
func brainEdgeNumber(v float64) string {
	if v == 0 {
		// Negative zero, which is what a negative weight carrying nothing
		// produces and what SIGNAL is full of: "-0.00" reads as a quantity
		// with a direction where there is neither.
		return "0.00"
	}
	label := fmt.Sprintf("%.2f", v)
	if v != 0 && (label == "0.00" || label == "-0.00") {
		return fmt.Sprintf("%.1g", v)
	}
	return label
}

// brainNarrowestGap is the least distance between two columns of nodes. The
// columns are evenly spaced, so this is the one gap to a rounding — taking the
// smallest is what makes the whole-diagram decision below safe either way.
func brainNarrowestGap(centres []float64) int {
	gap := 0
	for c := 1; c < len(centres); c++ {
		if d := int(centres[c] - centres[c-1]); c == 1 || d < gap {
			gap = d
		}
	}
	return gap
}

// brainDiagramLabelsFit is the one decision: a diagram writes a number on
// every connection it draws, or on none of them.
//
// Taken on the widest column feeding a layer, since that is the layer whose
// numbers are hardest to keep apart, and a diagram annotated in some layers
// and not others reads as a fault rather than as a limit.
//
// With a node picked out the test is different in kind rather than in degree.
// Whether a diagram writes numbers on its connections at all is two separate
// questions kept separate: brainDiagramLabelled asks the reader's toggle and
// then the room, and brainDiagramLabelsFit is the room alone. Folding the
// toggle into the geometry would make a function named for fitting answer
// something that has nothing to do with fitting.
//
// Only that node's own connections carry numbers, and they are a FAN: every
// one of them meets the picked-out node, so placing them all at one fraction
// of their length spreads them by the row pitch and stacks them in one column
// where nothing competes for the room. The band existing is then the whole
// test, which is why a 223-connection net the diagram cannot label at all can
// still be read a node at a time.
func brainDiagramLabelled(shown brainShown, cols []int, rowH, gap, widest int, face font.Face) bool {
	return !shown.numbersHidden && brainDiagramLabelsFit(shown, cols, rowH, gap, widest, face)
}

func brainDiagramLabelsFit(shown brainShown, cols []int, rowH, gap, widest int, face font.Face) bool {
	if gap <= 0 || widest <= 0 {
		return false
	}
	lo, hi := brainEdgeLabelBand(rowH, gap, widest, face)
	if hi <= lo {
		return false
	}
	if shown.focus != nil {
		return true
	}
	feeding := 0
	for c := 1; c < len(cols); c++ {
		feeding = max(feeding, cols[c-1])
	}
	return brainEdgeLabelsFit(feeding, rowH, gap, widest, face)
}

// brainFocusLabelFraction is where a picked-out node's numbers sit on their
// lines: the MIDDLE, which is the one place a number plainly belongs to the
// line it is on rather than to a node at either end of it.
//
// They were drawn at the end of the band FURTHEST from the picked-out node,
// on the reasoning that a fan is widest there. True, and not needed: a fan's
// numbers stand t x rowH apart a fraction t from the point they share, the
// band is exactly the fractions at which that clears a line of text, and the
// band is symmetric about the middle. A diagram is only annotated at all
// where lo < hi, which is lo < 0.5 — so wherever these numbers are drawn,
// half a row already holds them apart. Reported as the numbers jumping to the
// far ends of their links as soon as a node was picked out.
const brainFocusLabelFraction = 0.5

// brainEdgeFace is the face the numbers on the connections are set in, fixed
// rather than following the layout's own face, so what is annotated does not
// change with the size the node labels happen to land on.
//
// The size is a real trade: it decides whether a net carries numbers at all,
// since a wider slot needs more room between the lines. 10 point against the 8
// it shipped at costs a net or two at the margin (`LABEL_PROBE=1` prints
// which), and the NUMBERS button is what makes that affordable — a diagram too
// dense to read annotated can be switched back rather than being the only
// thing the view offers.
func brainEdgeFace() font.Face { return r.FontSourceCodePro10 }

// brainEdgeSample is the figure the labels are laid out against: five
// characters, which is every value under 10 with its sign. Bigger ones happen
// and overhang their slot, which is the right way round for a number and is
// why the sample is what the values usually need rather than the widest one
// they could reach.
const brainEdgeSample = "-9.99"

// brainEdgeLabelW is the slot one number is given, measured against that fixed
// sample rather than against the numbers a net is currently showing.
//
// SIGNAL is what forces it: its numbers change every cycle, so measuring them
// would walk the labels sideways as the organism acts and flip a marginal net
// between labelled and not from one frame to the next — the same reason the
// output value column is sized against `brainValueSample`. One sample for both
// views, so switching view changes the numbers on the lines and never which
// lines carry one. A value wider than the sample overhangs its slot rather
// than being cut, which is the right way round for a number.
func brainEdgeLabelW(face font.Face) int { return textAdvance(face, brainEdgeSample) }

// brainFeedingUnits is how many units feed a layer: the inputs for the first,
// the layer before it for the rest.
func brainFeedingUnits(net *neural.Net, layer int) int {
	if layer <= 0 {
		return len(net.Inputs())
	}
	return net.LayerSizes()[layer-1]
}

// brainEdgeLabelBand is the span of each line the labels are spread along. Its
// margin is set by two things: a label nearer the fan's origin than the row
// pitch can hold would overlap the ones beside it, and one nearer than half its
// own width would be drawn over the column of nodes.
func brainEdgeLabelBand(rowH, gap, widest int, face font.Face) (lo, hi float64) {
	byPitch := float64(face.Metrics().Height.Round()) / float64(max(rowH, 1))
	byWidth := (float64(widest)/2 + brainNodeRadius) / float64(max(gap, 1))
	lo = max(byPitch, byWidth)
	return lo, 1 - lo
}

// brainEdgeLabelFraction is how far along its line each connection out of node
// i of n is labelled: an even spread across the band, so in a three-node layer
// the three nodes label at about a quarter, a half and three quarters of the
// way across.
func brainEdgeLabelFraction(i, n, rowH, gap, widest int, face font.Face) float64 {
	lo, hi := brainEdgeLabelBand(rowH, gap, widest, face)
	return lo + (hi-lo)*float64(i+1)/float64(n+1)
}

// brainEdgeLabelsFit says whether a layer's weights can be written on its lines
// without landing on each other. Two conditions, one per axis: the band has to
// exist at all, which is the row pitch holding a line of text twice over, and
// consecutive nodes in the feeding column have to label far enough apart
// horizontally to clear a number between them.
func brainEdgeLabelsFit(n, rowH, gap, widest int, face font.Face) bool {
	if n <= 0 || gap <= 0 || widest <= 0 {
		return false
	}
	lo, hi := brainEdgeLabelBand(rowH, gap, widest, face)
	if hi <= lo {
		return false
	}
	return (hi-lo)*float64(gap)/float64(n+1) >= float64(widest+brainLabelGap)
}

// brainNumberInk is a connection number's ink: the brightest the theme has, at
// a readable alpha whatever its line is carrying. It used to take its line's
// own hue, which is no longer what distinguishes it — the number is read over
// the line rather than beside it, and its sign is already in its own minus and
// in the colour of the line under it.
//
// Themed, not simply white: the light theme's fill is 240-grey, where a white
// number is not merely wrong but invisible.
func brainNumberInk(dim bool) color.Color {
	alpha := uint8(0xFF)
	if dim {
		alpha = 0x90
	}
	return withAlpha(chrome(
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
		color.RGBA{R: 20, G: 20, B: 20, A: 255},
	), alpha)
}

// unweightedEdgeColor is a connection with no weight at all: grey, since it is
// arguing neither for its unit nor against it, at the same share of full ink
// as the faintest carrying connection.
//
// Drawn rather than left out, because an absence in a diagram of a thing that
// IS fully connected reads as a rendering fault — which is how the missing
// one-unit-layer line came to be reported. Every founder's net is entirely
// this, since each connection arrives at exactly the 0 it was introduced with.
func unweightedEdgeColor(dim bool) color.Color { return greyEdge(brainFaintestEdge, dim) }

// mutedEdgeColor is a connection outside the picked-out node's own: set aside
// rather than removed, so the shape of the net still reads behind the answer.
func mutedEdgeColor(dim bool) color.Color { return greyEdge(brainMutedEdge, dim) }

func greyEdge(share float64, dim bool) color.Color {
	base := 0xE0
	if dim {
		base = 0x70
	}
	grey := chrome(
		color.RGBA{R: 170, G: 170, B: 180, A: 255},
		color.RGBA{R: 70, G: 70, B: 80, A: 255},
	)
	return withAlpha(grey, uint8(float64(base)*share))
}

// brainSignColor is the hue a connection and its weight are drawn in: one for
// a connection arguing for its unit, one for a connection arguing against.
// Both are written here rather than at each use, so a line and the number on
// it cannot be inked differently.
func brainSignColor(negative bool) color.RGBA {
	if negative {
		return chrome(color.RGBA{R: 200, G: 90, B: 90, A: 255}, color.RGBA{R: 160, G: 40, B: 40, A: 255})
	}
	return chrome(color.RGBA{R: 110, G: 170, B: 230, A: 255}, color.RGBA{R: 30, G: 90, B: 170, A: 255})
}

// labelBaseline centres a label on its node rather than hanging it below.
func labelBaseline(cy float64, face font.Face) int {
	return int(cy) + face.Metrics().Ascent.Round()/2
}

// fitLabel trims a label to its column. The short forms are written to fit, so
// this only catches a node type with no short form, which falls back to its
// full name.
func fitLabel(label string, width int, face font.Face) string {
	if textAdvance(face, label) <= width {
		return label
	}
	for len(label) > 1 {
		label = label[:len(label)-1]
		if textAdvance(face, label+".") <= width {
			return label + "."
		}
	}
	return label
}

// spreadColumn is the y of each unit in one column, one row apart and centred
// on the diagram, so a short column sits level with the middle of a long one.
func spreadColumn(top, height, n, rowH int) []float64 {
	if n <= 0 {
		return nil
	}
	out := make([]float64, n)
	offset := float64(height-n*rowH) / 2
	for i := range out {
		out[i] = float64(top) + offset + float64(i*rowH) + float64(rowH)/2
	}
	return out
}

// edgeWidth is how thick a connection is drawn: by how much of the strongest
// signal in the net it is carrying right now.
//
// Against this cycle's own maximum, because a hidden unit's value has no fixed
// ceiling — it is a ReLU of a sum of weights a lineage has drifted to. So a
// thick line means "the strongest thing flowing in this picture", and widths
// compare within a frame rather than between frames. A connection carrying
// nothing is still drawn at the minimum, so the shape of the net can be read
// on a cycle when almost nothing is flowing.
func edgeWidth(flow, strongest float64) float64 {
	if strongest <= 0 {
		return brainMinEdgeW
	}
	share := math.Min(math.Abs(flow)/strongest, 1)
	return brainMinEdgeW + share*(brainMaxEdgeW-brainMinEdgeW)
}

// edgeColor splits the sign: a connection arguing for its unit and one arguing
// against it are opposite things, and one ink for both would draw a net that
// only ever agrees with itself.
//
// The ink follows the signal as well as the width, because width alone does
// not separate them: a net is fully connected, so a grown one draws a few
// hundred lines through a column 100px wide, and at one alpha the weak
// majority is a mesh the strong few are lost in. Fading them leaves the
// structure visible underneath and lets the carrying connections read as a
// path through it.
func edgeColor(flow, strongest float64, dim bool) color.Color {
	base := 0xE0
	if dim {
		base = 0x70
	}
	share := 0.0
	if strongest > 0 {
		share = math.Min(math.Abs(flow)/strongest, 1)
	}
	alpha := uint8(float64(base) * (brainFaintestEdge + (1-brainFaintestEdge)*share))
	return withAlpha(brainSignColor(flow < 0), alpha)
}

// brainViewButtonW is what one view button is drawn at. Fixed rather than a
// share of the width, because two buttons spread across the expanded view
// would be 400px each to say one word.
const brainViewButtonW = 96

// brainViewButtonsH is the height the buttons take, which the popup has to
// know before it lays the diagram out.
func brainViewButtonsH() int { return graphButtonRowHeight + brainRowGap }

// brainNumbersLabel is the toggle's text. It says nothing about its state,
// because it is lit when the numbers are on, like the two view buttons beside
// it, and what it does is visible in the diagram the moment it is pressed —
// unlike the replay menu's music button, which has to spell its state out.
const brainNumbersLabel = "NUMBERS"

// brainControls is the row of buttons above a diagram: the two views always,
// and the numbers toggle only where numbers can be drawn at all.
type brainControls struct {
	views []brainViewHitbox
	// numbers is the NUMBERS button, nil for a caller that has none — the
	// tab, whose diagram labels nothing at any net size. Nil is also what
	// keeps a click from matching a button nobody drew.
	numbers *graphRect
}

// drawBrainControls draws the row and returns where it landed. The panel and
// the expanded view draw the same views, and each writes straight back to the
// panel, so switching view in one leaves the other on it — the arrangement the
// graph popup already has with the graph's own controls.
//
// The CALLER keeps the rects rather than the panel holding one set for both.
// The two are drawn in different coordinate spaces, and the expanded view no
// longer swallows every click, so a click meant for the panel must not be
// matched against the expanded view's geometry.
func (p *Panel) drawBrainControls(img *ebiten.Image, x, y, width int, withNumbers bool) brainControls {
	controls := brainControlRects(x, y, width, withNumbers)
	for _, rect := range controls.views {
		drawGraphButton(img, rect.x, rect.y, rect.w, rect.h,
			brainViewLabels[rect.view], rect.view == p.brainView)
	}
	if rect := controls.numbers; rect != nil {
		drawGraphButton(img, rect.x, rect.y, rect.w, rect.h,
			brainNumbersLabel, p.brainNumbersShown())
	}
	return controls
}

// brainControlRects is where the row goes, apart from the drawing, so what it
// covers can be measured without a render target. One function for the whole
// row rather than one per kind of button: the row's width is shared out among
// however many it holds, so two functions would each be laying out half of a
// decision neither could see.
func brainControlRects(x, y, width int, withNumbers bool) brainControls {
	n := len(allBrainViews)
	if withNumbers {
		n++
	}
	w := min(brainViewButtonW, (width-(n-1)*brainLabelGap)/n)
	at := func(i int) graphRect {
		return graphRect{x: x + i*(w+brainLabelGap), y: y, w: w, h: graphButtonRowHeight}
	}
	controls := brainControls{views: make([]brainViewHitbox, 0, len(allBrainViews))}
	for i, view := range allBrainViews {
		controls.views = append(controls.views, brainViewHitbox{graphRect: at(i), view: view})
	}
	if withNumbers {
		rect := at(len(allBrainViews))
		controls.numbers = &rect
	}
	return controls
}

type brainViewHitbox struct {
	graphRect
	view brainView
}

// takeBrainControlClick switches the view, or shows and hides the numbers, if
// the click landed on one of the buttons drawn at the rects given.
func (p *Panel) takeBrainControlClick(controls brainControls, mx, my int) bool {
	for _, rect := range controls.views {
		if rect.contains(mx, my) {
			p.brainView = rect.view
			return true
		}
	}
	if rect := controls.numbers; rect != nil && rect.contains(mx, my) {
		p.brainNumbersHidden = !p.brainNumbersHidden
		return true
	}
	return false
}
