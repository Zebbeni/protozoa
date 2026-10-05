package decision

import (
	"math"
	"testing"
)

// TestActionWeightsAreShares: a big tree and a small one that do the same
// thing should read the same, so the weights are shares of the action nodes
// rather than counts.
func TestActionWeightsAreShares(t *testing.T) {
	loadDefaults(t)
	tree := TreeFromNode(nodeOver(IsFoodHere,
		NodeFromAction(ActEat),
		nodeOver(IsWallAhead, NodeFromAction(ActEat), NodeFromAction(ActMove))))

	w := tree.ActionWeights()
	if len(w) != len(Actions) {
		t.Fatalf("weights have %d entries, want one per action (%d)", len(w), len(Actions))
	}
	if got, want := w[ActEat], 2.0/3.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("ActEat weighs %v, want %v: two of three action nodes", got, want)
	}
	if got, want := w[ActMove], 1.0/3.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("ActMove weighs %v, want %v", got, want)
	}
	if w[ActDig] != 0 {
		t.Errorf("an action the tree never takes weighs %v, want 0", w[ActDig])
	}

	total := 0.0
	for _, v := range w {
		total += v
	}
	if math.Abs(total-1) > 1e-9 {
		t.Errorf("the weights sum to %v, want 1", total)
	}
}

// TestActionWeightsCountRepeats: a tree reaching one action down three
// branches is more of that organism than one reaching it once, which is the
// sense in which an action has weight in a tree.
func TestActionWeightsCountRepeats(t *testing.T) {
	loadDefaults(t)
	once := TreeFromNode(nodeOver(IsFoodHere, NodeFromAction(ActDig), NodeFromAction(ActMove)))
	twice := TreeFromNode(nodeOver(IsFoodHere, NodeFromAction(ActDig), NodeFromAction(ActDig)))
	if !(twice.ActionWeights()[ActDig] > once.ActionWeights()[ActDig]) {
		t.Error("reaching an action twice does not weigh more than reaching it once")
	}
}

// TestActionWeightsOfALoneAction: a one-node tree is entirely its action.
func TestActionWeightsOfALoneAction(t *testing.T) {
	loadDefaults(t)
	if got := TreeFromAction(ActChemosynthesis).ActionWeights()[ActChemosynthesis]; got != 1 {
		t.Errorf("a lone action weighs %v, want 1", got)
	}
}

// TestActionWeightsOfANilTree: Restore is handed a tree a snapshot may not
// carry, and the weights are computed there alongside the appearance, which
// has always tolerated nil.
func TestActionWeightsOfANilTree(t *testing.T) {
	var tree *Tree
	w := tree.ActionWeights()
	if len(w) != len(Actions) {
		t.Fatalf("a nil tree gave %d weights, want one per action", len(w))
	}
	for i, v := range w {
		if v != 0 {
			t.Errorf("action %d weighs %v on a nil tree, want 0", i, v)
		}
	}
}
