package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/utils"
)

func TestFoodNeverLandsOnAnOrganism(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "settings", "settings_seed_1789794294595329300.json"))
	if err != nil {
		t.Skipf("the reported run's settings are gone: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	// A food spawn attempt every cycle rather than one in fifty.
	g.ChanceToAddFoodItem = 1
	config.SetGlobals(&g)

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: g.Seed, CheckpointInterval: 1 << 30})

	prevOrg := map[utils.Point]int{}
	prevTotal := map[utils.Point]int{}
	sawOverlap := false
	for c := 0; c < 4800 && !sim.IsDone(); c++ {
		sim.Update()

		curOrg := map[utils.Point]int{}
		for _, o := range sim.organismManager.Organisms() {
			curOrg[o.Location] = o.ID
		}
		curTotal := map[utils.Point]int{}
		for p, item := range sim.foodManager.GetFoodItems() {
			curTotal[p] += item.Value
		}
		for p, v := range sim.foodManager.GetBuriedFood() {
			curTotal[p] += v
		}

		for p, id := range curOrg {
			if curTotal[p] == 0 {
				continue
			}
			sawOverlap = true
			was, stayed := prevOrg[p]
			if stayed && was == id && curTotal[p] > prevTotal[p] {
				t.Fatalf("cycle %d: food at %v went from %d to %d under organism %d, which did not move — "+
					"digging moves food between layers without changing the total, so this is food arriving from nowhere",
					sim.Cycle(), p, prevTotal[p], curTotal[p], id)
			}
		}
		prevOrg, prevTotal = curOrg, curTotal
	}

	// If food and organisms never shared a cell at all, the check above never ran and the test proves nothing.
	if !sawOverlap {
		t.Skip("no organism ever stood on food in this run, so the arrival check never fired")
	}
}

// TestCorpseFoodStillDrops: the fix must not take the corpse with it.
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
