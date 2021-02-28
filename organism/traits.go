package organism

import (
	"image/color"
	"math"
	"math/rand"

	c "github.com/Zebbeni/protozoa/config"

	u "github.com/Zebbeni/protozoa/utils"
)

// Traits contains organism-specific values that dictate how and when organisms
// perform certain activities, which are passed down from parents to children.
type Traits struct {
	OrganismColor color.Color
	// MaxSize represents the maximum size an organism can reach.
	MaxSize float64
	// SpawnHealth: The health value - and size - this organism and its
	// children start with, also equal to what it loses when spawning a child.
	SpawnHealth float64
	// MinHealthToSpawn: the minimum health needed in order to spawn-
	// must be greater than spawnHealth and less than maxSize
	MinHealthToSpawn             float64
	MinCyclesBetweenSpawns       int
	ChanceToMutateDecisionTree   float64
	CyclesToEvaluateDecisionTree int
}

func newRandomTraits() Traits {
	organismColor := u.GetRandomColor()
	maxSize := rand.Float64() * c.MaximumMaxSize()
	spawnHealth := rand.Float64() * maxSize * c.MaxSpawnHealthPercent()
	minHealthToSpawn := spawnHealth + rand.Float64()*(maxSize-spawnHealth)
	minCyclesBetweenSpawns := rand.Intn(c.MaxCyclesBetweenSpawns())
	chanceToMutateDecisionTree := math.Max(c.MinChanceToMutateDecisionTree(), rand.Float64()*c.MaxChanceToMutateDecisionTree())
	maxCycles, minCycles := c.MaxCyclesToEvaluateDecisionTree(), c.MinCyclesToEvaluateDecisionTree()
	cyclesToEvaluateDecisionTree := minCycles + rand.Intn(maxCycles-minCycles)
	return Traits{
		OrganismColor:                organismColor,
		MaxSize:                      maxSize,
		SpawnHealth:                  spawnHealth,
		MinHealthToSpawn:             minHealthToSpawn,
		MinCyclesBetweenSpawns:       minCyclesBetweenSpawns,
		ChanceToMutateDecisionTree:   chanceToMutateDecisionTree,
		CyclesToEvaluateDecisionTree: cyclesToEvaluateDecisionTree,
	}
}

func (t Traits) copyMutated() Traits {
	organismColor := mutateColor(t.OrganismColor)
	// maxSize = previous +- previous +- <5.0, bounded by MinimumMaxSize and MaximumMaxSize
	maxSize := mutateFloat(t.MaxSize, 5.0, c.MinimumMaxSize(), c.MaximumMaxSize())
	// minCyclesBetweenSpawns = previous +- <=5, bounded by 0 and MaxCyclesBetweenSpawns
	minCyclesBetweenSpawns := mutateInt(t.MinCyclesBetweenSpawns, 5, 0, c.MaxCyclesBetweenSpawns())
	// spawnHealth = previous +- <0.5, bounded by MinSpawnHealth and maxSize
	spawnHealth := mutateFloat(t.SpawnHealth, 0.5, c.MinSpawnHealth(), maxSize*c.MaxSpawnHealthPercent())
	// minHealthToSpawn = previous +- <5.0, bounded by spawnHealthPercent and maxSize (both calculated above)
	minHealthToSpawn := mutateFloat(t.MinHealthToSpawn, 5.0, spawnHealth, maxSize)
	// chanceToMutateDecisionTree = previous +- <0.05, bounded by MinChanceToMutateDecisionTree and MaxChanceToMutateDecisionTree
	chanceToMutateDecisionTree := mutateFloat(t.ChanceToMutateDecisionTree, 0.05, c.MinChanceToMutateDecisionTree(), c.MaxChanceToMutateDecisionTree())
	// cyclesToEvaluateDecisionTree = previous +- <5, +0, bounded by MinCyclesToEvaluateDecisionTree and MaxCyclesToEvaluateDecisionTree
	cyclesToEvaluateDecisionTree := mutateInt(t.CyclesToEvaluateDecisionTree, 5, c.MinCyclesToEvaluateDecisionTree(), c.MaxCyclesToEvaluateDecisionTree())
	return Traits{
		OrganismColor:                organismColor,
		MaxSize:                      maxSize,
		SpawnHealth:                  spawnHealth,
		MinHealthToSpawn:             minHealthToSpawn,
		MinCyclesBetweenSpawns:       minCyclesBetweenSpawns,
		ChanceToMutateDecisionTree:   chanceToMutateDecisionTree,
		CyclesToEvaluateDecisionTree: cyclesToEvaluateDecisionTree,
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
	return color.RGBA{R: r, G: g, B: b, A: a}
}

func mutateColorValue(v uint32) uint8 {
	converted := int(uint8(v)) // cast to uint8 and back to int to avoid overflow
	mutated := math.Max(math.Min(float64(converted+rand.Intn(21)-10), 255), 50)
	return uint8(mutated)
}
