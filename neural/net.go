// Package neural is the alternative to a decision tree: a feed-forward
// network whose inputs are the decision package's conditions and whose
// outputs are its actions.
//
// Which of the two an organism runs is a per-simulation setting
// (`use_neural_nets`), so the two never mix inside one world. Everything a
// tree reads, a net reads: the inputs are drawn from the same
// decision.EnabledConditions pool the tree's conditions come from, and the
// outputs from decision.EnabledActions, so disabling a node type holds it
// back from both.
package neural

import (
	"fmt"
	"math"

	d "github.com/Zebbeni/protozoa/decision"
)

// Net is one organism's brain: a list of input conditions, a stack of hidden
// layers, and an output layer with one unit per action it can choose.
//
// layers always holds at least the output layer, so layers[len(layers)-1] has
// one unit per entry in outputs, and layers[0] has one weight per entry in
// inputs. Depth is the number of HIDDEN layers, len(layers)-1, which is what
// `max_neural_net_depth` bounds.
type Net struct {
	inputs  []d.Condition
	outputs []d.Action
	layers  []layer

	// acts is every column's values from the most recent Choose: acts[0] is
	// the inputs and acts[i+1] is what layer i produced. Kept for the panel,
	// which draws each connection at the strength of what actually flowed
	// along it rather than at its weight — a weight says what an edge COULD
	// carry, and an edge out of a false condition carries nothing.
	//
	// Allocated once and reused, so evaluating a net still allocates nothing:
	// Choose runs for every organism on every cycle.
	acts [][]float64
}

// layer is one layer's weights and biases. weights[unit][prev] is the weight
// from unit `prev` of the layer before it, so the sum below runs in a fixed
// index order and cannot depend on map iteration.
type layer struct {
	weights [][]float64
	biases  []float64
}

// Layer is one layer's weights and biases as NewFrom takes them.
// Weights[unit][prev] is the weight from unit prev of the column before it,
// which is the order the net stores them in and the order the diagram draws.
type Layer struct {
	Weights [][]float64
	Biases  []float64
}

// NewFrom builds a net whose weights are chosen rather than evolved.
//
// Nothing in a running world calls it — every net an organism carries comes
// from NewFounder and mutation — but a net with KNOWN weights is what an
// illustration or a test outside this package needs, and the alternative is
// callers that cannot reach the unexported fields doing without.
//
// layers is the hidden layers followed by the OUTPUT layer, the same way the
// net stores them, so a 3>2>3 net is two entries: two units of three weights,
// then three units of two. It panics on a shape that does not line up, which
// is a mistake in a literal rather than anything a user can cause.
func NewFrom(inputs []d.Condition, outputs []d.Action, layers []Layer) *Net {
	if len(layers) == 0 {
		panic("neural.NewFrom: a net needs at least an output layer")
	}
	n := &Net{
		inputs:  append([]d.Condition(nil), inputs...),
		outputs: append([]d.Action(nil), outputs...),
		layers:  make([]layer, len(layers)),
	}
	prev := len(inputs)
	for i, spec := range layers {
		if len(spec.Weights) != len(spec.Biases) {
			panic(fmt.Sprintf("neural.NewFrom: layer %d has %d units of weights against %d biases",
				i, len(spec.Weights), len(spec.Biases)))
		}
		l := layer{
			weights: make([][]float64, len(spec.Weights)),
			biases:  append([]float64(nil), spec.Biases...),
		}
		for u, w := range spec.Weights {
			if len(w) != prev {
				panic(fmt.Sprintf("neural.NewFrom: layer %d unit %d has %d weights for a column of %d",
					i, u, len(w), prev))
			}
			l.weights[u] = append([]float64(nil), w...)
		}
		n.layers[i] = l
		prev = len(spec.Weights)
	}
	if prev != len(outputs) {
		panic(fmt.Sprintf("neural.NewFrom: the last layer has %d units against %d outputs",
			prev, len(outputs)))
	}
	return n
}

// Inputs and Outputs are the conditions the net reads and the actions it can
// choose, in the order its units are wired.
func (n *Net) Inputs() []d.Condition { return n.inputs }
func (n *Net) Outputs() []d.Action   { return n.outputs }

// Depth is the number of hidden layers: 0 is inputs wired straight to
// outputs.
func (n *Net) Depth() int { return len(n.layers) - 1 }

// Width is the widest hidden layer, or 0 when there are none.
func (n *Net) Width() int {
	w := 0
	for _, l := range n.layers[:len(n.layers)-1] {
		w = max(w, len(l.biases))
	}
	return w
}

// LayerSizes is the unit count of every layer including the output layer,
// for the panel.
func (n *Net) LayerSizes() []int {
	out := make([]int, len(n.layers))
	for i, l := range n.layers {
		out[i] = len(l.biases)
	}
	return out
}

// Nodes is what the per-cycle upkeep is charged on: every input, every hidden
// unit and every output.
//
// Inputs count because reading a condition is work the organism does — the
// tree pays nothing for the same reads, which is the asymmetry the setting
// exists to let a user price.
func (n *Net) Nodes() int {
	total := len(n.inputs)
	for _, l := range n.layers {
		total += len(l.biases)
	}
	return total
}

// Weights is the number of connections and biases, which is what a
// recording pays for per organism per snapshot.
func (n *Net) Weights() int {
	total := 0
	for _, l := range n.layers {
		total += len(l.biases)
		for _, w := range l.weights {
			total += len(w)
		}
	}
	return total
}

// LastOutputs is the output layer's values from the most recent Choose, in
// Outputs order, or nil before the first one. Display only.
func (n *Net) LastOutputs() []float64 { return n.Activations(len(n.layers)) }

// Activations is one column's values from the most recent Choose: column 0 is
// the inputs, where a condition that read true is 1, and column i+1 is what
// layer i produced. Nil before the first Choose, and nil for a column that
// does not exist — the panel draws from a copy taken on another goroutine.
func (n *Net) Activations(column int) []float64 {
	if column < 0 || column >= len(n.acts) {
		return nil
	}
	return n.acts[column]
}

// Transmitted is what actually flowed along one connection on the most recent
// Choose: the weight times the value of the unit feeding it.
//
// This is what the panel draws each edge at, rather than the weight. A weight
// is what an edge COULD carry; an edge out of a condition that read false
// carries nothing however heavy it is, and a diagram drawn from weights alone
// shows the same picture every cycle whatever the organism is looking at.
func (n *Net) Transmitted(layer, unit, prev int) float64 {
	w := n.WeightAt(layer, unit, prev)
	if w == 0 {
		return 0
	}
	in := n.Activations(layer)
	if prev < 0 || prev >= len(in) {
		return 0
	}
	return w * in[prev]
}

// StrongestTransmission is the largest magnitude flowing anywhere in the net
// right now, which is what the panel scales its line widths against.
//
// Against this cycle's own maximum rather than any fixed ceiling, because
// there isn't one: a hidden unit's value is a ReLU of a sum of weights a
// lineage has drifted to, with nothing bounding it. So a line's width says
// how strong that signal is against the strongest signal in the same picture,
// and widths are comparable within a frame rather than between frames.
func (n *Net) StrongestTransmission() float64 {
	strongest := 0.0
	for li := range n.layers {
		in := n.Activations(li)
		for j, row := range n.layers[li].weights {
			_ = j
			for i, w := range row {
				if i >= len(in) {
					break
				}
				if a := math.Abs(w * in[i]); a > strongest {
					strongest = a
				}
			}
		}
	}
	return strongest
}

// WeightAt is the connection from unit `prev` of the layer before `layer` to
// unit `unit` of it, for the panel's diagram. Out-of-range is 0 rather than a
// panic: the panel draws from a copy taken on another goroutine, and a net
// that grew between the copy and the draw must not take the render loop down.
func (n *Net) WeightAt(layer, unit, prev int) float64 {
	if layer < 0 || layer >= len(n.layers) {
		return 0
	}
	l := n.layers[layer]
	if unit < 0 || unit >= len(l.weights) {
		return 0
	}
	if prev < 0 || prev >= len(l.weights[unit]) {
		return 0
	}
	return l.weights[unit][prev]
}

// InputPull is the total magnitude of the connections leaving input i: how
// hard the net leans on that read, whatever the read happens to be saying this
// cycle. It is what the sensor overlay is derived from, where a decision tree
// counts the nodes that ask a condition — a net reads every input every cycle,
// so presence alone says nothing about how much of the brain is built on it.
//
// Magnitudes, so a read that pushes hard against an action counts as much as
// one that pushes for it. The connections OUT of the input only: what the net
// does with the value afterwards is not sensing.
func (n *Net) InputPull(i int) float64 {
	if len(n.layers) == 0 {
		return 0
	}
	total := 0.0
	for _, row := range n.layers[0].weights {
		if i >= 0 && i < len(row) {
			total += math.Abs(row[i])
		}
	}
	return total
}

// BiasAt is unit `unit` of `layer`'s own constant, added to everything
// reaching it before the activation. It is NOT a connection and the diagram
// draws nothing for it, which is why a unit can hold a value on a cycle when
// every input reads 0: with nothing flowing, a hidden unit is relu(its bias).
// Out of range is 0, like WeightAt.
func (n *Net) BiasAt(layer, unit int) float64 {
	if layer < 0 || layer >= len(n.layers) {
		return 0
	}
	l := n.layers[layer]
	if unit < 0 || unit >= len(l.biases) {
		return 0
	}
	return l.biases[unit]
}

// StrongestWeight is the largest magnitude in the net, which is what the
// diagram's WEIGHTS view scales its edges against so it reads the same
// whatever absolute size a lineage's weights have drifted to. Biases are left
// out: they are not connections and nothing draws them.
func (n *Net) StrongestWeight() float64 {
	strongest := 0.0
	for _, l := range n.layers {
		for _, row := range l.weights {
			for _, w := range row {
				if a := math.Abs(w); a > strongest {
					strongest = a
				}
			}
		}
	}
	return strongest
}

// relu is the activation, and it is deliberately NOT tanh or a sigmoid.
//
// A replay has to reproduce the run bit for bit, and this project ships a
// native build and a wasm one. math.Exp has had architecture-specific
// implementations, so a transcendental here would risk a browser replay
// diverging from the recording that made it. max(0, x) is one comparison
// and is exact everywhere.
func relu(x float64) float64 {
	if x < 0 {
		return 0
	}
	return x
}

// Choose evaluates the net against the world and returns the winning action.
//
// eval answers one input condition; the organism supplies it, since reading
// the world is the organism's job and the net knows only numbers. A true
// condition is 1 and a false one 0.
//
// Ties go to the lowest output index, which is the order the actions were
// wired in. An untied answer is not worth an rng draw: consuming one here
// would be a draw per organism per cycle, which no other decision path takes
// and which the replay stream would have to carry forever.
func (n *Net) Choose(eval func(d.Condition) bool) d.Action {
	n.sizeActivations()

	act := n.acts[0]
	for i, c := range n.inputs {
		act[i] = 0
		if eval(c) {
			act[i] = 1
		}
	}

	last := len(n.layers) - 1
	for li := range n.layers {
		l := &n.layers[li]
		next := n.acts[li+1]
		for j := range l.biases {
			sum := l.biases[j]
			for i, w := range l.weights[j] {
				sum += w * act[i]
			}
			// The output layer is left raw: ReLU would floor every negative
			// output at 0 and make a net with no positive output a tie its
			// wiring order decides, rather than a comparison.
			if li < last {
				sum = relu(sum)
			}
			next[j] = sum
		}
		act = next
	}

	best := 0
	for j := 1; j < len(act); j++ {
		if act[j] > act[best] {
			best = j
		}
	}
	return n.outputs[best]
}

// sizeActivations grows the activation buffers to this net's shape, which it
// only ever does on the first call or after a structural mutation.
func (n *Net) sizeActivations() {
	if len(n.acts) != len(n.layers)+1 {
		n.acts = make([][]float64, len(n.layers)+1)
	}
	n.sizeColumn(0, len(n.inputs))
	for li := range n.layers {
		n.sizeColumn(li+1, len(n.layers[li].biases))
	}
}

func (n *Net) sizeColumn(i, size int) {
	if cap(n.acts[i]) < size {
		n.acts[i] = make([]float64, size)
		return
	}
	n.acts[i] = n.acts[i][:size]
}

// ActionWeights is each action's share of the net's outputs, indexed by the
// action's own code — the counterpart of decision.Tree.ActionWeights, which
// the ACTION colour mode and the organism panel both read.
//
// A tree's weight is how much of the tree an action occupies; a net's is a
// flat share of what it can choose at all, because every output is reachable
// from every input and there is no branch structure to weigh. So the two
// numbers mean related but different things, which is why the colour key
// words itself differently in each mode.
func (n *Net) ActionWeights() []float64 {
	out := make([]float64, len(d.Actions))
	if n == nil || len(n.outputs) == 0 {
		return out
	}
	share := 1.0 / float64(len(n.outputs))
	for _, a := range n.outputs {
		if int(a) >= 0 && int(a) < len(out) {
			out[a] += share
		}
	}
	return out
}
