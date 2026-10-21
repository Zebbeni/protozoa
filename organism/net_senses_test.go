package organism

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"

	"github.com/Zebbeni/protozoa/neural"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
)

// grownTestNet is a net with enough generations behind it to have several
// inputs and real weights on them.
func grownTestNet(t *testing.T, seed uint64, generations int) *neural.Net {
	t.Helper()
	loadDesignGlobals(t)
	// Free mutation, which is what "grown" means here: min_neural_net_inputs
	// leaves removeInput out of the candidates once a net is down to the
	// floor, so the shipped value decides which reads this lineage ends with
	// and the test would be measuring that instead of the sense weighting.
	g := *config.GetCurrentGlobals()
	g.MinNeuralNetInputs = 0
	config.SetGlobals(&g)
	rng := simrand.New(seed)
	net := neural.Starter(d.ActChemosynthesis)
	for i := 0; i < generations; i++ {
		net = neural.Inherit(rng, net)
	}
	return net
}

// TestNetSensesPairEachReadWithItsOwnWeight is the join between the two halves
// of the rule, and an index slipped by one here would dress organisms by a
// read they do not make. Checked against the net directly rather than against
// a second copy of the sum.
func TestNetSensesPairEachReadWithItsOwnWeight(t *testing.T) {
	net := grownTestNet(t, 23, 200)
	inputs := net.Inputs()
	if len(inputs) < 2 {
		t.Fatalf("the net grew %d inputs; this needs at least two to be about anything", len(inputs))
	}

	senses := netSenses(net)
	if len(senses) != len(inputs) {
		t.Fatalf("%d senses against %d inputs", len(senses), len(inputs))
	}
	spread := false
	for i, s := range senses {
		if s.Condition != inputs[i] {
			t.Errorf("sense %d reads %v where the net's input is %v", i, s.Condition, inputs[i])
		}
		if s.Pull != net.InputPull(i) {
			t.Errorf("sense %d pulls %v where its input pulls %v", i, s.Pull, net.InputPull(i))
		}
		if i > 0 && s.Pull != senses[0].Pull {
			spread = true
		}
	}
	if !spread {
		t.Error("every input pulls alike, so a swapped index would not have shown")
	}
}

// TestANetsLookComesFromItsSenses: the three construction paths all go through
// appearanceOf, so the one thing to pin here is that a net organism is dressed
// by the weighted rule rather than by the counted one a tree gets.
func TestANetsLookComesFromItsSenses(t *testing.T) {
	net := grownTestNet(t, 23, 200)
	scores := physiology.Scores{8, 2, 2, 2, 2, 2, 2}
	if err := scores.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}

	got := appearanceOf(scores, nil, net)
	want := physiology.AppearanceForSenses(scores, netSenses(net))
	if got != want {
		t.Errorf("a net organism is dressed %+v, not the %+v its senses give", got, want)
	}
}
