package organism

import (
	"image/color"
	"math"
	"math/rand"

	c "github.com/Zebbeni/protozoa/constants"
	u "github.com/Zebbeni/protozoa/utils"
)

// Traits contains organism-specific values that dictate how and when organisms
// perform certain activities, which are passed down from parents to children.
type Traits struct {
	organismColor color.Color
	// maxSize represents the maximum size an organism can reach.
	maxSize float64
	// spawnHealth: The health value - and size - this organism and its
	// children start with, also equal to what it loses when spawning a child.
	spawnHealth float64
	// minHealthToSpawn: the minimum health needed in order to spawn-
	// must be greater than spawnHealth and less than maxSize
	minHealthToSpawn             float64
	minCyclesBetweenSpawns       int
	chanceToMutateDecisionTree   float64
	cyclesToEvaluateDecisionTree int
}

func (t *Traits) copy() *Traits {
	return &Traits{
		organismColor:                t.organismColor,
		maxSize:                      t.maxSize,
		spawnHealth:                  t.spawnHealth,
		minHealthToSpawn:             t.minHealthToSpawn,
		minCyclesBetweenSpawns:       t.minCyclesBetweenSpawns,
		cyclesToEvaluateDecisionTree: t.cyclesToEvaluateDecisionTree,
		chanceToMutateDecisionTree:   t.chanceToMutateDecisionTree,
	}
}

func newRandomTraits() *Traits {
	organismColor := u.GetRandomColor()
	maxSize := rand.Float64() * c.MaximumMaxSize
	spawnHealth := rand.Float64() * maxSize * c.MaxSpawnHealthPercent
	minHealthToSpawn := spawnHealth + rand.Float64()*(maxSize-spawnHealth)
	minCyclesBetweenSpawns := rand.Intn(c.MaxCyclesBetweenSpawns)
	chanceToMutateDecisionTree := math.Max(c.MinChanceToMutateDecisionTree, rand.Float64()*c.MaxChanceToMutateDecisionTree)
	cyclesToEvaluateDecisionTree := rand.Intn(c.MaxCyclesToEvaluateDecisionTree)
	return &Traits{
		organismColor:                organismColor,
		maxSize:                      maxSize,
		spawnHealth:                  spawnHealth,
		minHealthToSpawn:             minHealthToSpawn,
		minCyclesBetweenSpawns:       minCyclesBetweenSpawns,
		chanceToMutateDecisionTree:   chanceToMutateDecisionTree,
		cyclesToEvaluateDecisionTree: cyclesToEvaluateDecisionTree,
	}
}

func (t *Traits) copyMutated() *Traits {
	organismColor := mutateColor(t.organismColor)
	// maxSize = previous +- previous +- <5.0, bounded by MinimumMaxSize and MaximumMaxSize
	maxSize := mutateFloat(t.maxSize, 5.0, c.MinimumMaxSize, c.MaximumMaxSize)
	// minCyclesBetweenSpawns = previous +- <=5, bounded by 0 and MaxCyclesBetweenSpawns
	minCyclesBetweenSpawns := mutateInt(t.minCyclesBetweenSpawns, 5, 0, c.MaxCyclesBetweenSpawns)
	// spawnHealth = previous +- <0.05, bounded by MinSpawnHealth and maxSize
	spawnHealth := mutateFloat(t.spawnHealth, 0.1, c.MinSpawnHealth, maxSize*c.MaxSpawnHealthPercent)
	// minHealthToSpawn = previous +- <5.0, bounded by spawnHealthPercent and maxSize (both calculated above)
	minHealthToSpawn := mutateFloat(t.minHealthToSpawn, 5.0, spawnHealth, maxSize)
	// chanceToMutateDecisionTree = previous +- <0.01, bounded by MinChanceToMutateDecisionTree and MaxChanceToMutateDecisionTree
	chanceToMutateDecisionTree := mutateFloat(t.chanceToMutateDecisionTree, 0.01, c.MinChanceToMutateDecisionTree, c.MaxChanceToMutateDecisionTree)
	// cyclesToEvaluateDecisionTree = previous +1, +0 or +(-1), bounded by 1 and CyclesToEvaluateDecisionTree
	cyclesToEvaluateDecisionTree := mutateInt(t.cyclesToEvaluateDecisionTree, 1, 1, c.MaxCyclesToEvaluateDecisionTree)
	return &Traits{
		organismColor:                organismColor,
		maxSize:                      maxSize,
		spawnHealth:                  spawnHealth,
		minHealthToSpawn:             minHealthToSpawn,
		minCyclesBetweenSpawns:       minCyclesBetweenSpawns,
		chanceToMutateDecisionTree:   chanceToMutateDecisionTree,
		cyclesToEvaluateDecisionTree: cyclesToEvaluateDecisionTree,
	}
}

func mutateFloat(value, maxChange, min, max float64) float64 {
	mutated := value + maxChange - rand.Float64()*maxChange*2.0
	return math.Min(math.Max(mutated, min), max)
}

func mutateInt(value, maxChange, min, max int) int {
	mutated := math.Round(float64(value) + rand.Float64()*float64(maxChange)*2.0 - (float64(maxChange)))
	return int(math.Min(math.Max(mutated, float64(min)), float64(max)))
}

// MutateColor returns a slight variation on a given color
func mutateColor(originalColor color.Color) color.Color {
	r32, g32, b32, a32 := originalColor.RGBA()
	r := mutateColorValue(r32)
	g := mutateColorValue(g32)
	b := mutateColorValue(b32)
	a := uint8(a32)
	return color.RGBA{r, g, b, a}
}

func mutateColorValue(v uint32) uint8 {
	converted := int(uint8(v)) // cast to uint8 and back to int to avoid overflow
	mutated := math.Max(math.Min(float64(converted+rand.Intn(21)-10), 255), 50)
	return uint8(mutated)
}
