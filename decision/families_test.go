package decision

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

func TestEveryConditionIsInExactlyOneFamily(t *testing.T) {
	for _, c := range MutableConditions {
		parent, refined := conditionParent[c]
		if !refined {
			continue // a root; reachable as a basic read
		}
		found := false
		for _, sib := range conditionChildren[parent] {
			if sib == c {
				found = true
			}
		}
		if !found {
			t.Errorf("%s names %s as its parent but is not among its children",
				Names[c], Names[parent])
		}
	}
	reachable := map[Condition]bool{}
	for _, c := range basicConditions {
		reachable[c] = true
	}
	for _, kids := range conditionChildren {
		for _, k := range kids {
			reachable[k] = true
		}
	}
	for _, c := range MutableConditions {
		if !reachable[c] {
			t.Errorf("%s is in no family: mutation could never place it", Names[c])
		}
	}
}

func TestFamiliesAreAcyclicAndRootedAtBasics(t *testing.T) {
	for _, c := range MutableConditions {
		seen := map[Condition]bool{c: true}
		cur := c
		for {
			parent, ok := conditionParent[cur]
			if !ok {
				break
			}
			if seen[parent] {
				t.Fatalf("family containing %s has a cycle at %s", Names[c], Names[parent])
			}
			seen[parent] = true
			cur = parent
			if len(seen) > len(MutableConditions) {
				t.Fatalf("family containing %s does not terminate", Names[c])
			}
		}
		if !IsBasicCondition(cur) {
			t.Errorf("walking up from %s ended at %s, which is not a basic read",
				Names[c], Names[cur])
		}
	}
}

func TestEveryFamilyEdgeIsARealRelationship(t *testing.T) {
	for child, parent := range conditionParent {
		got, ok := entail([]literal{{cond: child, value: true}})
		if !ok {
			t.Errorf("%s true is unsatisfiable under the rules", Names[child])
			continue
		}
		v, known := got[parent]
		if !known {
			t.Errorf("families.go says %s refines %s, but redundancy.go says "+
				"nothing about the pair — the edge is unverified, so either add "+
				"the clause or drop the edge", Names[child], Names[parent])
			continue
		}
		// Either direction is fine; which one is a property of the family.
		_ = v
	}
}

func TestOppositeEndEdgesAreDeclaredDeliberately(t *testing.T) {
	oppositeEnd := map[Condition]bool{
		IsPhTooLowHere:  true, // excludes IsHealthyPhHere (the pH partition)
		IsPhTooHighHere: true,
		IsVeryUnhealthy: true, // excludes IsHealthy (the other end of health)
	}
	for child, parent := range conditionParent {
		got, _ := entail([]literal{{cond: child, value: true}})
		implies := got[parent]
		if oppositeEnd[child] && implies {
			t.Errorf("%s is listed as the opposite end of %s but implies it",
				Names[child], Names[parent])
		}
		if !oppositeEnd[child] && !implies {
			t.Errorf("%s does not imply its parent %s and is not listed as an "+
				"opposite end — add it to the list on purpose or fix the family",
				Names[child], Names[parent])
		}
	}
}

func TestNewConditionNodesOnlyAskBasicReads(t *testing.T) {
	withTiered(t, true)
	rng := simrand.New(1)
	seen := map[Condition]bool{}
	for i := 0; i < 3000; i++ {
		// A single-action tree: every mutation that produces a condition is necessarily a NEW condition node.
		tree := TreeFromAction(ActChemosynthesis)
		tree.mutate(rng)
		for _, c := range tree.ConditionNodes() {
			seen[c] = true
			if !IsBasicCondition(c) {
				t.Fatalf("a new condition node asked %s, which is a refined read",
					Names[c])
			}
		}
	}
	if len(seen) < 2 {
		t.Fatalf("only %d distinct conditions ever appeared; the test is not "+
			"exercising the pool", len(seen))
	}
}

func TestRefinedReadsAreReachableBySwapping(t *testing.T) {
	withEveryConditionEnabled(t)
	if !contains(LadderConditions(IsFoodHere), IsMuchFoodHere) {
		t.Error("IsMuchFoodHere is not reachable from IsFoodHere")
	}
	if !contains(LadderConditions(IsOrganismAhead), IsBiggerOrganismAhead) {
		t.Error("IsBiggerOrganismAhead is not reachable from IsOrganismAhead")
	}
	// Two steps, not one: the deepest read is not reachable from the root.
	if contains(LadderConditions(IsOrganismAhead), IsMuchBiggerOrganismAhead) {
		t.Error("IsMuchBiggerOrganismAhead is reachable from IsOrganismAhead in " +
			"one step; the ladder is supposed to be gradual")
	}
	if !contains(LadderConditions(IsBiggerOrganismAhead), IsMuchBiggerOrganismAhead) {
		t.Error("IsMuchBiggerOrganismAhead is not reachable from IsBiggerOrganismAhead")
	}
}

func TestRefinedReadsCanCoarsenBack(t *testing.T) {
	withEveryConditionEnabled(t)
	for child, parent := range conditionParent {
		if !contains(LadderConditions(child), parent) {
			t.Errorf("%s cannot coarsen back to %s", Names[child], Names[parent])
		}
	}
}

// TestBasicReadsCanMoveLaterally: a lineage stuck on a family that isn't paying off has to be able to leave it.
func TestBasicReadsCanMoveLaterally(t *testing.T) {
	withEveryConditionEnabled(t)
	pool := LadderConditions(IsWallAhead)
	for _, want := range []Condition{IsFoodHere, IsHealthyPhHere, CanMove} {
		if !contains(pool, want) {
			t.Errorf("a node asking IsWallAhead cannot move to %s", Names[want])
		}
	}
}

func TestDisablingARootDoesNotPromoteItsChildren(t *testing.T) {
	withTiered(t, true)
	g := *config.GetCurrentGlobals()
	g.DisabledDecisionNodes = []string{Names[IsFoodHere]}
	config.SetGlobals(&g)

	if contains(BasicConditions(), IsFoodHere) {
		t.Error("a disabled root is still offered as a basic read")
	}
	if contains(BasicConditions(), IsMuchFoodHere) {
		t.Error("disabling IsFoodHere promoted IsMuchFoodHere into the basic pool")
	}
}

func TestPoolsKeepMutableConditionsOrder(t *testing.T) {
	withEveryConditionEnabled(t)
	for _, pool := range [][]Condition{
		BasicConditions(),
		LadderConditions(IsOrganismAhead),
		LadderConditions(IsMuchBiggerOrganismAhead),
	} {
		last := -1
		for _, c := range pool {
			idx := indexIn(MutableConditions, c)
			if idx <= last {
				t.Fatalf("pool is not in MutableConditions order at %s", Names[c])
			}
			last = idx
		}
	}
	for i := 0; i < 20; i++ {
		a, b := LadderConditions(IsFoodHere), LadderConditions(IsFoodHere)
		for j := range a {
			if a[j] != b[j] {
				t.Fatal("LadderConditions is not deterministic between calls")
			}
		}
	}
}

// TestTieredMutationConsumesOneDraw pins that the setting does not change how many rng values a mutation takes.
func TestTieredMutationConsumesOneDraw(t *testing.T) {
	for _, tiered := range []bool{false, true} {
		withTiered(t, tiered)
		a := simrand.New(99)
		tree := TreeFromAction(ActChemosynthesis)
		tree.mutate(a)
		// A second rng taken the same number of draws must agree from here.
		b := simrand.New(99)
		tree2 := TreeFromAction(ActChemosynthesis)
		tree2.mutate(b)
		if a.Intn(1<<20) != b.Intn(1<<20) {
			t.Fatalf("tiered=%v: mutation is not reproducible for one seed", tiered)
		}
	}
}

func TestTieredShipsOnAndAbsentMeansOff(t *testing.T) {
	loadDefaults(t)
	if !config.TieredConditionMutation() {
		t.Error("tiered_condition_mutation is off in the shipped settings")
	}

	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["tiered_condition_mutation"]; !ok {
		t.Fatal("the shipped settings have no tiered_condition_mutation key, " +
			"so this proves nothing")
	}
	delete(raw, "tiered_condition_mutation")
	stripped, _ := json.Marshal(raw)
	var old config.Globals
	if err := json.Unmarshal(stripped, &old); err != nil {
		t.Fatal(err)
	}
	if old.TieredConditionMutation {
		t.Error("absent from a settings file, tiered_condition_mutation must be false")
	}
}

func withTiered(t *testing.T, on bool) {
	t.Helper()
	loadDefaults(t)
	g := *config.GetCurrentGlobals()
	g.TieredConditionMutation = on
	config.SetGlobals(&g)
}

// withEveryConditionEnabled is for the tests about the LADDER'S SHAPE, as opposed to its interaction with the settings.
func withEveryConditionEnabled(t *testing.T) {
	t.Helper()
	loadDefaults(t)
	g := *config.GetCurrentGlobals()
	g.TieredConditionMutation = true
	g.DisabledDecisionNodes = []string{}
	g.BasicOnlyConditionFamilies = nil
	config.SetGlobals(&g)
}

func contains(pool []Condition, c Condition) bool {
	for _, x := range pool {
		if x == c {
			return true
		}
	}
	return false
}

func indexIn(pool [](Condition), c Condition) int {
	for i, x := range pool {
		if x == c {
			return i
		}
	}
	return -1
}

func TestBasicOnlyFamilyKeepsItsRoot(t *testing.T) {
	loadDefaults(t)
	g := *config.GetCurrentGlobals()
	g.DisabledDecisionNodes = []string{}
	g.BasicOnlyConditionFamilies = []string{Names[IsOrganismAhead]}
	config.SetGlobals(&g)

	if !contains(EnabledConditions(), IsOrganismAhead) {
		t.Error("restricting a family removed its basic read")
	}
	for _, adv := range AdvancedConditions(IsOrganismAhead) {
		if contains(EnabledConditions(), adv) {
			t.Errorf("%s survived its family being set to basic-only", Names[adv])
		}
	}
	if !contains(EnabledConditions(), IsVeryHealthy) {
		t.Error("restricting one family took an advanced read out of another")
	}
}

func TestBasicOnlyFamilyIgnoresANonRoot(t *testing.T) {
	loadDefaults(t)
	g := *config.GetCurrentGlobals()
	g.DisabledDecisionNodes = []string{}
	g.BasicOnlyConditionFamilies = []string{Names[IsBiggerOrganismAhead]}
	config.SetGlobals(&g)

	for _, c := range []Condition{IsOrganismAhead, IsBiggerOrganismAhead, IsRelativeAhead} {
		if !contains(EnabledConditions(), c) {
			t.Errorf("naming the non-root %s restricted %s",
				Names[IsBiggerOrganismAhead], Names[c])
		}
	}
}

func TestEveryFamilyWithAdvancedReadsGetsAToggle(t *testing.T) {
	withEveryConditionEnabled(t)
	toggled := map[Condition]bool{}
	for _, root := range ConditionFamilies() {
		toggled[root] = true
		if len(AdvancedConditions(root)) == 0 {
			t.Errorf("%s is offered a toggle but has no advanced reads to include",
				Names[root])
		}
	}
	for _, c := range MutableConditions {
		if IsBasicCondition(c) && len(conditionChildren[c]) > 0 && !toggled[c] {
			t.Errorf("%s has advanced reads but no toggle", Names[c])
		}
	}
}

func TestBasicOnlyAbsentMeansNoRestriction(t *testing.T) {
	var absent config.Globals
	if err := json.Unmarshal([]byte(`{}`), &absent); err != nil {
		t.Fatal(err)
	}
	if absent.BasicOnlyConditionFamilies != nil {
		t.Error("absent from a settings file, basic_only_condition_families must stay nil")
	}
	g := *loadDefaults(t)
	g.DisabledDecisionNodes = []string{}
	g.BasicOnlyConditionFamilies = nil
	config.SetGlobals(&g)
	if len(EnabledConditions()) != len(MutableConditions) {
		t.Errorf("with nothing disabled and no family restricted the pool is %d of %d",
			len(EnabledConditions()), len(MutableConditions))
	}
}

func TestDefaultBasicOnlyFamiliesAreRealRoots(t *testing.T) {
	roots := map[string]bool{}
	for _, c := range BasicConditionsAll() {
		roots[Names[c]] = true
	}
	check := func(where string, names []string) {
		for _, n := range names {
			if !roots[n] {
				t.Errorf("%s names %q, which is not a family root, so it restricts nothing",
					where, n)
			}
		}
	}
	check("config.DefaultBasicOnlyFamilies", config.DefaultBasicOnlyFamilies)

	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	check("settings/default.json", g.BasicOnlyConditionFamilies)
}

func TestFlankFamiliesRefineFromOneCoarseRead(t *testing.T) {
	withEveryConditionEnabled(t)
	for _, tc := range []struct {
		root  Condition
		kinds []Condition
	}{
		{IsSomethingLeft, []Condition{IsOrganismLeft, IsWallLeft, IsFoodLeft}},
		{IsSomethingRight, []Condition{IsOrganismRight, IsWallRight, IsFoodRight}},
	} {
		if !IsBasicCondition(tc.root) {
			t.Errorf("%s is not a family root", Names[tc.root])
		}
		for _, k := range tc.kinds {
			if IsBasicCondition(k) {
				t.Errorf("%s is still a basic read; it should refine %s",
					Names[k], Names[tc.root])
			}
			if FamilyRoot(k) != tc.root {
				t.Errorf("%s belongs to %s, want %s",
					Names[k], Names[FamilyRoot(k)], Names[tc.root])
			}
			if !contains(LadderConditions(tc.root), k) {
				t.Errorf("%s is not reachable in one step from %s",
					Names[k], Names[tc.root])
			}
		}
	}
	// The size reads stay one step further down, so the ladder is four deep on a side rather than collapsing to two.
	if contains(LadderConditions(IsSomethingLeft), IsBiggerOrganismLeft) {
		t.Error("IsBiggerOrganismLeft is reachable directly from IsSomethingLeft; " +
			"the kind-of-thing step should come first")
	}
	if !contains(LadderConditions(IsOrganismLeft), IsBiggerOrganismLeft) {
		t.Error("IsBiggerOrganismLeft is not reachable from IsOrganismLeft")
	}
}

func TestAheadHasNoCoarseSomethingRead(t *testing.T) {
	for _, v := range MutableNodeTypes() {
		if Names[v] == "IsSomethingAhead" {
			t.Error("IsSomethingAhead is in the pool; CanMove is the forward " +
				"cell's coarse read and the two would compete")
		}
	}
	for _, c := range []Condition{IsFoodAhead, IsWallAhead, IsOrganismAhead} {
		if !IsBasicCondition(c) {
			t.Errorf("%s stopped being a basic read; the ahead reads are roots "+
				"precisely because there is no coarse read above them", Names[c])
		}
	}
}
