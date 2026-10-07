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

// TestAParentKeepsItsShareOfTheThreshold is what max_spawn_health_percent
// means: a child gets at most that share of the health the parent had to
// reach to spawn, so the parent keeps the rest of it.
//
// It used to be a share of MAX SIZE, which says nothing about what the
// parent is holding when it actually spawns — an organism with max size 100,
// a threshold of 10 and a 0.1 share could still hand over all 10 and die
// doing it. The cap has to be against the threshold for the setting to mean
// what its name says.
func TestAParentKeepsItsShareOfTheThreshold(t *testing.T) {
	const eps = 1e-9
	for _, percent := range []float64{0.1, 0.25, 0.5, 0.9} {
		spawnGlobals(t, percent)
		rng := simrand.New(1)
		traits := newRandomTraits(rng)
		for i := 0; i < 20000; i++ {
			traits = traits.copyMutated(rng)
			kept := traits.MinHealthToSpawn - traits.SpawnHealth
			want := (1 - percent) * traits.MinHealthToSpawn
			if kept < want-eps {
				t.Fatalf("percent %.2f, generation %d: spawns at %.4f and gives %.4f, "+
					"keeping %.4f of the %.4f its share promises",
					percent, i, traits.MinHealthToSpawn, traits.SpawnHealth, kept, want)
			}
		}
	}
}

// TestNoParentGivesEverythingAway: whatever the share, a parent under 1 keeps
// something, which is the failure first reported — SPAWN HP and CHILD HP
// reading the same number on the panel.
func TestNoParentGivesEverythingAway(t *testing.T) {
	for _, percent := range []float64{0.1, 0.5, 0.9, 0.99} {
		spawnGlobals(t, percent)
		rng := simrand.New(6)
		traits := newRandomTraits(rng)
		for i := 0; i < 5000; i++ {
			traits = traits.copyMutated(rng)
			if kept := traits.MinHealthToSpawn - traits.SpawnHealth; kept <= 0 {
				t.Fatalf("percent %.2f, generation %d: spawns at %.4f and gives %.4f",
					percent, i, traits.MinHealthToSpawn, traits.SpawnHealth)
			}
		}
	}
}

// TestRandomFoundersKeepTheirShare covers the other trait path.
func TestRandomFoundersKeepTheirShare(t *testing.T) {
	const eps = 1e-9
	for _, percent := range []float64{0.25, 0.5} {
		spawnGlobals(t, percent)
		rng := simrand.New(2)
		for i := 0; i < 20000; i++ {
			traits := newRandomTraits(rng)
			kept := traits.MinHealthToSpawn - traits.SpawnHealth
			if want := (1 - percent) * traits.MinHealthToSpawn; kept < want-eps {
				t.Fatalf("percent %.2f: founder %d keeps %.4f of %.4f",
					percent, i, kept, want)
			}
		}
	}
}

// TestTheCapIsTheShareOfTheThreshold states the rule directly, against the
// helper rather than through a trait walk, so a change to the formula fails
// here with the formula in the message.
func TestTheCapIsTheShareOfTheThreshold(t *testing.T) {
	// Inside the bounds: a share of 1 is clamped on the way in, so asking
	// for it here would test the clamp rather than the formula.
	for _, percent := range []float64{0.1, 0.5, c.MaxSpawnHealthShare} {
		spawnGlobals(t, percent)
		for _, threshold := range []float64{2, 10, 40, 100} {
			if got, want := spawnHealthCap(threshold), threshold*percent; got != want {
				t.Errorf("percent %.2f, threshold %.1f: cap %.4f, want %.4f",
					percent, threshold, got, want)
			}
		}
	}
}

// TestTheThresholdFloorKeepsChildrenViable: a child still has to be worth
// producing, so the threshold cannot sit so low that its share is under
// min_spawn_health. The floor is what stops a lineage evolving toward
// spawning constantly for nothing.
func TestTheThresholdFloorKeepsChildrenViable(t *testing.T) {
	for _, percent := range []float64{0.1, 0.25, 0.5} {
		spawnGlobals(t, percent)
		want := c.MinSpawnHealth() / percent
		if got := spawnThresholdFloor(); got != want {
			t.Errorf("percent %.2f: floor %.4f, want %.4f", percent, got, want)
		}
		// At the floor, the share is exactly a viable child.
		if got := spawnHealthCap(want); got < c.MinSpawnHealth()-1e-9 {
			t.Errorf("percent %.2f: at the floor a child gets %.4f, under the %.4f minimum",
				percent, got, c.MinSpawnHealth())
		}
	}
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

// TestDesignedOrganismsGetTheSameFloor: a hand-built design goes through
// its own clamps, so the floor has to be applied there too or the designer
// can save an organism that dies on its first spawn.
func TestDesignedOrganismsGetTheSameFloor(t *testing.T) {
	spawnGlobals(t, 0.25)
	ds := NewDesign("spawn-floor")
	ds.MaxSize = 40
	ds.SpawnHealth = 8
	ds.MinHealthToSpawn = 8 // asking to give away everything it has
	traits, err := ds.Traits()
	if err != nil {
		t.Fatal(err)
	}
	kept := traits.MinHealthToSpawn - traits.SpawnHealth
	if want := 0.75 * traits.MinHealthToSpawn; kept < want-1e-9 {
		t.Errorf("a design asking to spawn at exactly its child HP was allowed: "+
			"spawns at %.3f, gives %.3f, keeps %.3f of the %.3f a 0.25 share promises",
			traits.MinHealthToSpawn, traits.SpawnHealth, kept, want)
	}
}

// TestTheSpawnShareIsBoundedBelowOne: at a share of 1 a parent hands over
// everything it had to reach and dies spawning, which is the failure the
// setting exists to prevent, and a share near 1 is that failure scaled down.
// Both ends are clamped on the way in, so a hand-edited file or an old replay
// header cannot ask for it.
func TestTheSpawnShareIsBoundedBelowOne(t *testing.T) {
	for _, asked := range []float64{1, 1.5, 100} {
		spawnGlobals(t, asked)
		if got := c.MaxSpawnHealthPercent(); got != c.MaxSpawnHealthShare {
			t.Errorf("asked for a share of %.2f, got %.2f, want it held to %.2f",
				asked, got, c.MaxSpawnHealthShare)
		}
	}
	for _, asked := range []float64{0, -1} {
		spawnGlobals(t, asked)
		if got := c.MaxSpawnHealthPercent(); got != c.MinSpawnHealthShare {
			t.Errorf("asked for a share of %.2f, got %.2f, want it held to %.2f",
				asked, got, c.MinSpawnHealthShare)
		}
	}
	// A value inside the range is left alone.
	spawnGlobals(t, 0.25)
	if got := c.MaxSpawnHealthPercent(); got != 0.25 {
		t.Errorf("a share of 0.25 was changed to %.4f", got)
	}
}

// TestAParentAlwaysKeepsATenth follows from the ceiling: whatever a file
// asks for, a parent keeps at least 1 - MaxSpawnHealthShare of its threshold.
func TestAParentAlwaysKeepsATenth(t *testing.T) {
	const eps = 1e-9
	spawnGlobals(t, 1) // clamped to the ceiling
	rng := simrand.New(9)
	traits := newRandomTraits(rng)
	for i := 0; i < 20000; i++ {
		traits = traits.copyMutated(rng)
		kept := traits.MinHealthToSpawn - traits.SpawnHealth
		if want := (1 - c.MaxSpawnHealthShare) * traits.MinHealthToSpawn; kept < want-eps {
			t.Fatalf("generation %d keeps %.4f of the %.4f the ceiling promises", i, kept, want)
		}
	}
}
