package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"

	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/neural"
	r "github.com/Zebbeni/protozoa/resources"
)

// rulesNeuralBlock draws a sample net through drawBrainExpanded, the same
// function the replay viewer's expanded brain view uses, so the rules show
// the picture the player will meet rather than a second drawing of it that
// could drift.
type rulesNeuralBlock struct {
	net *neural.Net
	// acting is the action the net chose on the reading below, which is what
	// lights one of the output names.
	acting d.Action
	// caption names the shape under the picture, since the diagram draws the
	// layers without saying how many of each there are.
	caption string
}

// rulesNeuralH is the room the diagram gets. Three rows is the tallest
// column, so this is what decides how far apart they sit.
const rulesNeuralH = 170

// exampleNet is a 3>2>3 net with weights chosen to be read rather than
// evolved: one hidden unit leans on food being there, the other on being able
// to move, and the output layer prefers eating when the first of them fires.
//
// The weights are round numbers on purpose — these are drawn at two decimals
// beside their lines, and a net lifted from a real organism carries values
// like -1.3590 that say nothing to someone meeting the idea for the first
// time.
func exampleNet() *neural.Net {
	return neural.NewFrom(
		[]d.Condition{d.IsFoodHere, d.IsHealthy, d.CanMove},
		[]d.Action{d.ActEat, d.ActMove, d.ActChemosynthesis},
		[]neural.Layer{
			// Two hidden units, each reading all three conditions.
			{
				Weights: [][]float64{
					{1.20, -0.40, 0.10},
					{-0.30, 0.60, 0.80},
				},
				Biases: []float64{0.05, -0.20},
			},
			// One output per action, each reading both hidden units.
			{
				Weights: [][]float64{
					{1.50, -0.20},
					{-0.50, 1.10},
					{0.20, 0.30},
				},
				Biases: []float64{0.10, 0, 0.40},
			},
		})
}

// exampleReads is what the world looks like to the net in the picture: food
// underneath it, health under half, nothing in the way. Two of the three
// conditions read true, so the diagram shows a lit and an unlit input rather
// than one state of the three.
func exampleReads(c d.Condition) bool {
	return c == d.IsFoodHere || c == d.CanMove
}

func newRulesNeuralBlock() rulesNeuralBlock {
	net := exampleNet()
	// Choose fills the activations the diagram draws its labels and output
	// scores from, so the picture is one cycle of one net rather than a
	// wiring diagram with nothing flowing.
	return rulesNeuralBlock{
		net:     net,
		acting:  net.Choose(exampleReads),
		caption: "an example neural network with 3 inputs (conditions), a single hidden 2-node layer, and 3 outputs (actions)",
	}
}

func (b rulesNeuralBlock) height() int {
	return rulesNeuralH + rulesCaptionH + rulesParaGap*2
}

func (b rulesNeuralBlock) draw(screen *ebiten.Image, x, y int, _ rulesTick) {
	// WEIGHTS rather than SIGNAL: a weight is what the lineage evolved and
	// does not change from cycle to cycle, which is what this section is
	// explaining. Nothing is picked out, so the whole net is drawn lit.
	shown := brainShown{view: brainViewWeights}
	top := y + rulesParaGap
	drawBrainExpanded(screen, b.net, b.acting, false, shown, x, top, rulesPanelW, rulesNeuralH)

	cb := boundString(r.FontSourceCodePro8, b.caption)
	text.Draw(screen, b.caption, r.FontSourceCodePro8,
		x+(rulesPanelW-cb.Dx())/2, top+rulesNeuralH+rulesCaptionH,
		themedForegroundDim())
}
