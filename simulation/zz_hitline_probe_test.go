package simulation

import (
	"os"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// HITLINE_PROBE=1 go test ./simulation/ -run TestHitLineProbe -v
//
// Classifies every non-zero "hit" line in the panel ledger by what produced
// it, to answer a report of hit damage on an organism nothing attacked.
func TestHitLineProbe(t *testing.T) {
	if os.Getenv("HITLINE_PROBE") == "" {
		t.Skip("set HITLINE_PROBE=1")
	}
	loadDefaultGlobals(t)
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 101, CheckpointInterval: 1 << 30})

	attackedOnly, thornsOnly, both, neither := 0, 0, 0, 0
	for cycle := 0; cycle < 3000; cycle++ {
		// Where everyone stood when this cycle's actions were decided.
		// UpdateAction runs INSIDE Update, so the actions themselves have to
		// be read afterwards: reading them here returns the previous
		// cycle's choices, which is what made a first run of this probe
		// report 14,286 unexplained hit lines.
		stood := map[int]utils.Point{}
		for _, o := range sim.organismManager.Organisms() {
			stood[o.ID] = o.Location
		}
		sim.Update()

		attacking := map[int]bool{}
		aimedAt := map[utils.Point]int{}
		for _, o := range sim.organismManager.Organisms() {
			if o.Action() != d.ActAttack {
				continue
			}
			attacking[o.ID] = true
			// An attack does not move or turn its attacker, so the cell it
			// aimed at is its decide-phase cell plus its heading.
			if at, ok := stood[o.ID]; ok {
				aimedAt[at.Add(o.Direction)]++
			}
		}
		for _, o := range sim.organismManager.Organisms() {
			l := o.HealthLedger()
			if !l.Recorded || l.Amounts[organism.HealthFromAttack] == 0 {
				continue
			}
			at, ok := stood[o.ID]
			if !ok {
				continue
			}
			wasAttacked := aimedAt[at] > 0
			switch {
			case wasAttacked && attacking[o.ID]:
				both++
			case wasAttacked:
				attackedOnly++
			case attacking[o.ID]:
				thornsOnly++
			default:
				neither++
			}
		}
	}
	t.Logf("non-zero 'hit' lines by cause: attacked by someone %d, attacked someone (thorns) %d, "+
		"both %d, neither %d", attackedOnly, thornsOnly, both, neither)
}
