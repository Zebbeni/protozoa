package simulation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

type eaterCfg struct {
	chemoZero, chemoMax, eatZero, eatMax, food, eatPh float64
}

var eaterCfgs = map[string]eaterCfg{
	"base-ph0.1":    {0.6, 1.75, 0.6, 1.75, 0.02, 0.1},
	"base-ph0.02":   {0.6, 1.75, 0.6, 1.75, 0.02, 0.02},
	"base-ph0.005":  {0.6, 1.75, 0.6, 1.75, 0.02, 0.005},
	"base-ph0":      {0.6, 1.75, 0.6, 1.75, 0.02, 0},
	"combo-ph0.1":   {0.1, 1.25, 0.3, 3.0, 0.02, 0.1},
	"combo-ph0.02":  {0.1, 1.25, 0.3, 3.0, 0.02, 0.02},
	"combo-ph0.005": {0.1, 1.25, 0.3, 3.0, 0.02, 0.005},
	"combo-ph0":     {0.1, 1.25, 0.3, 3.0, 0.02, 0},
}

func TestEaterExperiment(t *testing.T) {
	name := os.Getenv("EATER_CFG")
	cfg, ok := eaterCfgs[name]
	if !ok {
		t.Skip("set EATER_CFG")
	}
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	var g config.Globals
	json.Unmarshal(data, &g)
	g.ChemoMultAtZero, g.ChemoMultAtMax = cfg.chemoZero, cfg.chemoMax
	g.EatingMultAtZero, g.EatingMultAtMax = cfg.eatZero, cfg.eatMax
	g.ChanceToAddFoodItem = cfg.food
	g.EatingPhEffectPerFood = cfg.eatPh
	config.SetGlobals(&g)

	const capCycles = 10000
	for _, seed := range []int{101, 53, 202, 999, 31, 44, 128, 12345} {
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
		type tally struct{ orgCycles, chemo, eat, low, eaterBody, eats, chemos, pop, cycles float64 }
		var all []tally
		cycles := 0
		for cycles < capCycles && !sim.IsDone() {
			sim.Update()
			cycles++
			var tl tally
			tl.cycles = 1
			for _, o := range sim.organismManager.Organisms() {
				ab := o.Traits().Abilities
				ch, ea := ab[physiology.AbilityChemosynthesis], ab[physiology.AbilityEating]
				tl.orgCycles++
				tl.chemo += float64(ch)
				tl.eat += float64(ea)
				if ch <= 20 {
					tl.low++
					if ea >= 25 {
						tl.eaterBody++
					}
				}
				switch o.Status {
				case organism.StatusEatSuccess:
					tl.eats++
				case organism.StatusChemoSuccess:
					tl.chemos++
				}
			}
			tl.pop = tl.orgCycles
			all = append(all, tl)
		}
		// second half of whatever the run lasted
		var h tally
		for _, tl := range all[len(all)/2:] {
			h.orgCycles += tl.orgCycles
			h.chemo += tl.chemo
			h.eat += tl.eat
			h.low += tl.low
			h.eaterBody += tl.eaterBody
			h.eats += tl.eats
			h.chemos += tl.chemos
			h.pop += tl.pop
			h.cycles += tl.cycles
		}
		end := fmt.Sprintf("ended@%d", cycles)
		if cycles >= capCycles && !sim.IsDone() {
			end = "survived"
		}
		div := func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		}
		t.Logf("RESULT cfg=%-16s seed=%-5d %-13s pop=%7.1f chemo=%5.1f eat=%5.1f lowChemo%%=%5.1f eaterBody%%=%5.1f eatShare%%=%5.1f food=%d",
			name, seed, end, div(h.pop, h.cycles), div(h.chemo, h.orgCycles), div(h.eat, h.orgCycles),
			100*div(h.low, h.orgCycles), 100*div(h.eaterBody, h.orgCycles),
			100*div(h.eats, h.eats+h.chemos), sim.FoodCount())
	}
}
