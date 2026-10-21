package ux

import (
	"golang.org/x/image/font"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/neural"
	r "github.com/Zebbeni/protozoa/resources"
)

// TestWhatANodeTouchesIsOneStep is the rule the whole feature rests on.
// Transitively every input reaches every output — between two layers every
// unit is wired to every unit — so "connected to this node" down the whole
// path is the entire diagram, and picking a node out would light all of it.
// One step is also the question a click is asking.
func TestWhatANodeTouchesIsOneStep(t *testing.T) {
	n := brainNode{col: 2, row: 3}

	if !n.touches(2, 3) {
		t.Error("a node does not touch itself")
	}
	if n.touches(2, 4) {
		t.Error("a node touches its own neighbour in the same column, which nothing connects it to")
	}
	for _, col := range []int{1, 3} {
		for _, row := range []int{0, 3, 7} {
			if !n.touches(col, row) {
				t.Errorf("column %d row %d is one step away and is not touched", col, row)
			}
		}
	}
	for _, col := range []int{0, 4, 5} {
		if n.touches(col, 3) {
			t.Errorf("column %d is %d steps away and is touched", col, col-2)
		}
	}
}

// TestNothingPickedOutLightsEverything: the ordinary diagram is the same code
// path with no focus, so it has to light every node and every connection.
func TestNothingPickedOutLightsEverything(t *testing.T) {
	var shown brainShown
	for col := 0; col < 4; col++ {
		for row := 0; row < 6; row++ {
			if !shown.lit(col, row) {
				t.Fatalf("node %d,%d is faded with nothing picked out", col, row)
			}
		}
	}
	for c := 0; c < 3; c++ {
		for i := 0; i < 6; i++ {
			for j := 0; j < 6; j++ {
				if !shown.edgeLit(c, i, j) {
					t.Fatalf("the connection %d:%d->%d is faded with nothing picked out", c, i, j)
				}
			}
		}
	}
}

// TestOnlyThePickedNodesOwnConnectionsAreLit: a connection needs an END at the
// picked-out node, not merely two ends that are each one step from it. Two
// nodes either side of it are both lit and the connection between them is not
// its business — without that, picking out a hidden unit would light every
// connection in the two layers around it.
func TestOnlyThePickedNodesOwnConnectionsAreLit(t *testing.T) {
	focus := brainNode{col: 1, row: 2}
	shown := brainShown{focus: &focus}

	if !shown.edgeLit(0, 4, 2) {
		t.Error("a connection arriving at the picked-out node is faded")
	}
	if !shown.edgeLit(1, 2, 5) {
		t.Error("a connection leaving the picked-out node is faded")
	}
	if shown.edgeLit(0, 4, 3) {
		t.Error("a connection into a NEIGHBOUR of the picked-out node is lit")
	}
	// Both ends of this one are lit nodes — column 0 and column 2 are each
	// one step away — and it is still not the focused node's connection.
	if shown.edgeLit(1, 3, 5) {
		t.Error("a connection between two lit nodes, touching neither end of the focus, is lit")
	}
}

// TestADiagramIsAnnotatedOrItIsNot: the decision used to be taken per layer,
// on the size of the column feeding it, so a net could carry numbers between
// two of its layers and not the third. That reads as a fault rather than as a
// limit, and it is what a user reported — "numerical labels on the hidden
// lines" only.
func TestADiagramIsAnnotatedOrItIsNot(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	face := r.FontSourceCodePro8
	slot := brainEdgeLabelW(face)

	// A net whose layers are fed by very different numbers of nodes is where
	// a per-layer decision came apart: 1 node feeding the first layer and 9
	// feeding the third.
	cols := []int{1, 3, 9, 3}
	for _, rowH := range []int{16, 25, 53, 120} {
		for _, gap := range []int{25, 120, 290, 710} {
			whole := brainDiagramLabelsFit(brainShown{}, cols, rowH, gap, slot, face)
			for c := 1; c < len(cols); c++ {
				perLayer := brainEdgeLabelsFit(cols[c-1], rowH, gap, slot, face)
				if whole && !perLayer {
					t.Errorf("pitch %d gap %d: the diagram is annotated but layer %d (fed by %d nodes) cannot hold its numbers",
						rowH, gap, c, cols[c-1])
				}
			}
		}
	}
	// And it is not simply refusing everything: a diagram with room says yes.
	if !brainDiagramLabelsFit(brainShown{}, cols, 53, 710, slot, face) {
		t.Error("a diagram with 710px between its layers is left unannotated")
	}
}

// TestAPickedOutNodesFanIsAlwaysAnnotated: its numbers all sit at one fraction
// of their lines, so they stack in a single column spread by the row pitch
// with nothing competing for the room. That is what lets a net far too dense
// to annotate whole be read a node at a time.
func TestAPickedOutNodesFanIsAlwaysAnnotated(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	face := r.FontSourceCodePro8
	slot := brainEdgeLabelW(face)
	cols := []int{9, 8, 8, 7}
	focus := brainNode{col: 2, row: 3}
	shown := brainShown{focus: &focus}

	const rowH, gap = 53, 290
	if brainDiagramLabelsFit(brainShown{}, cols, rowH, gap, slot, face) {
		t.Fatal("this net is annotated whole, so picking a node out proves nothing")
	}
	if !brainDiagramLabelsFit(shown, cols, rowH, gap, slot, face) {
		t.Error("a picked-out node in a net too dense to annotate carries no numbers either")
	}

	// The numbers sit in the MIDDLE of their lines, which is only legible
	// because the band says so: a fan's numbers stand t x rowH apart a
	// fraction t from the point they share.
	lo, hi := brainEdgeLabelBand(rowH, gap, slot, face)
	if lo > brainFocusLabelFraction || hi < brainFocusLabelFraction {
		t.Errorf("the middle is outside the band %.3f-%.3f, so these numbers are drawn where they do not fit", lo, hi)
	}
	if spread := brainFocusLabelFraction * rowH; spread < float64(face.Metrics().Height.Round()) {
		t.Errorf("a fan's numbers stand %.1fpx apart in the middle of their lines, under the %dpx a line of text needs",
			spread, face.Metrics().Height.Round())
	}
}

// TestTheMiddleOfALineIsAlwaysInsideTheBand is what lets a picked-out node's
// numbers be drawn in one place rather than at whichever end of the line is
// furthest from it. The band is symmetric about the middle and a diagram is
// annotated only where it has width (lo < hi), so lo < 0.5 wherever anything
// is drawn at all. Without that the middle would be a choice that holds for
// the layouts it was tried on.
func TestTheMiddleOfALineIsAlwaysInsideTheBand(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	cols := []int{6, 4, 4, 2}
	checked := 0
	for _, face := range []font.Face{r.FontSourceCodePro8, r.FontSourceCodePro10, r.FontSourceCodePro12} {
		slot := brainEdgeLabelW(face)
		for rowH := 4; rowH <= 120; rowH += 2 {
			for gap := 10; gap <= 800; gap += 10 {
				focus := brainNode{col: 1, row: 0}
				if !brainDiagramLabelsFit(brainShown{focus: &focus}, cols, rowH, gap, slot, face) {
					continue
				}
				checked++
				lo, hi := brainEdgeLabelBand(rowH, gap, slot, face)
				if lo > brainFocusLabelFraction || hi < brainFocusLabelFraction {
					t.Fatalf("rowH %d, gap %d: the band is %.3f-%.3f, which does not hold the middle",
						rowH, gap, lo, hi)
				}
			}
		}
	}
	t.Logf("%d annotated layouts checked", checked)
}

// TestEveryDotGetsAClickTarget: the dots are buttons now, so one without a
// target is a control a reader can see and cannot press.
func TestEveryDotGetsAClickTarget(t *testing.T) {
	centres := []float64{10, 60, 110}
	units := [][]float64{{5, 20, 35}, {12, 28}, {20}}

	hits := brainNodeHits(centres, units, 8)
	if len(hits) != 6 {
		t.Fatalf("%d targets for 6 dots", len(hits))
	}
	seen := map[brainNode]bool{}
	for _, hit := range hits {
		if seen[hit.brainNode] {
			t.Errorf("%v has two targets", hit.brainNode)
		}
		seen[hit.brainNode] = true
		if !hit.contains(int(hit.x), int(hit.y)) {
			t.Errorf("%v does not contain its own dot", hit.brainNode)
		}
		if hit.contains(int(hit.x)+9, int(hit.y)) {
			t.Errorf("%v reaches 9px away from a dot with an 8px target", hit.brainNode)
		}
	}
	// The targets are bigger than the dots and must still not swallow each
	// other, or a click would pick out whichever happened to be listed first.
	for a := range hits {
		for b := a + 1; b < len(hits); b++ {
			if hits[a].contains(int(hits[b].x), int(hits[b].y)) {
				t.Errorf("%v's target covers %v's dot", hits[a].brainNode, hits[b].brainNode)
			}
		}
	}
}

// TestClickingADotPicksItOutAndClickingItAgainPutsItBack, and clicking the
// diagram anywhere else puts it back too — a reader should not have to find
// the same few pixels again to undo it.
func TestClickingADotPicksItOutAndClickingItAgainPutsItBack(t *testing.T) {
	loadKeyGlobals(t)
	b := NewBrainPopup(&Panel{})
	b.Open()
	b.nodeHits = brainNodeHits([]float64{100, 300}, [][]float64{{50, 90}, {70}}, 8)

	b.pickNode(100, 50)
	if b.focus == nil || *b.focus != (brainNode{col: 0, row: 0}) {
		t.Fatalf("clicking the first dot picked out %v", b.focus)
	}
	b.pickNode(300, 70)
	if b.focus == nil || *b.focus != (brainNode{col: 1, row: 0}) {
		t.Fatalf("clicking another dot picked out %v", b.focus)
	}
	b.pickNode(300, 70)
	if b.focus != nil {
		t.Errorf("clicking the same dot again left %v picked out", b.focus)
	}

	b.pickNode(100, 90)
	if b.focus == nil {
		t.Fatal("clicking a dot picked out nothing")
	}
	b.pickNode(200, 200)
	if b.focus != nil {
		t.Errorf("clicking away from every dot left %v picked out", b.focus)
	}
}

// TestClosingPutsTheWholeNetBack: the next opening should not still be
// filtered to a node the reader stopped thinking about several organisms ago.
func TestClosingPutsTheWholeNetBack(t *testing.T) {
	loadKeyGlobals(t)
	b := NewBrainPopup(&Panel{})
	b.Open()
	b.nodeHits = brainNodeHits([]float64{100}, [][]float64{{50}}, 8)
	b.pickNode(100, 50)
	if b.focus == nil {
		t.Fatal("nothing was picked out to begin with")
	}
	b.Close()
	if b.focus != nil {
		t.Errorf("closing left %v picked out", b.focus)
	}
}

// TestAPickedNodeThatIsNoLongerThereIsDropped: the panel hands the expanded
// view a different organism whenever the selection moves, and a lineage gains
// and loses units. A focus pointing past the end would match no node at all,
// which draws every node faded with nothing to show why.
func TestAPickedNodeThatIsNoLongerThereIsDropped(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	config.GetCurrentGlobals().ScreenWidth = 1400
	config.GetCurrentGlobals().ScreenHeight = 800

	net := grownNet(t, 400)
	cols := append([]int{len(net.Inputs())}, net.LayerSizes()...)

	for col := range cols {
		for row := 0; row < cols[col]; row++ {
			if !brainHasNode(net, brainNode{col: col, row: row}) {
				t.Errorf("node %d,%d is in the net and reads as gone", col, row)
			}
		}
		if brainHasNode(net, brainNode{col: col, row: cols[col]}) {
			t.Errorf("column %d holds %d nodes and row %d reads as present", col, cols[col], cols[col])
		}
	}
	for _, gone := range []brainNode{{col: -1}, {col: len(cols)}, {col: 0, row: -1}} {
		if brainHasNode(net, gone) {
			t.Errorf("%v reads as present", gone)
		}
	}
	// A starter net has one column, so anything a click could have picked out
	// of a grown one is gone.
	if brainHasNode(neural.Starter(d.ActChemosynthesis), brainNode{col: 2, row: 3}) {
		t.Error("a node of a grown net reads as present in a starter")
	}
}
