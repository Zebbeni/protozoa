package simulation

import (
	"os"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
)

func TestDigProbe(t *testing.T) {
	if os.Getenv("DIG_PROBE") == "" {
		t.Skip("set DIG_PROBE=1 to count digs")
	}
	for _, seed := range []int{53, 7, 21, 1, 11} {
		for _, yield := range []int{0, 20} {
			tiered := true
			g := digFoodGlobals(t)
			g.InitialFood = 2000
			g.BurialAmount, g.BurialInterval = 1, 5
			g.FoodFromDiggingSmall, g.FoodFromDiggingMedium, g.FoodFromDiggingLarge = yield, yield, yield
			g.FoodFromDiggingAtZero = yield
			g.TieredConditionMutation = tiered
			config.SetGlobals(g)

			sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
			digs, orgCycles := 0, 0
			for i := 0; i < 1500 && !sim.IsDone(); i++ {
				sim.Update()
				for _, o := range sim.organismManager.Organisms() {
					orgCycles++
					if o.Status == organism.StatusDigging {
						digs++
					}
				}
			}
			buried := 0
			for _, v := range sim.foodManager.GetBuriedFood() {
				buried += v
			}
			// How many living trees even contain ActDig.
			withDig := 0
			live := 0
			for _, o := range sim.organismManager.Organisms() {
				live++
				for _, a := range o.GetDecisionTreeCopy().ActionNodes() {
					if a == d.ActDig {
						withDig++
						break
					}
				}
			}
			t.Logf("seed=%-3d yield=%2d  digs=%5d  buried=%7d  live=%4d  treesWithActDig=%4d",
				seed, yield, digs, buried, live, withDig)
		}
	}
}
