package organism

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

func spawnGlobals(t *testing.T, percent float64) *c.Globals {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g c.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	g.MaxSpawnHealthPercent = percent
	c.SetGlobals(&g)
	return &g
}

// TestAParentSurvivesItsOwnSpawn: an organism spawns when its health
// reaches MinHealthToSpawn and hands SpawnHealth to the child, so a
// threshold equal to what it gives away leaves it with nothing.
//
// Both trait paths floored MinHealthToSpawn at SpawnHealth itself, which
// made that legal, and mutation walks into it because spawning as early as
// possible is selected for right up to the point it is lethal: 5.9% of
// 20,000 mutated generations ended with the two exactly equal, showing in
// the panel as SPAWN HP and CHILD HP reading the same number.
func TestAParentSurvivesItsOwnSpawn(t *testing.T) {
	const eps = 1e-9
	for _, percent := range []float64{0.1, 0.25, 0.5, 0.9} {
		spawnGlobals(t, percent)
		rng := simrand.New(1)
		traits := newRandomTraits(rng)
		for i := 0; i < 20000; i++ {
			traits = traits.copyMutated(rng)
			kept := traits.MinHealthToSpawn - traits.SpawnHealth
			if kept < c.MinSpawnHealth()-eps {
				t.Fatalf("percent %.2f, generation %d: spawns at %.3f and gives %.3f, "+
					"keeping %.3f — under the %.3f minimum, so the parent dies spawning",
					percent, i, traits.MinHealthToSpawn, traits.SpawnHealth,
					kept, c.MinSpawnHealth())
			}
		}
	}
}

// TestRandomFoundersKeepSomething covers the other trait path, and asks
// less of it than TestAParentSurvivesItsOwnSpawn asks of a mutated lineage.
//
// A founder's MaxSize is drawn against maximum_initial_size, which ships at
// 1.0 while minimum_max_size — the floor every later generation is held to
// — is 20. So a founder's whole capacity is under 1 health and it cannot
// hold a child's worth plus the min_spawn_health reserve whatever the
// clamps do; the strong invariant is unsatisfiable there by arithmetic, not
// by a missing check. What is still required is that it keeps something.
//
// Worth knowing rather than asserting away: under the shipped numbers a
// founder spawns at under 1 health and hands most of it over, which is a
// candidate for some of the founding-phase deaths in the sweep records.
// Raising maximum_initial_size is a balance change, not a bug fix.
func TestRandomFoundersKeepSomething(t *testing.T) {
	spawnGlobals(t, 0.25)
	rng := simrand.New(2)
	for i := 0; i < 20000; i++ {
		traits := newRandomTraits(rng)
		if kept := traits.MinHealthToSpawn - traits.SpawnHealth; kept <= 0 {
			t.Fatalf("founder %d spawns at %.3f and gives away %.3f, keeping %.3f",
				i, traits.MinHealthToSpawn, traits.SpawnHealth, kept)
		}
	}
}

// TestTheReserveIsKeptWheneverItFits is the precise statement, and the one
// that tests the code rather than the settings: whenever an organism's max
// size can hold a child's worth of health PLUS the minimum reserve, the
// floor has to deliver that reserve. Only when the arithmetic makes it
// impossible may the parent keep less.
//
// A founder's max size is rng.Float64() * maximum_initial_size — a uniform
// draw from zero, with no lower bound at any setting — so the impossible
// case is always reachable there and never reachable for a mutated lineage,
// which is floored at minimum_max_size.
func TestTheReserveIsKeptWheneverItFits(t *testing.T) {
	const eps = 1e-9
	g := spawnGlobals(t, 0.25)
	// Widened so the draw spans both cases. At the shipped
	// maximum_initial_size of 1.0 a founder's whole capacity is under the
	// min_spawn_health of 1, so the reserve is NEVER arithmetically
	// possible and this would only ever exercise one branch.
	g.MaximumInitialSize = g.MaximumMaxSize
	c.SetGlobals(g)
	rng := simrand.New(4)
	fits, impossible := 0, 0
	for i := 0; i < 20000; i++ {
		traits := newRandomTraits(rng)
		kept := traits.MinHealthToSpawn - traits.SpawnHealth
		if traits.SpawnHealth+c.MinSpawnHealth() <= traits.MaxSize+eps {
			fits++
			if kept < c.MinSpawnHealth()-eps {
				t.Fatalf("founder %d had room (max size %.3f, child HP %.3f) but kept only %.3f",
					i, traits.MaxSize, traits.SpawnHealth, kept)
			}
		} else {
			impossible++
			if kept <= 0 {
				t.Fatalf("founder %d kept nothing at all (%.3f)", i, kept)
			}
		}
	}
	if fits == 0 || impossible == 0 {
		t.Fatalf("only one branch was exercised (%d with room, %d without), "+
			"so this checked less than it claims", fits, impossible)
	}
	t.Logf("%d of 20000 founders had room for the reserve and kept it; "+
		"%d were too small to hold one and kept a positive remainder",
		fits, impossible)
}

// TestSpawnHealthStaysUnderTheConfiguredShare is the setting the symptom
// was first attributed to. It was being applied correctly; this pins that,
// so the next time SPAWN HP and CHILD HP look wrong the percent is already
// ruled out.
func TestSpawnHealthStaysUnderTheConfiguredShare(t *testing.T) {
	const eps = 1e-9
	for _, percent := range []float64{0.1, 0.25, 0.5} {
		spawnGlobals(t, percent)
		rng := simrand.New(3)
		traits := newRandomTraits(rng)
		for i := 0; i < 20000; i++ {
			traits = traits.copyMutated(rng)
			if traits.SpawnHealth > traits.MaxSize*percent+eps {
				t.Fatalf("percent %.2f: child HP %.3f exceeds %.3f (%.0f%% of max size %.3f)",
					percent, traits.SpawnHealth, traits.MaxSize*percent, percent*100, traits.MaxSize)
			}
		}
	}
}

// TestSpawnHealthLeavesRoomForAReserve: the share of max size is not the
// only bound. An organism also cannot promise more than it can hold
// alongside the minimum it must keep, which binds when the percent is high.
func TestSpawnHealthLeavesRoomForAReserve(t *testing.T) {
	spawnGlobals(t, 0.9)
	for _, maxSize := range []float64{2, 5, 20, 50, 100} {
		cap := spawnHealthCap(maxSize)
		if cap > maxSize-c.MinSpawnHealth() && cap > c.MinSpawnHealth() {
			t.Errorf("at max size %.1f the cap is %.3f, leaving under the %.3f reserve",
				maxSize, cap, c.MinSpawnHealth())
		}
		if cap > maxSize*0.9+1e-9 {
			t.Errorf("at max size %.1f the cap %.3f exceeds the configured share", maxSize, cap)
		}
	}
}

// TestDesignedOrganismsGetTheSameFloor: a hand-built design goes through
// its own clamps, so the floor has to be applied there too or the designer
// can save an organism that dies on its first spawn.
func TestDesignedOrganismsGetTheSameFloor(t *testing.T) {
	spawnGlobals(t, 0.25)
	ds := NewDesign("spawn-floor")
	ds.MaxSize = 40
	ds.SpawnHealth = 8
	ds.MinHealthToSpawn = 8 // asking to die
	traits, err := ds.Traits()
	if err != nil {
		t.Fatal(err)
	}
	if kept := traits.MinHealthToSpawn - traits.SpawnHealth; kept < c.MinSpawnHealth()-1e-9 {
		t.Errorf("a design asking to spawn at exactly its child HP was allowed: "+
			"spawns at %.3f, gives %.3f, keeps %.3f",
			traits.MinHealthToSpawn, traits.SpawnHealth, kept)
	}
}
