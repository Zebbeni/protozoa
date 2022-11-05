package organism

import (
	"math"
	"math/rand"

	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
)

const (
	maxHueMutation        = 5.0
	maxSaturationMutation = 0.05
	maxSaturation         = 1.0
	minSaturation         = 0.5
	maxLuminanceMutation  = 0.05
	maxLuminance          = 0.8
	minLuminance          = 0.4
)

// Traits contains organism-specific values that dictate how and when organisms
// perform certain activities, which are passed down from parents to children.
type Traits struct {
	OrganismColor colorful.Color
	// MaxSize represents the maximum size an organism can reach.
	MaxSize float64
	// SpawnHealth: The health value - and size - this organism and its
	// children start with, also equal to what it loses when spawning a child.
	SpawnHealth float64
	// MinHealthToSpawn: the minimum health needed in order to spawn-
	// must be greater than spawnHealth and less than maxSize
	MinHealthToSpawn           float64
	MinCyclesBetweenSpawns     int
	ChanceToMutateDecisionTree float64
	// IdealPh: the middle of the ph range the organism can tolerate without
	// suffering health damage
	IdealPh float64
	// PhTolerance: the distance from IdealPh the organism can handle without
	// suffering health effects due to ph
	PhTolerance float64
	// PhGrowthEffect: the effect organism has on the environment's ph level at its
	// current location, a small positive or negative number which gets
	// multiplied by the organism's current size
	PhGrowthEffect float64
}

func newRandomTraits() Traits {
	organismColor := getRandomColor()
	maxSize := rand.Float64() * c.MaximumMaxSize()
	spawnHealth := rand.Float64() * maxSize * c.MaxSpawnHealthPercent()
	minHealthToSpawn := spawnHealth + rand.Float64()*(maxSize-spawnHealth)
	minCyclesBetweenSpawns := rand.Intn(c.MaxCyclesBetweenSpawns())
	chanceToMutateDecisionTree := math.Max(c.MinChanceToMutateDecisionTree(), rand.Float64()*c.MaxChanceToMutateDecisionTree())
	idealPh := rand.Float64()*(c.MaxIdealPh()-c.MinIdealPh()) + c.MinIdealPh()
	phTolerance := rand.Float64() * c.MaxPhTolerance()
	phGrowthEffect := rand.Float64()*(c.MaxOrganismPhGrowthEffect()*2.0) - c.MaxOrganismPhGrowthEffect()
	return Traits{
		OrganismColor:              organismColor,
		MaxSize:                    maxSize,
		SpawnHealth:                spawnHealth,
		MinHealthToSpawn:           minHealthToSpawn,
		MinCyclesBetweenSpawns:     minCyclesBetweenSpawns,
		ChanceToMutateDecisionTree: chanceToMutateDecisionTree,
		IdealPh:                    idealPh,
		PhTolerance:                phTolerance,
		PhGrowthEffect:             phGrowthEffect,
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
	// phEffect = previous +- 0.001, bounded by MaxOrganismPhGrowthEffect (and -1 * MaxOrganismPhGrowthEffect)
	phEffect := mutateFloat(t.PhGrowthEffect, .001, c.MaxOrganismPhGrowthEffect()*-1, c.MaxOrganismPhGrowthEffect())
	// ideaLPh = previous += 0.1, bounded by MinIdealPh and MaxIdealPh
	idealPh := mutateFloat(t.IdealPh, 0.1, c.MinIdealPh(), c.MaxIdealPh())
	// phTolerance = previous +- 0.1, bounded by MinPhTolerance and MaxPhTolerance
	phTolerance := mutateFloat(t.PhTolerance, 0.1, c.MinPhTolerance(), c.MaxPhTolerance())
	return Traits{
		OrganismColor:              organismColor,
		MaxSize:                    maxSize,
		SpawnHealth:                spawnHealth,
		MinHealthToSpawn:           minHealthToSpawn,
		MinCyclesBetweenSpawns:     minCyclesBetweenSpawns,
		ChanceToMutateDecisionTree: chanceToMutateDecisionTree,
		IdealPh:                    idealPh,
		PhTolerance:                phTolerance,
		PhGrowthEffect:             phEffect,
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
func mutateColor(originalColor colorful.Color) colorful.Color {
	h, s, l := originalColor.HSLuv()
	h = mutateHue(h)
	s = mutateSaturation(s)
	l = mutateLuminance(l)
	return colorful.HSLuv(h, s, l)
}

func mutateHue(h float64) float64 {
	return math.Mod(h+360.0+(rand.Float64()*maxHueMutation*2.0)-maxHueMutation, 360)
}

func mutateSaturation(s float64) float64 {
	s += rand.Float64()*maxSaturationMutation*2.0 - maxSaturationMutation
	return math.Min(math.Max(s, minSaturation), maxSaturation)
}

func mutateLuminance(l float64) float64 {
	l += rand.Float64()*maxLuminanceMutation*2.0 - maxLuminanceMutation
	return math.Min(math.Max(l, minLuminance), maxLuminance)
}

func getRandomColor() colorful.Color {
	h := rand.Float64() * 360.0
	s := minSaturation + (rand.Float64() * (maxSaturation - minSaturation))
	l := minLuminance + (rand.Float64() * (maxLuminance - minLuminance))
	return colorful.HSLuv(h, s, l)
}
