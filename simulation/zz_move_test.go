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

func TestMoveCostExperiment(t *testing.T) {
	costStr := os.Getenv("MOVE_COST")
	if costStr == "" {
		t.Skip("set MOVE_COST")
	}
	cost, _ := strconv.ParseFloat(costStr, 64)
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	var g config.Globals
	json.Unmarshal(data, &g)
	g.HealthChangeFromMoving = cost
	config.SetGlobals(&g)

	var seeds []int
	for _, f := range strings.Split(os.Getenv("MOVE_SEEDS"), ",") {
		n, _ := strconv.Atoi(f)
		seeds = append(seeds, n)
	}
	const capCycles = 10000
	for _, seed := range seeds {
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
		type tally struct{ orgs, ph, moves, spawns, dist, age, moveScore float64 }
		var per []tally
		cycles := 0
		for cycles < capCycles && !sim.IsDone() {
			sim.Update()
			cycles++
			tl := tally{ph: sim.AveragePh()}
			for _, o := range sim.organismManager.Organisms() {
				tl.orgs++
				switch o.Status {
				case organism.StatusMoveSuccess:
					tl.moves++
				case organism.StatusSpawning:
					tl.spawns++
				}
				tl.dist += float64(o.TraveledDist)
				tl.age += float64(o.Age)
				tl.moveScore += float64(o.Traits().Abilities[physiology.AbilityMovement])
			}
			per = append(per, tl)
		}
		var h tally
		half := per[len(per)/2:]
		for _, tl := range half {
			h.orgs += tl.orgs; h.ph += tl.ph; h.moves += tl.moves; h.spawns += tl.spawns
			h.dist += tl.dist; h.age += tl.age; h.moveScore += tl.moveScore
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
		t.Logf("RESULT cost=%-7s seed=%-5d %-13s pop=%7.1f avgPh=%5.2f move%%=%6.2f spawn%%=%5.2f movesPerSpawn=%6.2f distPer100Age=%6.2f moveScore=%5.1f",
			costStr, seed, end, h.orgs/n, h.ph/n, 100*div(h.moves, h.orgs), 100*div(h.spawns, h.orgs),
			div(h.moves, h.spawns), 100*div(h.dist, h.age), div(h.moveScore, h.orgs))
	}
}
