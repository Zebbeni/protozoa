package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestBurialSourceProbe measures which path fills the buried layer, by running the four combinations of gradual burial and wall creation.
func TestBurialSourceProbe(t *testing.T) {
	if os.Getenv("BURIAL_PROBE") == "" {
		t.Skip("set BURIAL_PROBE=1 to measure what fills the buried layer")
	}
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	for _, tc := range []struct {
		name                  string
		interval, wallCreated int
	}{
		{"both on (shipped)", 10, -1},
		{"gradual burial off", 0, -1},
		{"wall creation off", 10, 0},
		{"both off", 0, 0},
	} {
		var buried, cells, orgs, walls int
		seeds := []int{53, 7, 21}
		for _, seed := range seeds {
			var g config.Globals
			json.Unmarshal(data, &g)
			g.BurialInterval = tc.interval
			if tc.wallCreated == 0 {
				g.WallCreatedAtZero = 0
				g.WallCreatedSmall, g.WallCreatedMedium, g.WallCreatedLarge = 0, 0, 0
			}
			config.SetGlobals(&g)
			sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
			for i := 0; i < 20000 && !sim.IsDone(); i++ {
				sim.Update()
			}
			for _, v := range sim.GetBuriedFood() {
				buried += v
				cells++
			}
			orgs += sim.OrganismCount()
			walls += sim.WallCount()
		}
		n := len(seeds)
		t.Logf("%-20s buried=%7d  cells=%5d (%2.0f%%)  walls=%5d  orgs=%5d",
			tc.name, buried/n, cells/n, 100*float64(cells/n)/8000, walls/n, orgs/n)
	}
}
