package organism

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// loadDesignGlobals installs the shipped settings, which the clamps and the starting values read.
func loadDesignGlobals(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	config.SetGlobals(&g)
}

func sameDesign(a, b Design) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// TestNewDesignIsUsable: the design the editor opens on must already be a legal organism.
func TestNewDesignIsUsable(t *testing.T) {
	loadDesignGlobals(t)
	ds := NewDesign("first")
	if err := ds.Validate(); err != nil {
		t.Fatalf("a fresh design doesn't validate: %v", err)
	}
	scores, err := ds.Scores()
	if err != nil {
		t.Fatalf("fresh scores: %v", err)
	}
	if scores.Total() != physiology.PointTotal {
		t.Errorf("fresh design spends %d points, want the %d budget", scores.Total(), physiology.PointTotal)
	}
	tree, err := ds.Tree()
	if err != nil {
		t.Fatalf("fresh tree: %v", err)
	}
	if got := tree.Size(); got != 3 {
		t.Errorf("the starter tree has %d nodes, want the 3 that make a decision", got)
	}
	if !tree.Node.IsCondition() {
		t.Error("the starter tree's root should be the condition it branches on")
	}
}

func TestDesignRoundTripsThroughDisk(t *testing.T) {
	loadDesignGlobals(t)
	dir := t.TempDir()

	ds := NewDesign("Tunneller One")
	ds.Color, ds.SecondaryColor = "#3366cc", "#cc9933"
	ds.MaxSize, ds.IdealPh = 40, 4.5
	ds.MinCyclesBetweenSpawns = 3
	ds.Abilities = []int{physiology.MaxAbilityScore, 1, 1, 2, 1, 1, physiology.PointTotal - physiology.MaxAbilityScore - 6}

	root := d.NodeFromCondition(d.IsFoodAhead)
	root.YesNode = d.NodeFromAction(d.ActEat)
	root.NoNode = d.NodeFromAction(d.ActDig)
	ds.DecisionTree = d.TreeFromNode(root).Serialize()

	path, err := SaveDesign(dir, ds)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if want := filepath.Join(dir, "tunneller-one.json"); path != want {
		t.Errorf("saved to %q, want %q", path, want)
	}

	loaded := LoadDesigns(dir)
	if len(loaded) != 1 {
		t.Fatalf("loaded %d designs, want 1", len(loaded))
	}
	if !sameDesign(loaded[0], ds) {
		t.Errorf("round trip changed the design:\n got %+v\nwant %+v", loaded[0], ds)
	}
}

func TestDesignsByNamePicksInOrder(t *testing.T) {
	loadDesignGlobals(t)
	dir := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if _, err := SaveDesign(dir, NewDesign(name)); err != nil {
			t.Fatal(err)
		}
	}

	got := DesignsByName(dir, []string{"beta", "missing", "alpha"})
	if len(got) != 2 || got[0].Name != "beta" || got[1].Name != "alpha" {
		t.Errorf("got %v, want beta then alpha with the missing name skipped", names(got))
	}
	if got := DesignsByName(dir, nil); len(got) != 0 {
		t.Errorf("no names should select nothing, got %v", names(got))
	}
}

func names(designs []Design) []string {
	out := make([]string, len(designs))
	for i, ds := range designs {
		out[i] = ds.Name
	}
	return out
}

func TestBrokenDesignsAreSkipped(t *testing.T) {
	loadDesignGlobals(t)
	dir := t.TempDir()
	if _, err := SaveDesign(dir, NewDesign("good")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "junk.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Valid JSON, but an ability split that doesn't add up.
	bad := NewDesign("overspent")
	bad.Abilities = []int{physiology.MaxAbilityScore, physiology.MaxAbilityScore, physiology.MaxAbilityScore, 0, 0, 0, 0}
	data, _ := json.Marshal(bad)
	if err := os.WriteFile(filepath.Join(dir, "overspent.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded := LoadDesigns(dir)
	if len(loaded) != 1 || loaded[0].Name != "good" {
		t.Errorf("loaded %v, want just the good design", names(loaded))
	}
}

func TestDesignedOrganismMatchesItsDesign(t *testing.T) {
	loadDesignGlobals(t)
	ds := NewDesign("grazer")
	ds.Abilities = []int{1, physiology.MaxAbilityScore, 1, 1, 1, 1, physiology.PointTotal - physiology.MaxAbilityScore - 5}
	root := d.NodeFromCondition(d.IsFoodAhead)
	root.YesNode = d.NodeFromAction(d.ActEat)
	root.NoNode = d.NodeFromAction(d.ActMove)
	ds.DecisionTree = d.TreeFromNode(root).Serialize()

	o, err := NewDesigned(simrand.New(1), 7, utils.Point{X: 3, Y: 4}, nil, ds)
	if err != nil {
		t.Fatalf("building from a design: %v", err)
	}
	if o.ID != 7 {
		t.Errorf("id %d, want 7", o.ID)
	}
	want, _ := ds.Scores()
	if o.Traits().Abilities != want {
		t.Errorf("abilities %v, want %v", o.Traits().Abilities, want)
	}
	if got := o.GetDecisionTreeCopy().Serialize(); got != ds.DecisionTree {
		t.Errorf("tree %q, want %q", got, ds.DecisionTree)
	}
	if o.Appearance() != physiology.AppearanceFor(want, o.GetDecisionTreeCopy()) {
		t.Error("a designed organism's appearance should be derived from its scores and tree, like any other")
	}
	// initialSize, not SpawnHealth: initial_organism_size_fraction starts
	// FOUNDERS at a share of their max size, and a designed organism is a
	// founder. At a fraction of 0 the two are the same number, which is
	// what this asserted before the setting existed.
	if o.Health != initialSize(o.Traits()) {
		t.Errorf("starts with %v health, want the founder start %v for its design",
			o.Health, initialSize(o.Traits()))
	}
}

func TestTraitsAreClampedToTheCurrentSettings(t *testing.T) {
	loadDesignGlobals(t)
	ds := NewDesign("huge")
	ds.MaxSize = 1e9
	ds.IdealPh = 99
	ds.MinCyclesBetweenSpawns = 10000

	traits, err := ds.Traits()
	if err != nil {
		t.Fatalf("traits: %v", err)
	}
	if traits.MaxSize > config.MaximumMaxSize() {
		t.Errorf("max size %v exceeds the configured ceiling %v", traits.MaxSize, config.MaximumMaxSize())
	}
	if traits.IdealPh > config.MaxIdealPh() {
		t.Errorf("ideal pH %v is off the scale (max %v)", traits.IdealPh, config.MaxIdealPh())
	}
	if traits.MinCyclesBetweenSpawns > config.MaxCyclesBetweenSpawns() {
		t.Errorf("spawn cooldown %d exceeds the configured max %d",
			traits.MinCyclesBetweenSpawns, config.MaxCyclesBetweenSpawns())
	}
}

func TestDesignNamesBecomeFileNames(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"Alpha", "alpha.json"},
		{"Two Words", "two-words.json"},
		{"Weird/Name!", "weirdname.json"},
		{"   ", "design.json"},
	} {
		if got := DesignFileName(tc.name); got != tc.want {
			t.Errorf("DesignFileName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDeleteDesign(t *testing.T) {
	loadDesignGlobals(t)
	dir := t.TempDir()
	for _, name := range []string{"keep", "Throw Away"} {
		if _, err := SaveDesign(dir, NewDesign(name)); err != nil {
			t.Fatal(err)
		}
	}

	if err := DeleteDesign(dir, "Throw Away"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	left := LoadDesigns(dir)
	if len(left) != 1 || left[0].Name != "keep" {
		t.Errorf("after deleting, the directory holds %v, want just keep", names(left))
	}
	if err := DeleteDesign(dir, "never existed"); err != nil {
		t.Errorf("deleting a design that isn't there: %v", err)
	}
}

func TestExceedsTreeLimitIsSettingsDependent(t *testing.T) {
	loadDesignGlobals(t)

	// The active config is deliberately left somewhere else entirely.
	config.GetCurrentGlobals().MaxDecisionTreeSize = 2

	ds := NewDesign("branchy")
	root := d.NodeFromCondition(d.IsFoodAhead)
	root.YesNode = d.NodeFromCondition(d.IsWallAhead)
	root.YesNode.YesNode = d.NodeFromAction(d.ActDig)
	root.YesNode.NoNode = d.NodeFromAction(d.ActEat)
	root.NoNode = d.NodeFromAction(d.ActMove)
	ds.DecisionTree = d.TreeFromNode(root).Serialize()

	if got := ds.TreeSize(); got != 5 {
		t.Fatalf("tree is %d nodes, want 5", got)
	}

	if ds.ExceedsTreeLimit(5) {
		t.Error("a tree exactly at the limit should be allowed")
	}
	if err := ds.Validate(); err != nil {
		t.Errorf("the limit shouldn't affect validity: %v", err)
	}

	if !ds.ExceedsTreeLimit(4) {
		t.Error("a tree over the limit should be flagged")
	}
	if err := ds.Validate(); err != nil {
		t.Errorf("an oversized design must still load so it can be fixed: %v", err)
	}
	if over := OversizedDesigns([]Design{ds, NewDesign("small")}, 4); len(over) != 1 || over[0] != "branchy" {
		t.Errorf("OversizedDesigns returned %v, want just branchy", over)
	}
}
