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
//	LR_SEED, LR_CYCLES,
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
	g.CorpseFoodMultiplier = f("LR_CORPSE", 1)
	g.AttackDamageAtFullAttack = f("LR_ATTACK", g.AttackDamageAtFullAttack)
	g.ThornsDamageAtFullDefense = f("LR_THORNS", g.ThornsDamageAtFullDefense)
	g.EatingGrowthFactor = f("LR_EAT_GROWTH", g.EatingGrowthFactor)
	if k := os.Getenv("LR_K"); k != "" {
		x := f("LR_K", 0.5)
		g.ChemosynthesisCosineK, g.EatingCosineK, g.MovementCostCosineK, g.DiggingCostCosineK = x, x, x, x
		g.DiggingStrengthCosineK, g.AttackCosineK, g.DamageTakenCosineK = x, x, x
	}
	// LR_SET overlays settings by json tag: "tag=value,tag=value".
	if set := os.Getenv("LR_SET"); set != "" {
		overlay := map[string]any{}
		for _, kv := range strings.Split(set, ",") {
			parts := strings.SplitN(kv, "=", 2)
			// A value that isn't a number is passed through as a string,
			// so curve shapes ("linear", "quadratic") can be swept the
			// same way the numeric knobs are.
			if x, err := strconv.ParseFloat(parts[1], 64); err == nil {
				overlay[parts[0]] = x
			} else {
				overlay[parts[0]] = parts[1]
			}
		}
		base, _ := json.Marshal(g)
		var merged map[string]any
		json.Unmarshal(base, &merged)
		for k, v := range overlay {
			if _, ok := merged[k]; !ok {
				t.Fatalf("unknown setting %q", k)
			}
			merged[k] = v
		}
		data, _ := json.Marshal(merged)
		g = config.Globals{}
		if err := json.Unmarshal(data, &g); err != nil {
			t.Fatal(err)
		}
	}
	scale := f("LR_SCALE", 1)
	g.MaxChemosynthesisGain *= scale
	g.MaxFoodPerEatAtFullEating *= scale
	g.AttackDamageAtFullAttack *= scale
	config.SetGlobals(&g)

	seed := int(f("LR_SEED", 1))
	cycles := int(f("LR_CYCLES", 20000))
	tag := os.Getenv("LR_TAG")
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})

	const window = 250
	// halfCap is "has put real points here", scaled to whatever the
	// ability cap currently is.
	halfCap := physiology.MaxAbilityScore / 2
	type acc struct {
		orgs, chemo, eat, atk, mov, dig, eatOK, eatFail, atkAct, chemoOK, predators, grazers, chemoSpec, def, tanks, eatSpec, maxEat float64
	}
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
			a.mov += float64(s[physiology.AbilityMovement])
			a.dig += float64(s[physiology.AbilityDigging])
			// Thresholds are fractions of the cap, not literals: they
			// were written as 25 / 50 / 60 against a 100-point scale and
			// silently counted nothing at all once the scale became 0-10.
			if s[physiology.AbilityAttack] >= halfCap {
				a.predators++
			}
			if s[physiology.AbilityEating] >= halfCap {
				a.grazers++
			}
			if s[physiology.AbilityChemosynthesis] >= physiology.SpecialistScore {
				a.chemoSpec++
			}
			if s[physiology.AbilityEating] >= physiology.SpecialistScore {
				a.eatSpec++
			}
			a.maxEat = max(a.maxEat, float64(s[physiology.AbilityEating]))
			a.def += float64(s[physiology.AbilityDefense])
			if s[physiology.AbilityDefense] >= physiology.SpecialistScore {
				a.tanks++
			}
			switch o.Status {
			case organism.StatusEatSuccess:
				a.eatOK++
			case organism.StatusEatFailed:
				a.eatFail++
			case organism.StatusAttacking:
				a.atkAct++
			case organism.StatusChemoSuccess:
				a.chemoOK++
			}
		}
		if c%window == 0 {
			// eatTrees is the diagnostic that says whether eating is even
			// in the population's repertoire: no amount of reward moves a
			// lineage whose tree never chooses ActEat.
			relTrees, eatTrees, foodTrees, live := 0.0, 0.0, 0.0, 0.0
			for _, o := range sim.organismManager.Organisms() {
				live++
				tree := o.GetDecisionTreeCopy()
				for _, cond := range tree.ConditionNodes() {
					if cond == d.IsRelativeAhead {
						relTrees++
						break
					}
				}
				for _, cond := range tree.ConditionNodes() {
					if cond == d.IsFoodAhead || cond == d.IsFoodLeft || cond == d.IsFoodRight {
						foodTrees++
						break
					}
				}
				for _, act := range tree.ActionNodes() {
					if act == d.ActEat {
						eatTrees++
						break
					}
				}
			}
			n := max(a.orgs, 1)
			lines = append(lines, fmt.Sprintf("%d,%.0f,%.2f,%.2f,%.2f,%.4f,%.4f,%.4f,%.4f,%.4f,%.4f,%d,%.4f,%.2f,%.4f,%.4f,%.0f,%.3f,%.2f,%.2f,%.4f,%.4f,%.4f",
				c, a.orgs/window, a.chemo/n, a.eat/n, a.atk/n, a.predators/n, a.grazers/n, a.chemoSpec/n,
				a.chemoOK/n, a.eatOK/n, a.atkAct/n, sim.FoodCount(), relTrees/max(live, 1), a.def/n, a.tanks/n, a.eatSpec/n, a.maxEat, sim.AveragePh(), a.mov/n, a.dig/n,
				eatTrees/max(live, 1), foodTrees/max(live, 1), a.eatFail/n))
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
