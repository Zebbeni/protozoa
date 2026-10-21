package neural

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/simrand"
)

// withInputFloor is the shipped settings with the floor set to n.
func withInputFloor(t *testing.T, n int) {
	t.Helper()
	loadGlobals(t)
	g := *config.GetCurrentGlobals()
	g.MinNeuralNetInputs = n
	config.SetGlobals(&g)
}

// hasKind reports whether a mutation kind is among the net's candidates.
func hasKind(n *Net, want kind) bool {
	kinds, _ := n.available()
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

// TestMutationWillNotStripANetBelowItsFloor is the point of the setting: a
// net reading the minimum is not offered removeInput at all, the same way the
// last output is never taken away. A brain that senses nothing repeats one
// action whatever is in front of it, and nothing about being unable to react
// stops it being successful by luck — which is how it was reported.
func TestMutationWillNotStripANetBelowItsFloor(t *testing.T) {
	withInputFloor(t, 2)
	rng := simrand.New(7)

	net := NewFounder(rng, d.ActChemosynthesis)
	if got := len(net.Inputs()); got != 2 {
		t.Fatalf("a founder under a floor of 2 reads %d conditions", got)
	}
	if hasKind(net, removeInput) {
		t.Error("a net at its floor is offered removeInput")
	}

	// One more read, and taking one away is on the table again.
	net.addInput(rng)
	if !hasKind(net, removeInput) {
		t.Error("a net above its floor is not offered removeInput")
	}
}

// TestALineageNeverFallsBelowTheFloor walks the mutation path a real lineage
// takes rather than inspecting the candidate list, because the floor is only
// worth anything if it holds across hundreds of generations of drift.
func TestALineageNeverFallsBelowTheFloor(t *testing.T) {
	const floor = 2
	withInputFloor(t, floor)
	rng := simrand.New(23)

	net := NewFounder(rng, d.ActChemosynthesis)
	low := len(net.Inputs())
	for i := 0; i < 400; i++ {
		net = Inherit(rng, net)
		if got := len(net.Inputs()); got < floor {
			t.Fatalf("generation %d reads %d conditions, below the floor of %d", i, got, floor)
		} else if got < low {
			low = got
		}
	}
	t.Logf("over 400 generations the lineage read between %d and %d conditions", low, len(net.Inputs()))
}

// TestNoFloorLeavesMutationAsItWas: 0 is what a settings file written before
// this decodes to, so it has to mean exactly the old behaviour — a founder
// that senses nothing, and a net of one read that can still lose it.
func TestNoFloorLeavesMutationAsItWas(t *testing.T) {
	withInputFloor(t, 0)
	rng := simrand.New(7)

	net := NewFounder(rng, d.ActChemosynthesis)
	if got := len(net.Inputs()); got != 0 {
		t.Errorf("with no floor a founder reads %d conditions, want none", got)
	}
	net.addInput(rng)
	if !hasKind(net, removeInput) {
		t.Error("with no floor a net of one read is not offered removeInput")
	}
	net.removeInput()
	if got := len(net.Inputs()); got != 0 {
		t.Errorf("with no floor a net kept %d conditions after a removal", got)
	}
	if hasKind(net, removeInput) {
		t.Error("a net reading nothing is offered removeInput")
	}
}

// TestAFloorAboveThePoolStopsAtThePool: the floor is a setting and the
// condition pool is not, so a value above it must come to rest rather than
// spin looking for a read that does not exist.
func TestAFloorAboveThePoolStopsAtThePool(t *testing.T) {
	pool := len(d.EnabledConditions())
	withInputFloor(t, pool+5)
	rng := simrand.New(7)

	net := NewFounder(rng, d.ActChemosynthesis)
	if got := len(net.Inputs()); got != pool {
		t.Errorf("a founder under an unreachable floor reads %d of the %d conditions available",
			got, pool)
	}
	if hasKind(net, removeInput) {
		t.Error("a net holding the whole pool under an unreachable floor is offered removeInput")
	}
}
