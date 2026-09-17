package simulation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// guard is one condition on the path chooseAction took, with the branch
// it followed.
type guard struct {
	cond d.Condition
	yes  bool
}

// pathGuards walks the branch chooseAction took last cycle. Compared by
// typed constant, not by label: decision.Map holds display names ("If
// Food Ahead"), and matching those silently classifies everything as
// unguarded.
func pathGuards(t *d.Tree) []guard {
	var out []guard
	n := t.Node
	for n != nil && n.IsCondition() {
		c, ok := n.NodeType.(d.Condition)
		if !ok {
			break
		}
		yes := n.YesNode != nil && n.YesNode.UsedLastCycle
		out = append(out, guard{c, yes})
		if yes {
			n = n.YesNode
		} else {
			n = n.NoNode
		}
	}
	return out
}

func taken(gs []guard, c d.Condition, yes bool) bool {
	for _, g := range gs {
		if g.cond == c && g.yes == yes {
			return true
		}
	}
	return false
}

// TestEatMissDiagnosis reports what led to each eat attempt: the guard
// the tree used, and where the food actually was.
func TestEatMissDiagnosis(t *testing.T) {
	if os.Getenv("EAT_MISS") == "" {
		t.Skip()
	}
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if v := os.Getenv("EAT_COST"); v != "" {
		var cost float64
		fmt.Sscanf(v, "%g", &cost)
		g.HealthChangeFromEatingAttempt = cost
	}
	seed := 101
	if v := os.Getenv("EAT_SEED"); v != "" {
		fmt.Sscanf(v, "%d", &seed)
	}
	config.SetGlobals(&g)
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})

	guardHits := map[string][2]int{} // guard class -> {success, fail}
	where := map[string]int{}        // for misses: where the food was
	for c := 1; c <= 3000 && !sim.IsDone(); c++ {
		sim.Update()
		for _, o := range sim.organismManager.Organisms() {
			if o.Status != organism.StatusEatSuccess && o.Status != organism.StatusEatFailed {
				continue
			}
			tree := o.GetDecisionTreeCopy()
			guards := pathGuards(tree)
			class := "unguarded (no food condition on the path)"
			switch {
			case taken(guards, d.IsFoodAhead, true):
				class = "gated on food AHEAD = true"
			case taken(guards, d.IsFoodAhead, false):
				class = "fired with food ahead = FALSE"
			case taken(guards, d.IsFoodLeft, true) || taken(guards, d.IsFoodRight, true):
				class = "gated on food BESIDE (eats forward)"
			case len(guards) == 0:
				class = "bare tree: the root IS the action"
			}
			hit := guardHits[class]
			if o.Status == organism.StatusEatSuccess {
				hit[0]++
			} else {
				hit[1]++
				// Where was food, relative to a miss?
				dir := o.Direction
				for name, p := range map[string]utils.Point{
					"left":   o.Location.Add(dir.Left()),
					"right":  o.Location.Add(dir.Right()),
					"behind": o.Location.Sub(dir),
				} {
					if _, ok := sim.GetFoodAtPoint(p); ok {
						where[name]++
					}
				}
				where["misses"]++
			}
			guardHits[class] = hit
		}
	}

	keys := make([]string, 0, len(guardHits))
	for k := range guardHits {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := guardHits[keys[i]], guardHits[keys[j]]
		return a[0]+a[1] > b[0]+b[1]
	})
	total := 0
	for _, k := range keys {
		total += guardHits[k][0] + guardHits[k][1]
	}
	t.Logf("cost %v seed %d: %d eat attempts over 3000 cycles, %d alive at end",
		config.GetCurrentGlobals().HealthChangeFromEatingAttempt, seed, total, sim.OrganismCount())
	for _, k := range keys {
		v := guardHits[k]
		n := v[0] + v[1]
		t.Logf("  %-42s %7d attempts (%4.1f%%)  hit %5.1f%%", k, n, 100*float64(n)/float64(total),
			100*float64(v[0])/float64(max(n, 1)))
	}
	m := max(where["misses"], 1)
	t.Logf("on a miss, food was: left %4.1f%%  right %4.1f%%  behind %4.1f%%  (of %d misses)",
		100*float64(where["left"])/float64(m), 100*float64(where["right"])/float64(m),
		100*float64(where["behind"])/float64(m), where["misses"])
}
