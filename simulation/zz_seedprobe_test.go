package simulation

import (
	"os"
	"testing"
)

func TestFindTailOffSeed(t *testing.T) {
	if os.Getenv("SEED_PROBE") == "" {
		t.Skip("set SEED_PROBE=1 to search for a tail-off seed")
	}
	const min = 10
	for seed := 1; seed <= 24; seed++ {
		globalsForTailOff(t, min)
		sim, peak := runUntilDone(t, seed, 20000)
		alive := sim.OrganismCount()
		usable := sim.IsDone() && (alive == 0 ||
			(sim.minOrganismsArmed && peak >= 2*min && alive < min))
		t.Logf("seed %2d: ended=%-5v cycle %6d alive %3d peak %3d usable=%v",
			seed, sim.IsDone(), sim.Cycle(), alive, peak, usable)
	}
}
