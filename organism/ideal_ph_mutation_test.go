package organism

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

func globalsWithIdealPhStep(t *testing.T, step float64) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	g.IdealPhMutationStep = step
	config.SetGlobals(&g)
}

// widestIdealPhDrift mutates one lineage repeatedly and returns how far
// its ideal pH ever wandered from where it started.
func widestIdealPhDrift(step float64, generations int) float64 {
	const start = 5.0
	traits := Traits{IdealPh: start, MaxSize: 50, SpawnHealth: 1, MinHealthToSpawn: 10}
	rng := simrand.New(7)
	widest := 0.0
	for i := 0; i < generations; i++ {
		traits = traits.copyMutated(rng)
		widest = max(widest, math.Abs(traits.IdealPh-start))
	}
	return widest
}

// TestIdealPhMutationStepBoundsTheDrift: the setting is how far a child's
// ideal pH can land from its parent's, so 0 pins a lineage to the pH it
// started at and a bigger step lets it chase a drifting world faster.
func TestIdealPhMutationStepBoundsTheDrift(t *testing.T) {
	globalsWithIdealPhStep(t, 0)
	if drift := widestIdealPhDrift(0, 200); drift != 0 {
		t.Errorf("with a step of 0 the ideal pH drifted %v; lineages should be pinned", drift)
	}

	globalsWithIdealPhStep(t, 0.1)
	small := widestIdealPhDrift(0.1, 200)
	globalsWithIdealPhStep(t, 0.5)
	large := widestIdealPhDrift(0.5, 200)
	if !(small > 0 && large > small) {
		t.Errorf("drift over 200 generations: %v at step 0.1, %v at step 0.5; want a bigger step to drift further", small, large)
	}
}

// TestOneMutationStaysWithinTheStep: a single generation can't jump
// further than the configured step.
func TestOneMutationStaysWithinTheStep(t *testing.T) {
	const step = 0.25
	globalsWithIdealPhStep(t, step)
	rng := simrand.New(11)
	traits := Traits{IdealPh: 5, MaxSize: 50, SpawnHealth: 1, MinHealthToSpawn: 10}
	for i := 0; i < 500; i++ {
		child := traits.copyMutated(rng)
		if jump := math.Abs(child.IdealPh - traits.IdealPh); jump > step+1e-12 {
			t.Fatalf("generation %d jumped %v, over the %v step", i, jump, step)
		}
		traits = child
	}
}
