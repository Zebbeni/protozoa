package neural

import (
	"math"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/simrand"
)

// Starter is the net a founder begins with: no inputs, no hidden layers and
// one output.
//
// The counterpart of decision.TreeFromAction(ActChemosynthesis), and minimal
// for the same reason — a founder that already read the world would start the
// search somewhere a lineage was meant to arrive at. With one output the
// argmax is that output whatever the weights say, so this net chemosynthesises
// every cycle until a mutation gives it a second action to choose between.
func Starter(action d.Action) *Net {
	return &Net{
		outputs: []d.Action{action},
		layers: []layer{{
			weights: [][]float64{{}},
			biases:  []float64{0},
		}},
	}
}

// Copy is a deep copy. The scratch buffers are deliberately not carried over:
// they are workspace, and sharing them would make two organisms share one.
//
// The activations ARE carried, because a copy is what the panel is handed to
// draw — the simulation may be writing to the organism's own net while the
// panel reads it — and without them the tab has no values to show and no way
// to tell which connections are carrying anything.
func (n *Net) Copy() *Net {
	out := &Net{
		inputs:  append([]d.Condition(nil), n.inputs...),
		outputs: append([]d.Action(nil), n.outputs...),
		layers:  make([]layer, len(n.layers)),
		acts:    make([][]float64, len(n.acts)),
	}
	for i, a := range n.acts {
		out.acts[i] = append([]float64(nil), a...)
	}
	for i, l := range n.layers {
		cl := layer{
			weights: make([][]float64, len(l.weights)),
			biases:  append([]float64(nil), l.biases...),
		}
		for j, w := range l.weights {
			cl.weights[j] = append([]float64(nil), w...)
		}
		out.layers[i] = cl
	}
	return out
}

// Inherit is what a child's net is built with: a copy of the parent's, every
// weight drifted a little, then at most one structural change.
//
// The drift is unconditional and the structure is a chance, which is the
// shape the two kinds of change want: weights are how a lineage tunes what it
// already has, and structure is how it reaches for something new. Drifting
// first means a connection a structural mutation adds this generation starts
// at exactly the 0 it was introduced with, so gaining an input cannot change
// behaviour on the cycle it arrives — only in the generation after it.
func Inherit(rng *simrand.RNG, parent *Net) *Net {
	child := parent.Copy()
	child.drift(rng, config.NeuralWeightMutationStep())
	if rng.Float64() < config.ChanceToMutateNeuralNet() {
		child.Mutate(rng)
	}
	return child
}

// drift moves every weight and bias by up to step either way.
//
// One draw per weight, so the number of draws depends on the parent's net
// rather than on luck — the same property the tree's mutation has, and what a
// replay needs.
func (n *Net) drift(rng *simrand.RNG, step float64) {
	if step <= 0 {
		return
	}
	for li := range n.layers {
		l := &n.layers[li]
		for j := range l.biases {
			l.biases[j] += (rng.Float64()*2 - 1) * step
			for i := range l.weights[j] {
				l.weights[j][i] += (rng.Float64()*2 - 1) * step
			}
		}
	}
}

// kind is one structural change a net can undergo.
type kind int

const (
	addInput kind = iota
	removeInput
	addOutput
	removeOutput
	addLayer
	widenLayer
	narrowLayer
)

// Mutate applies exactly one structural change, or none when the net has no
// room for any. Every change is introduced with zero weights, so it is
// behaviour-neutral on arrival and selection judges it only once drift has
// given it something to say.
func (n *Net) Mutate(rng *simrand.RNG) {
	kinds, weights := n.available()
	if len(kinds) == 0 {
		return
	}
	switch kinds[pickWeighted(rng, weights)] {
	case addInput:
		n.addInput(rng)
	case removeInput:
		n.removeInput()
	case addOutput:
		n.addOutput(rng)
	case removeOutput:
		n.removeOutput()
	case addLayer:
		n.addLayer()
	case widenLayer:
		n.widenLayer(rng)
	case narrowLayer:
		n.narrowLayer()
	}
}

// available is the kinds this net has room for, with their settings weights
// beside them. A kind that cannot apply is left out rather than drawn and
// discarded, which would spend a lineage's mutation on nothing.
func (n *Net) available() ([]kind, []float64) {
	var kinds []kind
	var weights []float64
	// A kind with no weight is left out rather than offered at zero, so a
	// user who zeroes one has held it back — including when it is the only
	// kind the net has room for, which is where "offer it anyway and let the
	// draw sort it out" would shrink a net whose removals were switched off.
	add := func(k kind, w float64) {
		if w <= 0 {
			return
		}
		kinds = append(kinds, k)
		weights = append(weights, w)
	}
	if len(n.unwiredConditions()) > 0 {
		add(addInput, config.MutationWeightAddInput())
	}
	// A net is never reduced below the minimum it is asked to read, the same
	// way the last output is never taken: a net with nothing to sense repeats
	// one action whatever the world does, and nothing about being unable to
	// react stops it being successful by luck.
	if len(n.inputs) > max(config.MinNeuralNetInputs(), 0) {
		add(removeInput, config.MutationWeightRemoveInput())
	}
	if len(n.unwiredActions()) > 0 {
		add(addOutput, config.MutationWeightAddOutput())
	}
	// A net with one output has no choice to make but still has to answer
	// something, so the last one is never taken away.
	if len(n.outputs) > 1 {
		add(removeOutput, config.MutationWeightRemoveOutput())
	}
	if n.canAddLayer() {
		add(addLayer, config.MutationWeightAddLayer())
	}
	if n.Depth() > 0 && n.narrowestHidden() < config.MaxNeuralNetWidth() {
		add(widenLayer, config.MutationWeightWidenLayer())
	}
	if n.Depth() > 0 && n.widestHidden() > 1 {
		add(narrowLayer, config.MutationWeightNarrowLayer())
	}
	return kinds, weights
}

// canAddLayer reports whether a hidden layer can be inserted WITHOUT losing
// anything, which is the only way it is offered.
//
// The new layer passes its input through unchanged, so it needs one unit per
// value it is handed — and the width cap bounds hidden layers. Beyond the
// first hidden layer that is always satisfiable, since what it is handed is
// another hidden layer and already fits. The case it rules out is the 0-to-1
// step for a net reading more conditions than `max_neural_net_width` allows:
// that net cannot gain depth until the cap is raised. The alternative was
// inserting a narrower layer that drops the inputs past the cap, which is not
// a step from what the lineage had but a reset of it, and lineages would lose
// a working net to every attempt.
func (n *Net) canAddLayer() bool {
	if n.Depth() >= config.MaxNeuralNetDepth() {
		return false
	}
	width := config.MaxNeuralNetWidth()
	if width < 1 {
		return false
	}
	// A net with no inputs passes nothing through, so a single unit holds it.
	return max(n.inputWidth(len(n.layers)-1), 1) <= width
}

// pickWeighted takes ONE draw whatever the weights are, so a settings change
// cannot shift how many values a mutation consumes and with it every later
// draw in the run. Every weight it is handed is positive — available() drops
// the rest — and the zero guards below are what keeps that an invariant
// rather than an assumption.
func pickWeighted(rng *simrand.RNG, weights []float64) int {
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	roll := rng.Float64()
	if total <= 0 {
		return 0
	}
	target := roll * total
	running := 0.0
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		running += w
		if target < running {
			return i
		}
	}
	// Only reachable through floating-point rounding at the very top of the
	// range; the last positive weight is the answer it was approaching.
	for i := len(weights) - 1; i >= 0; i-- {
		if weights[i] > 0 {
			return i
		}
	}
	return 0
}

// unwiredConditions is the enabled conditions this net does not already read.
// A net cannot wire one twice: a second copy would be a duplicate column
// carrying the same value, which drift can already express by moving the
// first one.
func (n *Net) unwiredConditions() []d.Condition {
	have := make(map[d.Condition]bool, len(n.inputs))
	for _, c := range n.inputs {
		have[c] = true
	}
	var out []d.Condition
	for _, c := range d.EnabledConditions() {
		if !have[c] {
			out = append(out, c)
		}
	}
	return out
}

func (n *Net) unwiredActions() []d.Action {
	have := make(map[d.Action]bool, len(n.outputs))
	for _, a := range n.outputs {
		have[a] = true
	}
	var out []d.Action
	for _, a := range d.EnabledActions() {
		if !have[a] {
			out = append(out, a)
		}
	}
	return out
}

// narrowestHidden is the hidden layer widenLayer will reach for, so the cap is
// tested against the layer that can actually grow.
func (n *Net) narrowestHidden() int {
	narrowest := -1
	for _, l := range n.layers[:len(n.layers)-1] {
		if narrowest < 0 || len(l.biases) < narrowest {
			narrowest = len(l.biases)
		}
	}
	if narrowest < 0 {
		return 0
	}
	return narrowest
}

// widestHidden is the hidden layer narrowLayer will reach for, so repeated
// narrowing evens the stack out the way repeated widening does.
func (n *Net) widestHidden() int {
	widest := 0
	for _, l := range n.layers[:len(n.layers)-1] {
		widest = max(widest, len(l.biases))
	}
	return widest
}

// The removals all take the LEAST connected candidate, and they take it
// without an rng draw.
//
// That makes removal the near-inverse of the addition it undoes: an addition
// introduces its connections at exactly 0, so the thing a removal reaches for
// first is whatever was added most recently and has not yet earned a weight.
// A net that grows a read it never uses gives it back, instead of paying its
// upkeep for the rest of the lineage. Picking at random would instead take a
// working connection as often as a dead one, which makes shrinking a cost
// rather than a correction.
//
// Ties go to the lowest index, like the output readout, so two equally unused
// candidates do not need a draw to separate them.

// leastConnected is the index of the column with the smallest total weight
// magnitude across the units reading it.
func leastConnected(units [][]float64, columns int) int {
	best, bestWeight := 0, math.Inf(1)
	for i := 0; i < columns; i++ {
		total := 0.0
		for _, w := range units {
			if i < len(w) {
				total += math.Abs(w[i])
			}
		}
		if total < bestWeight {
			best, bestWeight = i, total
		}
	}
	return best
}

// removeInput drops the condition the net leans on least, and its column from
// the first layer.
func (n *Net) removeInput() {
	idx := leastConnected(n.layers[0].weights, len(n.inputs))
	n.inputs = append(n.inputs[:idx], n.inputs[idx+1:]...)
	first := &n.layers[0]
	for j := range first.weights {
		first.weights[j] = append(first.weights[j][:idx], first.weights[j][idx+1:]...)
	}
}

// removeOutput drops the action the net is least likely to ever choose: the
// output unit with the smallest weights and bias behind it.
func (n *Net) removeOutput() {
	out := &n.layers[len(n.layers)-1]
	idx, best := 0, math.Inf(1)
	for j := range out.biases {
		total := math.Abs(out.biases[j])
		for _, w := range out.weights[j] {
			total += math.Abs(w)
		}
		if total < best {
			idx, best = j, total
		}
	}
	n.outputs = append(n.outputs[:idx], n.outputs[idx+1:]...)
	out.biases = append(out.biases[:idx], out.biases[idx+1:]...)
	out.weights = append(out.weights[:idx], out.weights[idx+1:]...)
}

// narrowLayer drops the least-used unit from the widest hidden layer, and the
// column it fed into the layer after it.
func (n *Net) narrowLayer() {
	widest := n.widestHidden()
	target := -1
	for i, l := range n.layers[:len(n.layers)-1] {
		if len(l.biases) == widest {
			target = i
			break
		}
	}
	if target < 0 {
		return
	}
	next := &n.layers[target+1]
	idx := leastConnected(next.weights, len(n.layers[target].biases))

	l := &n.layers[target]
	l.biases = append(l.biases[:idx], l.biases[idx+1:]...)
	l.weights = append(l.weights[:idx], l.weights[idx+1:]...)
	for j := range next.weights {
		next.weights[j] = append(next.weights[j][:idx], next.weights[j][idx+1:]...)
	}
}

// addInput wires one more condition, with a zero weight into every unit of the
// first layer.
func (n *Net) addInput(rng *simrand.RNG) {
	pool := n.unwiredConditions()
	n.inputs = append(n.inputs, pool[rng.Intn(len(pool))])
	first := &n.layers[0]
	for j := range first.weights {
		first.weights[j] = append(first.weights[j], 0)
	}
}

// addOutput wires one more action: a unit on the output layer with zero
// weights and a zero bias.
func (n *Net) addOutput(rng *simrand.RNG) {
	pool := n.unwiredActions()
	n.outputs = append(n.outputs, pool[rng.Intn(len(pool))])
	out := &n.layers[len(n.layers)-1]
	out.weights = append(out.weights, make([]float64, n.inputWidth(len(n.layers)-1)))
	out.biases = append(out.biases, 0)
}

// addLayer inserts a hidden layer immediately before the output layer, wired
// as an identity map, so the layer it follows reaches the output layer exactly
// as it did before.
//
// Identity rather than zeros, because a zeroed layer cuts every signal and
// turns a working net into its biases. It is exactly an identity — never a
// truncated one — because canAddLayer has already refused the case where the
// cap could not hold it. ReLU is what makes the pass-through exact: every
// value it carries is either an input (0 or 1) or a hidden activation, and
// both are already non-negative, so flooring them changes nothing.
//
// The output layer keeps its own weights untouched, since the values reaching
// it are the same values in the same order.
func (n *Net) addLayer() {
	prev := n.inputWidth(len(n.layers) - 1)
	width := max(prev, 1)
	hidden := layer{
		weights: make([][]float64, width),
		biases:  make([]float64, width),
	}
	for j := 0; j < width; j++ {
		hidden.weights[j] = make([]float64, prev)
		if j < prev {
			hidden.weights[j][j] = 1
		}
	}
	out := n.layers[len(n.layers)-1]
	if prev == 0 {
		// Nothing was reaching the output layer, and now one always-zero unit
		// does; its weight has to exist and has to be 0.
		for j := range out.weights {
			out.weights[j] = make([]float64, width)
		}
	}
	n.layers = append(n.layers[:len(n.layers)-1], hidden, out)
}

// widenLayer adds one unit to the narrowest hidden layer, with zero weights in
// and zero weights out.
func (n *Net) widenLayer(rng *simrand.RNG) {
	var candidates []int
	narrowest := n.narrowestHidden()
	for i, l := range n.layers[:len(n.layers)-1] {
		if len(l.biases) == narrowest {
			candidates = append(candidates, i)
		}
	}
	idx := candidates[rng.Intn(len(candidates))]
	l := &n.layers[idx]
	l.weights = append(l.weights, make([]float64, n.inputWidth(idx)))
	l.biases = append(l.biases, 0)
	// The layer after it gains a zero column for the new unit.
	next := &n.layers[idx+1]
	for j := range next.weights {
		next.weights[j] = append(next.weights[j], 0)
	}
}

// inputWidth is how many values layer i reads: the inputs for the first layer,
// the layer before it otherwise.
func (n *Net) inputWidth(i int) int {
	if i == 0 {
		return len(n.inputs)
	}
	return len(n.layers[i-1].biases)
}

// NewFounder is the net a founder starts the simulation with: a Starter, plus
// however many structural mutations the settings ask for.
func NewFounder(rng *simrand.RNG, action d.Action) *Net {
	net := Starter(action)
	// A founder STARTS at the minimum rather than merely being held above it.
	// The floor only stops removals, so a founder left at zero would sit there
	// until chance happened to add a read — and under a per-node upkeep that
	// pressure runs the other way.
	//
	// Bounded by the pool as well as by the setting, since a minimum higher
	// than the number of enabled conditions is not reachable and must not spin.
	for len(net.inputs) < config.MinNeuralNetInputs() && len(net.unwiredConditions()) > 0 {
		net.addInput(rng)
	}
	for i := 0; i < config.InitialNeuralNetMutations(); i++ {
		net.Mutate(rng)
	}
	return net
}
