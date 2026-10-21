package ux

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/neural"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simrand"
)

// grownNet is a net with enough history to have a real shape, built against
// the shipped caps.
func grownNet(t *testing.T, generations int) *neural.Net {
	t.Helper()
	loadKeyGlobals(t)
	g := *config.GetCurrentGlobals()
	g.UseNeuralNets = true
	config.SetGlobals(&g)

	rng := simrand.New(3)
	net := neural.Starter(d.ActChemosynthesis)
	for i := 0; i < generations; i++ {
		net = neural.Inherit(rng, net)
	}
	return net
}

// TestEveryBrainNodeHasAShortLabel: the diagram names each input and output
// beside its node in a column a tenth the width the full names need, so every
// node type mutation can reach has to have a short form. Without one it falls
// back to its full name, which is drawn truncated — legible, but not a
// reading of the world.
func TestEveryBrainNodeHasAShortLabel(t *testing.T) {
	for _, c := range d.MutableConditions {
		if _, ok := brainConditionLabels[c]; !ok {
			t.Errorf("condition %q has no short label", d.Map[c])
		}
	}
	for _, a := range d.MutableActions {
		if _, ok := brainActionLabels[a]; !ok {
			t.Errorf("action %q has no short label", d.Map[a])
		}
	}
	// ActSpawn is not mutable — no tree or net chooses it — but an organism
	// promoted to spawn is drawn with it as the acting output.
	if _, ok := brainActionLabels[d.ActSpawn]; !ok {
		t.Error("ActSpawn has no short label")
	}
}

// TestNoTwoBrainLabelsReadAlike: two different reads of the world drawn with
// the same label are worse than one long label, because nothing on screen says
// they are different.
func TestNoTwoBrainLabelsReadAlike(t *testing.T) {
	seen := map[string]string{}
	for c, label := range brainConditionLabels {
		if other, clash := seen[label]; clash {
			t.Errorf("%q labels both %q and %q", label, other, d.Map[c])
		}
		seen[label] = d.Map[c]
	}
	actions := map[string]string{}
	for a, label := range brainActionLabels {
		if other, clash := actions[label]; clash {
			t.Errorf("%q labels both %q and %q", label, other, d.Map[a])
		}
		actions[label] = d.Map[a]
	}
}

// TestBrainLabelsFitTheirColumns measures every short form against the column
// it is drawn in, in the font it is drawn with. A label that does not fit is
// truncated with a dot, which is the thing the short forms exist to avoid.
func TestBrainLabelsFitTheirColumns(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	face := r.FontSourceCodePro8

	for c, label := range brainConditionLabels {
		if got := textAdvance(face, label); got > brainInputLabelW {
			t.Errorf("%q (for %q) is %dpx in a %dpx column", label, d.Map[c], got, brainInputLabelW)
		}
	}
	for a, label := range brainActionLabels {
		if got := textAdvance(face, label); got > brainOutputLabelW {
			t.Errorf("%q (for %q) is %dpx in a %dpx column", label, d.Map[a], got, brainOutputLabelW)
		}
	}
}

// TestEveryDirectionalLabelNamesItsCell is what replaced the key line under
// the diagram: a label that reads a neighbouring cell spells that cell out, so
// there is nothing left to decode. A new condition labelled "Food A" would
// reintroduce the key for one node.
func TestEveryDirectionalLabelNamesItsCell(t *testing.T) {
	for _, c := range d.MutableConditions {
		label, ok := brainConditionLabels[c]
		if !ok {
			continue
		}
		for _, cell := range []string{"Ahead", "Left", "Right", "Here"} {
			if strings.Contains(d.Names[c], cell) && !strings.Contains(label, cell) {
				t.Errorf("%s reads the cell %s, and its label %q does not say so",
					d.Names[c], cell, label)
			}
		}
	}
}

// TestTheOutputValueColumnHoldsWhatANetProduces: an output has no fixed
// ceiling — it is a sum of weights a lineage has drifted to — so the column
// is measured against a value far past anything seen, since a score running
// into the action name beside it is an overflow rather than a clipped tail.
func TestTheOutputValueColumnHoldsWhatANetProduces(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	value := fmt.Sprintf("%.2f", -9999.99)
	if got := textAdvance(r.FontSourceCodePro8, value); got > brainValueW {
		t.Errorf("a score of %s is %dpx in a %dpx column", value, got, brainValueW)
	}
}

// TestTheDiagramKeepsRoomBetweenItsLabels: the two label columns and the
// diagram share the content width, and the diagram is what is left — so a
// wider label column takes it away, and at some point there is no diagram.
func TestTheDiagramKeepsRoomBetweenItsLabels(t *testing.T) {
	w := brainDiagramW(infoAreaWidth)
	if w < 60 {
		t.Errorf("the diagram gets %dpx of a %dpx column, which is not enough to draw layers in",
			w, infoAreaWidth)
	}
	if left := brainDiagramX(selectedXOffset); left < selectedXOffset+brainInputLabelW {
		t.Errorf("the diagram starts at %d, inside the input labels ending at %d",
			left, selectedXOffset+brainInputLabelW)
	}
	if right := brainDiagramX(selectedXOffset) + w; right > selectedXOffset+infoAreaWidth {
		t.Errorf("the diagram ends at %d, past the content column at %d",
			right, selectedXOffset+infoAreaWidth)
	}
}

// TestEveryNodeGetsItsOwnRow is what lets each node carry a label: columns are
// laid out one row apart rather than spread to a fixed height, so two labels
// can never land on the same line.
func TestEveryNodeGetsItsOwnRow(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	rowH := brainRowHeight(400, 20, 0, r.FontSourceCodePro8)

	for _, n := range []int{1, 2, 5, 20, 36} {
		ys := spreadColumn(0, n*rowH, n, rowH)
		if len(ys) != n {
			t.Fatalf("a column of %d units laid out %d rows", n, len(ys))
		}
		for i := 1; i < len(ys); i++ {
			if gap := ys[i] - ys[i-1]; gap < float64(rowH) {
				t.Errorf("rows %d and %d are %vpx apart, under the %dpx a label needs",
					i-1, i, gap, rowH)
			}
		}
	}
	if got := spreadColumn(0, 100, 0, rowH); got != nil {
		t.Errorf("an empty column laid out %v, want nothing to draw", got)
	}
}

// TestAShortColumnSitsLevelWithALongOne: the layers are different heights, and
// a column pinned to the top would draw a two-unit layer beside the first two
// rows of a twenty-input one rather than beside its middle.
func TestAShortColumnSitsLevelWithALongOne(t *testing.T) {
	const rowH, tall = 11, 20
	height := tall * rowH
	long := spreadColumn(0, height, tall, rowH)
	short := spreadColumn(0, height, 2, rowH)

	longMid := (long[0] + long[len(long)-1]) / 2
	shortMid := (short[0] + short[len(short)-1]) / 2
	if diff := longMid - shortMid; diff > 0.01 || diff < -0.01 {
		t.Errorf("a 2-unit column centres at %v and a %d-unit one at %v", shortMid, tall, longMid)
	}
}

// TestLineWidthFollowsWhatIsFlowing is the encoding stated directly: a
// connection is drawn by how much of the net's strongest signal it carries, so
// one carrying nothing is at the minimum and one carrying the maximum is at
// the maximum.
func TestLineWidthFollowsWhatIsFlowing(t *testing.T) {
	const strongest = 4.0
	if got := edgeWidth(0, strongest); got != brainMinEdgeW {
		t.Errorf("a connection carrying nothing is %vpx wide, want the %vpx minimum", got, brainMinEdgeW)
	}
	if got := edgeWidth(strongest, strongest); got != brainMaxEdgeW {
		t.Errorf("the strongest connection is %vpx wide, want the %vpx maximum", got, brainMaxEdgeW)
	}
	// The sign is the colour's job, so width reads the magnitude alone.
	if edgeWidth(-strongest, strongest) != edgeWidth(strongest, strongest) {
		t.Error("a negative connection is drawn at a different width from its mirror")
	}

	prev := 0.0
	for i := 0; i <= 10; i++ {
		flow := float64(i) / 10 * strongest
		w := edgeWidth(flow, strongest)
		if w < prev {
			t.Errorf("width fell from %v to %v as the signal grew", prev, w)
		}
		if w < brainMinEdgeW || w > brainMaxEdgeW {
			t.Errorf("width %v is outside [%v, %v]", w, brainMinEdgeW, brainMaxEdgeW)
		}
		prev = w
	}
	// A net with nothing flowing anywhere must not divide by its own zero.
	if got := edgeWidth(0, 0); got != brainMinEdgeW {
		t.Errorf("with no signal anywhere a connection is %vpx wide, want the minimum", got)
	}
	// A value past the maximum is clamped rather than drawn off the scale.
	if got := edgeWidth(strongest*10, strongest); got != brainMaxEdgeW {
		t.Errorf("an over-scale signal is %vpx wide, want the %vpx maximum", got, brainMaxEdgeW)
	}
}

// TestALitLabelIsBrighterThanAQuietOne pins the highlight both ways round,
// since a dead organism's labels are dimmed and the lit one still has to be
// the brighter of the two.
func TestALitLabelIsBrighterThanAQuietOne(t *testing.T) {
	loadKeyGlobals(t)
	bright := func(c color.Color) float64 {
		cr, cg, cb, _ := c.RGBA()
		return relativeLuminance(color.RGBA{
			R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: 0xFF,
		})
	}
	for _, dim := range []bool{false, true} {
		lit := bright(brainInk(dim, true))
		quiet := bright(brainInk(dim, false))
		if lit <= quiet {
			t.Errorf("dim=%v: the lit label is no brighter than the quiet one (%v vs %v)",
				dim, lit, quiet)
		}
	}
}

// TestAnEmptyNetDrawsWithoutPanicking: a founder reads nothing and has one
// output, so its first column is empty and nothing is flowing anywhere — the
// division the line width would do.
func TestAnEmptyNetDrawsWithoutPanicking(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	net := neural.Starter(d.ActChemosynthesis)
	if got := edgeWidth(0, net.StrongestTransmission()); got != brainMinEdgeW {
		t.Errorf("a connection in a net with nothing flowing is %vpx wide, want the minimum", got)
	}
	if summary := brainSummary(net); summary == "" {
		t.Error("a founder's net summarises to nothing")
	}
	if got := len(net.Activations(0)); got != 0 {
		t.Errorf("a net reading nothing has %d input values", got)
	}
}

// TestTheBrainSummaryFitsTheColumn measures the shape line for a net that has
// run long enough to reach the shipped caps.
func TestTheBrainSummaryFitsTheColumn(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	net := grownNet(t, 600)

	summary := brainSummary(net)
	if got := textAdvance(r.FontSourceCodePro10, summary); got > infoAreaWidth {
		t.Errorf("the summary %q is %dpx in a %dpx column", summary, got, infoAreaWidth)
	}
	t.Logf("summary for a %d-node net: %q", net.Nodes(), summary)
}

// TestInkFollowsTheSignalToo: width alone does not separate a few hundred
// lines in a hundred-pixel column, so the weak majority is faded and the
// carrying few are solid. Both ends are pinned, and the direction between.
func TestInkFollowsTheSignalToo(t *testing.T) {
	loadKeyGlobals(t)
	const strongest = 3.0
	alphaOf := func(flow float64) uint8 {
		_, _, _, a := edgeColor(flow, strongest, false).RGBA()
		return uint8(a >> 8)
	}

	quiet, loud := alphaOf(0), alphaOf(strongest)
	if quiet >= loud {
		t.Errorf("a connection carrying nothing is inked at %d and the strongest at %d", quiet, loud)
	}
	if quiet == 0 {
		t.Error("a connection carrying nothing is invisible; the structure should still read")
	}
	prev := uint8(0)
	for i := 0; i <= 10; i++ {
		a := alphaOf(float64(i) / 10 * strongest)
		if a < prev {
			t.Errorf("ink fell from %d to %d as the signal grew", prev, a)
		}
		prev = a
	}
	// A net with nothing flowing anywhere must not divide by its own zero.
	if _, _, _, a := edgeColor(0, 0, false).RGBA(); a == 0 {
		t.Error("with no signal anywhere the connections are invisible")
	}
	// The sign is the hue, so the two directions ink alike.
	if alphaOf(-strongest) != alphaOf(strongest) {
		t.Error("a negative connection is inked differently from its mirror")
	}
	negR, _, _, _ := edgeColor(-1, strongest, false).RGBA()
	posR, _, _, _ := edgeColor(1, strongest, false).RGBA()
	if negR == posR {
		t.Error("a connection arguing for its unit is drawn the same hue as one arguing against")
	}
}

// TestTheInputNamesClearTheNodesTheyName: a node is drawn CENTRED on its
// column, so it reaches a radius back into the gap between the diagram and the
// names beside it. At the expanded view's radius that left nothing at all
// between a name and its dot, which is why the columns start a radius further
// in than the gap alone.
func TestTheInputNamesClearTheNodesTheyName(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	net := grownNet(t, 400)
	layouts := map[string]struct {
		lay  brainLayout
		x, w int
	}{
		"panel":    {panelBrainLayout(net, infoAreaWidth, panelBrainHeight(120)), selectedXOffset, infoAreaWidth},
		"expanded": {expandedBrainLayout(net, 900, 700), 0, 900},
	}
	for name, l := range layouts {
		nameRight := l.x + l.lay.inputLabelW
		dotLeft := float64(l.lay.diagramX(l.x)) - l.lay.nodeRadius
		if gap := dotLeft - float64(nameRight); gap < brainLabelGap {
			t.Errorf("%s: %.1fpx between the input names and the dots they name, under the %dpx gap",
				name, gap, brainLabelGap)
		}
		// A lit name is drawn over a plate that reaches past the text, so
		// what has to clear the dots is the plate rather than the glyphs.
		if gap := dotLeft - float64(nameRight+brainLitPlatePad); gap <= 0 {
			t.Errorf("%s: a lit name's plate overlaps the dot it names by %.1fpx", name, -gap)
		}
		// The diagram still ends inside the column it was given.
		if right := l.lay.diagramX(l.x) + l.lay.diagramW(l.w); right > l.x+l.w {
			t.Errorf("%s: the diagram ends at %d, past the %d it was given", name, right, l.x+l.w)
		}
	}
}

// TestAConnectionWithNoWeightIsDrawnGrey: it used to be left out, and an
// absence in a diagram of a thing that is fully connected reads as a rendering
// fault rather than as an absence — which is how the missing line between two
// one-unit layers came to be reported in the first place. Grey is also what
// separates it from a weighted connection whose unit reads 0 this cycle, which
// is a different statement about the net.
func TestAConnectionWithNoWeightIsDrawnGrey(t *testing.T) {
	loadKeyGlobals(t)

	const strongest = 3.0
	dead := unweightedEdgeColor(false)
	idle := edgeColor(0, strongest, false)

	deadR, deadG, deadB, deadA := dead.RGBA()
	if deadA == 0 {
		t.Fatal("a connection with no weight is invisible, which is what drawing it is meant to fix")
	}
	if deadR != deadG {
		t.Errorf("a connection with no weight is drawn %d,%d,%d rather than grey", deadR>>8, deadG>>8, deadB>>8)
	}
	idleR, idleG, idleB, idleA := idle.RGBA()
	if deadR == idleR && deadG == idleG && deadB == idleB {
		t.Error("a connection with no weight is drawn the same hue as one carrying nothing")
	}
	// No louder than the quietest connection that does carry a weight: it is
	// the least significant thing in the net and must not read as the most.
	if deadA > idleA {
		t.Errorf("a connection with no weight is inked at %d against the %d of one carrying nothing",
			deadA>>8, idleA>>8)
	}
	// And quieter than one that is carrying the whole net.
	if _, _, _, loud := edgeColor(strongest, strongest, false).RGBA(); deadA >= loud {
		t.Errorf("a connection with no weight is inked at %d against the %d of the strongest", deadA>>8, loud>>8)
	}
}

// TestALitBrainLabelIsReversedOutOfItsPlate: a condition reading true and the
// action being taken are the two facts the tab exists to answer, and they were
// told from the rest by one step of ink brightness. The plate states them the
// other way round — dark text on a light plate where the window is dark, light
// text on a dark plate where it is light — so a theme assumed in either half
// of the pair is a label drawn in the window's own colour.
func TestALitBrainLabelIsReversedOutOfItsPlate(t *testing.T) {
	loadContrastGlobals(t)

	for _, theme := range []string{"dark", "light"} {
		config.SetTheme(theme)
		window := relativeLuminance(backgroundRGBA())
		plate := relativeLuminance(brainLitPlateFill(false))
		ink := relativeLuminance(brainLitPlateInk(false))

		if theme == "dark" && !(plate > window && ink < plate) {
			t.Errorf("dark theme: window %.3f, plate %.3f, ink %.3f — wanted dark text on a light plate",
				window, plate, ink)
		}
		if theme == "light" && !(plate < window && ink > plate) {
			t.Errorf("light theme: window %.3f, plate %.3f, ink %.3f — wanted light text on a dark plate",
				window, plate, ink)
		}
	}
}

// TestOnlyALitLabelIsPlated: the plate marks a state, so a label that is not
// lit, or one set aside because another node is picked out, keeps the ordinary
// ink. Plating every name would make two solid columns and mark nothing.
func TestOnlyALitLabelIsPlated(t *testing.T) {
	loadContrastGlobals(t)

	for _, theme := range []string{"dark", "light"} {
		config.SetTheme(theme)
		for _, c := range []struct {
			lit, inFocus, plated bool
		}{
			{lit: true, inFocus: true, plated: true},
			{lit: false, inFocus: true, plated: false},
			{lit: true, inFocus: false, plated: false},
			{lit: false, inFocus: false, plated: false},
		} {
			if got := brainLabelIsPlated(c.lit, c.inFocus); got != c.plated {
				t.Errorf("%s theme: lit=%v inFocus=%v is plated=%v, want %v",
					theme, c.lit, c.inFocus, got, c.plated)
			}
		}
	}
}

// TestEachViewAnnotatesItsNodesWithItsOwnQuantity: the numbers on the lines
// are the quantity the view is drawn at, and the numbers at the nodes follow
// the same rule — the bias added there in WEIGHTS, what the unit is holding in
// SIGNAL. An input has a value but no bias of its own, so it carries a number
// in one view and none in the other.
//
// The bias is why this exists at all: a unit is relu(bias + what reached it)
// and nothing connects to it to deliver the bias, so a net whose inputs all
// read 0 held non-zero hidden units with every line into them drawn at 0.00.
// Reported twice, on a 1>3>4>3 and a 1>2>2>2.
func TestEachViewAnnotatesItsNodesWithItsOwnQuantity(t *testing.T) {
	loadKeyGlobals(t)

	net := grownNet(t, 400)
	if len(net.LayerSizes()) == 0 || len(net.Inputs()) == 0 {
		t.Fatalf("the grown net is %v with %d inputs; nothing to annotate",
			net.LayerSizes(), len(net.Inputs()))
	}
	// Every condition false, which is the state the biases were reported
	// from: nothing flows, and a hidden unit is relu(its own bias).
	net.Choose(func(d.Condition) bool { return false })

	if _, ok := brainNodeNumber(net, brainViewWeights, 0, 0); ok {
		t.Error("WEIGHTS writes a number on an input, which has no bias of its own")
	}
	if _, ok := brainNodeNumber(net, brainViewSignal, 0, 0); !ok {
		t.Error("SIGNAL writes nothing on an input, which does hold a value")
	}

	// Every unit of every layer, in both views.
	for layer, units := range net.LayerSizes() {
		for unit := 0; unit < units; unit++ {
			col := layer + 1
			got, ok := brainNodeNumber(net, brainViewWeights, col, unit)
			if !ok {
				t.Fatalf("WEIGHTS writes nothing at layer %d unit %d", layer, unit)
			}
			if want := brainBiasNumber(net.BiasAt(layer, unit)); got != want {
				t.Errorf("WEIGHTS writes %q at layer %d unit %d, want the bias %q",
					got, layer, unit, want)
			}
			if !strings.HasPrefix(got, "+") && !strings.HasPrefix(got, "-") {
				t.Errorf("the bias %q at layer %d unit %d is written without its sign, so it reads as a reading rather than a term added there",
					got, layer, unit)
			}
			got, ok = brainNodeNumber(net, brainViewSignal, col, unit)
			if !ok {
				t.Fatalf("SIGNAL writes nothing at layer %d unit %d", layer, unit)
			}
			if want := brainEdgeNumber(net.Activations(col)[unit]); got != want {
				t.Errorf("SIGNAL writes %q at layer %d unit %d, want the unit's value %q",
					got, layer, unit, want)
			}
		}
	}
}

// TestTheNodeNumbersAreAnExpandedViewReading: the panel's columns are 24px
// apart against a 30px number, so one centred on a dot would be drawn across
// the next column of dots. Same answer the numbers on the lines reach there,
// and for the same reason.
func TestTheNodeNumbersAreAnExpandedViewReading(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	net := grownNet(t, 400)
	face := brainEdgeFace()

	panelLay := panelBrainLayout(net, infoAreaWidth, panelBrainHeight(120))
	panelGap := brainDiagramW(infoAreaWidth) / max(len(net.LayerSizes()), 1)
	if brainNodeNumbersFit(panelLay, panelGap, face) {
		t.Errorf("the panel's %dpx columns write a %dpx number at every dot",
			panelGap, brainEdgeLabelW(face))
	}

	expanded := expandedBrainLayout(net, 900, 700)
	gap := 900 / (len(net.LayerSizes()) + 1)
	if !brainNodeNumbersFit(expanded, gap, face) {
		t.Errorf("the expanded view leaves %dpx between columns and %dpx rows and still writes nothing at its nodes",
			gap, expanded.rowH)
	}
}
