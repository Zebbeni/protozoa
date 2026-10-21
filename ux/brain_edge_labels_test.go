package ux

import (
	"image"
	"math"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	r "github.com/Zebbeni/protozoa/resources"
)

// labelRects is where one layer's weight labels land, as the boxes their text
// occupies. It is the arithmetic drawBrainEdgeLabels draws with, so a test
// against these is a test of what is on screen.
func labelRects(lay brainLayout, srcN, rowH, gap, widest int, srcYs, dstYs []float64) []image.Rectangle {
	lineH := lay.face.Metrics().Height.Round()
	var out []image.Rectangle
	for i, py := range srcYs {
		t := brainEdgeLabelFraction(i, srcN, rowH, gap, widest, lay.face)
		lx := t * float64(gap)
		for _, cy := range dstYs {
			ly := py + t*(cy-py)
			out = append(out, image.Rect(
				int(lx)-widest/2, int(ly)-lineH/2,
				int(lx)+widest/2, int(ly)+lineH/2))
		}
	}
	return out
}

// TestWeightLabelsNeverOverlapWhereTheyAreDrawn is the whole point of the
// fraction rule. Lines cross in the middle of the diagram, so a number centred
// on each line would land on the numbers of every line crossing it; labelling
// each node's lines at one fraction of their length spreads them by the fan
// instead, and brainEdgeLabelsFit refuses a layer too tight for that to work.
// Wherever it says yes, no two numbers may touch.
func TestWeightLabelsNeverOverlapWhereTheyAreDrawn(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	lay := brainLayout{face: r.FontSourceCodePro8}
	widest := textAdvance(lay.face, "-0.09")
	checked := 0
	for _, rowH := range []int{11, 16, 22, 25, 40, 53, 120, 300} {
		for _, gap := range []int{25, 60, 120, 240, 400, 800} {
			for _, srcN := range []int{1, 2, 3, 4, 8, 12} {
				for _, dstN := range []int{1, 2, 4, 8} {
					if !brainEdgeLabelsFit(srcN, rowH, gap, widest, lay.face) {
						continue
					}
					checked++
					height := max(srcN, dstN) * rowH
					src := spreadColumn(0, height, srcN, rowH)
					dst := spreadColumn(0, height, dstN, rowH)
					rects := labelRects(lay, srcN, rowH, gap, widest, src, dst)
					for a := range rects {
						for b := a + 1; b < len(rects); b++ {
							if rects[a].Overlaps(rects[b]) {
								t.Errorf("pitch %d gap %d, %d nodes into %d: labels %v and %v overlap",
									rowH, gap, srcN, dstN, rects[a], rects[b])
							}
						}
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no layout in the sweep was labelled at all, so nothing was checked")
	}
	t.Logf("%d labelled layouts checked", checked)
}

// TestAWeightLabelClearsItsOwnNodeColumn: the band's margin is bounded by half
// a number's width as well as by the row pitch, or the nearest label would be
// drawn over the dots it belongs to.
func TestAWeightLabelClearsItsOwnNodeColumn(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	face := r.FontSourceCodePro8
	widest := textAdvance(face, "-0.09")
	for _, rowH := range []int{16, 53, 300} {
		for _, gap := range []int{60, 240, 800} {
			for _, n := range []int{1, 3, 8} {
				if !brainEdgeLabelsFit(n, rowH, gap, widest, face) {
					continue
				}
				lo, hi := brainEdgeLabelBand(rowH, gap, widest, face)
				if left := lo*float64(gap) - float64(widest)/2; left < brainNodeRadius {
					t.Errorf("pitch %d gap %d: the first label starts %.1fpx from its node", rowH, gap, left)
				}
				if right := hi*float64(gap) + float64(widest)/2; right > float64(gap)-brainNodeRadius {
					t.Errorf("pitch %d gap %d: the last label ends %.1fpx past the next column",
						rowH, gap, right-float64(gap))
				}
			}
		}
	}
}

// TestTheFractionsSpreadEvenlyAcrossTheBand states the rule as it reads: in a
// layer of three the three nodes label at a quarter, a half and three quarters
// of the band, so a node's numbers are always found at one distance from it and
// the middle node of an odd layer labels at the middle of its lines.
func TestTheFractionsSpreadEvenlyAcrossTheBand(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	face := r.FontSourceCodePro8
	const rowH, gap = 53, 300
	widest := textAdvance(face, "-0.09")
	lo, hi := brainEdgeLabelBand(rowH, gap, widest, face)

	const n = 3
	var got []float64
	for i := 0; i < n; i++ {
		got = append(got, brainEdgeLabelFraction(i, n, rowH, gap, widest, face))
	}
	for i, share := range []float64{0.25, 0.5, 0.75} {
		want := lo + (hi-lo)*share
		if diff := got[i] - want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("node %d of %d labels at %.4f of its line, not the %.4f that is %.2f of the band",
				i, n, got[i], want, share)
		}
	}
	// The band is symmetric about the middle of the line, so the middle node
	// of an odd layer labels there — and so does the only node of a layer of
	// one, which is the case with no fan to spread.
	if got[1] < 0.499 || got[1] > 0.501 {
		t.Errorf("the middle node of three labels at %.4f, not the middle of its lines", got[1])
	}
	if mid := brainEdgeLabelFraction(0, 1, rowH, gap, widest, face); mid < 0.499 || mid > 0.501 {
		t.Errorf("the only node of a one-node layer labels at %.4f, not the middle of its lines", mid)
	}
}

// TestThePanelLeavesNoRoomForWeightLabels is the measurement behind leaving
// them out of the tab. The panel's diagram is 76px wide between the label
// columns, so two layers stand a fraction of a number apart and a label drawn
// there would cover both the column it came from and the one it points at.
func TestThePanelLeavesNoRoomForWeightLabels(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	net := grownNet(t, 400)
	width := infoAreaWidth
	lay := panelBrainLayout(net, width, panelBrainHeight(120))
	gap := brainLayerGap(net, lay, width)
	face := brainEdgeFace()
	slot := brainEdgeLabelW(face)
	cols := append([]int{len(net.Inputs())}, net.LayerSizes()...)
	if brainDiagramLabelsFit(brainShown{}, cols, lay.rowH, gap, slot, face) {
		t.Errorf("the panel's %dpx diagram claims to hold %dpx numbers between layers %dpx apart",
			lay.diagramW(width), slot, gap)
	}
	t.Logf("panel: %dpx diagram, %dpx between layers, %dpx pitch, %dpx numbers",
		lay.diagramW(width), gap, lay.rowH, slot)
}

// TestASmallNetIsLabelledInTheExpandedView: the expanded view is the half of
// this the panel cannot do, so a net small enough to read there has to actually
// get its numbers.
func TestASmallNetIsLabelledInTheExpandedView(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	config.GetCurrentGlobals().ScreenWidth = 1400
	config.GetCurrentGlobals().ScreenHeight = 900

	// A few generations of drift, since every connection a net is given
	// starts at exactly 0 and a weight of 0 is not drawn at all.
	net := grownNet(t, 6)
	net.Choose(func(d.Condition) bool { return true })

	dr := NewBrainPopup(&Panel{}).diagramRect()
	lay := expandedBrainLayout(net, dr.Dx(), dr.Dy())
	gap := brainLayerGap(net, lay, dr.Dx())
	face := brainEdgeFace()
	widest := brainEdgeLabelW(face)
	cols := append([]int{len(net.Inputs())}, net.LayerSizes()...)
	if !brainDiagramLabelsFit(brainShown{}, cols, lay.rowH, gap, widest, face) {
		t.Errorf("a %d-node net in a %dx%d box is left unlabelled: %dpx between layers, %dpx pitch, %dpx numbers",
			net.Nodes(), dr.Dx(), dr.Dy(), gap, lay.rowH, widest)
	}
	t.Logf("expanded: %dpx between layers, %dpx pitch, %dpx numbers", gap, lay.rowH, widest)
}

// TestTheEdgeNumbersFaceIsFixedAndNoBiggerThanTheNames: the numbers used to be
// pinned to the smallest face, and the size is a trade rather than a rule —
// what has to hold is that it does not follow the layout, since then what a
// net was annotated with would change with the size its node labels landed
// on, and that it never outgrows the names beside the nodes. An annotation set
// larger than the thing it annotates reads as the heading.
func TestTheEdgeNumbersFaceIsFixedAndNoBiggerThanTheNames(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	config.GetCurrentGlobals().ScreenWidth = 1400
	config.GetCurrentGlobals().ScreenHeight = 900

	net := grownNet(t, 6)
	dr := NewBrainPopup(&Panel{}).diagramRect()
	edge := brainEdgeFace()

	layouts := map[string]brainLayout{
		"panel":    panelBrainLayout(net, infoAreaWidth, panelBrainHeight(120)),
		"expanded": expandedBrainLayout(net, dr.Dx(), dr.Dy()),
	}
	for where, lay := range layouts {
		if lay.face == nil {
			t.Fatalf("%s: the layout has no face", where)
		}
		if got := brainEdgeFace(); got != edge {
			t.Errorf("%s: the numbers are set in a different face from the other layout", where)
		}
	}
	// The second half is asked of the expanded view alone, because that is
	// the only place a number is drawn: the panel's own face is smaller than
	// this one and its 73px diagram annotates nothing at any net size, which
	// TestThePanelLeavesTheNumbersToTheExpandedView pins separately.
	expanded := layouts["expanded"]
	if textAdvance(edge, brainEdgeSample) > textAdvance(expanded.face, brainEdgeSample) {
		t.Errorf("the numbers are %dpx against the %dpx node names beside them",
			textAdvance(edge, brainEdgeSample), textAdvance(expanded.face, brainEdgeSample))
	}
	t.Logf("numbers %dpx a line, %dpx for %q", edge.Metrics().Height.Round(),
		textAdvance(edge, brainEdgeSample), brainEdgeSample)
}

// TestTheLabelSlotDoesNotMoveWithTheCycle: in SIGNAL the numbers are what is
// flowing right now, so laying the labels out against them would walk the
// whole annotation sideways as the organism acts, and flip a marginal net
// between labelled and not from one frame to the next. The slot is a fixed
// figure instead, and the same one in both views, so switching view changes
// the numbers and never which lines carry one.
func TestTheLabelSlotDoesNotMoveWithTheCycle(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	face := r.FontSourceCodePro8
	net := grownNet(t, 400)
	want := brainEdgeLabelW(face)
	if want <= 0 {
		t.Fatal("a number is given no room at all")
	}
	net.Choose(func(d.Condition) bool { return true })
	if got := brainEdgeLabelW(face); got != want {
		t.Errorf("the slot was %dpx and is %dpx after the net ran", want, got)
	}
	net.Choose(func(d.Condition) bool { return false })
	if got := brainEdgeLabelW(face); got != want {
		t.Errorf("the slot was %dpx and is %dpx after a different cycle", want, got)
	}
	// The slot holds the ordinary numbers — two decimals at a magnitude under
	// ten, which is where essentially every weight and flow sits. A value
	// outside that overhangs rather than being cut, which is the right way
	// round for a number, and the overhang has to stay small enough to read
	// against the line beside it.
	digit := textAdvance(face, "0")
	overhung := 0
	for _, view := range allBrainViews {
		for layer := range net.LayerSizes() {
			for j := 0; j < net.LayerSizes()[layer]; j++ {
				for i := 0; i < brainFeedingUnits(net, layer); i++ {
					value := brainEdgeValue(net, view, layer, j, i)
					label := brainEdgeNumber(value)
					got := textAdvance(face, label)
					if got <= want {
						continue
					}
					overhung++
					if m := math.Abs(value); m >= 0.005 && m < 10 {
						t.Errorf("%q is an ordinary value and measures %dpx in the %dpx slot", label, got, want)
					}
					if got > want+2*digit {
						t.Errorf("%q overhangs its %dpx slot by %dpx", label, want, got-want)
					}
				}
			}
		}
	}
	t.Logf("%d of this net's numbers overhang the %dpx slot", overhung, want)
}

// TestTheNumbersFollowTheView is the bug this replaced: the lines changed with
// the buttons and the numbers on them did not, so SIGNAL was drawn at what was
// flowing and annotated with what could flow.
func TestTheNumbersFollowTheView(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	net := grownNet(t, 400)
	net.Choose(func(d.Condition) bool { return true })

	differed := 0
	for layer := range net.LayerSizes() {
		for j := 0; j < net.LayerSizes()[layer]; j++ {
			for i := 0; i < brainFeedingUnits(net, layer); i++ {
				w := net.WeightAt(layer, j, i)
				if w == 0 {
					continue
				}
				if got, want := brainEdgeValue(net, brainViewWeights, layer, j, i), w; got != want {
					t.Fatalf("WEIGHTS labels %v where the weight is %v", got, want)
				}
				signal := brainEdgeValue(net, brainViewSignal, layer, j, i)
				if want := net.Transmitted(layer, j, i); signal != want {
					t.Fatalf("SIGNAL labels %v where %v is flowing", signal, want)
				}
				if signal != w {
					differed++
				}
			}
		}
	}
	if differed == 0 {
		t.Error("every connection carries exactly its weight, so the two views would read alike")
	}
}

// TestOnlyATrueZeroIsLabelledZero: 0.00 is a real answer on this diagram — a
// connection the lineage has made nothing of, or one carrying nothing this
// cycle — so it has to keep meaning that. A weight of 0.001 printing as 0.00
// would be claiming to be the first of those over a line drawn as the second.
func TestOnlyATrueZeroIsLabelledZero(t *testing.T) {
	for _, w := range []float64{
		0.004, -0.004, 0.0001, -0.0001, 1e-7, -1e-7, 0.005, -0.005, 0.009,
	} {
		label := brainEdgeNumber(w)
		if label == "0.00" || label == "-0.00" || label == "0" {
			t.Errorf("a weight of %v is written %q, which reads as no connection", w, label)
		}
	}
	// Everything a lineage normally holds still reads as a plain two-decimal
	// number, which is what makes two weights comparable at a glance.
	for w, want := range map[float64]string{
		0.08: "0.08", -0.37: "-0.37", 1.5: "1.50", -12.345: "-12.35",
	} {
		if got := brainEdgeNumber(w); got != want {
			t.Errorf("a weight of %v is written %q, not %q", w, got, want)
		}
	}
	// Exactly zero reads plainly, and so does the NEGATIVE zero a negative
	// weight carrying nothing produces — which is most of what SIGNAL shows
	// on a cycle where a unit is clipped. A minus in front of nothing is a
	// direction the quantity does not have.
	for _, zero := range []float64{0, math.Copysign(0, -1)} {
		if got := brainEdgeNumber(zero); got != "0.00" {
			t.Errorf("a value of %v is written %q", zero, got)
		}
	}
}
