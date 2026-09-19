package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/utils"
)

// TestFoodNeverLandsOnAnOrganism is the regression test for food
// rendering underneath a living organism and staying there for cycles.
//
// Organisms can never step onto food — food blocks movement — so the only
// way the two shared a cell was food arriving underneath a stationary
// one, which random spawns did freely: addFood checked for a wall and
// nothing else. On the reported seed, 973 of the first 5,600 cycles had
// at least one overlapping cell.
//
// The rule itself is pinned by TestFoodIsNotPlacedOnAnOrganism in the
// manager package, which is the guard that fails if the check is removed.
// This one plays the reported run forward instead, because the bug was
// found by looking at a world rather than at a function — and a smaller,
// shorter world than the one that produced it did *not* reproduce it, so
// the settings and seed are the reported ones on purpose.
func TestFoodNeverLandsOnAnOrganism(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "settings", "settings_seed_1789794294595329300.json"))
	if err != nil {
		t.Skipf("the reported run's settings are gone: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	config.SetGlobals(&g)

	// The first overlap on this seed was at cycle 4628.
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: g.Seed, CheckpointInterval: 1 << 30})
	for c := 0; c < 4800 && !sim.IsDone(); c++ {
		sim.Update()
		for p := range sim.foodManager.GetFoodItems() {
			if sim.organismManager.IsOrganismAtPoint(p) {
				t.Fatalf("cycle %d: food at %v sits under a living organism", sim.Cycle(), p)
			}
		}
	}
}

// TestCorpseFoodStillDrops: the fix must not take the corpse with it.
// A dying organism drops food at its own location, and that is the one
// place food legitimately appears where an organism was — it works
// because the corpse is cleared off the grid before the food is added,
// so by the time addFood sees the cell it is empty.
func TestCorpseFoodStillDrops(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	// Nothing else may produce food, so anything that appears is a corpse.
	g.InitialFood = 0
	g.ChanceToAddFoodItem = 0
	g.FoodFromDiggingSmall, g.FoodFromDiggingMedium = 0, 0
	g.FoodFromDiggingLarge, g.FoodFromDiggingAtZero = 0, 0
	g.CorpseFoodMultiplier = 5
	config.SetGlobals(&g)

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 1, CheckpointInterval: 1 << 30})
	seen := map[utils.Point]bool{}
	for c := 0; c < 4000 && !sim.IsDone(); c++ {
		sim.Update()
		for p := range sim.foodManager.GetFoodItems() {
			seen[p] = true
		}
	}
	if len(seen) == 0 {
		t.Error("no corpse ever left food behind; the drop was lost with the fix")
	}
}
