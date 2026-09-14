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
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

// TestLongRunScratch records a time series of ability mix and action
// shares for one seed under one lever configuration. Env:
//
//	LR_SEED, LR_CYCLES, LR_GAIN (attack_health_gain),
//	LR_CROWD (chemo_crowding_penalty), LR_CORPSE (corpse_food_multiplier), LR_TAG
func TestLongRunScratch(t *testing.T) {
	if os.Getenv("LR_SEED") == "" {
		t.Skip()
	}
	f := func(k string, def float64) float64 {
		if v := os.Getenv(k); v != "" {
			x, _ := strconv.ParseFloat(v, 64)
			return x
		}
		return def
	}
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	var g config.Globals
	json.Unmarshal(data, &g)
	g.AttackHealthGain = f("LR_GAIN", 0)
	g.ChemoCrowdingPenalty = f("LR_CROWD", 0)
	g.CorpseFoodMultiplier = f("LR_CORPSE", 1)
	g.HealthChangeInflictedByAttack = f("LR_ATTACK", g.HealthChangeInflictedByAttack)
	g.ThornsDamagePerPoint = f("LR_THORNS", g.ThornsDamagePerPoint)
	g.EatingMultAtMax = f("LR_EAT_MAX", g.EatingMultAtMax)
	g.EatingMultAtZero = f("LR_EAT_ZERO", g.EatingMultAtZero)
	g.EatingGrowthFactor = f("LR_EAT_GROWTH", g.EatingGrowthFactor)
	config.SetGlobals(&g)

	seed := int(f("LR_SEED", 1))
	cycles := int(f("LR_CYCLES", 20000))
	tag := os.Getenv("LR_TAG")
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})

	const window = 250
	type acc struct{ orgs, chemo, eat, atk, eatOK, atkAct, chemoOK, predators, grazers, chemoSpec, def, tanks, eatSpec, maxEat float64 }
	var a acc
	var lines []string
	for c := 1; c <= cycles && !sim.IsDone(); c++ {
		sim.Update()
		for _, o := range sim.organismManager.Organisms() {
			s := o.Traits().Abilities
			a.orgs++
			a.chemo += float64(s[physiology.AbilityChemosynthesis])
			a.eat += float64(s[physiology.AbilityEating])
			a.atk += float64(s[physiology.AbilityAttack])
			if s[physiology.AbilityAttack] >= 25 {
				a.predators++
			}
			if s[physiology.AbilityEating] >= 25 {
				a.grazers++
			}
			if s[physiology.AbilityChemosynthesis] >= 50 {
				a.chemoSpec++
			}
			if s[physiology.AbilityEating] >= physiology.SpecialistScore(physiology.AbilityEating) {
				a.eatSpec++
			}
			a.maxEat = max(a.maxEat, float64(s[physiology.AbilityEating]))
			a.def += float64(s[physiology.AbilityDefense])
			if s[physiology.AbilityDefense] >= 60 {
				a.tanks++
			}
			switch o.Status {
			case organism.StatusEatSuccess:
				a.eatOK++
			case organism.StatusAttacking:
				a.atkAct++
			case organism.StatusChemoSuccess:
				a.chemoOK++
			}
		}
		if c%window == 0 {
			relTrees, live := 0.0, 0.0
			for _, o := range sim.organismManager.Organisms() {
				live++
				for _, cond := range o.GetDecisionTreeCopy().ConditionNodes() {
					if cond == d.IsRelativeAhead {
						relTrees++
						break
					}
				}
			}
			n := max(a.orgs, 1)
			lines = append(lines, fmt.Sprintf("%d,%.0f,%.2f,%.2f,%.2f,%.4f,%.4f,%.4f,%.4f,%.4f,%.4f,%d,%.4f,%.2f,%.4f,%.4f,%.0f",
				c, a.orgs/window, a.chemo/n, a.eat/n, a.atk/n, a.predators/n, a.grazers/n, a.chemoSpec/n,
				a.chemoOK/n, a.eatOK/n, a.atkAct/n, sim.FoodCount(), relTrees/max(live, 1), a.def/n, a.tanks/n, a.eatSpec/n, a.maxEat))
			a = acc{}
		}
	}
	end := "survived"
	if sim.IsDone() {
		end = fmt.Sprintf("ended@%d", sim.Cycle())
	}
	out := fmt.Sprintf("TAG %s SEED %d END %s\n", tag, seed, end) + strings.Join(lines, "\n") + "\n"
	os.WriteFile(os.Getenv("LR_OUT"), []byte(out), 0o644)
}
