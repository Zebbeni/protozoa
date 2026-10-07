package simulation

import (
	"os"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// KILLID_PROBE=1 go test ./simulation/ -run TestKillerIdProbe -v
//
// Whether a killer's id is above or below the body it stands on decides
// whether a plain descending-id draw order puts the body underneath.
func TestKillerIdProbe(t *testing.T) {
	if os.Getenv("KILLID_PROBE") == "" {
		t.Skip("set KILLID_PROBE=1")
	}
	loadDefaultGlobals(t)
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 101, CheckpointInterval: 1 << 30})

	killerYounger, killerOlder := 0, 0
	for cycle := 0; cycle < 4000; cycle++ {
		sim.Update()
		at := map[utils.Point][]*organism.Organism{}
		for _, o := range sim.organismManager.Organisms() {
			at[o.Location] = append(at[o.Location], o)
		}
		for _, here := range at {
			if len(here) != 2 {
				continue
			}
			var body, killer *organism.Organism
			for _, o := range here {
				if o.Status == organism.StatusDying {
					body = o
				} else {
					killer = o
				}
			}
			if body == nil || killer == nil {
				continue
			}
			if killer.ID > body.ID {
				killerYounger++
			} else {
				killerOlder++
			}
		}
	}
	total := killerYounger + killerOlder
	t.Logf("of %d body-and-killer cells: killer id ABOVE the body %d (%.0f%%), below %d (%.0f%%)",
		total, killerYounger, 100*float64(killerYounger)/float64(max(total, 1)),
		killerOlder, 100*float64(killerOlder)/float64(max(total, 1)))
	t.Logf("a descending-id order draws the HIGHER id first, so it puts the body underneath "+
		"only in the %d cases where the body's id is higher", killerOlder)
}
