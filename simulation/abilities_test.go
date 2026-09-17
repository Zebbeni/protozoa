package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

func loadDefaultGlobals(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	config.SetGlobals(&g)
}

// TestAbilitiesDivergeAndStayValid runs a live simulation and checks
// that ability scores actually spread across the population while every
// organism keeps a valid budget.
//
// Divergence is the point of the scores — if inheritance dropped the
// mutation, every organism would sit on the genesis distribution
// forever and the whole system would look like it worked while
// selecting on nothing.
func TestAbilitiesDivergeAndStayValid(t *testing.T) {
	loadDefaultGlobals(t)

	sim := NewSimulation(&config.Options{
		IsHeadless: true, Seed: 101, CheckpointInterval: 1 << 30,
	})

	genesis := physiology.BalancedScores()
	distinct := map[physiology.Scores]struct{}{}

	for i := 0; i < 4000; i++ {
		sim.Update()
		if sim.OrganismCount() == 0 {
			break
		}
		for _, o := range sim.organismManager.Organisms() {
			s := o.Traits().Abilities
			if err := s.Validate(); err != nil {
				t.Fatalf("organism %d has an invalid budget at cycle %d: %v", o.ID, sim.Cycle(), err)
			}
			distinct[s] = struct{}{}
		}
	}

	if len(distinct) < 2 {
		t.Fatalf("every organism still holds the genesis distribution %v — "+
			"ability mutation is not reaching children", genesis)
	}
	t.Logf("%d distinct ability distributions seen across the run", len(distinct))
}

// TestAbilitiesSurviveSnapshotRoundtrip pins inheritance through the
// save path. Scores don't drive behaviour yet, so a restore that
// dropped them would be completely invisible today — and would surface
// later as an unexplained balance problem once they do.
func TestAbilitiesSurviveSnapshotRoundtrip(t *testing.T) {
	loadDefaultGlobals(t)

	sim := NewSimulation(&config.Options{
		IsHeadless: true, Seed: 53, CheckpointInterval: 1 << 30,
	})
	// Long enough for mutation to move some lineages off genesis, so
	// the test would catch a restore that silently reset everyone.
	for i := 0; i < 3000; i++ {
		sim.Update()
		if sim.OrganismCount() == 0 {
			t.Fatal("population died out before the snapshot could be taken")
		}
	}

	snap := sim.CaptureSnapshot()
	if snap == nil {
		t.Fatal("CaptureSnapshot returned nil")
	}

	before := map[int]physiology.Scores{}
	offGenesis := 0
	genesis := physiology.BalancedScores()
	for _, o := range sim.organismManager.Organisms() {
		before[o.ID] = o.Traits().Abilities
		if o.Traits().Abilities != genesis {
			offGenesis++
		}
	}
	if offGenesis == 0 {
		t.Skip("no lineage drifted off genesis in this run; nothing to distinguish")
	}

	restored, err := RestoreFromSnapshot(snap, &config.Options{
		IsHeadless: true, Seed: 53, CheckpointInterval: 1 << 30,
	})
	if err != nil {
		t.Fatalf("RestoreFromSnapshot: %v", err)
	}

	checked := 0
	for _, o := range restored.organismManager.Organisms() {
		want, ok := before[o.ID]
		if !ok {
			continue
		}
		if got := o.Traits().Abilities; got != want {
			t.Errorf("organism %d: abilities %v after restore, want %v", o.ID, got, want)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no organisms matched by ID across the restore — nothing was verified")
	}
	t.Logf("verified %d organisms (%d off the genesis distribution)", checked, offGenesis)
}

// TestAppearanceVariesInLiveSim is the end-to-end check on the derived
// look. Thresholds set too high would leave every organism drawn as a
// bare basic body forever — the feature would be silently inert, and
// nothing in the unit tests would say so because each one sets the
// scores it needs by hand.
func TestAppearanceVariesInLiveSim(t *testing.T) {
	loadDefaultGlobals(t)

	sim := NewSimulation(&config.Options{
		IsHeadless: true, Seed: 101, CheckpointInterval: 1 << 30,
	})

	bodies := map[physiology.BodyClass]int{}
	motors := map[physiology.MotorClass]int{}
	mouths := map[physiology.MouthClass]int{}
	sensors := map[physiology.SensorClass]int{}

	for i := 0; i < 8000; i++ {
		sim.Update()
		if sim.OrganismCount() == 0 {
			break
		}
		for _, o := range sim.organismManager.Organisms() {
			app := o.Appearance()
			bodies[app.Body]++
			motors[app.Motor]++
			mouths[app.Mouth]++
			sensors[app.Sensor]++
		}
	}

	t.Logf("bodies  %v", bodies)
	t.Logf("motors  %v", motors)
	t.Logf("mouths  %v", mouths)
	t.Logf("sensors %v", sensors)

	for _, tc := range []struct {
		name     string
		distinct int
	}{
		{"body", len(bodies)},
		{"motor", len(motors)},
		{"mouth", len(mouths)},
		{"sensor", len(sensors)},
	} {
		if tc.distinct < 2 {
			t.Errorf("%s never varied across the run (%d class seen) — "+
				"its threshold is likely unreachable", tc.name, tc.distinct)
		}
	}
}

// TestEverySpecialistIsReachable is the ecological check the unit tests
// can't make: across a real run, does every ability get invested in
// heavily by someone?
//
// An ability no lineage ever concentrates points into is one the
// strategy space has effectively lost, however good its curve looks in
// isolation. This is what caught the fitness valley — when the top of
// the multiplier curve was anchored at the 100-point budget, non-chemo
// abilities cost nine times more to grow than to abandon, and nothing
// but chemosynthesis ever specialised.
func TestEverySpecialistIsReachable(t *testing.T) {
	loadDefaultGlobals(t)

	// "Specialist" = physiology.SpecialistScore (60 of a possible 100).
	peak := map[physiology.Ability]int{}
	specialists := map[physiology.Ability]int{}

	// Six seeds, not three: rarer specialisations (Attack especially)
	// appear in most seeds but not all, and a three-seed sample missed
	// Attack by a single point even though full attack specialists show
	// up in 6 of 8 seeds over longer runs.
	for _, seed := range []int{101, 53, 202, 999, 31, 44} {
		sim := NewSimulation(&config.Options{
			IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30,
		})
		for i := 0; i < 8000; i++ {
			sim.Update()
			if sim.OrganismCount() == 0 {
				break
			}
			for _, o := range sim.organismManager.Organisms() {
				s := o.Traits().Abilities
				for _, a := range physiology.AllAbilities {
					if s[a] > peak[a] {
						peak[a] = s[a]
					}
					if s[a] >= physiology.SpecialistScore {
						specialists[a]++
					}
				}
			}
		}
	}

	for _, a := range physiology.AllAbilities {
		t.Logf("%-16s peak %3d   full specialists seen: %d",
			a.Name(), peak[a], specialists[a])
	}
	for _, a := range physiology.AllAbilities {
		if specialists[a] == 0 {
			t.Errorf("no organism ever fully specialised in %s (peak %d) — "+
				"that ability is unreachable as a strategy", a.Name(), peak[a])
		}
	}
}
