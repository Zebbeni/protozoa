package organism

import (
	"math"

	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

// Traits contains organism-specific values that dictate how and when organisms
// perform certain activities, which are passed down from parents to children.
type Traits struct {
	OrganismColor colorful.Color
	MaxSize       float64
	SpawnHealth   float64
	// MinHealthToSpawn: the minimum health needed in order to spawn
	MinHealthToSpawn           float64
	MinCyclesBetweenSpawns     int
	ChanceToMutateDecisionTree float64
	IdealPh                    float64
	PhTolerance                float64
	PhGrowthEffect             float64
	// MaxLifespan is the number of cycles the organism lives before dying
	// automatically. Zero means "no lifespan limit" — the manager only
	// enforces this trait when config.MaxMaxLifespan() > 0.
	MaxLifespan int
}

func newRandomTraits(rng *simrand.RNG) Traits {
	maxSize := rng.Float64() * c.MaximumInitialSize()
	spawnHealth := rng.Float64() * math.Min(maxSize*c.MaxSpawnHealthPercent(), c.MaximumInitialSpawnHealth())
	minHealthToSpawn := spawnHealth + rng.Float64()*(maxSize-spawnHealth)
	minCyclesBetweenSpawns := rng.Intn(c.MaxInitialCyclesBetweenSpawns() + 1)
	chanceToMutateDecisionTree := math.Max(c.MinChanceToMutateDecisionTree(), rng.Float64()*c.MaxChanceToMutateDecisionTree())
	idealPh := (c.MaxIdealPh() + c.MinIdealPh()) / 2.0
	phTolerance := rng.Float64() * c.MaxPhToleranceRange()
	phGrowthEffect := rng.Float64()*(c.MaxOrganismPhGrowthEffect()*2.0) - c.MaxOrganismPhGrowthEffect()
	maxLifespan := randomMaxLifespan(rng)
	return Traits{
		MaxSize:                    maxSize,
		SpawnHealth:                spawnHealth,
		MinHealthToSpawn:           minHealthToSpawn,
		MinCyclesBetweenSpawns:     minCyclesBetweenSpawns,
		ChanceToMutateDecisionTree: chanceToMutateDecisionTree,
		IdealPh:                    idealPh,
		PhTolerance:                phTolerance,
		PhGrowthEffect:             phGrowthEffect,
		MaxLifespan:                maxLifespan,
	}
}

func (t Traits) copyMutated(rng *simrand.RNG) Traits {
	maxSize := mutateFloat(rng, t.MaxSize, 5.0, c.MinimumMaxSize(), c.MaximumMaxSize())
	minCyclesBetweenSpawns := mutateInt(rng, t.MinCyclesBetweenSpawns, 5, 0, c.MaxCyclesBetweenSpawns())
	spawnHealth := mutateFloat(rng, t.SpawnHealth, 0.5, c.MinSpawnHealth(), maxSize*c.MaxSpawnHealthPercent())
	minHealthToSpawn := mutateFloat(rng, t.MinHealthToSpawn, 5.0, spawnHealth, maxSize)
	chanceToMutateDecisionTree := mutateFloat(rng, t.ChanceToMutateDecisionTree, 0.05, c.MinChanceToMutateDecisionTree(), c.MaxChanceToMutateDecisionTree())
	phEffect := mutateFloat(rng, t.PhGrowthEffect, c.MaxPhEffectChange(), c.MaxOrganismPhGrowthEffect()*-1, c.MaxOrganismPhGrowthEffect())
	idealPh := mutateFloat(rng, t.IdealPh, 0.1, c.MinIdealPh(), c.MaxIdealPh())
	phTolerance := mutateFloat(rng, t.PhTolerance, 0.1, c.MinPhToleranceRange(), c.MaxPhToleranceRange())
	maxLifespan := mutateMaxLifespan(rng, t.MaxLifespan)
	return Traits{
		MaxSize:                    maxSize,
		SpawnHealth:                spawnHealth,
		MinHealthToSpawn:           minHealthToSpawn,
		MinCyclesBetweenSpawns:     minCyclesBetweenSpawns,
		ChanceToMutateDecisionTree: chanceToMutateDecisionTree,
		IdealPh:                    idealPh,
		PhTolerance:                phTolerance,
		PhGrowthEffect:             phEffect,
		MaxLifespan:                maxLifespan,
	}
}

// randomMaxLifespan picks an initial MaxLifespan value in [MinMaxLifespan,
// MaxMaxLifespan]. Returns 0 when lifespan enforcement is disabled
// (MaxMaxLifespan <= 0).
func randomMaxLifespan(rng *simrand.RNG) int {
	max := c.MaxMaxLifespan()
	if max <= 0 {
		return 0
	}
	min := c.MinMaxLifespan()
	if min < 0 {
		min = 0
	}
	if min > max {
		min = max
	}
	return min + rng.Intn(max-min+1)
}

// mutateMaxLifespan mutates a parent's lifespan by up to MaxMaxLifespanChange
// cycles in either direction, clamped to [MinMaxLifespan, MaxMaxLifespan].
// Returns 0 whenever lifespan enforcement is disabled.
func mutateMaxLifespan(rng *simrand.RNG, parent int) int {
	max := c.MaxMaxLifespan()
	if max <= 0 {
		return 0
	}
	min := c.MinMaxLifespan()
	if min < 0 {
		min = 0
	}
	if min > max {
		min = max
	}
	return mutateInt(rng, parent, c.MaxMaxLifespanChange(), min, max)
}

func mutateFloat(rng *simrand.RNG, value, maxChange, min, max float64) float64 {
	mutated := value + maxChange - rng.Float64()*maxChange*2.0
	return math.Min(math.Max(mutated, min), max)
}

func mutateInt(rng *simrand.RNG, value, maxChange, min, max int) int {
	mutated := math.Round(float64(value) + rng.Float64()*float64(maxChange)*2.0 - (float64(maxChange)))
	return int(math.Min(math.Max(mutated, float64(min)), float64(max)))
}
