package organism

import (
	"math"

	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
)

// Traits contains organism-specific values that dictate how and when organisms
// perform certain activities, which are passed down from parents to children.
//
// Former traits now applied as globals (same value to every organism):
// ChanceToMutateDecisionTree, PhGrowthEffect (now action-driven via
// ChemoPhEffect / EatingPhEffect), PhTolerance, MaxLifespan.
type Traits struct {
	OrganismColor colorful.Color
	// SecondaryColor tints high-res feature overlays (pili /
	// teeth / sensors) while OrganismColor tints the body variant.
	// Inherited and mutated alongside the primary so the two-tone
	// look stays a family trait. At low-res (single-layer sprites)
	// SecondaryColor isn't used because nothing is drawn over the body.
	SecondaryColor colorful.Color
	MaxSize        float64
	SpawnHealth    float64
	// MinHealthToSpawn: the minimum health needed in order to spawn
	MinHealthToSpawn       float64
	MinCyclesBetweenSpawns int
	IdealPh                float64
	// Abilities is the organism's ability-score distribution: a fixed
	// budget (physiology.PointTotal, capped per ability) split across chemosynthesis,
	// eating, movement, digging, attack and defense. Every organism can
	// perform every action; these scores decide how effective it is and
	// what the action costs. Initial organisms start from the configured
	// distribution, and any specialisation a lineage evolves is paid for
	// out of its other abilities.
	Abilities physiology.Scores
}

func newRandomTraits(rng *simrand.RNG) Traits {
	maxSize := rng.Float64() * c.MaximumInitialSize()
	spawnHealth := rng.Float64() * math.Min(maxSize*c.MaxSpawnHealthPercent(), c.MaximumInitialSpawnHealth())
	minHealthToSpawn := spawnHealth + rng.Float64()*(maxSize-spawnHealth)
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
		// Initial organisms start from the configured distribution
		// (near-pure chemosynthesizers by default) or a random split;
		// every later capability has to be paid for out of that.
		Abilities: physiology.InitialScores(rng),
	}
}

func (t Traits) copyMutated(rng *simrand.RNG) Traits {
	maxSize := mutateFloat(rng, t.MaxSize, 5.0, c.MinimumMaxSize(), c.MaximumMaxSize())
	minCyclesBetweenSpawns := mutateInt(rng, t.MinCyclesBetweenSpawns, 5, 0, c.MaxCyclesBetweenSpawns())
	spawnHealth := mutateFloat(rng, t.SpawnHealth, 0.5, c.MinSpawnHealth(), maxSize*c.MaxSpawnHealthPercent())
	minHealthToSpawn := mutateFloat(rng, t.MinHealthToSpawn, 5.0, spawnHealth, maxSize)
	idealPh := mutateFloat(rng, t.IdealPh, c.IdealPhMutationStep(), c.MinIdealPh(), c.MaxIdealPh())
	abilities := t.Abilities.Mutated(rng)
	// A visible shift in what the organism IS drives a larger colour
	// step, so related lineages stay recognisable while a
	// re-specialising branch visibly diverges from its parent.
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

// Color mutation parameters. Tuned so that immediate parent-child
// pairs are visibly similar (clearly the same family at a glance) but
// drift across many generations is enough to make distantly-related
// branches distinguishable. Saturation and lightness drift are kept
// tight so colours stay in a vivid, panel-readable band rather than
// drifting toward grey or pure white/black.
//
// When the child's feature set differs from the parent's, the small
// hue drift is replaced by a forced jump in [colorHueJumpMin,
// colorHueJumpMax] with a randomly-chosen sign. This makes a
// physiology-changing spawn visually distinct from its parent — a
// branch point on the family tree reads at a glance instead of
// blending into normal generational drift.
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

// newRandomColor picks a fresh organism colour: random hue across the
// full wheel, fixed mid-band saturation and lightness so the result
// is vivid without being eye-bleeding. HSLuv is the working space —
// perceptually uniform, so equal hue steps look like equal hue steps
// across the wheel.
func newRandomColor(rng *simrand.RNG) colorful.Color {
	return colorful.HSLuv(rng.Float64()*360, colorInitialSat, colorInitialLight)
}

// mutateColor returns a perturbation of the parent colour in HSLuv
// space. Hue wraps; saturation and lightness clamp to the readable
// band defined by colorMin*/colorMax* so a long lineage can't drift
// into grey or near-white.
//
// featuresChanged toggles the hue behaviour: false → small ±drift
// per generation, true → forced jump in [colorHueJumpMin,
// colorHueJumpMax] (random sign) so a child whose physiology differs
// from its parent looks visibly distinct. Saturation and lightness
// drift stay small in either case to keep family identity recognisable.
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
