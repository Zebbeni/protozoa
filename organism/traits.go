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

// spawnHealthCap is the most health an organism may hand a child: the
// configured share of its max size, reduced further if that would not leave
// MinSpawnHealth behind for the parent.
//
// Never above the configured share and never above maxSize. An organism
// whose whole capacity is under MinSpawnHealth cannot leave that reserve at
// all, and there the share alone applies — it still keeps a positive
// remainder, which is the most the arithmetic allows.
func spawnHealthCap(maxSize float64) float64 {
	cap := maxSize * c.MaxSpawnHealthPercent()
	if maxSize-cap < c.MinSpawnHealth() && maxSize > c.MinSpawnHealth() {
		cap = maxSize - c.MinSpawnHealth()
	}
	return cap
}

// spawnThresholdFloor is the lowest spawn threshold that leaves the parent
// alive: it hands over spawnHealth, so it has to hold at least
// MinSpawnHealth more than that to still exist afterwards.
func spawnThresholdFloor(spawnHealth, maxSize float64) float64 {
	return math.Min(spawnHealth+c.MinSpawnHealth(), maxSize)
}

func newRandomTraits(rng *simrand.RNG) Traits {
	maxSize := rng.Float64() * c.MaximumInitialSize()
	spawnHealth := rng.Float64() * math.Min(spawnHealthCap(maxSize), c.MaximumInitialSpawnHealth())
	floor := spawnThresholdFloor(spawnHealth, maxSize)
	minHealthToSpawn := floor + rng.Float64()*(maxSize-floor)
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
	spawnHealth := mutateFloat(rng, t.SpawnHealth, 0.5, c.MinSpawnHealth(), spawnHealthCap(maxSize))
	minHealthToSpawn := mutateFloat(rng, t.MinHealthToSpawn, 5.0, spawnThresholdFloor(spawnHealth, maxSize), maxSize)
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
