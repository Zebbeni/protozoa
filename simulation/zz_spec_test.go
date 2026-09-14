package simulation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// Which abilities reach full specialisation, over more seeds and longer
// runs than the guard test, at a given chemosynthesis tolerance.
func TestSpecReach(t *testing.T) {
	tolStr := os.Getenv("SPEC_TOL")
	if tolStr == "" {
		t.Skip("set SPEC_TOL")
	}
	tol, _ := strconv.ParseFloat(tolStr, 64)
	data, _ := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	var g config.Globals
	json.Unmarshal(data, &g)
	g.ChemosynthesisTolerance = tol
	config.SetGlobals(&g)

	peak := map[physiology.Ability]int{}
	spec := map[physiology.Ability]int{}
	seedsWith := map[physiology.Ability]int{}
	for _, seed := range []int{101, 53, 202, 999, 31, 44, 128, 12345} {
		seen := map[physiology.Ability]bool{}
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
		for i := 0; i < 10000 && !sim.IsDone(); i++ {
			sim.Update()
			for _, o := range sim.organismManager.Organisms() {
				s := o.Traits().Abilities
				for _, a := range physiology.AllAbilities {
					if s[a] > peak[a] {
						peak[a] = s[a]
					}
					if s[a] >= physiology.SpecialistScore(a) {
						spec[a]++
						seen[a] = true
					}
				}
			}
		}
		for a := range seen {
			seedsWith[a]++
		}
	}
	for _, a := range physiology.AllAbilities {
		t.Logf("RESULT tol=%-4s %-15s peak %3d  specialist organism-cycles %8d  seeds with a specialist %d/8",
			tolStr, a.Name(), peak[a], spec[a], seedsWith[a])
	}
	_ = fmt.Sprint
}
