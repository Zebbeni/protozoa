package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	r "github.com/Zebbeni/protozoa/resources"
)

// TestEveryBrainViewHasItsOwnLabel: the two buttons are the only thing on
// screen saying which quantity the lines are drawn at, so a view added without
// one would draw an empty button and leave the diagram meaning either thing.
func TestEveryBrainViewHasItsOwnLabel(t *testing.T) {
	seen := map[string]brainView{}
	for _, view := range allBrainViews {
		label, ok := brainViewLabels[view]
		if !ok || label == "" {
			t.Errorf("view %d has no label", view)
			continue
		}
		if other, clash := seen[label]; clash {
			t.Errorf("%q labels both view %d and view %d", label, other, view)
		}
		seen[label] = view
	}
	if len(allBrainViews) != len(brainViewLabels) {
		t.Errorf("%d views are drawn as buttons against %d labels",
			len(allBrainViews), len(brainViewLabels))
	}
	// The default is the view that answers "why this action", which is the
	// question the tab was built for.
	if (&Panel{}).brainView != brainViewSignal {
		t.Error("the tab opens on a view other than SIGNAL")
	}
}

// TestTheWeightsViewDoesNotMoveWithTheCycle is the whole difference between
// the two. What a connection carries depends on what the organism is looking
// at; what it weighs does not, so a lineage's strongest connection stays
// visible on a cycle when the unit feeding it reads zero.
func TestTheWeightsViewDoesNotMoveWithTheCycle(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	net := grownNet(t, 400)
	read := func(view brainView) []float64 {
		var out []float64
		for layer := range net.LayerSizes() {
			for j := 0; j < net.LayerSizes()[layer]; j++ {
				for i := 0; i < brainFeedingUnits(net, layer); i++ {
					out = append(out, brainEdgeValue(net, view, layer, j, i))
				}
			}
		}
		return append(out, brainEdgeReference(net, view))
	}

	net.Choose(func(d.Condition) bool { return true })
	weightsAllTrue, signalAllTrue := read(brainViewWeights), read(brainViewSignal)
	net.Choose(func(d.Condition) bool { return false })
	weightsAllFalse, signalAllFalse := read(brainViewWeights), read(brainViewSignal)

	for i := range weightsAllTrue {
		if weightsAllTrue[i] != weightsAllFalse[i] {
			t.Fatalf("the weights view moved with the cycle: %v became %v",
				weightsAllTrue[i], weightsAllFalse[i])
		}
	}
	moved := false
	for i := range signalAllTrue {
		if signalAllTrue[i] != signalAllFalse[i] {
			moved = true
			break
		}
	}
	if !moved {
		t.Error("the signal view did not move when every condition flipped, so the two views are the same picture")
	}
}

// TestEachViewMeasuresAgainstItsOwnQuantity: a line's width is its share of
// the biggest thing in the picture it is part of, and the two pictures have
// different biggest things.
func TestEachViewMeasuresAgainstItsOwnQuantity(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	net := grownNet(t, 400)
	net.Choose(func(d.Condition) bool { return true })

	if got, want := brainEdgeReference(net, brainViewWeights), net.StrongestWeight(); got != want {
		t.Errorf("the weights view scales against %v, not the strongest weight %v", got, want)
	}
	if got, want := brainEdgeReference(net, brainViewSignal), net.StrongestTransmission(); got != want {
		t.Errorf("the signal view scales against %v, not the strongest transmission %v", got, want)
	}
	// Scaling one against the other would draw a net reading nothing as blank.
	if net.StrongestWeight() <= 0 {
		t.Fatal("a grown net has no weight to scale against")
	}
}

// TestAViewButtonSwitchesTheView, from either place it is drawn: the buttons
// write straight back to the panel, so picking WEIGHTS in the expanded view
// leaves the tab behind it on WEIGHTS too.
func TestAViewButtonSwitchesTheView(t *testing.T) {
	p := &Panel{}
	controls := brainControlRects(40, 200, infoAreaWidth, true)

	for _, rect := range controls.views {
		p.brainView = brainViewSignal
		if !p.takeBrainControlClick(controls, rect.x+1, rect.y+1) {
			t.Errorf("a click on the %q button was not consumed", brainViewLabels[rect.view])
			continue
		}
		if p.brainView != rect.view {
			t.Errorf("clicking %q left the view on %d", brainViewLabels[rect.view], p.brainView)
		}
	}
	if p.takeBrainControlClick(controls, 40, 190) {
		t.Error("a click above the buttons was consumed")
	}
	// A view showing no net draws no buttons, so nothing is left clickable.
	if p.takeBrainControlClick(brainControls{}, 41, 201) {
		t.Error("a button was clickable after the view stopped drawing one")
	}
}

// TestTheNumbersToggleShowsAndHidesTheNumbers: the whole point of the button
// is that it goes both ways, and the diagram starts annotated — a Panel that
// had to be built through a constructor to draw its numbers would open blank
// everywhere one is made directly.
func TestTheNumbersToggleShowsAndHidesTheNumbers(t *testing.T) {
	p := &Panel{}
	if !p.brainNumbersShown() {
		t.Fatal("a fresh panel opens with the numbers hidden")
	}
	controls := brainControlRects(40, 200, infoAreaWidth, true)
	if controls.numbers == nil {
		t.Fatal("a row asked for the numbers toggle and did not get one")
	}
	at := [2]int{controls.numbers.x + 1, controls.numbers.y + 1}

	if !p.takeBrainControlClick(controls, at[0], at[1]) {
		t.Fatal("a click on the numbers toggle was not consumed")
	}
	if p.brainNumbersShown() {
		t.Error("the numbers are still shown after the toggle was pressed")
	}
	p.takeBrainControlClick(controls, at[0], at[1])
	if !p.brainNumbersShown() {
		t.Error("pressing the toggle again did not bring the numbers back")
	}
	// It must not reach the view, which shares the row with it.
	if p.brainView != brainViewSignal {
		t.Errorf("the numbers toggle moved the view to %d", p.brainView)
	}
}

// TestOnlyTheExpandedViewOffersTheNumbersToggle: the tab's diagram labels
// nothing at any net size, so a button there is one a reader can press and
// not see act. A nil rect is also what keeps a click from matching a button
// nobody drew.
func TestOnlyTheExpandedViewOffersTheNumbersToggle(t *testing.T) {
	tab := brainControlRects(40, 200, infoAreaWidth, false)
	if tab.numbers != nil {
		t.Error("the tab's row holds a numbers toggle")
	}
	if len(tab.views) != len(allBrainViews) {
		t.Errorf("the tab's row holds %d view buttons against %d views", len(tab.views), len(allBrainViews))
	}
	// The tab's row stops after its views: a click where a third button would
	// have gone must not be taken. Measured from the tab's own geometry, not
	// the expanded view's — the two share a width among a different number of
	// buttons, so the expanded toggle's x lands inside a real tab button.
	last := tab.views[len(tab.views)-1]
	p := &Panel{}
	if p.takeBrainControlClick(tab, last.x+last.w+brainLabelGap+1, last.y+1) {
		t.Error("a click landed on a numbers toggle the tab never drew")
	}
	if !p.brainNumbersShown() {
		t.Error("that click hid the numbers anyway")
	}
}

// TestHidingTheNumbersLeavesNothingToDraw is the toggle acting on the diagram
// rather than only on the button: the fit rule is what decides whether a net
// is annotated, and the toggle has to come before it.
func TestHidingTheNumbersLeavesNothingToDraw(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	config.GetCurrentGlobals().ScreenWidth = 1400
	config.GetCurrentGlobals().ScreenHeight = 900

	net := grownNet(t, 6)
	net.Choose(func(d.Condition) bool { return true })
	dr := NewBrainPopup(&Panel{}).diagramRect()
	lay := expandedBrainLayout(net, dr.Dx(), dr.Dy())
	gap := brainLayerGap(net, lay, dr.Dx())
	face := brainEdgeFace()
	slot := brainEdgeLabelW(face)
	cols := append([]int{len(net.Inputs())}, net.LayerSizes()...)

	// This net has the room, so the only thing that can stop it being
	// annotated is the toggle.
	if !brainDiagramLabelled(brainShown{}, cols, lay.rowH, gap, slot, face) {
		t.Skip("this net has no room for its numbers, so the toggle is not what is being measured")
	}
	if brainDiagramLabelled(brainShown{numbersHidden: true}, cols, lay.rowH, gap, slot, face) {
		t.Error("a diagram with the numbers switched off still reports that it holds them")
	}
	// The room is unchanged by the toggle: what the reader switched off is the
	// annotation, not the space for it.
	if !brainDiagramLabelsFit(brainShown{numbersHidden: true}, cols, lay.rowH, gap, slot, face) {
		t.Error("switching the numbers off changed whether they would fit")
	}
}

// TestTheViewButtonsHoldTheirLabels measures each label against the button it
// is centred in, at both widths the buttons are drawn at — the panel's content
// column and the expanded view's.
func TestTheViewButtonsHoldTheirLabels(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	config.GetCurrentGlobals().ScreenWidth = 1400
	config.GetCurrentGlobals().ScreenHeight = 900

	widths := map[string]int{
		"panel":    infoAreaWidth,
		"expanded": NewBrainPopup(&Panel{}).diagramRect().Dx(),
	}
	for where, width := range widths {
		controls := brainControlRects(0, 0, width, true)
		rects := controls.views
		labels := map[*graphRect]string{}
		for i := range rects {
			labels[&rects[i].graphRect] = brainViewLabels[rects[i].view]
		}
		labels[controls.numbers] = brainNumbersLabel
		for rect, label := range labels {
			if got := textAdvance(r.FontSourceCodePro8, label); got > rect.w-2*brainLabelGap {
				t.Errorf("%s: %q measures %dpx in a %dpx button", where, label, got, rect.w)
			}
		}
		// The whole row side by side, inside the width it was given.
		row := append([]graphRect{}, rects[0].graphRect, rects[1].graphRect, *controls.numbers)
		for i := 1; i < len(row); i++ {
			if row[i-1].x+row[i-1].w >= row[i].x {
				t.Errorf("%s: buttons %d and %d overlap", where, i-1, i)
			}
		}
		if last := row[len(row)-1]; last.x+last.w > width {
			t.Errorf("%s: the buttons run %dpx past the %dpx they were given",
				where, last.x+last.w-width, width)
		}
	}
}

// TestTheExpandedViewLeavesRoomForItsButtons: the buttons sit between the
// title and the diagram, so the diagram has to start below both — drawn over
// each other they would be unreadable and unclickable at once.
func TestTheExpandedViewLeavesRoomForItsButtons(t *testing.T) {
	loadKeyGlobals(t)
	b := NewBrainPopup(&Panel{})

	for _, size := range [][2]int{{1400, 800}, {1920, 1057}, {900, 600}} {
		config.GetCurrentGlobals().ScreenWidth = size[0]
		config.GetCurrentGlobals().ScreenHeight = size[1]

		outer, dr := b.rect(), b.diagramRect()
		buttonsY := outer.Min.Y + brainPopupTitleH
		if dr.Min.Y < buttonsY+graphButtonRowHeight {
			t.Errorf("at %dx%d the diagram starts at %d, over buttons drawn at %d",
				size[0], size[1], dr.Min.Y, buttonsY)
		}
		if dr.Max.Y > outer.Max.Y {
			t.Errorf("at %dx%d the diagram runs past the popup", size[0], size[1])
		}
	}
}
