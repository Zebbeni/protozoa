package simulation

import (
	"encoding/json"
	"strconv"
	"strings"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

type ventureCfg struct{ chemoTol, prot float64 }

var ventureCfgs = map[string]ventureCfg{
	"t0.25-p0.5": {0.25, 0.5},
	"t2-p0":      {2, 0},
	"t2-p0.5":    {2, 0.5},
	"t2-p1":      {2, 1},
	"t3-p0":      {3, 0},
	"t3-p0.5":    {3, 0.5},
	"t3-p1":      {3, 1},
	"t4-p1":      {4, 1},
}

func TestVentureExperiment(t *testing.T) {
	name := os.Getenv("VENTURE_CFG")
	cfg, ok := ventureCfgs[name]
	if !ok {
		t.Skip("set VENTURE_CFG")
	}
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	var g config.Globals
	json.Unmarshal(data, &g)
	g.ChemosynthesisTolerance = cfg.chemoTol
	g.DefensePhProtection = cfg.prot
	config.SetGlobals(&g)

	const capCycles = 10000
	phTol := config.PhTolerance()
	specialist := physiology.SpecialistScore(physiology.AbilityDefense)
	seeds := []int{101, 53, 202, 999, 31, 44, 128, 12345}
	if env := os.Getenv("VENTURE_SEEDS"); env != "" {
		seeds = nil
		for _, f := range strings.Split(env, ",") {
			n, _ := strconv.Atoi(f)
			seeds = append(seeds, n)
		}
	}
	for _, seed := range seeds {
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
		type tally struct {
			orgs, pop, cycles                  float64
			def, spec                          float64
			outOrgs, defOut, inOrgs, defIn     float64
			chemoOut, chemoAll                 float64
		}
		var per []tally
		cycles := 0
		for cycles < capCycles && !sim.IsDone() {
			sim.Update()
			cycles++
			var tl tally
			tl.cycles = 1
			for _, o := range sim.organismManager.Organisms() {
				d := float64(o.Traits().Abilities[physiology.AbilityDefense])
				tl.orgs++
				tl.def += d
				if d >= float64(specialist) {
					tl.spec++
				}
				outside := math.Abs(o.Traits().IdealPh-sim.GetPhAtPoint(o.Location)) > phTol
				if outside {
					tl.outOrgs++
					tl.defOut += d
				} else {
					tl.inOrgs++
					tl.defIn += d
				}
				if o.Status == organism.StatusChemoSuccess {
					tl.chemoAll++
					if outside {
						tl.chemoOut++
					}
				}
			}
			tl.pop = tl.orgs
			per = append(per, tl)
		}
		var h tally
		for _, tl := range per[len(per)/2:] {
			h.orgs += tl.orgs; h.pop += tl.pop; h.cycles += tl.cycles
			h.def += tl.def; h.spec += tl.spec
			h.outOrgs += tl.outOrgs; h.defOut += tl.defOut
			h.inOrgs += tl.inOrgs; h.defIn += tl.defIn
			h.chemoOut += tl.chemoOut; h.chemoAll += tl.chemoAll
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
		t.Logf("RESULT cfg=%-11s seed=%-5d %-13s pop=%7.1f meanDef=%5.1f defSpec%%=%5.1f outside%%=%5.1f defOut=%5.1f defIn=%5.1f chemoOut%%=%5.1f avgPh=%.2f",
			name, seed, end, div(h.pop, h.cycles), div(h.def, h.orgs), 100*div(h.spec, h.orgs),
			100*div(h.outOrgs, h.orgs), div(h.defOut, h.outOrgs), div(h.defIn, h.inOrgs),
			100*div(h.chemoOut, h.chemoAll), sim.AveragePh())
	}
}
