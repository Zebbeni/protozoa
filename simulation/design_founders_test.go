package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

// TestDesignsFoundTheSimulation: with designs configured, the founders
// are those organisms rather than random ones, dealt round-robin so two
// designs can be pitted against each other. This is the whole point of
// saving a design, and nothing else in the pipeline checks it end to
// end.
func TestDesignsFoundTheSimulation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}

	// Installed before the designs are built: NewDesign reads the
	// settings for its starting traits.
	config.SetGlobals(&g)

	// Two designs that differ in a way the organism carries: a digger
	// that always digs and a grazer that always eats.
	digger := organism.NewDesign("digger")
	digger.Abilities = abilitiesWith(physiology.AbilityDigging)
	digger.DecisionTree = d.TreeFromAction(d.ActDig).Serialize()

	grazer := organism.NewDesign("grazer")
	grazer.Abilities = abilitiesWith(physiology.AbilityEating)
	grazer.DecisionTree = d.TreeFromAction(d.ActEat).Serialize()

	// SaveDesign writes relative to the designs directory the simulation
	// reads, so this test writes there and cleans up after itself.
	for _, ds := range []organism.Design{digger, grazer} {
		path, err := organism.SaveDesign(filepath.Join("..", organism.DesignsDir), ds)
		if err != nil {
			t.Fatalf("save %s: %v", ds.Name, err)
		}
		t.Cleanup(func() { os.Remove(path) })
	}

	g.InitialOrganisms = 4
	g.InitialDesigns = []string{"digger", "grazer"}
	config.SetGlobals(&g)

	// The manager reads designs relative to the working directory, which
	// for a test is the package dir; hop up so "designs/" resolves.
	restore := chdir(t, "..")
	defer restore()

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 11, CheckpointInterval: 1 << 30})
	counts := map[string]int{}
	for _, o := range sim.organismManager.Organisms() {
		scores := o.Traits().Abilities
		switch {
		case scores[physiology.AbilityDigging] == physiology.MaxAbilityScore:
			counts["digger"]++
		case scores[physiology.AbilityEating] == physiology.MaxAbilityScore:
			counts["grazer"]++
		default:
			counts["random"]++
		}
	}
	if counts["random"] > 0 {
		t.Errorf("%d founders were random, want all of them from designs", counts["random"])
	}
	if counts["digger"] == 0 || counts["grazer"] == 0 {
		t.Errorf("founders were %v; both designs should be dealt", counts)
	}
}

// abilitiesWith spends the cap on one ability and spreads the remaining
// points one at a time across the others, never past the cap.
func abilitiesWith(a physiology.Ability) []int {
	out := make([]int, physiology.AbilityCount)
	out[a] = physiology.MaxAbilityScore
	left := physiology.PointTotal - physiology.MaxAbilityScore
	for left > 0 {
		for i := range out {
			if physiology.Ability(i) == a || left == 0 || out[i] >= physiology.MaxAbilityScore {
				continue
			}
			out[i]++
			left--
		}
	}
	return out
}

// chdir moves to dir for the duration of a test.
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { os.Chdir(prev) }
}
