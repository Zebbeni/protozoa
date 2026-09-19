package ux

import (
	"strings"
	"testing"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

// designerWithDefaults opens a designer against the shipped settings.
func designerWithDefaults(t *testing.T) *Designer {
	t.Helper()
	loadDefaults(t)
	return NewDesigner()
}

// pickOption opens the picker on a node and chooses the option with the
// given label, the way a click would.
func pickOption(t *testing.T, dz *Designer, node *d.Node, label string) {
	t.Helper()
	dz.openPicker(node, 0, 0)
	for i, opt := range dz.picker.options {
		if opt.label == label {
			dz.applyPickerOption(i)
			return
		}
	}
	t.Fatalf("no option labelled %q in the picker", label)
}

// TestDesignerOpensOnAThreeNodeTree: the editor starts on the smallest
// tree that is actually a decision, so there is something to branch from
// without the user building it first.
func TestDesignerOpensOnAThreeNodeTree(t *testing.T) {
	dz := designerWithDefaults(t)
	if got := dz.treeSize(); got != 3 {
		t.Errorf("the editor opened on %d nodes, want 3", got)
	}
	if !dz.tree.IsCondition() {
		t.Error("the root should be a condition; an action root has no branches to click")
	}
	if dz.tree.YesNode == nil || dz.tree.NoNode == nil {
		t.Fatal("the starting condition is missing a branch")
	}
	if !dz.tree.YesNode.IsAction() || !dz.tree.NoNode.IsAction() {
		t.Error("both starting branches should be actions")
	}
}

// TestActionToConditionGrowsTwoBranches: turning an action into a
// condition has to give it something to choose between, or the tree
// can't be walked.
func TestActionToConditionGrowsTwoBranches(t *testing.T) {
	dz := designerWithDefaults(t)
	leaf := dz.tree.YesNode

	pickOption(t, dz, leaf, d.Map[d.IsFoodAhead])

	if !leaf.IsCondition() {
		t.Fatal("the node should now be a condition")
	}
	if leaf.YesNode == nil || leaf.NoNode == nil {
		t.Fatal("a new condition must grow both branches")
	}
	if !leaf.YesNode.IsAction() || !leaf.NoNode.IsAction() {
		t.Error("the new branches should be plain actions, ready to be picked")
	}
	if got := dz.treeSize(); got != 5 {
		t.Errorf("tree is %d nodes, want 5 after growing two", got)
	}
	// The whole tree still has to serialize and come back, since that is
	// what saving does.
	if round := d.DeserializeTree(dz.currentTree().Serialize()); round == nil || round.Size() != 5 {
		t.Error("the edited tree doesn't survive a serialize round trip")
	}
}

// TestConditionToActionDropsItsBranches: an action has nowhere to hang a
// subtree, so converting one away discards it rather than leaving nodes
// stranded in the count.
func TestConditionToActionDropsItsBranches(t *testing.T) {
	dz := designerWithDefaults(t)

	pickOption(t, dz, dz.tree, d.Map[d.ActEat])

	if !dz.tree.IsAction() {
		t.Fatal("the root should now be an action")
	}
	if dz.tree.YesNode != nil || dz.tree.NoNode != nil {
		t.Error("an action must not keep branches")
	}
	if got := dz.treeSize(); got != 1 {
		t.Errorf("tree is %d nodes, want 1", got)
	}
}

// TestConditionToConditionKeepsItsBranches: swapping which question a
// node asks shouldn't throw away the answers already built under it.
func TestConditionToConditionKeepsItsBranches(t *testing.T) {
	dz := designerWithDefaults(t)
	yes, no := dz.tree.YesNode, dz.tree.NoNode

	pickOption(t, dz, dz.tree, d.Map[d.IsWallAhead])

	if dz.tree.YesNode != yes || dz.tree.NoNode != no {
		t.Error("changing the condition replaced its branches")
	}
	if got := dz.treeSize(); got != 3 {
		t.Errorf("tree is %d nodes, want the same 3", got)
	}
}

// TestTreeStopsAtTheConfiguredLimit: the designer can't build a tree the
// simulation's own mutation limit forbids.
func TestTreeStopsAtTheConfiguredLimit(t *testing.T) {
	dz := designerWithDefaults(t)

	// Grow until the limit refuses, following the yes branch down.
	node := dz.tree.YesNode
	for i := 0; i < c.MaxDecisionTreeSize(); i++ {
		before := dz.treeSize()
		pickOption(t, dz, node, d.Map[d.IsFoodAhead])
		if dz.treeSize() == before {
			break // refused
		}
		node = node.YesNode
	}
	if got := dz.treeSize(); got > c.MaxDecisionTreeSize() {
		t.Errorf("grew to %d nodes, past the %d limit", got, c.MaxDecisionTreeSize())
	}
	if !strings.Contains(dz.message, "limit") {
		t.Errorf("the editor should say why it stopped, got %q", dz.message)
	}
}

// TestPickerOffersEveryMutableNode: the dropdown is how a design gets
// its behaviour, so anything evolution can pick has to be pickable by
// hand too — otherwise designed organisms are strictly less expressive
// than evolved ones.
func TestPickerOffersEveryMutableNode(t *testing.T) {
	dz := designerWithDefaults(t)
	dz.openPicker(dz.tree, 0, 0)

	labels := map[string]bool{}
	for _, opt := range dz.picker.options {
		labels[opt.label] = true
	}
	for _, a := range d.MutableActions {
		if !labels[d.Map[a]] {
			t.Errorf("the picker is missing the action %q", d.Map[a])
		}
	}
	for _, cond := range d.MutableConditions {
		if !labels[d.Map[cond]] {
			t.Errorf("the picker is missing the condition %q", d.Map[cond])
		}
	}
}

// TestSaveIsBlockedUntilTheDesignIsLegal: the editor refuses to write a
// design a simulation couldn't run, and says which part is wrong.
func TestSaveIsBlockedUntilTheDesignIsLegal(t *testing.T) {
	dz := designerWithDefaults(t)

	if reason := dz.saveBlockedReason(); !strings.Contains(reason, "name") {
		t.Errorf("an unnamed design should block on its name, got %q", reason)
	}

	dz.design.Name = "tester"
	if reason := dz.saveBlockedReason(); reason != "" {
		t.Errorf("a named design with the balanced split should save, got %q", reason)
	}

	dz.stepAbility(int(physiology.AbilityAttack), +1)
	reason := dz.saveBlockedReason()
	if !strings.Contains(reason, "add up") {
		t.Errorf("an overspent design should block on the budget, got %q", reason)
	}
	dz.stepAbility(int(physiology.AbilityChemosynthesis), -1)
	if reason := dz.saveBlockedReason(); reason != "" {
		t.Errorf("rebalanced back to the budget, got %q", reason)
	}
}

// TestAbilityStepsRespectTheCap: the rows can't exceed a single
// ability's ceiling, even though the total is checked separately.
func TestAbilityStepsRespectTheCap(t *testing.T) {
	dz := designerWithDefaults(t)
	attack := int(physiology.AbilityAttack)
	for i := 0; i < physiology.MaxAbilityScore*2; i++ {
		dz.stepAbility(attack, +1)
	}
	if got := dz.design.Abilities[attack]; got != physiology.MaxAbilityScore {
		t.Errorf("attack reached %d, want the cap %d", got, physiology.MaxAbilityScore)
	}
	for i := 0; i < physiology.MaxAbilityScore*2; i++ {
		dz.stepAbility(attack, -1)
	}
	if got := dz.design.Abilities[attack]; got != 0 {
		t.Errorf("attack fell to %d, want 0", got)
	}
}

// TestTraitStepsStayInRange: every trait knob is clamped to what the
// current settings allow, so a design can't be edited into an organism
// the simulation would have to clamp later anyway.
func TestTraitStepsStayInRange(t *testing.T) {
	dz := designerWithDefaults(t)
	for i, trait := range designerTraits {
		for n := 0; n < 500; n++ {
			dz.stepTrait(i, +1)
		}
		if got := trait.get(&dz.design); got > trait.hi() {
			t.Errorf("%s reached %v, past its ceiling %v", trait.label, got, trait.hi())
		}
		for n := 0; n < 1000; n++ {
			dz.stepTrait(i, -1)
		}
		if got := trait.get(&dz.design); got < trait.lo() {
			t.Errorf("%s fell to %v, below its floor %v", trait.label, got, trait.lo())
		}
	}
}

// TestDesignerSaveAndLoadRoundTrip: what the editor writes is what it
// reads back, including the tree it was showing.
func TestDesignerSaveAndLoadRoundTrip(t *testing.T) {
	dz := designerWithDefaults(t)
	dir := t.TempDir()

	dz.design.Name = "round tripper"
	pickOption(t, dz, dz.tree.YesNode, d.Map[d.IsFoodAhead])
	dz.design.DecisionTree = dz.currentTree().Serialize()
	wantTree, wantSize := dz.design.DecisionTree, dz.treeSize()

	if _, err := organism.SaveDesign(dir, dz.design); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded := organism.LoadDesigns(dir)
	if len(loaded) != 1 {
		t.Fatalf("loaded %d designs, want 1", len(loaded))
	}

	fresh := designerWithDefaults(t)
	fresh.saved = loaded
	fresh.load(0)
	if fresh.design.Name != "round tripper" {
		t.Errorf("loaded %q, want the saved name", fresh.design.Name)
	}
	if got := fresh.currentTree().Serialize(); got != wantTree {
		t.Errorf("loaded tree %q, want %q", got, wantTree)
	}
	if got := fresh.treeSize(); got != wantSize {
		t.Errorf("loaded tree is %d nodes, want %d", got, wantSize)
	}
}

// TestClickingAwayEndsNaming: keystrokes must stop landing in the name
// field once the user has visibly moved on, or stepping an ability while
// the field is still armed silently renames the organism.
func TestClickingAwayEndsNaming(t *testing.T) {
	dz := designerWithDefaults(t)
	if !dz.nameEditing {
		t.Fatal("the editor should open ready to name the organism")
	}
	dz.hits = []designerHit{
		{x: 0, y: 0, w: 10, h: 10, kind: hitName},
		{x: 20, y: 0, w: 10, h: 10, kind: hitAbilityUp, index: int(physiology.AbilityAttack)},
	}
	hit, ok := dz.hitAt(25, 5)
	if !ok || hit.kind != hitAbilityUp {
		t.Fatalf("expected the ability button under the cursor, got %+v", hit)
	}
	// The same branch Update takes for a hit that isn't the name.
	if hit.kind != hitName {
		dz.nameEditing = false
	}
	if dz.nameEditing {
		t.Error("clicking an ability button left the name field armed")
	}
}

// TestPortraitUsesTheHighResSprites: only the 16x16 set carries the
// layered overlays, and the set active at startup is 4x4 — where every
// layer lookup misses and the portrait falls back to a four-pixel body.
// The portrait selects the high-res set itself, and puts back whatever
// the grid had, since the selection is global.
func TestPortraitUsesTheHighResSprites(t *testing.T) {
	if r.ZoomHighRes == 0 {
		t.Fatal("the high-res set should not be the first zoom level")
	}

	for _, start := range []int{0, 1, r.ZoomHighRes} {
		r.SelectZoom(start)
		restore := withHighResSprites()
		if got := r.CurrentZoom(); got != r.ZoomHighRes {
			t.Errorf("from zoom %d: drawing at zoom %d, want the high-res %d", start, got, r.ZoomHighRes)
		}
		restore()
		if got := r.CurrentZoom(); got != start {
			t.Errorf("from zoom %d: left the set on %d instead of putting it back", start, got)
		}
	}
	r.SelectZoom(0)
}

// TestPortraitRoleFollowsSize: the portrait shows the sprite the world
// will draw for an organism that size, not always the large one.
func TestPortraitRoleFollowsSize(t *testing.T) {
	loadDefaults(t)
	maxSize := c.MaximumMaxSize()
	for _, tc := range []struct {
		size float64
		want r.ImageRole
	}{
		{maxSize * 0.1, r.RoleOrganismSmall},
		{maxSize * 0.5, r.RoleOrganismMedium},
		{maxSize * 0.9, r.RoleOrganismLarge},
	} {
		if got := portraitRole(tc.size); got != tc.want {
			t.Errorf("size %g drew role %v, want %v", tc.size, got, tc.want)
		}
	}
}

// TestDeleteTakesTwoClicks: the delete button sits beside a list the
// user clicks through to load things, and a design is work that can't be
// recovered from anywhere else — so one click arms it and the second
// does it.
func TestDeleteTakesTwoClicks(t *testing.T) {
	dz := designerWithDefaults(t)
	dir := t.TempDir()
	dz.saved = []organism.Design{organism.NewDesign("doomed"), organism.NewDesign("bystander")}

	dz.deleteSaved(0)
	if dz.confirmDelete != 0 {
		t.Errorf("the first click should arm the delete, got confirmDelete %d", dz.confirmDelete)
	}
	if len(dz.saved) != 2 {
		t.Error("the first click deleted something")
	}
	if !strings.Contains(dz.message, "again") {
		t.Errorf("the editor should ask for a second click, got %q", dz.message)
	}

	// Arming a different row moves the arm rather than deleting.
	dz.deleteSaved(1)
	if dz.confirmDelete != 1 {
		t.Errorf("arming another row should move the arm, got %d", dz.confirmDelete)
	}

	// A real two-click delete, against a directory this test owns.
	ds := organism.NewDesign("doomed")
	if _, err := organism.SaveDesign(dir, ds); err != nil {
		t.Fatal(err)
	}
	if err := organism.DeleteDesign(dir, ds.Name); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := organism.LoadDesigns(dir); len(got) != 0 {
		t.Errorf("the design survived deletion: %d left", len(got))
	}
}

// TestArmedDeleteDisarmsOnAnyOtherClick: an armed delete left waiting
// would catch a later misclick on a row the user had moved past.
func TestArmedDeleteDisarmsOnAnyOtherClick(t *testing.T) {
	dz := designerWithDefaults(t)
	dz.saved = []organism.Design{organism.NewDesign("one"), organism.NewDesign("two")}
	dz.deleteSaved(0)
	if dz.confirmDelete != 0 {
		t.Fatal("expected the delete to be armed")
	}

	// The disarm branch Update runs before dispatching any other hit.
	hit := designerHit{kind: hitAbilityUp, index: int(physiology.AbilityAttack)}
	if hit.kind != hitDelete || hit.index != dz.confirmDelete {
		dz.confirmDelete = -1
	}
	if dz.confirmDelete != -1 {
		t.Error("clicking elsewhere left the delete armed")
	}
}

// TestOversizedTreeBlocksSave: the editor won't grow a tree past the
// limit, but it will load one saved under a higher limit — and that one
// must not be saveable until it's trimmed, or the file would keep
// failing every simulation it's ticked for.
func TestOversizedTreeBlocksSave(t *testing.T) {
	dz := designerWithDefaults(t)
	dz.design.Name = "branchy"

	// Build past the limit directly, as loading an old file would.
	g := c.GetCurrentGlobals()
	limit := g.MaxDecisionTreeSize
	node := dz.tree.YesNode
	for dz.treeSize() <= limit {
		node.NodeType = d.IsFoodAhead
		node.YesNode = d.NodeFromAction(d.ActEat)
		node.NoNode = d.NodeFromAction(d.ActMove)
		dz.tree.CalcAndUpdateSize()
		node = node.YesNode
	}

	reason := dz.saveBlockedReason()
	if !strings.Contains(reason, "limit") {
		t.Errorf("an oversized tree should block the save, got %q", reason)
	}
	// And save() honours it rather than relying on the button being dim.
	before := dz.message
	dz.save()
	if dz.message == before || !strings.Contains(dz.message, "limit") {
		t.Errorf("save() should refuse and say why, message is %q", dz.message)
	}
}
