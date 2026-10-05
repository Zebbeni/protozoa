package simulation

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
)

// TestEveryHealthSourceIsReachedInALiveWorld is the coverage the per-path
// tests in manager cannot give. The ledger is recorded BESIDE each health
// change rather than through it — so that no accounting can alter the float
// arithmetic a replay depends on — and the cost of that is a source nothing
// writes to, which shows as a panel line permanently absent with nothing to
// say so.
func TestEveryHealthSourceIsReachedInALiveWorld(t *testing.T) {
	loadDefaultGlobals(t)
	g := config.GetCurrentGlobals()
	// Both predation settings ship off and each has a ledger line, so they
	// are turned on here rather than left uncovered.
	g.AttackHealthGain = 0.5
	config.SetGlobals(g)

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 101, CheckpointInterval: 1 << 30})

	seen := map[organism.HealthSource]bool{}
	cycles := 0
	for ; cycles < 6000 && len(seen) < len(organism.AllHealthSources); cycles++ {
		sim.Update()
		if sim.OrganismCount() == 0 {
			t.Fatalf("the world died at cycle %d with %d of %d sources seen",
				cycles, len(seen), len(organism.AllHealthSources))
		}
		for _, o := range sim.organismManager.Organisms() {
			l := o.HealthLedger()
			if !l.Recorded {
				continue
			}
			for _, src := range organism.AllHealthSources {
				if l.Amounts[src] != 0 {
					seen[src] = true
				}
			}
		}
	}
	for _, src := range organism.AllHealthSources {
		if !seen[src] {
			t.Errorf("nothing in 6000 cycles ever charged the %q line", src.Label())
		}
	}
	t.Logf("all %d sources charged by cycle %d", len(seen), cycles)
}

// TestARestoredOrganismHasNoLedgerYet: the ledger is deliberately not
// snapshot state, so the panel has to be able to tell "not measured for this
// cycle" apart from "measured and free".
func TestARestoredOrganismHasNoLedgerYet(t *testing.T) {
	loadDefaultGlobals(t)
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 7, CheckpointInterval: 1 << 30})
	for i := 0; i < 20; i++ {
		sim.Update()
	}
	for _, o := range sim.organismManager.Organisms() {
		if !o.HealthLedger().Recorded {
			t.Errorf("organism %d resolved a cycle and reports no ledger", o.ID)
		}
		break
	}
	fresh := NewSimulation(&config.Options{IsHeadless: true, Seed: 7, CheckpointInterval: 1 << 30})
	for _, o := range fresh.organismManager.Organisms() {
		if o.HealthLedger().Recorded {
			t.Errorf("organism %d reports a ledger before any cycle resolved", o.ID)
		}
	}
}
