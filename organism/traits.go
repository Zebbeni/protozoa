package organism

import (
	"math"

	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

// Traits contains organism-specific values that dictate how and when organisms
// perform certain activities, which are passed down from parents to children.
//
// Former traits now applied as globals (same value to every organism):
// ChanceToMutateDecisionTree, PhGrowthEffect (now action-driven via
// ChemoPhEffectPerSize / EatingPhEffectPerFood), PhTolerance, MaxLifespan.
type Traits struct {
	OrganismColor colorful.Color
	MaxSize       float64
	SpawnHealth   float64
	// MinHealthToSpawn: the minimum health needed in order to spawn
	MinHealthToSpawn       float64
	MinCyclesBetweenSpawns int
	IdealPh                float64
}

func newRandomTraits(rng *simrand.RNG) Traits {
	maxSize := rng.Float64() * c.MaximumInitialSize()
	spawnHealth := rng.Float64() * math.Min(maxSize*c.MaxSpawnHealthPercent(), c.MaximumInitialSpawnHealth())
	minHealthToSpawn := spawnHealth + rng.Float64()*(maxSize-spawnHealth)
	minCyclesBetweenSpawns := rng.Intn(c.MaxInitialCyclesBetweenSpawns() + 1)
	idealPh := (c.MaxIdealPh() + c.MinIdealPh()) / 2.0
	return Traits{
		MaxSize:                maxSize,
		SpawnHealth:            spawnHealth,
		MinHealthToSpawn:       minHealthToSpawn,
		MinCyclesBetweenSpawns: minCyclesBetweenSpawns,
		IdealPh:                idealPh,
	}
}

func (t Traits) copyMutated(rng *simrand.RNG) Traits {
	maxSize := mutateFloat(rng, t.MaxSize, 5.0, c.MinimumMaxSize(), c.MaximumMaxSize())
	minCyclesBetweenSpawns := mutateInt(rng, t.MinCyclesBetweenSpawns, 5, 0, c.MaxCyclesBetweenSpawns())
	spawnHealth := mutateFloat(rng, t.SpawnHealth, 0.5, c.MinSpawnHealth(), maxSize*c.MaxSpawnHealthPercent())
	minHealthToSpawn := mutateFloat(rng, t.MinHealthToSpawn, 5.0, spawnHealth, maxSize)
	idealPh := mutateFloat(rng, t.IdealPh, 0.1, c.MinIdealPh(), c.MaxIdealPh())
	return Traits{
		MaxSize:                maxSize,
		SpawnHealth:            spawnHealth,
		MinHealthToSpawn:       minHealthToSpawn,
		MinCyclesBetweenSpawns: minCyclesBetweenSpawns,
		IdealPh:                idealPh,
	}
}

func mutateFloat(rng *simrand.RNG, value, maxChange, min, max float64) float64 {
	mutated := value + maxChange - rng.Float64()*maxChange*2.0
	return math.Min(math.Max(mutated, min), max)
}

func mutateInt(rng *simrand.RNG, value, maxChange, min, max int) int {
	mutated := math.Round(float64(value) + rng.Float64()*float64(maxChange)*2.0 - (float64(maxChange)))
	return int(math.Min(math.Max(mutated, float64(min)), float64(max)))
}
