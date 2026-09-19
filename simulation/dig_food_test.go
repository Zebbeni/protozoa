package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestDiggingRootsUpFood: with random food spawns and corpse food turned
// off, food only appears in the world when digging roots it up.
func TestDiggingRootsUpFood(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	foodAfter := func(fromDigging int) int {
		var g config.Globals
		if err := json.Unmarshal(data, &g); err != nil {
			t.Fatal(err)
		}
		g.InitialFood = 0
		g.ChanceToAddFoodItem = 0
		g.CorpseFoodMultiplier = 0
		g.FoodFromDiggingSmall = fromDigging
		g.FoodFromDiggingMedium = fromDigging
		g.FoodFromDiggingLarge = fromDigging
		g.FoodFromDiggingAtZero = 0
		// Wall creation off: a dig facing a wall wears it down instead of
		// rooting anything up, so with the shipped wall_created_at_zero of
		// 1 the diggers litter their own flanks with walls and then dig
		// those. This test is about DigFood, not about that interaction.
		g.WallCreatedAtZero = 0
		g.WallCreatedSmall, g.WallCreatedMedium, g.WallCreatedLarge = 0, 0, 0
		config.SetGlobals(&g)
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 53, CheckpointInterval: 1 << 30})
		peak := 0
		for i := 0; i < 1500 && !sim.IsDone(); i++ {
			sim.Update()
			peak = max(peak, sim.FoodCount())
		}
		return peak
	}
	if got := foodAfter(0); got != 0 {
		t.Fatalf("with digging food off, %d food items appeared from elsewhere; the test can't isolate digging", got)
	}
	// Enough that the whole-unit rounding lands at the Digging scores
	// organisms start with: at 1 the curve has to reach half before a dig
	// produces anything, which is a balance question, not this test's.
	if got := foodAfter(3); got == 0 {
		t.Error("digging never rooted up any food")
	}
}
