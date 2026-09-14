package simulation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

var chemoCurveCfgs = map[string][2]float64{ // {falloff, exponent}
	"flat":         {0, 1},
	"fall1":        {1, 1},
	"fall2":        {2, 1},
	"fall0.5":      {0.5, 1},
	"exp0.5":       {0, 0.5},
	"fall1-exp0.5": {1, 0.5},
	"fall2-exp0.5": {2, 0.5},
}

func TestChemoCurveExperiment(t *testing.T) {
	name := os.Getenv("CC_CFG")
	cfg, ok := chemoCurveCfgs[name]
	if !ok {
		t.Skip("set CC_CFG")
	}
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	var g config.Globals
	json.Unmarshal(data, &g)
	g.ChemoPhFalloff, g.ChemoCurveExponent = cfg[0], cfg[1]
	config.SetGlobals(&g)

	var seeds []int
	for _, f := range strings.Split(os.Getenv("CC_SEEDS"), ",") {
		n, _ := strconv.Atoi(f)
		seeds = append(seeds, n)
	}

	const capCycles = 10000
	for _, seed := range seeds {
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
		type tally struct{ orgs, ph, chemoScore, low, eaterBody, chemos, eats float64 }
		var per []tally
		cycles := 0
		for cycles < capCycles && !sim.IsDone() {
			sim.Update()
			cycles++
			tl := tally{ph: sim.AveragePh()}
			for _, o := range sim.organismManager.Organisms() {
				ab := o.Traits().Abilities
				ch, ea := ab[physiology.AbilityChemosynthesis], ab[physiology.AbilityEating]
				tl.orgs++
				tl.chemoScore += float64(ch)
				if ch <= 20 {
					tl.low++
					if ea >= 25 {
						tl.eaterBody++
					}
				}
				switch o.Status {
				case organism.StatusChemoSuccess:
					tl.chemos++
				case organism.StatusEatSuccess:
					tl.eats++
				}
			}
			per = append(per, tl)
		}
		var h tally
		half := per[len(per)/2:]
		for _, tl := range half {
			h.orgs += tl.orgs; h.ph += tl.ph; h.chemoScore += tl.chemoScore
			h.low += tl.low; h.eaterBody += tl.eaterBody; h.chemos += tl.chemos; h.eats += tl.eats
		}
		div := func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		}
		end := fmt.Sprintf("ended@%d", cycles)
		if cycles >= capCycles && !sim.IsDone() {
			end = "survived"
		}
		n := float64(len(half))
		t.Logf("RESULT cfg=%-13s seed=%-5d %-13s pop=%7.1f avgPh=%5.2f chemoScore=%5.1f lowChemo%%=%5.1f eaterBody%%=%5.1f eatShare%%=%5.1f",
			name, seed, end, h.orgs/n, h.ph/n, div(h.chemoScore, h.orgs), 100*div(h.low, h.orgs),
			100*div(h.eaterBody, h.orgs), 100*div(h.eats, h.eats+h.chemos))
	}
}
