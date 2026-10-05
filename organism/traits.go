package organism

import (
	"math"

	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
)

// Traits contains organism-specific values that dictate how and when organisms perform certain activities.
type Traits struct {
	OrganismColor colorful.Color
	// SecondaryColor tints high-res feature overlays (pili / teeth / sensors).
	SecondaryColor colorful.Color
	MaxSize        float64
	SpawnHealth    float64
	// MinHealthToSpawn: the minimum health needed in order to spawn
	MinHealthToSpawn       float64
	MinCyclesBetweenSpawns int
	IdealPh                float64
	Abilities              physiology.Scores
}

// spawnHealthCap is the most health an organism may hand a child: a share of
// the health it has to reach before it can spawn, so the parent keeps the
// rest of that.
//
// Against the SPAWN THRESHOLD, not against max size. A share of max size says
// nothing about what the parent is holding when it actually spawns — an
// organism with max size 100, a threshold of 10 and a 0.1 share could still
// hand over all 10 and die doing it, which is the shape of the bug this
// setting exists to prevent.
func spawnHealthCap(minHealthToSpawn float64) float64 {
	return minHealthToSpawn * c.MaxSpawnHealthPercent()
}

// spawnThresholdFloor is the lowest spawn threshold that can still produce a
// viable child: under it the share leaves less than MinSpawnHealth to give.
//
// A share of 0 means no child could ever be viable, so the floor falls back
// to MinSpawnHealth and the cap below does the clamping.
func spawnThresholdFloor() float64 {
	percent := c.MaxSpawnHealthPercent()
	if percent <= 0 {
		return c.MinSpawnHealth()
	}
	return c.MinSpawnHealth() / percent
}

// spawnTraits decides the spawn threshold and then what a child gets, in
// that order, because the cap on the child is a share of the threshold.
//
// The pair is worked out in one place so the random and mutated paths cannot
// drift on which depends on which.
func spawnTraits(threshold, maxSize float64) (spawnThreshold, childCap float64) {
	floor := math.Min(spawnThresholdFloor(), maxSize)
	spawnThreshold = math.Max(floor, math.Min(threshold, maxSize))
	childCap = spawnHealthCap(spawnThreshold)
	return spawnThreshold, childCap
}

// childFloor is the least a child may be given. Normally MinSpawnHealth, but
// never more than the cap allows, so an organism too small to give that much
// gives what it can rather than more than it holds.
func childFloor(cap float64) float64 {
	return math.Min(c.MinSpawnHealth(), cap)
}

// initialSize is the size and health a FOUNDER starts at: a fraction of its
// max size, or its spawn health when the fraction is 0.
//
// Clamped to at least the spawn health, so the setting can only ever start a
// founder BIGGER than it used to be.
func initialSize(t Traits) float64 {
	fraction := c.InitialOrganismSizeFraction()
	if fraction <= 0 {
		return t.SpawnHealth
	}
	return math.Max(t.SpawnHealth, math.Min(fraction, 1)*t.MaxSize)
}

func newRandomTraits(rng *simrand.RNG) Traits {
	maxSize := rng.Float64() * c.MaximumInitialSize()
	// The threshold is drawn first: what a child gets is a share of it.
	floor := math.Min(spawnThresholdFloor(), maxSize)
	minHealthToSpawn := floor + rng.Float64()*(maxSize-floor)
	_, cap := spawnTraits(minHealthToSpawn, maxSize)
	spawnHealth := clampFloat(rng.Float64()*math.Min(cap, c.MaximumInitialSpawnHealth()),
		childFloor(cap), cap)
	minCyclesBetweenSpawns := rng.Intn(c.MaxInitialCyclesBetweenSpawns() + 1)
	idealPh := c.InitialPh()
	return Traits{
		OrganismColor:          newRandomColor(rng),
		SecondaryColor:         newRandomColor(rng),
		MaxSize:                maxSize,
		SpawnHealth:            spawnHealth,
		MinHealthToSpawn:       minHealthToSpawn,
		MinCyclesBetweenSpawns: minCyclesBetweenSpawns,
		IdealPh:                idealPh,
		// Initial organisms start from the configured distribution (near-pure chemosynthesizers by default) or a random split.
		Abilities: physiology.InitialScores(rng),
	}
}

func (t Traits) copyMutated(rng *simrand.RNG) Traits {
	maxSize := mutateFloat(rng, t.MaxSize, 5.0, c.MinimumMaxSize(), c.MaximumMaxSize())
	minCyclesBetweenSpawns := mutateInt(rng, t.MinCyclesBetweenSpawns, 5, 0, c.MaxCyclesBetweenSpawns())
	minHealthToSpawn := mutateFloat(rng, t.MinHealthToSpawn, 5.0,
		math.Min(spawnThresholdFloor(), maxSize), maxSize)
	_, cap := spawnTraits(minHealthToSpawn, maxSize)
	spawnHealth := mutateFloat(rng, t.SpawnHealth, 0.5, childFloor(cap), cap)
	idealPh := mutateFloat(rng, t.IdealPh, c.IdealPhMutationStep(), c.MinIdealPh(), c.MaxIdealPh())
	abilities := t.Abilities.Mutated(rng)
	// A visible shift in what the organism IS drives a larger colour step.
	physiologyChanged := abilities != t.Abilities
	color := mutateColor(rng, t.OrganismColor, physiologyChanged)
	secondary := mutateColor(rng, t.SecondaryColor, physiologyChanged)
	return Traits{
		OrganismColor:          color,
		SecondaryColor:         secondary,
		MaxSize:                maxSize,
		SpawnHealth:            spawnHealth,
		MinHealthToSpawn:       minHealthToSpawn,
		MinCyclesBetweenSpawns: minCyclesBetweenSpawns,
		IdealPh:                idealPh,
		Abilities:              abilities,
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

const (
	colorHueDriftDeg  = 5.0
	colorHueJumpMin   = 10.0
	colorHueJumpMax   = 20.0
	colorSatDrift     = 0.03
	colorLightDrift   = 0.03
	colorMinSat       = 0.4
	colorMaxSat       = 0.8
	colorMinLight     = 0.4
	colorMaxLight     = 0.7
	colorInitialSat   = 0.6
	colorInitialLight = 0.55
)

func newRandomColor(rng *simrand.RNG) colorful.Color {
	return colorful.HSLuv(rng.Float64()*360, colorInitialSat, colorInitialLight)
}

// mutateColor returns a perturbation of the parent colour in HSLuv space.
func mutateColor(rng *simrand.RNG, parent colorful.Color, featuresChanged bool) colorful.Color {
	h, s, l := parent.HSLuv()

	var hueDelta float64
	if featuresChanged {
		hueDelta = colorHueJumpMin + rng.Float64()*(colorHueJumpMax-colorHueJumpMin)
		if rng.Float64() < 0.5 {
			hueDelta = -hueDelta
		}
	} else {
		hueDelta = (rng.Float64()*2 - 1) * colorHueDriftDeg
	}
	h = math.Mod(h+hueDelta+360, 360)
	s = math.Min(colorMaxSat, math.Max(colorMinSat, s+(rng.Float64()*2-1)*colorSatDrift))
	l = math.Min(colorMaxLight, math.Max(colorMinLight, l+(rng.Float64()*2-1)*colorLightDrift))
	return colorful.HSLuv(h, s, l)
}
