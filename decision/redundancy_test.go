package decision

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

// offered reports whether a condition survives the filter at a spot in a tree described by the facts above it and the conditions below it.
func offered(t *testing.T, cond Condition, above []literal, below []downstream) bool {
	t.Helper()
	for _, c := range usefulConditions(MutableConditions, above, below) {
		if c == cond {
			return true
		}
	}
	return false
}

// mustOffer / mustSkip read at the call site as the claim being made.
func mustOffer(t *testing.T, above []literal, conds ...Condition) {
	t.Helper()
	for _, c := range conds {
		if !offered(t, c, above, nil) {
			t.Errorf("%s was withheld, but nothing above it settles its answer", Names[c])
		}
	}
}

func mustSkip(t *testing.T, above []literal, conds ...Condition) {
	t.Helper()
	for _, c := range conds {
		if offered(t, c, above, nil) {
			t.Errorf("%s was offered, but its answer is already settled above it", Names[c])
		}
	}
}

func TestAConditionIsNotAskedTwice(t *testing.T) {
	mustSkip(t, []literal{yes(IsFoodAhead)}, IsFoodAhead)
	mustSkip(t, []literal{no(IsFoodAhead)}, IsFoodAhead)
}

func TestAStrongerReadSettlesTheWeakerOnes(t *testing.T) {
	above := []literal{yes(IsMuchBiggerOrganismAhead)}
	mustSkip(t, above,
		IsMuchBiggerOrganismAhead,
		IsBiggerOrganismAhead,
		IsOrganismAhead,
		IsMuchSmallerOrganismAhead,
	)
	// Its neighbours to the sides are a different cell and say nothing.
	mustOffer(t, above, IsOrganismLeft, IsBiggerOrganismRight, IsWallAhead, IsFoodAhead)
}

func TestTheNoBranchOfAStrongerReadSettlesNothing(t *testing.T) {
	above := []literal{no(IsMuchBiggerOrganismAhead)}
	mustOffer(t, above,
		IsOrganismAhead,
		IsBiggerOrganismAhead,
		IsMuchSmallerOrganismAhead,
		IsRelativeAhead,
	)
	mustSkip(t, above, IsMuchBiggerOrganismAhead)
}

func TestAWeakerAnswerLeavesTheStrongerWorthAsking(t *testing.T) {
	above := []literal{yes(IsOrganismAhead)}
	mustOffer(t, above,
		IsBiggerOrganismAhead,
		IsMuchBiggerOrganismAhead,
		IsMuchSmallerOrganismAhead,
		IsRelativeAhead,
	)
	mustSkip(t, above, IsOrganismAhead)
}

func TestNoOrganismSettlesEveryReadThatNeedsOne(t *testing.T) {
	for _, f := range organismFamilies {
		above := []literal{no(f.occupied)}
		mustSkip(t, above, f.bigger, f.muchBigger, f.muchSmaller, f.relative)
	}
}

func TestPhConditionsPartitionTheScale(t *testing.T) {
	mustSkip(t, []literal{yes(IsHealthyPhHere)}, IsPhTooLowHere, IsPhTooHighHere)
	mustSkip(t, []literal{yes(IsPhTooLowHere)}, IsHealthyPhHere, IsPhTooHighHere)
	mustSkip(t, []literal{no(IsHealthyPhHere), no(IsPhTooLowHere)}, IsPhTooHighHere)

	// One negative on its own leaves two possibilities open.
	mustOffer(t, []literal{no(IsHealthyPhHere)}, IsPhTooLowHere, IsPhTooHighHere)
}

func TestNestedThresholdsSettleTheLooserOne(t *testing.T) {
	mustSkip(t, []literal{yes(IsVeryHealthy)}, IsHealthy)
	mustSkip(t, []literal{no(IsHealthy)}, IsVeryHealthy)
	mustOffer(t, []literal{yes(IsHealthy)}, IsVeryHealthy)

	mustSkip(t, []literal{yes(IsAgeMultipleOfTen)}, IsAgeMultipleOfTwo)
	mustOffer(t, []literal{yes(IsAgeMultipleOfTwo)}, IsAgeMultipleOfTen)
}

func TestCanMoveSettlesWhatIsInTheWay(t *testing.T) {
	above := []literal{yes(CanMove)}
	mustSkip(t, above, IsOrganismAhead, IsBiggerOrganismAhead)
	mustOffer(t, above, IsWallAhead, IsFoodAhead)

	mustSkip(t, []literal{yes(IsOrganismAhead)}, CanMove)
	mustOffer(t, []literal{yes(IsWallAhead)}, CanMove)
	mustOffer(t, []literal{yes(IsFoodAhead)}, CanMove)
}

// condTree builds a condition node over two action leaves, for the downstream tests.
func condTree(c Condition) *Node {
	n := NodeFromCondition(c)
	n.YesNode = NodeFromAction(ActIdle)
	n.NoNode = NodeFromAction(ActIdle)
	n.CalcAndUpdateSize()
	return n
}

// nodeOver builds a condition node whose branches are the two given subtrees, either of which may be nil for a plain action leaf.
func nodeOver(c Condition, yesSub, noSub *Node) *Node {
	n := NodeFromCondition(c)
	n.YesNode, n.NoNode = yesSub, noSub
	if n.YesNode == nil {
		n.YesNode = NodeFromAction(ActIdle)
	}
	if n.NoNode == nil {
		n.NoNode = NodeFromAction(ActIdle)
	}
	n.CalcAndUpdateSize()
	return n
}

func TestAConditionIsNotOfferedIfItStrandsOneBelow(t *testing.T) {
	// A node whose YES branch goes on to ask whether an organism is ahead.
	node := nodeOver(IsWallLeft, condTree(IsOrganismAhead), nil)
	below := node.downstreamConditions()
	if len(below) != 1 {
		t.Fatalf("expected one condition below, got %d", len(below))
	}

	if offered(t, IsMuchBiggerOrganismAhead, nil, below) {
		t.Error("IsMuchBiggerOrganismAhead was offered; its YES branch settles the IsOrganismAhead below it")
	}
	if offered(t, IsOrganismAhead, nil, below) {
		t.Error("IsOrganismAhead was offered over a node asking the same thing")
	}
	// Something reading elsewhere leaves the lower node doing its job.
	if !offered(t, IsFoodRight, nil, below) {
		t.Error("IsFoodRight was withheld; it settles nothing below it")
	}
}

func TestStrandingIsBranchAware(t *testing.T) {
	yesSide := nodeOver(IsWallLeft, condTree(IsOrganismAhead), nil)
	noSide := nodeOver(IsWallLeft, nil, condTree(IsOrganismAhead))

	if offered(t, IsMuchBiggerOrganismAhead, nil, yesSide.downstreamConditions()) {
		t.Error("stranded on the YES branch but still offered")
	}
	if !offered(t, IsMuchBiggerOrganismAhead, nil, noSide.downstreamConditions()) {
		t.Error("withheld over a NO branch, where not-much-bigger settles nothing")
	}
}

func TestStrandingLooksAllTheWayDown(t *testing.T) {
	deep := nodeOver(IsWallLeft, nodeOver(IsFoodLeft, condTree(IsOrganismAhead), nil), nil)
	if offered(t, IsMuchBiggerOrganismAhead, nil, deep.downstreamConditions()) {
		t.Error("a grandchild was left stranded")
	}
}

func TestAlreadySettledSubtreeBlocksNothing(t *testing.T) {
	deep := condTree(IsHealthy)
	settledAlready := nodeOver(IsWallLeft, nodeOver(IsFoodLeft, nodeOver(IsVeryHealthy, deep, nil), nil), nil)
	below := settledAlready.downstreamConditions()
	if strandsSomethingBelow(nil, IsHealthy, below) {
		t.Error("a candidate was rejected for a redundancy that was already there")
	}
	// Checked against the pool as well as the predicate, because the two fail differently.
	pool := usefulConditions(MutableConditions, nil, below)
	if len(pool) >= len(MutableConditions) {
		t.Errorf("the filter returned all %d conditions, so it fell back instead of filtering", len(pool))
	}
	if !offered(t, IsHealthy, nil, below) {
		t.Error("a candidate was rejected for a redundancy that was already there")
	}

	// The same shape without that intermediate: now the candidate is what settles the lower node, and is withheld.
	freshlyStranded := nodeOver(IsWallLeft, nodeOver(IsFoodLeft, condTree(IsHealthy), nil), nil)
	if offered(t, IsHealthy, nil, freshlyStranded.downstreamConditions()) {
		t.Error("a candidate that strands a lower node on its own was offered")
	}
}

func TestFilteredPoolKeepsDeclarationOrder(t *testing.T) {
	got := usefulConditions(MutableConditions, []literal{yes(IsOrganismAhead)}, nil)
	if len(got) >= len(MutableConditions) {
		t.Fatal("the filter removed nothing, so this proves nothing")
	}
	last := -1
	for _, c := range got {
		idx := -1
		for i, m := range MutableConditions {
			if m == c {
				idx = i
				break
			}
		}
		if idx <= last {
			t.Fatalf("%s is out of declaration order", Names[c])
		}
		last = idx
	}
}

func TestFilterFallsBackRatherThanEmptying(t *testing.T) {
	pool := []Condition{IsOrganismAhead, IsBiggerOrganismAhead}
	got := usefulConditions(pool, []literal{no(IsOrganismAhead)}, nil)
	if len(got) != len(pool) {
		t.Fatalf("filtering to nothing returned %d conditions, want the whole pool", len(got))
	}
}

func TestAContradictoryPathFallsBack(t *testing.T) {
	above := []literal{yes(IsMuchBiggerOrganismAhead), no(IsOrganismAhead)}
	got := usefulConditions(MutableConditions, above, nil)
	if len(got) != len(MutableConditions) {
		t.Fatalf("a contradictory path filtered the pool to %d; it should fall back", len(got))
	}
}

func TestEveryRuleIsAboutRegisteredConditions(t *testing.T) {
	known := map[Condition]bool{}
	for _, c := range Conditions {
		known[c] = true
	}
	for i, cl := range conditionRules {
		if len(cl) == 0 {
			t.Errorf("rule %d is empty", i)
		}
		for _, l := range cl {
			if !known[l.cond] {
				t.Errorf("rule %d names %v, which is not a registered condition", i, l.cond)
			}
		}
	}
}

func TestEveryDirectionGetsTheSameRules(t *testing.T) {
	if len(organismFamilies) != 3 {
		t.Fatalf("expected three directions, got %d", len(organismFamilies))
	}
	seen := map[Condition]bool{}
	for _, f := range organismFamilies {
		for _, c := range []Condition{f.occupied, f.bigger, f.muchBigger, f.muchSmaller, f.relative} {
			if seen[c] {
				t.Errorf("%s appears in two direction families", Names[c])
			}
			seen[c] = true
		}
		// Each family answers its own four reads from an empty cell and nothing in another direction.
		mustSkip(t, []literal{no(f.occupied)}, f.bigger, f.muchBigger, f.muchSmaller, f.relative)
	}
	// Every condition naming an organism in some direction is in a family.
	for _, c := range MutableConditions {
		switch c {
		case IsOrganismAhead, IsOrganismLeft, IsOrganismRight,
			IsBiggerOrganismAhead, IsBiggerOrganismLeft, IsBiggerOrganismRight,
			IsMuchBiggerOrganismAhead, IsMuchBiggerOrganismLeft, IsMuchBiggerOrganismRight,
			IsMuchSmallerOrganismAhead, IsMuchSmallerOrganismLeft, IsMuchSmallerOrganismRight,
			IsRelativeAhead, IsRelativeLeft, IsRelativeRight:
			if !seen[c] {
				t.Errorf("%s reads a neighbouring organism but is in no direction family", Names[c])
			}
		}
	}
}

func TestSmartMutationShipsOnAndAbsentMeansOff(t *testing.T) {
	loadDefaults(t)
	if !config.SmartTreeMutation() {
		t.Error("smart_tree_mutation is off in the shipped settings")
	}

	// The same settings with the key removed.
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["smart_tree_mutation"]; !ok {
		t.Fatal("the shipped settings have no smart_tree_mutation key, so this proves nothing")
	}
	delete(raw, "smart_tree_mutation")
	stripped, _ := json.Marshal(raw)
	var old config.Globals
	if err := json.Unmarshal(stripped, &old); err != nil {
		t.Fatal(err)
	}
	config.SetGlobals(&old)
	if config.SmartTreeMutation() {
		t.Error("a settings file without the key decoded to smart mutation on; it would replay differently from how it was recorded")
	}
}

// enableEverything switches on every node type and turns smart mutation on.
func enableEverything(t *testing.T) {
	t.Helper()
	g := loadDefaults(t)
	g.DisabledDecisionNodes = []string{}
	g.SmartTreeMutation = true
	g.MaxDecisionTreeSize = 32
	config.SetGlobals(g)
}

// settledNodes walks a tree and returns every node asking a question its own ancestors have already answered.
func settledNodes(t *Tree) []Condition {
	var out []Condition
	var walk func(n *Node, above []literal)
	walk = func(n *Node, above []literal) {
		if n == nil || n.IsAction() {
			return
		}
		cond := n.NodeType.(Condition)
		if known, ok := entail(above); ok {
			if _, isSettled := known[cond]; isSettled {
				out = append(out, cond)
			}
		}
		walk(n.YesNode, append(append([]literal{}, above...), yes(cond)))
		walk(n.NoNode, append(append([]literal{}, above...), no(cond)))
	}
	walk(t.Node, nil)
	return out
}

func TestSmartMutationGrowsNoSettledBranches(t *testing.T) {
	enableEverything(t)
	rng := simrand.New(20260926)

	for seed := 0; seed < 200; seed++ {
		tree := TreeFromAction(ActIdle)
		for i := 0; i < 40; i++ {
			tree = MutateTree(rng, tree)
		}
		if bad := settledNodes(tree); len(bad) > 0 {
			t.Fatalf("tree %d asks %s, which its ancestors already answered:\n%s",
				seed, Names[bad[0]], tree.Print())
		}
	}
}

func TestWithoutSmartMutationTheNoiseIsThere(t *testing.T) {
	g := loadDefaults(t)
	g.DisabledDecisionNodes = []string{}
	g.SmartTreeMutation = false
	g.MaxDecisionTreeSize = 32
	config.SetGlobals(g)

	rng := simrand.New(20260926)
	const trees = 200
	withSettled, settledCount, conditionCount := 0, 0, 0
	for seed := 0; seed < trees; seed++ {
		tree := TreeFromAction(ActIdle)
		for i := 0; i < 40; i++ {
			tree = MutateTree(rng, tree)
		}
		bad := len(settledNodes(tree))
		if bad > 0 {
			withSettled++
		}
		settledCount += bad
		conditionCount += len(tree.ConditionNodes())
	}
	if withSettled == 0 {
		t.Error("no tree grew a settled branch without the filter, so the filter is not what the other test measured")
	}
	// The size of the problem, as measured when this shipped.
	t.Logf("filter off: %d of %d trees carry a settled branch; %d of %d condition nodes (%.1f%%) are dead",
		withSettled, trees, settledCount, conditionCount,
		100*float64(settledCount)/float64(conditionCount))
}

func TestMutationStaysReproducible(t *testing.T) {
	enableEverything(t)

	grow := func() string {
		rng := simrand.New(7)
		tree := TreeFromAction(ActIdle)
		for i := 0; i < 60; i++ {
			tree = MutateTree(rng, tree)
		}
		return tree.Serialize()
	}
	first := grow()
	for i := 0; i < 20; i++ {
		if got := grow(); got != first {
			t.Fatalf("run %d produced a different tree from the same seed", i+2)
		}
	}
}

// TestConditionPoolIsNeverEmpty: mutation draws with rng.Intn(len(pool)), which panics on an empty slice.
func TestConditionPoolIsNeverEmpty(t *testing.T) {
	enableEverything(t)
	rng := simrand.New(4242)

	for i := 0; i < 100; i++ {
		tree := TreeFromAction(ActIdle)
		for j := 0; j < 40; j++ {
			tree = MutateTree(rng, tree)
		}
		for _, n := range tree.getNodes() {
			if len(tree.conditionPool(n)) == 0 {
				t.Fatalf("no condition on offer at a node of:\n%s", tree.Print())
			}
		}
	}
}

func TestMuchFoodSettlesThePlainFoodRead(t *testing.T) {
	for _, pair := range [][2]Condition{
		{IsMuchFoodHere, IsFoodHere},
		{IsMuchFoodBuriedHere, IsFoodBuriedHere},
	} {
		much, any := pair[0], pair[1]

		// Much is true, so any is settled true.
		mustSkip(t, []literal{yes(much)}, any)
		// No food at all, so much is settled false (the contrapositive).
		mustSkip(t, []literal{no(any)}, much)
		// Food is there but the amount is open — the useful case.
		mustOffer(t, []literal{yes(any)}, much)
		mustOffer(t, []literal{no(much)}, any)
	}
}

func TestSurfaceAndBuriedFoodAreIndependent(t *testing.T) {
	for _, above := range [][]literal{
		{yes(IsFoodHere)}, {no(IsFoodHere)},
		{yes(IsMuchFoodHere)}, {no(IsMuchFoodHere)},
	} {
		mustOffer(t, above, IsFoodBuriedHere)
	}
	for _, above := range [][]literal{
		{yes(IsFoodBuriedHere)}, {no(IsFoodBuriedHere)},
	} {
		mustOffer(t, above, IsFoodHere)
	}
}
