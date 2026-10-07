package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/utils"
)

// digFoodGlobals loads the shipped settings with every food source except digging switched off.
func digFoodGlobals(t *testing.T) *config.Globals {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	g.ChanceToAddFoodItem = 0
	g.CorpseFoodMultiplier = 0
	g.WallCreatedAtZero = 0
	g.WallCreatedSmall, g.WallCreatedMedium, g.WallCreatedLarge = 0, 0, 0
	return &g
}

func TestDiggingCannotCreateFood(t *testing.T) {
	for _, yield := range []int{0, 3, 20} {
		g := digFoodGlobals(t)
		g.InitialFood = 0
		// "Nothing buried" is the premise, and the shipped settings seed the
		// ground, so digging would be recovering food exactly as it should.
		g.InitialBuriedFood = 0
		// The other two ways food enters a world, switched off so what is
		// counted is digging. A corpse is food an organism left behind, and
		// it would be counted here as food digging created.
		g.CorpseFoodMultiplier = 0
		g.ChanceToAddFoodItem = 0
		g.FoodFromDiggingSmall, g.FoodFromDiggingMedium, g.FoodFromDiggingLarge = yield, yield, yield
		g.FoodFromDiggingAtZero = yield
		config.SetGlobals(g)

		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 53, CheckpointInterval: 1 << 30})
		peak := 0
		for i := 0; i < 1500 && !sim.IsDone(); i++ {
			sim.Update()
			peak = max(peak, sim.FoodCount())
		}
		if peak != 0 {
			t.Errorf("yield %d: %d food items appeared in a world with nothing buried; "+
				"digging must recover food, not create it", yield, peak)
		}
	}
}

// TestBurialAndRecoveryAreWiredUpInALiveWorld covers the two halves of the buried layer through a real simulation.
func TestBurialAndRecoveryAreWiredUpInALiveWorld(t *testing.T) {
	g := digFoodGlobals(t)
	g.InitialFood = 500
	g.BurialAmount, g.BurialInterval = 1, 5
	config.SetGlobals(g)

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 53, CheckpointInterval: 1 << 30})
	surfaceAtStart := 0
	for _, item := range sim.foodManager.GetFoodItems() {
		surfaceAtStart += item.Value
	}
	if surfaceAtStart == 0 {
		t.Fatal("initial_food laid down nothing, so this test has no food to bury")
	}

	for i := 0; i < 300 && !sim.IsDone(); i++ {
		sim.Update()
	}

	buried := sim.foodManager.GetBuriedFood()
	total := 0
	for _, v := range buried {
		total += v
	}
	if total == 0 {
		t.Fatal("nothing settled out of reach in 300 cycles; burial is not running in an assembled world")
	}

	var cell utils.Point
	var had int
	for p, v := range buried {
		if v > 0 {
			cell, had = p, v
			break
		}
	}
	moved := sim.UnburyFoodAtPoint(cell, had)
	if moved <= 0 {
		t.Fatalf("unburying %d units at %v moved nothing", had, cell)
	}
	if item, ok := sim.GetFoodAtPoint(cell); !ok || item.Value < moved {
		t.Errorf("the %d unburied units did not arrive in the food layer at %v", moved, cell)
	}
	if left := sim.GetBuriedFoodAtPoint(cell); left != had-moved {
		t.Errorf("%d left buried at %v, want %d", left, cell, had-moved)
	}
}

func TestDiggingRecoversBuriedFoodInALiveWorld(t *testing.T) {
	seeds := []int{53, 7, 21, 1, 11}
	buriedAfter := func(yield int) int {
		total := 0
		for _, seed := range seeds {
			g := digFoodGlobals(t)
			g.InitialFood = 2000
			g.BurialAmount, g.BurialInterval = 1, 5
			g.FoodFromDiggingSmall, g.FoodFromDiggingMedium, g.FoodFromDiggingLarge = yield, yield, yield
			g.FoodFromDiggingAtZero = yield
			config.SetGlobals(g)

			sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
			for i := 0; i < 1500 && !sim.IsDone(); i++ {
				sim.Update()
			}
			for _, v := range sim.foodManager.GetBuriedFood() {
				total += v
			}
		}
		return total
	}

	none := buriedAfter(0)
	if none == 0 {
		t.Fatal("nothing ended up buried even with digging off; burial isn't running, so this proves nothing")
	}
	recovered := buriedAfter(20)
	if recovered >= none {
		t.Errorf("with a dig yield of 20, %d units are still buried across %d seeds against %d with no "+
			"yield at all; digging brought nothing back", recovered, len(seeds), none)
	}
	t.Logf("buried left: %d with no yield, %d with a yield of 20 (%d recovered)",
		none, recovered, none-recovered)
}
