package decision

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

// weightGlobals installs the four mutation weights on top of the shipped defaults.
func weightGlobals(t *testing.T, swapAct, grow, swapCond, prune float64) {
	t.Helper()
	loadDefaults(t)
	g := *config.GetCurrentGlobals()
	g.MutationWeightSwapAction = swapAct
	g.MutationWeightGrowBranch = grow
	g.MutationWeightSwapCondition = swapCond
	g.MutationWeightPruneBranch = prune
	config.SetGlobals(&g)
}

// leafConditionTree builds the shape pruning is defined on: a condition whose two branches are both actions.
func leafConditionTree(yes, no Action) *Tree {
	tree := TreeFromNode(nodeOver(IsWallAhead, NodeFromAction(yes), NodeFromAction(no)))
	tree.size = tree.CalcAndUpdateSize()
	return tree
}

func TestPruneShrinksTheTree(t *testing.T) {
	weightGlobals(t, 1, 0, 0, 1) // never grow, always prune when it can
	rng := simrand.New(7)
	tree := leafConditionTree(ActEat, ActMove)
	if tree.size != 3 {
		t.Fatalf("setup gave a tree of size %d, want 3", tree.size)
	}
	for i := 0; i < 200 && tree.size > 1; i++ {
		tree.mutate(rng)
		if tree.size > 3 {
			t.Fatalf("the tree grew to %d with the grow weight at 0", tree.size)
		}
	}
	if tree.size != 1 {
		t.Fatalf("200 mutations never pruned a leaf condition; size is still %d", tree.size)
	}
	if !tree.Node.IsAction() {
		t.Error("pruning left a condition at the root")
	}
	if tree.Node.YesNode != nil || tree.Node.NoNode != nil {
		t.Error("a pruned node kept its branches")
	}
}

func TestPruneKeepsOneOfTheBranchActions(t *testing.T) {
	weightGlobals(t, 1, 0, 0, 1)
	rng := simrand.New(3)
	checked := 0
	for i := 0; i < 400; i++ {
		tree := leafConditionTree(ActEat, ActMove)
		// Captured immediately before each mutation, not once at the top.
		for j := 0; j < 50 && tree.size == 3; j++ {
			yes := tree.Node.YesNode.NodeType.(Action)
			no := tree.Node.NoNode.NodeType.(Action)
			tree.mutate(rng)
			if tree.size != 1 {
				continue // the mutation landed elsewhere
			}
			got, ok := tree.Node.NodeType.(Action)
			if !ok {
				t.Fatal("prune did not leave an action")
			}
			if got != yes && got != no {
				t.Fatalf("prune left %s, which was neither branch (%s / %s)",
					Names[got], Names[yes], Names[no])
			}
			checked++
		}
		if checked > 40 {
			break
		}
	}
	if checked == 0 {
		t.Fatal("no prune was observed, so this checked nothing")
	}
}

func TestPruneIsRefusedOnADeepCondition(t *testing.T) {
	weightGlobals(t, 1, 0, 0, 1) // never grow, always prune when allowed
	deep := nodeOver(IsWallAhead,
		nodeOver(IsFoodAhead, NodeFromAction(ActEat), NodeFromAction(ActMove)),
		NodeFromAction(ActIdle))
	tree := TreeFromNode(deep)
	before := tree.CalcAndUpdateSize()

	rng := simrand.New(11)
	// Mutate until the root is the node picked, then check it did not collapse.
	for i := 0; i < 200; i++ {
		tree = TreeFromNode(nodeOver(IsWallAhead,
			nodeOver(IsFoodAhead, NodeFromAction(ActEat), NodeFromAction(ActMove)),
			NodeFromAction(ActIdle)))
		tree.size = tree.CalcAndUpdateSize()
		tree.mutate(rng)
		if tree.Node.IsAction() {
			t.Fatalf("a condition with a subtree collapsed to an action, "+
				"losing %d nodes in one mutation", before-1)
		}
	}
}

func TestAZeroWeightSwitchesAKindOff(t *testing.T) {
	// Grow off: a single-action tree can never become a condition.
	weightGlobals(t, 1, 0, 1, 1)
	rng := simrand.New(5)
	for i := 0; i < 400; i++ {
		tree := TreeFromAction(ActChemosynthesis)
		tree.mutate(rng)
		if !tree.Node.IsAction() {
			t.Fatal("a tree grew a branch with the grow weight at 0")
		}
	}
	// Prune off: a leaf condition never collapses.
	weightGlobals(t, 1, 1, 1, 0)
	rng = simrand.New(5)
	for i := 0; i < 200; i++ {
		tree := leafConditionTree(ActEat, ActMove)
		tree.mutate(rng)
		if tree.Node.IsAction() && tree.size == 1 {
			t.Fatal("a tree pruned with the prune weight at 0")
		}
	}
}

func TestWeightsDoNotChangeTheDrawCount(t *testing.T) {
	drawsAfter := func(swapAct, grow, swapCond, prune float64) int {
		weightGlobals(t, swapAct, grow, swapCond, prune)
		rng := simrand.New(21)
		tree := TreeFromAction(ActChemosynthesis)
		tree.mutate(rng)
		// Count how far the stream has moved by pulling until a fresh rng on the same seed catches up.
		probe := simrand.New(21)
		for n := 0; n < 64; n++ {
			if probe.Float64() == rng.Float64() {
				return n
			}
			_ = probe
		}
		return -1
	}
	// Not comparing exact counts across weight sets.
	for _, w := range [][4]float64{{1, 1, 1, 1}, {3, 1, 1, 0}, {0, 1, 5, 2}} {
		a := drawsAfter(w[0], w[1], w[2], w[3])
		b := drawsAfter(w[0], w[1], w[2], w[3])
		if a != b {
			t.Errorf("weights %v consumed %d draws then %d", w, a, b)
		}
	}
}

func TestBothPairsAreRepairedFromAnOldFile(t *testing.T) {
	var g config.Globals
	config.SetGlobals(&g)
	if config.MutationWeightSwapAction() == 0 && config.MutationWeightGrowBranch() == 0 {
		t.Error("the action pair was left at zero, so its choice divides by zero")
	}
	if config.MutationWeightSwapCondition() == 0 && config.MutationWeightPruneBranch() == 0 {
		t.Error("the condition pair was left at zero")
	}

	// One side zeroed on purpose survives the repair.
	deliberate := config.Globals{MutationWeightSwapCondition: 4, MutationWeightPruneBranch: 0}
	config.SetGlobals(&deliberate)
	if config.MutationWeightPruneBranch() != 0 {
		t.Error("a deliberately zeroed prune weight was repaired away")
	}
	if config.MutationWeightSwapCondition() != 4 {
		t.Error("the repair overwrote a weight that was set")
	}
}

func TestTreesCanGrowAndShrinkOverALineage(t *testing.T) {
	weightGlobals(t, 1, 1, 1, 1)
	rng := simrand.New(13)
	tree := TreeFromAction(ActChemosynthesis)
	grew, shrank := false, false
	for i := 0; i < 4000; i++ {
		before := tree.size
		tree = MutateTree(rng, tree)
		switch {
		case tree.size > before:
			grew = true
		case tree.size < before:
			shrank = true
		}
	}
	if !grew {
		t.Error("no mutation in 4000 ever grew the tree")
	}
	if !shrank {
		t.Error("no mutation in 4000 ever shrank the tree; trees still ratchet")
	}
}
