package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/neural"
	r "github.com/Zebbeni/protozoa/resources"
)

// TestBrainPopupGeometryFitsTheScreen: the expanded net opens in the same box
// the expanded graph does, so the area it draws into has to be inside that box
// at every window size, and bigger than the panel column it was opened from —
// which is the whole reason to open it.
func TestBrainPopupGeometryFitsTheScreen(t *testing.T) {
	loadKeyGlobals(t)
	b := NewBrainPopup(&Panel{})

	for _, size := range [][2]int{{1400, 800}, {1920, 1057}, {900, 600}} {
		config.GetCurrentGlobals().ScreenWidth = size[0]
		config.GetCurrentGlobals().ScreenHeight = size[1]

		outer, dr := b.rect(), b.diagramRect()
		if dr.Dx() <= 0 || dr.Dy() <= 0 {
			t.Errorf("at %dx%d the diagram area is %dx%d", size[0], size[1], dr.Dx(), dr.Dy())
			continue
		}
		if dr.Min.X < outer.Min.X || dr.Max.X > outer.Max.X ||
			dr.Min.Y < outer.Min.Y || dr.Max.Y > outer.Max.Y {
			t.Errorf("at %dx%d the diagram area runs outside the popup", size[0], size[1])
		}
		if dr.Dx() <= infoAreaWidth {
			t.Errorf("at %dx%d the expanded net gets %dpx against the panel column's %dpx",
				size[0], size[1], dr.Dx(), infoAreaWidth)
		}
		if !hitRect(b.closeRect().Min.X+1, b.closeRect().Min.Y+1, outer) {
			t.Errorf("at %dx%d the close control is outside the popup", size[0], size[1])
		}
		// The title sits above the diagram rather than over it.
		if dr.Min.Y <= outer.Min.Y+18 {
			t.Errorf("at %dx%d the diagram starts at %d, over the title baseline at %d",
				size[0], size[1], dr.Min.Y, outer.Min.Y+18)
		}
	}
}

// TestTheExpandedNetTakesOnlyItsOwnBox: it is the one popup here that is not
// modal. Clicking away used to close it, so stepping to the next cycle with it
// open both shut it and lost the click — and watching one organism's net
// change cycle by cycle is the reason to have it open. It claims the mouse
// over its own box, so a click on the diagram does not reach the panel under
// it, and nothing else.
func TestTheExpandedNetTakesOnlyItsOwnBox(t *testing.T) {
	loadKeyGlobals(t)
	config.GetCurrentGlobals().ScreenWidth = 1400
	config.GetCurrentGlobals().ScreenHeight = 800
	b := NewBrainPopup(&Panel{})

	if b.Update() {
		t.Error("a closed popup consumed input")
	}
	b.Open()
	if !b.IsOpen() {
		t.Fatal("Open did not open it")
	}

	box := b.rect()
	inside := [][2]int{
		{box.Min.X + 1, box.Min.Y + 1},
		{(box.Min.X + box.Max.X) / 2, (box.Min.Y + box.Max.Y) / 2},
	}
	for _, at := range inside {
		if !b.takesMouseAt(at[0], at[1]) {
			t.Errorf("a click at %v, inside the box, was left to the panel under it", at)
		}
	}
	// The panel's own controls sit left of the box, and the replay buttons
	// among them: those clicks have to get through.
	outside := [][2]int{{0, 0}, {box.Min.X - 2, box.Min.Y + 20}, {box.Max.X + 2, box.Max.Y - 20}}
	for _, at := range outside {
		if b.takesMouseAt(at[0], at[1]) {
			t.Errorf("a click at %v, outside the box, was swallowed by the expanded net", at)
		}
	}
}

// TestOnlyTheCloseControlClosesTheExpandedNet: clicking the diagram, a view
// button, or the empty space beside them leaves it up. It is a view being
// read, not a dialog waiting to be dismissed.
func TestOnlyTheCloseControlClosesTheExpandedNet(t *testing.T) {
	loadKeyGlobals(t)
	config.GetCurrentGlobals().ScreenWidth = 1400
	config.GetCurrentGlobals().ScreenHeight = 800
	b := NewBrainPopup(&Panel{})
	b.Open()

	box, dr := b.rect(), b.diagramRect()
	b.controls = brainControlRects(dr.Min.X, box.Min.Y+brainPopupTitleH, dr.Dx(), true)
	for _, at := range [][2]int{
		{(dr.Min.X + dr.Max.X) / 2, (dr.Min.Y + dr.Max.Y) / 2}, // the diagram
		{b.controls.views[1].x + 1, b.controls.views[1].y + 1}, // a view button
		{b.controls.numbers.x + 1, b.controls.numbers.y + 1},   // the numbers toggle
		{box.Min.X + 4, box.Max.Y - 4},                         // empty space in the box
	} {
		b.clickAt(at[0], at[1])
		if !b.IsOpen() {
			t.Fatalf("a click at %v closed the expanded net", at)
		}
	}
	// Each control still did its own job while not closing anything.
	if b.panel.brainView != b.controls.views[1].view {
		t.Error("clicking a view button inside the box did not switch the view")
	}
	if b.panel.brainNumbersShown() {
		t.Error("clicking the numbers toggle inside the box did not hide the numbers")
	}

	c := b.closeRect()
	b.clickAt(c.Min.X+1, c.Min.Y+1)
	if b.IsOpen() {
		t.Error("the close control did not close it")
	}
	if b.Update() {
		t.Error("a closed popup still claims the mouse")
	}
}

func TestBrainExpandRequestIsTakenOnce(t *testing.T) {
	p := &Panel{}

	if p.TakeBrainExpandRequest() {
		t.Error("an untouched panel reported an expand request")
	}
	p.brainExpandRequested = true
	if !p.TakeBrainExpandRequest() {
		t.Error("the request was not reported")
	}
	if p.TakeBrainExpandRequest() {
		t.Error("the request was reported twice")
	}
}

// TestTheBrainExpandControlOnlyExistsOnHover: the control covers the corner of
// the diagram, so it is drawn only while the cursor is over the tab — and with
// no control there is nothing to click.
func TestTheBrainExpandControlOnlyExistsOnHover(t *testing.T) {
	loadKeyGlobals(t)
	p := &Panel{brainExpandRect: &graphRect{w: 1, h: 1}}

	// A tab drawn far from where the test cursor sits (0, 0).
	p.drawBrainExpandControl(nil, 400, 400, infoAreaWidth, 600)
	if p.brainExpandRect != nil {
		t.Error("the expand control is clickable without the cursor over the tab")
	}
	if p.HandleBrainInput() {
		t.Error("the panel claimed the mouse with no control drawn")
	}
}

// TestTheBrainExpandControlStaysInTheContentColumn: the column is 258px of a
// 370px panel, so a control placed off the panel's full width would sit over
// the portrait — and it is drawn into a SubImage clipped to the column, where
// that would show as a control half cut off rather than as one out of place.
func TestTheBrainExpandControlStaysInTheContentColumn(t *testing.T) {
	const top = 200
	rect := expandControlRect(selectedXOffset, top, infoAreaWidth)

	if rect.x < selectedXOffset || rect.x+rect.w > selectedXOffset+infoAreaWidth {
		t.Errorf("the control spans %d..%d, outside the content column %d..%d",
			rect.x, rect.x+rect.w, selectedXOffset, selectedXOffset+infoAreaWidth)
	}
	if rect.y < top {
		t.Errorf("the control starts at %d, above the tab at %d", rect.y, top)
	}
}

// TestTheExpandedViewShowsNothingWithoutABrain: the stash is the BRAIN tab's to
// fill, so a panel showing any other tab has nothing to expand and the popup
// has to be able to say so rather than drawing a net nobody is looking at.
func TestTheExpandedViewShowsNothingWithoutABrain(t *testing.T) {
	p := &Panel{}
	if _, _, _, ok := p.ExpandedBrain(); ok {
		t.Error("a panel that has drawn no brain offered one to expand")
	}
}

// TestTheExpandedDiagramIsBiggerThanThePanels is the point of the popup stated
// as a measurement: more room between the rows and more room for the labels,
// for the same net.
func TestTheExpandedDiagramIsBiggerThanThePanels(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	net := grownNet(t, 600)

	panelLay := panelBrainLayout(net, infoAreaWidth, panelBrainHeight(600))
	b := NewBrainPopup(&Panel{})
	dr := b.diagramRect()
	expanded := expandedBrainLayout(net, dr.Dx(), dr.Dy())

	if expanded.diagramW(dr.Dx()) <= panelLay.diagramW(infoAreaWidth) {
		t.Errorf("the expanded diagram is %dpx wide against the panel's %dpx",
			expanded.diagramW(dr.Dx()), panelLay.diagramW(infoAreaWidth))
	}
	if expanded.rowH < panelLay.rowH {
		t.Errorf("the expanded rows are %dpx apart against the panel's %dpx",
			expanded.rowH, panelLay.rowH)
	}
	t.Logf("a %d-node net: panel rows %dpx / diagram %dpx, expanded rows %dpx / diagram %dpx",
		net.Nodes(), panelLay.rowH, panelLay.diagramW(infoAreaWidth),
		expanded.rowH, expanded.diagramW(dr.Dx()))
}

// TestTheExpandedLayoutFitsTheBoxItIsGiven: the popup does not scroll, so the
// layout has to come down to the box — a bigger face is worth nothing if its
// rows run off the bottom, and a wider label column is worth nothing if it
// leaves no diagram between the two.
func TestTheExpandedLayoutFitsTheBoxItIsGiven(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	for _, generations := range []int{0, 20, 200, 600} {
		net := grownNet(t, generations)
		tallest := max(brainTallestColumn(net), 1)

		for _, box := range [][2]int{{1000, 600}, {600, 400}, {400, 200}} {
			lay := expandedBrainLayout(net, box[0], box[1])
			lineH := lay.face.Metrics().Height.Round()

			if lay.rowH < lineH {
				t.Errorf("%d-generation net in a %dx%d box: rows %dpx apart under the %dpx a label needs",
					generations, box[0], box[1], lay.rowH, lineH)
			}
			if lay.diagramW(box[0]) < brainMinDiagramW {
				t.Errorf("%d-generation net in a %dx%d box: %dpx of diagram between the labels",
					generations, box[0], box[1], lay.diagramW(box[0]))
			}
			// Only when the labels themselves can fit: under that the rows are
			// floored at the label height and the diagram is clipped.
			if tallest*lineH <= box[1] && tallest*lay.rowH > box[1] {
				t.Errorf("%d-generation net in a %dx%d box: %d rows of %dpx is %dpx of a %dpx box",
					generations, box[0], box[1], tallest, lay.rowH, tallest*lay.rowH, box[1])
			}
		}
	}
}

// TestTheExpandedLabelsAreNotTruncated: the columns are measured against the
// labels this net draws, so nothing in the expanded view needs the dot that
// marks a label cut off in the panel.
func TestTheExpandedLabelsAreNotTruncated(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	net := grownNet(t, 600)

	b := NewBrainPopup(&Panel{})
	dr := b.diagramRect()
	lay := expandedBrainLayout(net, dr.Dx(), dr.Dy())

	for _, c := range net.Inputs() {
		label := brainConditionLabel(c)
		if fitLabel(label, lay.inputLabelW, lay.face) != label {
			t.Errorf("%q does not fit the expanded view's %dpx input column", label, lay.inputLabelW)
		}
	}
	for _, a := range net.Outputs() {
		label := brainActionLabel(a)
		if fitLabel(label, lay.outputLabelW, lay.face) != label {
			t.Errorf("%q does not fit the expanded view's %dpx output column", label, lay.outputLabelW)
		}
	}
}

// TestTheValueColumnIsMeasuredAgainstAFixedFigure: an output has no ceiling, so
// sizing its column on this cycle's values would change the column width every
// cycle and walk the whole diagram sideways as the organism acts.
func TestTheValueColumnIsMeasuredAgainstAFixedFigure(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	net := neural.Starter(d.ActChemosynthesis)
	first := measuredBrainLayout(net, r.FontSourceCodePro10).valueW
	net.Choose(func(d.Condition) bool { return true })
	if got := measuredBrainLayout(net, r.FontSourceCodePro10).valueW; got != first {
		t.Errorf("the value column was %dpx and is %dpx after the net ran", first, got)
	}
	if first <= 0 {
		t.Errorf("the value column is %dpx, so no score can be drawn in it", first)
	}
}

// TestRowsFillTheHeightTheyAreGiven is the spacing rule stated directly: the
// pitch is the height divided by the longest column, so the diagram uses what
// it is given rather than sitting in a band at a fixed pitch — and the whole
// of it is used, to within the rounding of one division.
func TestRowsFillTheHeightTheyAreGiven(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	face := r.FontSourceCodePro8
	lineH := face.Metrics().Height.Round()

	for _, available := range []int{180, 260, 400, 740} {
		for _, tallest := range []int{1, 2, 8, 9, 20, 39} {
			rowH := brainRowHeight(available, tallest, 0, face)
			if rowH < lineH {
				t.Errorf("%dpx over %d rows gives a %dpx pitch, under the %dpx a label needs",
					available, tallest, rowH, lineH)
			}
			if tallest*rowH > available && rowH > lineH {
				t.Errorf("%dpx over %d rows gives %dpx of diagram, past the height given",
					available, tallest, tallest*rowH)
			}
			// Nothing is left over: another pixel of pitch would not fit.
			if rowH > lineH && tallest*(rowH+1) <= available {
				t.Errorf("%dpx over %d rows left %dpx unused at a %dpx pitch",
					available, tallest, available-tallest*rowH, rowH)
			}
		}
	}
}

// TestTheThinnestConnectionCoversAPixel: the strokes are not antialiased, so
// a width under a pixel is not a faint line but a missing one — measured with
// HAIRLINE_PROBE=1, a 0.5px horizontal line at a whole-numbered y writes 0
// pixels where the same line at 1.0px writes 56. That is the line a one-unit
// layer feeding another one-unit layer draws, so a small net lost the whole
// middle of its diagram while a grown one, which is mostly diagonals, did not.
func TestTheThinnestConnectionCoversAPixel(t *testing.T) {
	if brainMinEdgeW < 1 {
		t.Errorf("the thinnest connection is %vpx; under a pixel it can rasterise to nothing",
			brainMinEdgeW)
	}
	if brainMaxEdgeW <= brainMinEdgeW {
		t.Errorf("connections run %v to %vpx, which is no range at all",
			brainMinEdgeW, brainMaxEdgeW)
	}
}

// TestThePitchStaysInsideTheLayerGap: height alone is the wrong thing to
// maximise. The panel leaves 76px for the diagram, so a net handed a screen of
// height there is drawn as a vertical ribbon with every connection running
// steeper than it can be followed. The pitch is held to the distance between
// the layers, which only bites where the diagram is narrow.
func TestThePitchStaysInsideTheLayerGap(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	face := r.FontSourceCodePro8

	// A tall screen over the panel's narrow column: the gap decides.
	if got := brainRowHeight(740, 9, 25, face); got != 25 {
		t.Errorf("a 740px screen over 9 rows gave a %dpx pitch against a 25px layer gap", got)
	}
	// The same column in a box short enough that the height decides.
	if got := brainRowHeight(180, 9, 25, face); got != 20 {
		t.Errorf("a 180px box over 9 rows gave a %dpx pitch, want 20", got)
	}
	// Wide enough and the gap never applies, which is the expanded view.
	if got := brainRowHeight(500, 9, 266, face); got != 55 {
		t.Errorf("a 500px box over 9 rows gave a %dpx pitch, want 55", got)
	}
	// The floor still wins over both.
	lineH := face.Metrics().Height.Round()
	if got := brainRowHeight(740, 9, 2, face); got != lineH {
		t.Errorf("a 2px layer gap gave a %dpx pitch, want the %dpx label height", got, lineH)
	}
}

// TestTheLayerGapIsTheDistanceBetweenColumns pins what the cap is measured
// against, since a net with one column has no connections to keep in
// proportion and must not be capped by a division by zero.
func TestTheLayerGapIsTheDistanceBetweenColumns(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	lay := panelBrainColumns()

	starter := neural.Starter(d.ActChemosynthesis)
	if got := brainLayerGap(starter, lay, infoAreaWidth); got <= 0 {
		t.Errorf("a founder net reports a %dpx layer gap; it has two columns", got)
	}
	grown := grownNet(t, 600)
	cols := 1 + len(grown.LayerSizes())
	want := lay.diagramW(infoAreaWidth) / (cols - 1)
	if got := brainLayerGap(grown, lay, infoAreaWidth); got != want {
		t.Errorf("a %d-column net reports a %dpx layer gap, want %dpx", cols, got, want)
	}
}

// TestEveryLayerTakesTheSamePitch: the layers are drawn side by side, so a
// short one spread to its own pitch would line up with the long one nowhere.
// It is centred against it instead, at the pitch the longest column set.
func TestEveryLayerTakesTheSamePitch(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	const tallest, available = 9, 400
	rowH := brainRowHeight(available, tallest, 0, r.FontSourceCodePro8)
	height := tallest * rowH

	long := spreadColumn(0, height, tallest, rowH)
	for _, short := range []int{1, 2, 8} {
		col := spreadColumn(0, height, short, rowH)
		for i := 1; i < len(col); i++ {
			if gap := col[i] - col[i-1]; gap != float64(rowH) {
				t.Errorf("a %d-unit column is spaced %vpx against the %dpx of the longest",
					short, gap, rowH)
			}
		}
		longMid := (long[0] + long[len(long)-1]) / 2
		shortMid := (col[0] + col[len(col)-1]) / 2
		if diff := longMid - shortMid; diff > 0.01 || diff < -0.01 {
			t.Errorf("a %d-unit column centres at %v against the longest at %v",
				short, shortMid, longMid)
		}
	}
}

// TestTheConnectionsThickenWithThePitch: the widths were chosen at a 22px
// pitch, so a diagram drawn at four times that would be read through the same
// hairlines — and one drawn at ten times would draw its weakest connection as
// a bar, which is what the cap is for.
func TestTheConnectionsThickenWithThePitch(t *testing.T) {
	if got := brainEdgeScale(brainEdgeRefRowH); got != 1 {
		t.Errorf("at the reference pitch the widths scale by %v, want 1", got)
	}
	if got := brainEdgeScale(brainEdgeRefRowH / 2); got != 1 {
		t.Errorf("under the reference pitch the widths scale by %v, want 1", got)
	}
	if got := brainEdgeScale(2 * brainEdgeRefRowH); got <= 1 {
		t.Errorf("at twice the reference pitch the widths scale by %v, want more than 1", got)
	}
	if got := brainEdgeScale(100 * brainEdgeRefRowH); got != brainMaxEdgeScale {
		t.Errorf("at a huge pitch the widths scale by %v, want the %v cap", got, brainMaxEdgeScale)
	}
}

// TestThePanelGivesTheDiagramWhatIsLeftOfTheScreen: the selected organism is
// the last section in the panel, so the space under the diagram is free. A
// window too short for that still gets a diagram worth drawing, reached by
// scrolling.
func TestThePanelGivesTheDiagramWhatIsLeftOfTheScreen(t *testing.T) {
	loadKeyGlobals(t)
	config.GetCurrentGlobals().ScreenHeight = 800

	if got := panelBrainHeight(560); got != 240 {
		t.Errorf("a diagram starting at 560 of an 800px screen got %dpx, want 240", got)
	}
	if got := panelBrainHeight(780); got != brainMinPanelHeight {
		t.Errorf("a diagram starting near the bottom got %dpx, want the %dpx floor",
			got, brainMinPanelHeight)
	}
	if got := panelBrainHeight(2000); got != brainMinPanelHeight {
		t.Errorf("a diagram scrolled past the screen got %dpx, want the %dpx floor",
			got, brainMinPanelHeight)
	}
}
