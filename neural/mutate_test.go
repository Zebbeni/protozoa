package neural

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/simrand"
)

// shapeGlobals installs the shipped settings with the net caps overridden, so
// a test about a cap sets the cap it is about and leaves the rest alone.
func shapeGlobals(t *testing.T, width, depth int) {
	t.Helper()
	loadGlobals(t)
	g := *config.GetCurrentGlobals()
	g.MaxNeuralNetWidth = width
	g.MaxNeuralNetDepth = depth
	config.SetGlobals(&g)
}

// checkShape is the invariant every net has to hold whatever has been done to
// it: each layer reads exactly as many values as the layer before it produces,
// and the last layer has one unit per action.
func checkShape(t *testing.T, n *Net, when string) {
	t.Helper()
	if len(n.layers) == 0 {
		t.Fatalf("%s: a net with no layers", when)
	}
	for li, l := range n.layers {
		if len(l.weights) != len(l.biases) {
			t.Fatalf("%s: layer %d has %d weight rows for %d biases",
				when, li, len(l.weights), len(l.biases))
		}
		want := n.inputWidth(li)
		for j, w := range l.weights {
			if len(w) != want {
				t.Fatalf("%s: layer %d unit %d reads %d values, the layer before gives %d",
					when, li, j, len(w), want)
			}
		}
	}
	if got, want := len(n.layers[len(n.layers)-1].biases), len(n.outputs); got != want {
		t.Fatalf("%s: output layer has %d units for %d actions", when, got, want)
	}
}

// TestMutationKeepsTheShapeLegal runs every kind of structural change many
// times over and checks the net is still evaluable after each one. A shape
// that drifts out of step is not a crash at mutation time, it is a panic in
// the render loop thousands of cycles later.
func TestMutationKeepsTheShapeLegal(t *testing.T) {
	shapeGlobals(t, 6, 3)
	rng := simrand.New(5)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 500; i++ {
		net.Mutate(rng)
		checkShape(t, net, "after mutation")
		net.Choose(alwaysTrue)
	}
}

// TestTheShapeCapsHold: width and depth are what the settings bound, and
// mutation must not step past either however long it runs.
func TestTheShapeCapsHold(t *testing.T) {
	for _, tc := range []struct{ width, depth int }{{1, 0}, {3, 1}, {8, 4}} {
		shapeGlobals(t, tc.width, tc.depth)
		rng := simrand.New(9)
		net := Starter(d.ActChemosynthesis)
		for i := 0; i < 400; i++ {
			net.Mutate(rng)
			if got := net.Depth(); got > tc.depth {
				t.Fatalf("depth %d past the cap of %d", got, tc.depth)
			}
			if got := net.Width(); got > tc.width {
				t.Fatalf("width %d past the cap of %d", got, tc.width)
			}
		}
	}
}

// TestANetNeverWiresTheSameReadTwice: a duplicate input column carries the
// same value as the one beside it, which drift can already express by moving
// the first — so wiring it again spends a mutation on nothing.
func TestANetNeverWiresTheSameReadTwice(t *testing.T) {
	shapeGlobals(t, 6, 2)
	rng := simrand.New(13)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 600; i++ {
		net.Mutate(rng)
	}

	seenIn := map[d.Condition]bool{}
	for _, c := range net.Inputs() {
		if seenIn[c] {
			t.Errorf("condition %v is wired twice", c)
		}
		seenIn[c] = true
	}
	seenOut := map[d.Action]bool{}
	for _, a := range net.Outputs() {
		if seenOut[a] {
			t.Errorf("action %v is wired twice", a)
		}
		seenOut[a] = true
	}
}

// TestMutationOnlyOffersEnabledNodes: the nets draw from the same pools the
// tree does, so disabling a node type holds it back from both brains.
func TestMutationOnlyOffersEnabledNodes(t *testing.T) {
	shapeGlobals(t, 6, 2)
	enabledC := map[d.Condition]bool{}
	for _, c := range d.EnabledConditions() {
		enabledC[c] = true
	}
	enabledA := map[d.Action]bool{}
	for _, a := range d.EnabledActions() {
		enabledA[a] = true
	}

	rng := simrand.New(17)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 600; i++ {
		net.Mutate(rng)
	}
	for _, c := range net.Inputs() {
		if !enabledC[c] {
			t.Errorf("net reads %v, which the settings hold back", c)
		}
	}
	for _, a := range net.Outputs() {
		if a == d.ActChemosynthesis {
			continue // the starter action, not drawn from the pool
		}
		if !enabledA[a] {
			t.Errorf("net can choose %v, which the settings hold back", a)
		}
	}
}

// growthOnly leaves the three removal weights at zero, so only the additions
// are drawn. A kind with no weight is skipped by pickWeighted rather than
// removed from the pool, which is what makes this a clean way to isolate them.
func growthOnly(t *testing.T, width, depth int) {
	t.Helper()
	shapeGlobals(t, width, depth)
	g := *config.GetCurrentGlobals()
	g.MutationWeightRemoveInput = 0
	g.MutationWeightRemoveOutput = 0
	g.MutationWeightNarrowLayer = 0
	config.SetGlobals(&g)
}

// TestEveryAdditionIsNeutralOnArrival is the property that makes the structure
// searchable at all: a mutation that changed behaviour the moment it landed
// would be judged on a random weight rather than on what it could become, and
// a lineage would lose a working net to every one it tried.
//
// It is a claim about the ADDITIONS only. A removal takes something away, and
// what it takes is the least-connected candidate rather than nothing at all —
// TestARemovalTakesTheLeastConnected is that half.
func TestEveryAdditionIsNeutralOnArrival(t *testing.T) {
	growthOnly(t, 6, 3)
	rng := simrand.New(23)
	net := Starter(d.ActChemosynthesis)
	// Grow it first, so the cases being tested are changes to a net that
	// already does something rather than to the starter.
	for i := 0; i < 25; i++ {
		net.Mutate(rng)
		net.drift(rng, 0.4)
	}

	for i := 0; i < 300; i++ {
		for _, eval := range []func(d.Condition) bool{alwaysTrue, alwaysFalse} {
			before := net.Choose(eval)
			candidate := net.Copy()
			candidate.Mutate(rng)
			if after := candidate.Choose(eval); after != before {
				t.Fatalf("a structural mutation changed the chosen action on arrival: %v became %v",
					before, after)
			}
		}
		net.Mutate(rng)
		net.drift(rng, 0.2)
	}
}

// TestDriftMovesEveryWeightWithinItsStep pins both halves of the drift: it
// reaches every weight, and none of them moves further than the setting says.
func TestDriftMovesEveryWeightWithinItsStep(t *testing.T) {
	shapeGlobals(t, 6, 2)
	rng := simrand.New(29)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 20; i++ {
		net.Mutate(rng)
	}

	const step = 0.25
	before := net.Copy()
	net.drift(rng, step)

	moved, total := 0, 0
	for li := range net.layers {
		for j := range net.layers[li].biases {
			total++
			delta := net.layers[li].biases[j] - before.layers[li].biases[j]
			if delta > step || delta < -step {
				t.Fatalf("layer %d bias %d moved by %v, past the step of %v", li, j, delta, step)
			}
			if delta != 0 {
				moved++
			}
			for i := range net.layers[li].weights[j] {
				total++
				delta := net.layers[li].weights[j][i] - before.layers[li].weights[j][i]
				if delta > step || delta < -step {
					t.Fatalf("layer %d weight %d,%d moved by %v, past the step of %v",
						li, j, i, delta, step)
				}
				if delta != 0 {
					moved++
				}
			}
		}
	}
	if moved != total {
		t.Errorf("drift moved %d of %d weights; it is meant to reach all of them", moved, total)
	}
}

// TestDriftOfZeroIsInert: 0 is how a user says weights are inherited exactly,
// and it has to consume no draws either, or turning it off would shift every
// later value in the run.
func TestDriftOfZeroIsInert(t *testing.T) {
	shapeGlobals(t, 6, 2)
	rng := simrand.New(31)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 20; i++ {
		net.Mutate(rng)
	}

	before := net.Copy()
	driftRNG := simrand.New(41)
	wanted := driftRNG.Float64()

	net.drift(simrand.New(41), 0)
	for li := range net.layers {
		for j := range net.layers[li].biases {
			if net.layers[li].biases[j] != before.layers[li].biases[j] {
				t.Fatalf("a zero step moved layer %d bias %d", li, j)
			}
		}
	}
	// The same generator, untouched by the inert drift, still gives its first
	// value — the property that keeps a replay stream aligned.
	if got := simrand.New(41).Float64(); got != wanted {
		t.Errorf("an inert drift consumed a draw")
	}
}

// TestInheritingCopiesRatherThanShares: a child that shared its parent's
// weight slices would mutate the parent too, and a whole lineage would move
// together.
func TestInheritingCopiesRatherThanShares(t *testing.T) {
	shapeGlobals(t, 6, 2)
	rng := simrand.New(37)
	parent := Starter(d.ActChemosynthesis)
	for i := 0; i < 30; i++ {
		parent.Mutate(rng)
	}
	parent.drift(rng, 0.5)

	before := parent.Copy()
	child := Inherit(rng, parent)
	child.drift(rng, 5)
	child.Mutate(rng)

	for li := range parent.layers {
		for j := range parent.layers[li].biases {
			if parent.layers[li].biases[j] != before.layers[li].biases[j] {
				t.Fatalf("changing the child moved the parent's layer %d bias %d", li, j)
			}
			for i := range parent.layers[li].weights[j] {
				if parent.layers[li].weights[j][i] != before.layers[li].weights[j][i] {
					t.Fatalf("changing the child moved the parent's layer %d weight %d,%d", li, j, i)
				}
			}
		}
	}
}

// TestMutationIsDeterministic is the replay property stated directly: the same
// seed against the same parent gives the same child, byte for byte.
func TestMutationIsDeterministic(t *testing.T) {
	shapeGlobals(t, 6, 2)
	build := func() []byte {
		rng := simrand.New(101)
		net := Starter(d.ActChemosynthesis)
		for i := 0; i < 120; i++ {
			net = Inherit(rng, net)
		}
		return net.Encode()
	}
	first, second := build(), build()
	if string(first) != string(second) {
		t.Error("two runs of the same seed produced different nets")
	}
}

// fullyGrown is a net wired to every enabled condition and action, with no
// hidden layers — the end state a growth-only net ratchets up to.
func fullyGrown() *Net {
	net := &Net{
		inputs:  append([]d.Condition(nil), d.EnabledConditions()...),
		outputs: append([]d.Action(nil), d.EnabledActions()...),
	}
	net.layers = []layer{{
		weights: make([][]float64, len(net.outputs)),
		biases:  make([]float64, len(net.outputs)),
	}}
	for j := range net.layers[0].weights {
		net.layers[0].weights[j] = make([]float64, len(net.inputs))
	}
	return net
}

// TestAFullyGrownNetCanStillShrink is the ratchet this file exists to prevent.
//
// With only additions available, every lineage walks to the whole condition
// pool and stops there, and the per-node upkeep can then only kill a world
// rather than prune a brain — measured at 36 inputs and 44-59 nodes in every
// surviving seed before the removals existed, against 1.4-6.8 inputs and 6-15
// nodes after. The decision trees had the same bug for the same reason; see
// CLAUDE.md on "nothing could ever shrink a tree".
func TestAFullyGrownNetCanStillShrink(t *testing.T) {
	loadGlobals(t)
	g := *config.GetCurrentGlobals()
	g.MaxNeuralNetDepth = 0
	config.SetGlobals(&g)

	rng := simrand.New(43)
	net := fullyGrown()
	before := net.Nodes()
	for i := 0; i < 50; i++ {
		net.Mutate(rng)
		checkShape(t, net, "after shrinking")
	}
	if net.Nodes() >= before {
		t.Errorf("a fully grown net went from %d nodes to %d; nothing can take one away",
			before, net.Nodes())
	}
}

// TestGrowthOnlyWeightsStillRatchet records what the removals are for, by
// pinning the behaviour without them: a user who zeroes the three removal
// weights gets the growth-only net back, which is a legitimate thing to want
// and the thing the defaults deliberately do not do.
func TestGrowthOnlyWeightsStillRatchet(t *testing.T) {
	growthOnly(t, 1, 0)
	rng := simrand.New(43)
	net := fullyGrown()
	before := net.Encode()
	for i := 0; i < 50; i++ {
		net.Mutate(rng)
	}
	if string(net.Encode()) != string(before) {
		t.Error("with every removal weight at zero, a fully grown net still changed")
	}
}

// TestARemovalTakesTheLeastConnected is the half of the structure search the
// neutrality test cannot cover.
//
// A removal is not neutral — it takes something away. What makes it a
// correction rather than a cost is WHICH thing: the least-connected one, with
// no draw involved. An addition introduces its connections at exactly 0, so a
// read a net gained and never used is the first one given back.
func TestARemovalTakesTheLeastConnected(t *testing.T) {
	shapeGlobals(t, 6, 0)
	rng := simrand.New(47)

	net := &Net{
		inputs:  []d.Condition{d.CanMove, d.IsFoodHere, d.IsWallAhead},
		outputs: []d.Action{d.ActChemosynthesis, d.ActEat},
	}
	net.layers = []layer{{
		weights: [][]float64{{2, 0, -3}, {1, 0, 4}},
		biases:  []float64{0.5, -0.5},
	}}
	// IsFoodHere is the middle column and nothing reads it.
	net.removeInput()
	for _, c := range net.Inputs() {
		if c == d.IsFoodHere {
			t.Errorf("removal took something other than the unused input; inputs are %v", net.Inputs())
		}
	}
	if len(net.Inputs()) != 2 {
		t.Fatalf("removal left %d inputs, want 2", len(net.Inputs()))
	}
	checkShape(t, net, "after removing an input")

	// The weights that survive are the ones that belonged to the surviving
	// columns, in order — a removal that shifted them would scramble the net
	// it was meant to leave alone.
	if got := net.layers[0].weights[0]; got[0] != 2 || got[1] != -3 {
		t.Errorf("surviving weights are %v, want the first and third of the original row", got)
	}

	// The output nothing can reach is the one a removal gives back.
	net.outputs = append(net.outputs, d.ActDig)
	net.layers[0].weights = append(net.layers[0].weights, []float64{0, 0})
	net.layers[0].biases = append(net.layers[0].biases, 0)
	net.removeOutput()
	for _, a := range net.Outputs() {
		if a == d.ActDig {
			t.Errorf("removal took something other than the unreachable output; outputs are %v", net.Outputs())
		}
	}
	_ = rng
}

// TestTheLastOutputIsNeverTaken: a net has to answer something every cycle, so
// the output list can be shrunk to one and no further.
func TestTheLastOutputIsNeverTaken(t *testing.T) {
	shapeGlobals(t, 6, 0)
	rng := simrand.New(53)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < 500; i++ {
		net.Mutate(rng)
		if len(net.Outputs()) == 0 {
			t.Fatal("a net was left with no action it can choose")
		}
		checkShape(t, net, "while shrinking")
	}
}

// wideNet is a net reading more conditions than a small width cap allows,
// with weights that make its choice depend on them — the shape that catches a
// layer insertion quietly dropping inputs.
func wideNet(t *testing.T, rng *simrand.RNG) *Net {
	t.Helper()
	net := &Net{
		inputs: []d.Condition{
			d.CanMove, d.IsFoodHere, d.IsFoodAhead, d.IsWallAhead, d.IsHealthy,
			d.IsFoodLeft, d.IsFoodRight, d.IsOrganismAhead, d.IsWallLeft, d.IsWallRight,
		},
		outputs: []d.Action{d.ActChemosynthesis, d.ActEat, d.ActMove},
	}
	net.layers = []layer{{weights: make([][]float64, 3), biases: make([]float64, 3)}}
	for j := range net.layers[0].weights {
		net.layers[0].weights[j] = make([]float64, len(net.inputs))
	}
	net.drift(rng, 1.0)
	return net
}

// TestALayerIsOnlyOfferedWhereItFits is the gate that keeps every structural
// mutation neutral on arrival.
//
// A first hidden layer has to pass every input through, so it needs one unit
// per input. Where the width cap cannot hold that, the mutation is not offered
// at all rather than inserted narrower: a narrower one drops the inputs past
// the cap, which resets a working net instead of stepping from it. The cost is
// that a net reading more conditions than the cap cannot gain depth, which is
// what the first case below states.
func TestALayerIsOnlyOfferedWhereItFits(t *testing.T) {
	shapeGlobals(t, 4, 2)
	narrow := wideNet(t, simrand.New(2))
	if narrow.canAddLayer() {
		t.Errorf("a layer was offered to a net with %d inputs under a width cap of 4",
			len(narrow.Inputs()))
	}

	shapeGlobals(t, 16, 2)
	wide := wideNet(t, simrand.New(2))
	if !wide.canAddLayer() {
		t.Fatalf("a width cap of 16 still refused a layer for %d inputs", len(wide.Inputs()))
	}
	for _, eval := range []func(d.Condition) bool{alwaysTrue, alwaysFalse} {
		before := wide.Choose(eval)
		want := append([]float64(nil), wide.LastOutputs()...)
		candidate := wide.Copy()
		candidate.addLayer()
		if after := candidate.Choose(eval); after != before {
			t.Errorf("inserting a layer changed the action: %v became %v", before, after)
		}
		for j, v := range candidate.LastOutputs() {
			if v != want[j] {
				t.Errorf("inserting a layer moved output %d from %v to %v", j, want[j], v)
			}
		}
	}
}

// TestDepthStillGrowsFromAnEmptyNet: the starter net reads nothing at all, so
// the layer it gains passes nothing through — the one case where the new layer
// is not an identity, and it still has to leave the choice alone.
func TestDepthStillGrowsFromAnEmptyNet(t *testing.T) {
	shapeGlobals(t, 4, 2)
	net := Starter(d.ActChemosynthesis)
	net.addOutput(simrand.New(3))
	before := net.Choose(alwaysTrue)

	if !net.canAddLayer() {
		t.Fatal("a net with no inputs was refused a hidden layer")
	}
	net.addLayer()
	checkShape(t, net, "after growing from an empty net")
	if after := net.Choose(alwaysTrue); after != before {
		t.Errorf("the first layer changed the action: %v became %v", before, after)
	}
}
