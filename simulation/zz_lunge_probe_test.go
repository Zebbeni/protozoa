package simulation

import (
	"os"
	"strconv"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

// LUNGE_PROBE=1 go test ./simulation/ -run TestLungeProbe -v
func TestLungeProbe(t *testing.T) {
	if os.Getenv("LUNGE_PROBE") == "" {
		t.Skip("set LUNGE_PROBE=1")
	}
	loadDefaultGlobals(t)
	const cycles = 6000
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: lungeSeed(), CheckpointInterval: 1 << 30})

	orgCycles, attacks, lunges := 0, 0, 0
	pop, atkScore, mvScore := 0.0, 0.0, 0.0
	samples := 0
	for cycle := 0; cycle < cycles; cycle++ {
		sim.Update()
		if sim.OrganismCount() == 0 {
			t.Fatalf("died at cycle %d", cycle)
		}
		for _, o := range sim.organismManager.Organisms() {
			orgCycles++
			if o.Action() == d.ActAttack {
				attacks++
			}
			if o.Status == organism.StatusAttackMove {
				lunges++
			}
		}
		if cycle%500 == 0 {
			samples++
			pop += float64(sim.OrganismCount())
			a, m := 0.0, 0.0
			n := 0
			for _, o := range sim.organismManager.Organisms() {
				a += float64(o.Abilities()[physiology.AbilityAttack])
				m += float64(o.Abilities()[physiology.AbilityMovement])
				n++
			}
			if n > 0 {
				atkScore += a / float64(n)
				mvScore += m / float64(n)
			}
		}
	}
	t.Logf("over %d cycles: attacks %.5f/organism-cycle, lunges %.5f/organism-cycle (%.1f%% of attacks), "+
		"final pop %d, mean pop %.0f",
		cycles, float64(attacks)/float64(orgCycles), float64(lunges)/float64(orgCycles),
		100*float64(lunges)/float64(max(attacks, 1)), sim.OrganismCount(), pop/float64(samples))
	t.Logf("mean ability score: Movement %.2f, Attack %.2f", mvScore/float64(samples), atkScore/float64(samples))
}

func lungeSeed() int {
	if s := os.Getenv("LUNGE_SEED"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return 101
}
