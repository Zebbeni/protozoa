package effects

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestBrainUpkeepScalesWithNodesAndSize is the exact arithmetic the live-world
// test can only approximate, since an organism grows within the cycle it is
// charged in.
func TestBrainUpkeepScalesWithNodesAndSize(t *testing.T) {
	g := &config.Globals{HealthChangePerBrainNode: -0.01}

	for _, nodes := range []int{1, 7, 40} {
		for _, size := range []float64{1, 12.5, 100} {
			want := size * float64(nodes) * g.HealthChangePerBrainNode
			if got := BrainUpkeep(g, nodes, size); got != want {
				t.Errorf("%d nodes at size %v cost %v, want %v", nodes, size, got, want)
			}
		}
	}
}

// TestABrainlessOrganismPaysNothing: a decision tree has no nodes to charge,
// and the upkeep has to be exactly zero rather than a rounding of one — it is
// added into the same apply as the pH damage, so a non-zero value here would
// move the health of every organism in a world that has no nets at all.
func TestABrainlessOrganismPaysNothing(t *testing.T) {
	g := &config.Globals{HealthChangePerBrainNode: -0.01}
	for _, nodes := range []int{0, -1} {
		if got := BrainUpkeep(g, nodes, 50); got != 0 {
			t.Errorf("%d nodes cost %v, want exactly 0", nodes, got)
		}
	}
}

// TestTheUpkeepIsOffAtZero: the setting at 0 has to leave the arithmetic
// untouched, which is what lets a neural world be run without it.
func TestTheUpkeepIsOffAtZero(t *testing.T) {
	g := &config.Globals{HealthChangePerBrainNode: 0}
	if got := BrainUpkeep(g, 200, 100); got != 0 {
		t.Errorf("a zero setting charged %v", got)
	}
}
