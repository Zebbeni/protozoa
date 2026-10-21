package neural

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/simrand"
)

// loadGlobals installs the shipped settings, which is what the mutation pools
// and the shape caps are read from.
func loadGlobals(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	config.SetGlobals(&g)
}

// loadGlobalsNoInputFloor is loadGlobals with min_neural_net_inputs set to 0.
//
// The floor changes which structural mutation a draw lands on — removeInput
// is left out of the candidates once a net is down to the minimum — so a test
// about what MUTATION produces is measuring the shipped floor unless it says
// otherwise. Tests about the floor itself set it themselves.
func loadGlobalsNoInputFloor(t *testing.T) {
	t.Helper()
	loadGlobals(t)
	g := *config.GetCurrentGlobals()
	g.MinNeuralNetInputs = 0
	config.SetGlobals(&g)
}

// alwaysFalse and alwaysTrue are the two trivial worlds a net can be asked
// about, so a test can evaluate one without building a simulation.
func alwaysFalse(d.Condition) bool { return false }
func alwaysTrue(d.Condition) bool  { return true }

func TestStarterChoosesItsOnlyAction(t *testing.T) {
	loadGlobals(t)
	net := Starter(d.ActChemosynthesis)

	if got := net.Depth(); got != 0 {
		t.Errorf("a starter net has %d hidden layers, want 0", got)
	}
	if got := net.Nodes(); got != 1 {
		t.Errorf("a starter net has %d nodes, want 1 (the single output)", got)
	}
	for _, eval := range []func(d.Condition) bool{alwaysFalse, alwaysTrue} {
		if got := net.Choose(eval); got != d.ActChemosynthesis {
			t.Errorf("a one-output net chose %v, want its only action", got)
		}
	}
}

// TestTheHighestOutputWins is the readout rule: argmax over the output layer,
// with no rng draw involved.
func TestTheHighestOutputWins(t *testing.T) {
	loadGlobals(t)
	net := &Net{
		inputs:  []d.Condition{d.IsFoodHere},
		outputs: []d.Action{d.ActChemosynthesis, d.ActEat},
		layers: []layer{{
			weights: [][]float64{{0}, {1}},
			biases:  []float64{0.5, 0},
		}},
	}
	// Food absent: eat scores 0, chemo 0.5.
	if got := net.Choose(alwaysFalse); got != d.ActChemosynthesis {
		t.Errorf("with no food the net chose %v, want chemosynthesis", got)
	}
	// Food present: eat scores 1, chemo still 0.5.
	if got := net.Choose(alwaysTrue); got != d.ActEat {
		t.Errorf("with food the net chose %v, want eat", got)
	}
}

// TestATieGoesToTheEarlierOutput pins the one case argmax does not settle. It
// has to be an order rather than a draw: an rng call here would be one per
// organism per cycle, which the replay stream would carry forever.
func TestATieGoesToTheEarlierOutput(t *testing.T) {
	loadGlobals(t)
	net := &Net{
		outputs: []d.Action{d.ActIdle, d.ActEat},
		layers: []layer{{
			weights: [][]float64{{}, {}},
			biases:  []float64{0, 0},
		}},
	}
	for i := 0; i < 10; i++ {
		if got := net.Choose(alwaysFalse); got != d.ActIdle {
			t.Fatalf("a tie chose %v, want the first-wired action", got)
		}
	}
}

// TestChoosingIsRepeatable is the property a replay rests on: the same net
// against the same world gives the same answer and the same numbers, however
// often it is asked.
func TestChoosingIsRepeatable(t *testing.T) {
	loadGlobals(t)
	rng := simrand.New(7)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 40; i++ {
		net = Inherit(rng, net)
	}

	first := net.Choose(alwaysTrue)
	want := append([]float64(nil), net.LastOutputs()...)
	for i := 0; i < 50; i++ {
		if got := net.Choose(alwaysTrue); got != first {
			t.Fatalf("call %d chose %v, first call chose %v", i, got, first)
		}
		for j, v := range net.LastOutputs() {
			if v != want[j] {
				t.Fatalf("call %d output %d is %v, first call gave %v", i, j, v, want[j])
			}
		}
	}
}

// TestEvaluatingANetAllocatesNothing: Choose runs for every organism on every
// cycle, so the activation buffers are reused rather than allocated.
func TestEvaluatingANetAllocatesNothing(t *testing.T) {
	loadGlobals(t)
	rng := simrand.New(3)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 60; i++ {
		net = Inherit(rng, net)
	}
	net.Choose(alwaysTrue) // first call sizes the buffers

	allocs := testing.AllocsPerRun(100, func() {
		net.Choose(alwaysTrue)
	})
	if allocs != 0 {
		t.Errorf("Choose allocated %.1f times per call, want 0", allocs)
	}
}

// TestNodesCountsEveryUnit pins what the per-cycle upkeep is charged on.
func TestNodesCountsEveryUnit(t *testing.T) {
	loadGlobals(t)
	net := &Net{
		inputs:  []d.Condition{d.IsFoodHere, d.CanMove},
		outputs: []d.Action{d.ActEat, d.ActMove},
		layers: []layer{
			{weights: [][]float64{{0, 0}, {0, 0}, {0, 0}}, biases: make([]float64, 3)},
			{weights: [][]float64{{0, 0, 0}, {0, 0, 0}}, biases: make([]float64, 2)},
		},
	}
	if got, want := net.Nodes(), 2+3+2; got != want {
		t.Errorf("Nodes is %d, want %d (inputs + hidden + outputs)", got, want)
	}
	if got := net.Depth(); got != 1 {
		t.Errorf("Depth is %d, want 1 hidden layer", got)
	}
	if got := net.Width(); got != 3 {
		t.Errorf("Width is %d, want the widest hidden layer", got)
	}
}

// TestActionWeightsSplitAcrossTheOutputs: the net's counterpart of a tree's
// action weights is a flat share of what it can choose at all.
func TestActionWeightsSplitAcrossTheOutputs(t *testing.T) {
	loadGlobals(t)
	net := Starter(d.ActChemosynthesis)
	w := net.ActionWeights()
	if w[d.ActChemosynthesis] != 1 {
		t.Errorf("a one-output net gives its action %v, want 1", w[d.ActChemosynthesis])
	}

	rng := simrand.New(11)
	for i := 0; i < 30; i++ {
		net.Mutate(rng)
	}
	w = net.ActionWeights()
	sum := 0.0
	for _, v := range w {
		sum += v
	}
	if len(net.Outputs()) > 1 && sum < 0.999 || sum > 1.001 {
		t.Errorf("action weights sum to %v across %d outputs, want 1", sum, len(net.Outputs()))
	}
	for _, a := range net.Outputs() {
		if w[a] <= 0 {
			t.Errorf("wired action %v has weight 0", a)
		}
	}
}

// TestACopyKeepsTheLastOutputs: the panel is handed a copy rather than the
// organism's own net, since the simulation may be writing to that one while
// the panel reads it. Without the activations the tab that explains which
// output won has nothing to explain it with.
func TestACopyKeepsTheLastOutputs(t *testing.T) {
	loadGlobals(t)
	rng := simrand.New(61)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 40; i++ {
		net.Mutate(rng)
	}
	net.drift(rng, 0.5)
	net.Choose(alwaysTrue)

	before := append([]float64(nil), net.LastOutputs()...)

	copied := net.Copy()
	if len(copied.LastOutputs()) != len(net.LastOutputs()) {
		t.Fatalf("the copy carries %d output values, the original %d",
			len(copied.LastOutputs()), len(net.LastOutputs()))
	}
	if len(copied.LastOutputs()) == 0 {
		t.Fatal("the original has no output values to copy; the test says nothing")
	}
	for i, v := range net.LastOutputs() {
		if copied.LastOutputs()[i] != v {
			t.Errorf("output %d copied as %v, want %v", i, copied.LastOutputs()[i], v)
		}
	}

	// Still a copy: evaluating one must not move the other. Compared against
	// a snapshot taken beforehand — reading the original's own slice twice
	// would compare it with itself and pass whatever Copy did.
	copied.Choose(alwaysFalse)
	for i, v := range net.LastOutputs() {
		if v != before[i] {
			t.Errorf("evaluating the copy moved the original output %d: %v became %v",
				i, before[i], v)
		}
	}
}

// TestTheWinningOutputIsTheOneShown ties the two halves of the panel's brain
// tab together: the action it marks as taken is the one with the highest value
// in the list beside it.
func TestTheWinningOutputIsTheOneShown(t *testing.T) {
	loadGlobals(t)
	rng := simrand.New(67)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 60; i++ {
		net = Inherit(rng, net)
	}

	for _, eval := range []func(d.Condition) bool{alwaysTrue, alwaysFalse} {
		chosen := net.Choose(eval)
		values := net.LastOutputs()
		best := 0
		for j, v := range values {
			if v > values[best] {
				best = j
			}
		}
		if net.Outputs()[best] != chosen {
			t.Errorf("the net chose %v but the highest of %v is %v",
				chosen, values, net.Outputs()[best])
		}
	}
}

// TestTransmittedIsWeightTimesWhatFeedsIt pins what the panel draws each
// connection at. A weight is what an edge COULD carry; what flows along it is
// the weight times the value of the unit behind it, so an edge out of a
// condition that read false carries nothing however heavy it is.
func TestTransmittedIsWeightTimesWhatFeedsIt(t *testing.T) {
	loadGlobals(t)
	net := &Net{
		inputs:  []d.Condition{d.IsFoodHere, d.CanMove},
		outputs: []d.Action{d.ActEat},
		layers: []layer{{
			weights: [][]float64{{3, -5}},
			biases:  []float64{0},
		}},
	}

	// Both conditions false: nothing flows, whatever the weights are.
	net.Choose(alwaysFalse)
	for _, prev := range []int{0, 1} {
		if got := net.Transmitted(0, 0, prev); got != 0 {
			t.Errorf("with every input false, edge %d carried %v, want 0", prev, got)
		}
	}
	if got := net.StrongestTransmission(); got != 0 {
		t.Errorf("a net reading nothing has a strongest transmission of %v, want 0", got)
	}

	// Both true: each edge carries its own weight.
	net.Choose(alwaysTrue)
	if got := net.Transmitted(0, 0, 0); got != 3 {
		t.Errorf("edge 0 carried %v, want its weight of 3", got)
	}
	if got := net.Transmitted(0, 0, 1); got != -5 {
		t.Errorf("edge 1 carried %v, want its weight of -5", got)
	}
	if got := net.StrongestTransmission(); got != 5 {
		t.Errorf("the strongest transmission is %v, want 5", got)
	}
}

// TestActivationsCoverEveryColumn: the panel needs the value of every unit,
// not just the outputs, because an edge between two hidden layers is drawn at
// what the layer behind it produced.
func TestActivationsCoverEveryColumn(t *testing.T) {
	shapeGlobals(t, 6, 2)
	rng := simrand.New(71)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 80; i++ {
		net = Inherit(rng, net)
	}
	net.Choose(alwaysTrue)

	if got, want := len(net.Activations(0)), len(net.Inputs()); got != want {
		t.Errorf("column 0 holds %d values for %d inputs", got, want)
	}
	for li, size := range net.LayerSizes() {
		if got := len(net.Activations(li + 1)); got != size {
			t.Errorf("column %d holds %d values for a layer of %d", li+1, got, size)
		}
	}
	if net.Activations(-1) != nil || net.Activations(len(net.LayerSizes())+1) != nil {
		t.Error("a column outside the net returned values")
	}
	// Every input read true, so column 0 is all ones.
	for i, v := range net.Activations(0) {
		if v != 1 {
			t.Errorf("input %d read true and came back as %v", i, v)
		}
	}
}

// TestHiddenValuesAreNeverNegative: the hidden layers are ReLU, so a line out
// of one carries the sign of its weight alone. Only the output column can go
// negative.
func TestHiddenValuesAreNeverNegative(t *testing.T) {
	shapeGlobals(t, 6, 2)
	rng := simrand.New(73)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 120; i++ {
		net = Inherit(rng, net)
	}

	for _, eval := range []func(d.Condition) bool{alwaysTrue, alwaysFalse} {
		net.Choose(eval)
		for col := 0; col < len(net.LayerSizes()); col++ {
			for i, v := range net.Activations(col) {
				if v < 0 {
					t.Errorf("column %d unit %d is %v; only the outputs may go negative", col, i, v)
				}
			}
		}
	}
}

// TestEvaluatingStillAllocatesNothingWithActivationsKept: the activations are
// held for the panel, and holding them must not put an allocation back into
// the hot path.
func TestEvaluatingStillAllocatesNothingWithActivationsKept(t *testing.T) {
	shapeGlobals(t, 6, 2)
	rng := simrand.New(79)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 100; i++ {
		net = Inherit(rng, net)
	}
	net.Choose(alwaysTrue)

	if allocs := testing.AllocsPerRun(100, func() { net.Choose(alwaysTrue) }); allocs != 0 {
		t.Errorf("Choose allocated %.1f times per call, want 0", allocs)
	}
}

// TestInputPullIsTheWeightLeavingThatInput: it is what the sensor overlay is
// derived from, so it has to be the connections out of the input and nothing
// else — a sum over the first layer's units at that input's index.
func TestInputPullIsTheWeightLeavingThatInput(t *testing.T) {
	// About the arithmetic of InputPull, not about the shipped floor, and it
	// needs a net that grew more than one read to be about anything.
	loadGlobalsNoInputFloor(t)
	rng := simrand.New(23)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 200; i++ {
		net = Inherit(rng, net)
	}
	if len(net.Inputs()) < 2 {
		t.Fatalf("the net grew %d inputs; this needs at least two to be about anything", len(net.Inputs()))
	}

	units := net.LayerSizes()[0]
	nonZero := 0
	for i := range net.Inputs() {
		want := 0.0
		for j := 0; j < units; j++ {
			want += math.Abs(net.WeightAt(0, j, i))
		}
		if got := net.InputPull(i); got != want {
			t.Errorf("input %d pulls %v against the %v leaving it", i, got, want)
		}
		if want > 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Error("every input pulls nothing, so the comparison above checked only zeroes")
	}
	// Out of range is 0 rather than a panic, the same as WeightAt: the panel
	// and the appearance both read a net that may have grown since.
	if got := net.InputPull(-1); got != 0 {
		t.Errorf("input -1 pulls %v", got)
	}
	if got := net.InputPull(len(net.Inputs()) + 5); got != 0 {
		t.Errorf("an input past the end pulls %v", got)
	}
}

// TestInputPullIgnoresWhatTheCycleIsReading: an appearance is computed once at
// construction and is fixed for an organism's lifetime, so the quantity behind
// it must not move with what the organism is looking at.
func TestInputPullIgnoresWhatTheCycleIsReading(t *testing.T) {
	loadGlobals(t)
	rng := simrand.New(29)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 200; i++ {
		net = Inherit(rng, net)
	}

	net.Choose(alwaysTrue)
	before := make([]float64, len(net.Inputs()))
	for i := range before {
		before[i] = net.InputPull(i)
	}
	net.Choose(func(d.Condition) bool { return false })
	for i := range before {
		if got := net.InputPull(i); got != before[i] {
			t.Errorf("input %d pulled %v and now pulls %v after a different cycle", i, before[i], got)
		}
	}
}

// TestANetActsOnAConditionBeingFalseWithNoNegatedInput answers whether the
// input pool needs an IsFoodNotHere beside IsFoodHere. It does not, and the
// negation costs nothing at all: an input is 0 or 1 and an output is
// sum(w*x) + b, so a NEGATIVE weight already means "prefer this action when
// the condition is false". No hidden layer is involved.
func TestANetActsOnAConditionBeingFalseWithNoNegatedInput(t *testing.T) {
	loadGlobals(t)
	// One input, two actions, no hidden layer. Eat is preferred by default,
	// and a single negative weight is what makes food being here switch it.
	net := &Net{
		inputs:  []d.Condition{d.IsFoodHere},
		outputs: []d.Action{d.ActEat, d.ActChemosynthesis},
		layers: []layer{{
			weights: [][]float64{{-1}, {0}},
			biases:  []float64{0.5, 0},
		}},
	}

	if got := net.Choose(func(d.Condition) bool { return false }); got != d.ActEat {
		t.Errorf("with no food here the net chose %v, want ActEat", d.Map[got])
	}
	if got := net.Choose(func(d.Condition) bool { return true }); got != d.ActChemosynthesis {
		t.Errorf("with food here the net chose %v, want ActChemosynthesis", d.Map[got])
	}
}

// TestAHiddenUnitComputesTheExactNegation is the stronger half: a negated
// input is not merely approximable, it is an exact affine function of the
// input, and one hidden unit holds it. relu(-x + 1) is 1 when the condition
// is false and 0 when it is true, with ReLU clipping nothing — both values
// are already non-negative.
//
// So a net whose inputs are x can represent everything a net whose inputs are
// (x, 1-x) can, because the first layer is affine and HAS A BIAS. Without the
// bias the claim would be false: every pre-activation would be 0 at x = 0.
func TestAHiddenUnitComputesTheExactNegation(t *testing.T) {
	loadGlobals(t)
	net := &Net{
		inputs:  []d.Condition{d.IsFoodHere},
		outputs: []d.Action{d.ActEat},
		layers: []layer{
			{weights: [][]float64{{-1}}, biases: []float64{1}}, // NOT food here
			{weights: [][]float64{{1}}, biases: []float64{0}},
		},
	}

	for _, tc := range []struct {
		food bool
		want float64
	}{{false, 1}, {true, 0}} {
		net.Choose(func(d.Condition) bool { return tc.food })
		if got := net.Activations(1)[0]; got != tc.want {
			t.Errorf("with food here = %v the hidden unit reads %v, want %v", tc.food, got, tc.want)
		}
	}
}

// TestDriftReachesANegativeWeightAsReadilyAsAPositiveOne: representable is
// not the same as reachable, so the other half of the answer is that nothing
// stands between a lineage and a negative weight. Drift is additive and
// symmetric about zero, and biases drift with the weights — which is what the
// hidden-unit negation above needs, since it is a weight AND a bias together.
func TestDriftReachesANegativeWeightAsReadilyAsAPositiveOne(t *testing.T) {
	loadGlobals(t)
	rng := simrand.New(97)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 300; i++ {
		net = Inherit(rng, net)
	}

	negative, positive, negBias := 0, 0, 0
	for li := range net.layers {
		for _, row := range net.layers[li].weights {
			for _, w := range row {
				switch {
				case w < 0:
					negative++
				case w > 0:
					positive++
				}
			}
		}
		for _, b := range net.layers[li].biases {
			if b < 0 {
				negBias++
			}
		}
	}
	if negative == 0 {
		t.Error("no weight in a 300-generation lineage is negative, so no condition can be acted on for being false")
	}
	if positive == 0 {
		t.Error("no weight is positive, which makes the count above say nothing")
	}
	t.Logf("%d negative and %d positive weights, %d negative biases", negative, positive, negBias)
}
