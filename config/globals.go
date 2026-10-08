package config

import (
	"encoding/json"
	"io"
	"math"
	"os"
	"reflect"
)

var defaultFilePath = "settings/default.json"
var constants *Globals

func SetGlobals(g *Globals) {
	g.Repair()
	constants = g
}

// Repair fills in what a settings file or replay header may be missing and
// normalises the signs, so a configuration decoded from JSON means the same
// thing whenever it was written.
//
// Exported because loading a settings file into the New Simulation screen
// has to apply it without installing the result as the active globals.
func (g *Globals) Repair() {
	g.NormalizeSigns()
	g.repairSpawnShare()
	g.repairWallBreak()
	g.repairDecisionNodes()
	g.repairMutationWeights()
	g.repairChemoPhWidth()
}

// settingSigns lists the settings that only make sense on one side of zero.
var settingSigns = map[string]int{
	"health_change_from_idle":                  -1,
	"health_change_from_turning":               -1,
	"health_change_from_moving":                -1,
	"health_change_from_moving_at_max":         -1,
	"health_change_from_digging_at_max":        -1,
	"health_change_from_turning_at_max":        -1,
	"health_change_from_eating_attempt":        -1,
	"health_change_from_eating_attempt_at_max": -1,
	"health_change_from_spawning":              -1,
	"health_change_from_attacking":             -1,
	"health_change_from_digging":               -1,
	"health_change_from_blocked_move":          -1,
	"health_change_inflicted_by_attack":        1,
	"health_change_inflicted_by_thorns":        1,
	"max_chemosynthesis_gain":                  1,
	"health_per_food_unit":                     1,
	"unhealthy_ph_damage":                      1,
	"chemo_ph_effect":                          1,
	"eating_ph_effect":                         1,
}

// SettingSign is 1 for a setting that must be positive, -1 for one that must be negative.
func SettingSign(jsonTag string) int { return settingSigns[jsonTag] }

// NormalizeSigns puts every sign-bound float on its own side of zero, keeping the magnitude.
func NormalizeSign(v float64, jsonTag string) float64 {
	if sign := SettingSign(jsonTag); sign != 0 {
		return float64(sign) * math.Abs(v)
	}
	return v
}

// DefaultWallBreakAtMax is the wall strength a full-Digging organism shoulders through per unit of size.
const DefaultWallBreakAtMax = 10

// repairWallBreak fills in the burrow endpoints for a settings file or replay header written before they existed.
func (g *Globals) repairWallBreak() {
	if g.WallBreakAtMaxDigging == 0 && g.WallBreakMultiplier != 0 {
		g.WallBreakAtMaxDigging = DefaultWallBreakAtMax
	}
}

// DefaultDisabledNodes are the node types held out of mutation unless a settings file says otherwise.
var DefaultBasicOnlyFamilies = []string{
	"IsHealthyPhHere",
	"IsFoodHere",
	"IsFoodBuriedHere",
}

// DefaultDisabledNodes now holds only what a family toggle cannot say.
var DefaultDisabledNodes = []string{
	"IsAgeMultipleOfTwo",
	"IsAgeMultipleOfTen",
}

// DefaultMaxChemosynthesisPhWidth is the band the simulation had before the setting existed: the curve multiplier alone, which tops out at 1. Multiplying by exactly 1.0 is bit-identical, so a repaired file runs the world it recorded.
// The share of its spawn threshold a parent may give a child is held inside
// these, because both ends are degenerate rather than extreme.
//
// At 1 the parent hands over everything it had to reach and dies spawning,
// which is the failure max_spawn_health_percent exists to prevent — and a
// share near 1 is the same failure scaled down, so the ceiling leaves a
// tenth rather than an epsilon. At 0 a child is born with no health at all.
//
// The floor also bounds how high spawnThresholdFloor can push the spawn
// threshold, since that is min_spawn_health divided by this.
const (
	MinSpawnHealthShare = 0.01
	MaxSpawnHealthShare = 0.9
)

const DefaultMaxChemosynthesisPhWidth = 1.0

// repairSpawnShare holds the spawn share inside its bounds. Unlike the other
// repairs this clamps a value that was SET rather than filling in one that
// was absent, because both ends of the range are degenerate: a file naming 1
// is asking for the organism that dies spawning.
func (g *Globals) repairSpawnShare() {
	if g.MaxSpawnHealthPercent > MaxSpawnHealthShare {
		g.MaxSpawnHealthPercent = MaxSpawnHealthShare
	}
	if g.MaxSpawnHealthPercent < MinSpawnHealthShare {
		g.MaxSpawnHealthPercent = MinSpawnHealthShare
	}
}

func (g *Globals) repairChemoPhWidth() {
	if g.MaxChemosynthesisPhWidth == 0 {
		g.MaxChemosynthesisPhWidth = DefaultMaxChemosynthesisPhWidth
	}
}

const DefaultMutationWeight = 1.0

// repairMutationWeights fills in the defaults for a settings file or replay header written before the weights existed, where all four decode to 0. The test is that the PAIR sums to zero, not that an individual weight does.
func (g *Globals) repairMutationWeights() {
	if g.MutationWeightSwapAction+g.MutationWeightGrowBranch == 0 {
		g.MutationWeightSwapAction = DefaultMutationWeight
		g.MutationWeightGrowBranch = DefaultMutationWeight
	}
	if g.MutationWeightSwapCondition+g.MutationWeightPruneBranch == 0 {
		g.MutationWeightSwapCondition = DefaultMutationWeight
		g.MutationWeightPruneBranch = DefaultMutationWeight
	}
}

func (g *Globals) repairDecisionNodes() {
	if g.DisabledDecisionNodes == nil {
		g.DisabledDecisionNodes = append([]string(nil), DefaultDisabledNodes...)
	}
}

// NormalizeSigns applies the sign convention to every field that has one.
func (g *Globals) NormalizeSigns() {
	val := reflect.ValueOf(g).Elem()
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Type.Kind() != reflect.Float64 {
			continue
		}
		tag := t.Field(i).Tag.Get("json")
		if SettingSign(tag) == 0 {
			continue
		}
		val.Field(i).SetFloat(NormalizeSign(val.Field(i).Float(), tag))
	}
}

func GetCurrentGlobals() *Globals {
	return constants
}

func Seed() int { return constants.Seed }

func GridUnitsWide() int { return constants.GridUnitsWide }
func GridUnitsHigh() int { return constants.GridUnitsHigh }
func ScreenWidth() int   { return constants.ScreenWidth }
func ScreenHeight() int  { return constants.ScreenHeight }

func InitialOrganisms() int        { return constants.InitialOrganisms }
func InitialFood() int             { return constants.InitialFood }
func InitialWalls() int            { return constants.InitialWalls }
func ChanceToAddFoodItem() float64 { return constants.ChanceToAddFoodItem }
func MinFoodValue() int            { return constants.MinFoodValue }
func BurialAmount() int            { return constants.BurialAmount }
func InitialBuriedFood() int       { return constants.InitialBuriedFood }
func MinInitialBuriedValue() int   { return constants.MinInitialBuriedValue }
func MaxInitialBuriedValue() int   { return constants.MaxInitialBuriedValue }
func BurialInterval() int          { return constants.BurialInterval }
func MaxFoodValue() int            { return constants.MaxFoodValue }

// MaxBuriedFoodValue is the per-cell ceiling on the buried layer, falling
// back to MaxFoodValue for a settings file written before it existed.
func MaxBuriedFoodValue() int {
	if constants.MaxBuriedFoodValue <= 0 {
		return constants.MaxFoodValue
	}
	return constants.MaxBuriedFoodValue
}

// The pH scale is fixed rather than configurable.
const (
	minPh = 0.0
	maxPh = 10.0
	// initialPh is the pH every cell starts at: the middle of the scale.
	initialPh = (minPh + maxPh) / 2
)

func MinPh() float64        { return minPh }
func MaxPh() float64        { return maxPh }
func InitialPh() float64    { return initialPh }
func IdealPhRange() float64 { return constants.IdealPhRange }

// MinIdealPh and MaxIdealPh are the ends of the ideal-pH range lineages can evolve within.
func MinIdealPh() float64          { return initialPh - constants.IdealPhRange/2 }
func MaxIdealPh() float64          { return initialPh + constants.IdealPhRange/2 }
func IdealPhMutationStep() float64 { return constants.IdealPhMutationStep }
func MaxPhToleranceWidth() float64 { return constants.MaxPhToleranceWidth }

func MaxChemosynthesisPhWidth() float64 { return constants.MaxChemosynthesisPhWidth }

func ChemoCrowdingPenalty() float64 { return constants.ChemoCrowdingPenalty }

func AttackHealthGain() float64 { return constants.AttackHealthGain }

func ChemoPhEffect() float64        { return constants.ChemoPhEffect }
func EatingPhEffect() float64       { return constants.EatingPhEffect }
func PhDiffuseFactor() float64      { return constants.PhDiffuseFactor }
func WallPhBlockAtMax() float64     { return constants.WallPhBlockAtMax }
func WallPhBlockCurve() float64     { return constants.WallPhBlockCurve }
func PhIncrementToDisplay() float64 { return constants.PhIncrementToDisplay }

func MinOrganisms() int               { return constants.MinOrganisms }
func MaxCycles() int                  { return constants.MaxCycles }
func MaxReplaySizeMb() int            { return constants.MaxReplaySizeMb }
func WallBreakMultiplier() float64    { return constants.WallBreakMultiplier }
func WallBreakAtZeroDigging() float64 { return constants.WallBreakAtZeroDigging }
func WallBreakAtMaxDigging() float64  { return constants.WallBreakAtMaxDigging }
func BurrowSpoilFraction() float64    { return constants.BurrowSpoilFraction }

func MuchBiggerSizeRatio() float64  { return constants.MuchBiggerSizeRatio }
func MuchSmallerSizeRatio() float64 { return constants.MuchSmallerSizeRatio }
func MuchFoodPerSize() float64      { return constants.MuchFoodPerSize }

// SmartTreeMutation reports whether mutation skips conditions whose answer is already settled where it would put them.
func SmartTreeMutation() bool { return constants.SmartTreeMutation }

func BasicOnlyConditionFamilies() []string { return constants.BasicOnlyConditionFamilies }

// TieredConditionMutation reports whether a new condition node is limited to the basic reads.
func TieredConditionMutation() bool { return constants.TieredConditionMutation }

func MutationWeightSwapAction() float64    { return constants.MutationWeightSwapAction }
func MutationWeightGrowBranch() float64    { return constants.MutationWeightGrowBranch }
func MutationWeightSwapCondition() float64 { return constants.MutationWeightSwapCondition }
func MutationWeightPruneBranch() float64   { return constants.MutationWeightPruneBranch }

// DisabledDecisionNodes is the set of node types mutation may not pick.
func DisabledDecisionNodes() []string { return constants.DisabledDecisionNodes }
func MaxOrganisms() int               { return constants.MaxOrganisms }
func GrowthFactor() float64           { return constants.GrowthFactor }
func EatingGrowthFactor() float64     { return constants.EatingGrowthFactor }
func MaximumMaxSize() float64         { return constants.MaximumMaxSize }
func MinimumMaxSize() float64         { return constants.MinimumMaxSize }
func MaximumInitialSize() float64     { return constants.MaximumInitialSize }

func InitialOrganismSizeFraction() float64 { return constants.InitialOrganismSizeFraction }
func MaximumInitialSpawnHealth() float64   { return constants.MaximumInitialSpawnHealth }
func MaxCyclesBetweenSpawns() int          { return constants.MaxCyclesBetweenSpawns }
func MaxInitialCyclesBetweenSpawns() int   { return constants.MaxInitialCyclesBetweenSpawns }
func MinSpawnHealth() float64              { return constants.MinSpawnHealth }
func MaxSpawnHealthPercent() float64       { return constants.MaxSpawnHealthPercent }
func MaxLifespan() int                     { return constants.MaxLifespan }

func InitialDecisionTreeMutations() int   { return constants.InitialDecisionTreeMutations }
func ChanceToMutateDecisionTree() float64 { return constants.ChanceToMutateDecisionTree }
func MaxDecisionTreeSize() int            { return constants.MaxDecisionTreeSize }

// --- Health Changes --- Per-action costs/gains paid by the actor; size-scaled at the apply site.
func MaxChemosynthesisGain() float64         { return constants.MaxChemosynthesisGain }
func HealthChangeFromIdle() float64          { return constants.HealthChangeFromIdle }
func HealthChangeFromTurning() float64       { return constants.HealthChangeFromTurning }
func HealthChangeFromMovingAtMax() float64   { return constants.HealthChangeFromMovingAtMax }
func HealthChangeFromTurningAtMax() float64  { return constants.HealthChangeFromTurningAtMax }
func HealthChangeFromDiggingAtMax() float64  { return constants.HealthChangeFromDiggingAtMax }
func HealthChangeFromMoving() float64        { return constants.HealthChangeFromMoving }
func HealthChangeFromEatingAttempt() float64 { return constants.HealthChangeFromEatingAttempt }
func HealthChangeFromEatingAttemptAtMax() float64 {
	return constants.HealthChangeFromEatingAttemptAtMax
}
func HealthChangeFromSpawning() float64  { return constants.HealthChangeFromSpawning }
func HealthChangeFromAttacking() float64 { return constants.HealthChangeFromAttacking }
func HealthChangeFromAttackingAtMax() float64 {
	return constants.HealthChangeFromAttackingAtMax
}
func HealthChangeFromDigging() float64     { return constants.HealthChangeFromDigging }
func HealthChangeFromBlockedMove() float64 { return constants.HealthChangeFromBlockedMove }

// Damage delivered to targets — size-scaled positive magnitudes, subtracted where they land.
func AttackDamageAtFullAttack() float64 { return constants.AttackDamageAtFullAttack }
func AttackDamageAtZeroAttack() float64 { return constants.AttackDamageAtZeroAttack }
func CorpseFoodMultiplier() float64     { return constants.CorpseFoodMultiplier }

func UnhealthyPhDamage() float64 { return constants.UnhealthyPhDamage }

// --- Ability scores --- Every organism distributes a fixed budget (physiology.PointTotal, 200) across seven ability scores, each capped at physiology.MaxAbilityScore (100).
func HealthPerFoodUnit() float64         { return constants.HealthPerFoodUnit }
func InitialAbilityScores() []int        { return constants.InitialAbilityScores }
func RandomInitialAbilities() bool       { return constants.RandomInitialAbilities }
func InitialDesigns() []string           { return constants.InitialDesigns }
func ChanceToMutateAbilities() float64   { return constants.ChanceToMutateAbilities }
func ThornsDamageAtFullDefense() float64 { return constants.ThornsDamageAtFullDefense }
func ThornsDamageAtZeroDefense() float64 { return constants.ThornsDamageAtZeroDefense }

// --- Appearance thresholds --- Sprite overlays are derived from ability scores, not inherited.
func ShellBodyThreshold() int     { return constants.ShellBodyThreshold }
func SpikesBodyThreshold() int    { return constants.SpikesBodyThreshold }
func PiliMotorThreshold() int     { return constants.PiliMotorThreshold }
func FlagellaMotorThreshold() int { return constants.FlagellaMotorThreshold }
func TeethMouthThreshold() int    { return constants.TeethMouthThreshold }
func FangsMouthThreshold() int    { return constants.FangsMouthThreshold }
func TusksMouthThreshold() int    { return constants.TusksMouthThreshold }
func SensorMinConditions() int    { return constants.SensorMinConditions }

func PopulationUpdateInterval() int { return constants.PopulationUpdateInterval }

func Theme() string { return constants.Theme }

const (
	PhColorSchemeGreenPink  = "green-pink"
	PhColorSchemeBlueOrange = "blue-orange"
)

func PhSmoothing() bool { return constants.PhSmoothing }

func PhColorScheme() string {
	switch constants.PhColorScheme {
	case PhColorSchemeBlueOrange:
		return PhColorSchemeBlueOrange
	default:
		return PhColorSchemeGreenPink
	}
}

// SetPhColorScheme swaps the active pH colour palette at runtime.
func SetPhColorScheme(name string) {
	switch name {
	case PhColorSchemeGreenPink, PhColorSchemeBlueOrange:
		constants.PhColorScheme = name
	default:
		constants.PhColorScheme = PhColorSchemeGreenPink
	}
}

func IsLightTheme() bool {
	return constants.Theme == "light"
}

// ThemeBackgroundRGB returns the window / panel fill colour for the active theme as RGB floats in [0, 1].
func ThemeBackgroundRGB() (r, g, b float64) {
	switch constants.Theme {
	case "light":
		return 240.0 / 255, 240.0 / 255, 240.0 / 255
	default: // "dark" or unknown
		return 0, 0, 0
	}
}

// PhTargetColorRGB returns the "extreme" colour that a pH cell blends towards as it moves away from neutral, in RGB floats [0, 1].
func PhTargetColorRGB(ph float64) (r, g, b float64) {
	neutral := (maxPh + minPh) / 2.0
	acid := ph < neutral
	switch PhColorScheme() {
	case PhColorSchemeBlueOrange:
		if acid {
			return 0x2C / 255.0, 0x7B / 255.0, 0xB6 / 255.0
		}
		return 0xE6 / 255.0, 0x61 / 255.0, 0x01 / 255.0
	default:
		if acid {
			return 0xA9 / 255.0, 0xC2 / 255.0, 0x18 / 255.0
		}
		return 0xE7 / 255.0, 0x47 / 255.0, 0x66 / 255.0
	}
}

// PhEffectHueRange returns the HSLuv hue endpoints for the active pH colour scheme.
func PhEffectHueRange() (acidHue, baseHue float64) {
	switch PhColorScheme() {
	case PhColorSchemeBlueOrange:
		// Blue (~hue 250°) → orange (~hue 40°).
		return 250.0, 40.0
	default:
		return 100.0, 0.0
	}
}

func SetTheme(theme string) {
	switch theme {
	case "dark", "light":
		constants.Theme = theme
	default:
		constants.Theme = "dark"
	}
}

type Globals struct {
	// --- Simulation --- Seed for the simulation RNG.
	Seed int `json:"seed"`

	GridUnitsWide int    `json:"grid_units_wide"`
	GridUnitsHigh int    `json:"grid_units_high"`
	ScreenWidth   int    `json:"screen_width"`
	ScreenHeight  int    `json:"screen_height"`
	Theme         string `json:"theme"`
	PhColorScheme string `json:"ph_color_scheme"`
	// Whether the pH layer is blended across cell boundaries when it is enlarged. Off draws every cell as one flat colour, which is what the simulation actually holds.
	PhSmoothing bool `json:"ph_smoothing"`

	InitialOrganisms    int     `json:"initial_organisms"`
	InitialFood         int     `json:"initial_food"`
	InitialWalls        int     `json:"initial_walls"`
	ChanceToAddFoodItem float64 `json:"chance_to_add_food_item"`
	MinFoodValue        int     `json:"min_food_value"`
	// BurialAmount and BurialInterval are the burial rate.
	BurialAmount   int `json:"burial_amount"`
	BurialInterval int `json:"burial_interval"`

	// InitialBuriedFood is how many cells start with food already in the buried layer.
	InitialBuriedFood     int `json:"initial_buried_food"`
	MinInitialBuriedValue int `json:"min_initial_buried_value"`
	MaxInitialBuriedValue int `json:"max_initial_buried_value"`
	MaxFoodValue          int `json:"max_food_value"`
	// The most that can sit buried under one cell. Separate from MaxFoodValue so the ground can hold far more than the surface, which is what lets a world bank potential energy underground. 0 falls back to MaxFoodValue.
	MaxBuriedFoodValue int `json:"max_buried_food_value"`

	// --- pH --- IdealPhRange is how wide a band of ideal pH values lineages can evolve across, centred on the middle of the pH scale.
	IdealPhRange float64 `json:"ideal_ph_range"`
	// IdealPhMutationStep is the furthest a child's ideal pH can land from its parent's, up or down.
	IdealPhMutationStep float64 `json:"ideal_ph_mutation_step"`
	// PhTolerance is the absolute pH distance every organism can sit from its IdealPh without taking unhealthy-pH damage.
	MaxPhToleranceWidth float64 `json:"max_ph_tolerance_width"`

	// The pH offset an organism at full Chemosynthesis can still gain health at; lower scores reach less of it along the chemosynthesis curve. 0 is not settable — max_chemosynthesis_gain 0 is what switches chemosynthesis off.
	MaxChemosynthesisPhWidth float64 `json:"max_chemosynthesis_ph_width"`

	// How much a chemosynthesising neighbour takes off the share of a chemosynthesis gain that comes from its cell. 0.5 halves that cell's contribution; 0 switches crowding off.
	ChemoCrowdingPenalty float64 `json:"chemo_crowding_penalty"`

	// The share of the health an attack actually removes that the attacker gains. 0 means a kill pays only in the corpse it leaves, which whoever eats it collects.
	AttackHealthGain float64 `json:"attack_health_gain"`

	// ChemoPhEffect and EatingPhEffect are how much pH an action moves **at full score**, scaled from there by CurveChemoPhEffect / CurveEatingPhEffect.
	ChemoPhEffect        float64 `json:"chemo_ph_effect"`
	EatingPhEffect       float64 `json:"eating_ph_effect"`
	PhDiffuseFactor      float64 `json:"ph_diffuse_factor"`
	PhIncrementToDisplay float64 `json:"ph_increment_to_display"`

	// A wall slows pH diffusion in proportion to its strength rather than stopping it.
	WallPhBlockAtMax float64 `json:"wall_ph_block_at_max"`
	WallPhBlockCurve float64 `json:"wall_ph_block_curve"`

	// --- Organisms --- MinOrganisms ends a run once the living population falls below it.
	MinOrganisms int `json:"min_organisms"`
	// MaxCycles ends a run once it reaches this cycle, so a simulation can be started and walked away from.
	MaxCycles int `json:"max_cycles"`
	// MaxReplaySizeMb ends a run once its replay file is projected to reach this many megabytes.
	MaxReplaySizeMb int     `json:"max_replay_size_mb"`
	MaxOrganisms    int     `json:"max_organisms"`
	GrowthFactor    float64 `json:"growth_factor"`
	// MaxFoodPerEatAtFullEating is the most food an organism with 100 Eating removes in one eating action, as a multiple of its size.
	MaxFoodPerEatAtFullEating float64 `json:"max_bite_at_full_eating"`
	// MaxFoodPerEatAtZeroEating is what one bite takes at 0 Eating.
	MaxFoodPerEatAtZeroEating float64 `json:"max_bite_at_zero_eating"`
	// HealthPerFoodUnit is the health one unit of food is worth to whoever eats it.
	HealthPerFoodUnit float64 `json:"health_per_food_unit"`
	// EatingGrowthFactor is the share of health gained from eating, beyond an organism's current size, that turns into growth.
	EatingGrowthFactor float64 `json:"eating_growth_factor"`
	MaximumMaxSize     float64 `json:"maximum_max_size"`
	MinimumMaxSize     float64 `json:"minimum_max_size"`
	MaximumInitialSize float64 `json:"maximum_initial_size"`
	// The size and health an INITIAL organism starts at, as a fraction of its own max size. 0 starts it at its spawn health, which is what founders did before this existed, and 1 starts it full grown and able to reproduce at once.
	InitialOrganismSizeFraction   float64 `json:"initial_organism_size_fraction"`
	MaximumInitialSpawnHealth     float64 `json:"maximum_initial_spawn_health"`
	MaxCyclesBetweenSpawns        int     `json:"max_cycles_between_spawns"`
	MaxInitialCyclesBetweenSpawns int     `json:"max_initial_cycles_between_spawns"`
	MinSpawnHealth                float64 `json:"min_spawn_health"`
	MaxSpawnHealthPercent         float64 `json:"max_spawn_health_percent"`
	// MaxLifespan is the global lifespan cap (in cycles) every organism dies at when reached.
	MaxLifespan int `json:"max_lifespan"`

	InitialDecisionTreeMutations int     `json:"initial_organism_decision_tree_mutations"`
	ChanceToMutateDecisionTree   float64 `json:"chance_to_mutate_decision_tree"`
	MaxDecisionTreeSize          int     `json:"max_decision_tree_size"`

	// --- Health Changes --- HealthChange* values follow a single naming convention.
	MaxChemosynthesisGain   float64 `json:"max_chemosynthesis_gain"`
	HealthChangeFromIdle    float64 `json:"health_change_from_idle"`
	HealthChangeFromTurning float64 `json:"health_change_from_turning"`
	// The far end of the cost curves.
	HealthChangeFromMovingAtMax  float64 `json:"health_change_from_moving_at_max"`
	HealthChangeFromTurningAtMax float64 `json:"health_change_from_turning_at_max"`
	HealthChangeFromDiggingAtMax float64 `json:"health_change_from_digging_at_max"`
	HealthChangeFromMoving       float64 `json:"health_change_from_moving"`
	// HealthChangeFromEatingAttempt is what one eating attempt costs at 0 Eating and HealthChangeFromEatingAttemptAtMax at full.
	HealthChangeFromEatingAttempt      float64 `json:"health_change_from_eating_attempt"`
	HealthChangeFromEatingAttemptAtMax float64 `json:"health_change_from_eating_attempt_at_max"`
	HealthChangeFromSpawning           float64 `json:"health_change_from_spawning"`
	HealthChangeFromAttacking          float64 `json:"health_change_from_attacking"`
	// The attack cost at a full Attack score, carried from HealthChangeFromAttacking along the attack cost curve. Equal ends make the curve inert, which is the flat cost attacking had before.
	HealthChangeFromAttackingAtMax float64 `json:"health_change_from_attacking_at_max"`
	HealthChangeFromDigging        float64 `json:"health_change_from_digging"`
	// HealthChangeFromBlockedMove is charged on top of the move cost, per unit of size, when an organism walks into something that was already there.
	HealthChangeFromBlockedMove float64 `json:"health_change_from_blocked_move"`
	// AttackDamageAtFullAttack is the damage an attacker with 100 Attack deals per unit of its size, before the target's Defense.
	AttackDamageAtFullAttack float64 `json:"health_change_inflicted_by_attack"`
	// AttackDamageAtZeroAttack is the damage an attack deals at 0 Attack.
	AttackDamageAtZeroAttack float64 `json:"attack_damage_at_zero"`
	// CorpseFoodMultiplier scales the food a dead organism leaves behind, as a multiple of its size.
	CorpseFoodMultiplier float64 `json:"corpse_food_multiplier"`
	UnhealthyPhDamage    float64 `json:"unhealthy_ph_damage"`

	// --- Ability scores --- InitialAbilityScores is the ability distribution the simulation's initial organisms start with, in physiology.AllAbilities order (chemosynthesis, eating, movement, digging, attack, defense, tolerance).
	InitialAbilityScores []int `json:"initial_ability_scores"`
	// RandomInitialAbilities gives each initial organism its own random split of the budget instead of InitialAbilityScores.
	RandomInitialAbilities bool `json:"random_initial_abilities"`
	// InitialDesigns names saved organism designs (from the designs directory) to start the simulation with, dealt round-robin across the initial organisms.
	InitialDesigns []string `json:"initial_designs"`
	// ChanceToMutateAbilities is the per-spawn probability that a child shifts points between two abilities.
	ChanceToMutateAbilities float64 `json:"chance_to_mutate_abilities"`
	// ThornsDamageAtFullDefense is the damage a defender with 100 Defense deals back to each organism that hits it, per unit of the defender's size, along the Thorns curve.
	ThornsDamageAtFullDefense float64 `json:"health_change_inflicted_by_thorns"`
	// ThornsDamageAtZeroDefense is what a defender with 0 Defense returns per hit taken.
	ThornsDamageAtZeroDefense float64 `json:"thorns_damage_at_zero"`

	// --- Appearance thresholds --- Score at which each overlay starts being drawn.
	ShellBodyThreshold     int `json:"shell_body_threshold"`
	SpikesBodyThreshold    int `json:"spikes_body_threshold"`
	PiliMotorThreshold     int `json:"pili_motor_threshold"`
	FlagellaMotorThreshold int `json:"flagella_motor_threshold"`
	TeethMouthThreshold    int `json:"teeth_mouth_threshold"`
	FangsMouthThreshold    int `json:"fangs_mouth_threshold"`
	TusksMouthThreshold    int `json:"tusks_mouth_threshold"`
	// SensorMinConditions is how many conditions of one sense category a decision tree must contain before the matching sensor overlay is drawn.
	SensorMinConditions int `json:"sensor_min_conditions"`

	ChemosynthesisCosineK      float64 `json:"chemosynthesis_cosine_k"`
	ChemosynthesisSaturatingK  float64 `json:"chemosynthesis_saturating_k"`
	EatingCosineK              float64 `json:"eating_cosine_k"`
	EatingSaturatingK          float64 `json:"eating_saturating_k"`
	MovementCostCosineK        float64 `json:"movement_cost_cosine_k"`
	MovementCostSaturatingK    float64 `json:"movement_cost_saturating_k"`
	DiggingCostCosineK         float64 `json:"digging_cost_cosine_k"`
	DiggingCostSaturatingK     float64 `json:"digging_cost_saturating_k"`
	DiggingStrengthCosineK     float64 `json:"digging_strength_cosine_k"`
	DiggingStrengthSaturatingK float64 `json:"digging_strength_saturating_k"`
	DiggingCreationCosineK     float64 `json:"digging_creation_cosine_k"`
	DiggingCreationSaturatingK float64 `json:"digging_creation_saturating_k"`
	AttackCosineK              float64 `json:"attack_cosine_k"`
	AttackSaturatingK          float64 `json:"attack_saturating_k"`
	DamageTakenCosineK         float64 `json:"damage_taken_cosine_k"`
	DamageTakenSaturatingK     float64 `json:"damage_taken_saturating_k"`
	ThornsCosineK              float64 `json:"thorns_cosine_k"`
	ThornsSaturatingK          float64 `json:"thorns_saturating_k"`
	PhToleranceCosineK         float64 `json:"ph_tolerance_cosine_k"`
	PhToleranceSaturatingK     float64 `json:"ph_tolerance_saturating_k"`
	ChemoPhEffectCosineK       float64 `json:"chemo_ph_effect_cosine_k"`
	ChemoPhEffectSaturatingK   float64 `json:"chemo_ph_effect_saturating_k"`
	EatingPhEffectCosineK      float64 `json:"eating_ph_effect_cosine_k"`
	EatingCostCosineK          float64 `json:"eating_cost_cosine_k"`
	AttackCostCosineK          float64 `json:"attack_cost_cosine_k"`
	EatingPhEffectSaturatingK  float64 `json:"eating_ph_effect_saturating_k"`
	EatingCostSaturatingK      float64 `json:"eating_cost_saturating_k"`
	AttackCostSaturatingK      float64 `json:"attack_cost_saturating_k"`

	ChemosynthesisCurveShape  string `json:"chemosynthesis_curve_shape"`
	EatingCurveShape          string `json:"eating_curve_shape"`
	MovementCostCurveShape    string `json:"movement_cost_curve_shape"`
	DiggingCostCurveShape     string `json:"digging_cost_curve_shape"`
	DiggingStrengthCurveShape string `json:"digging_strength_curve_shape"`
	DiggingCreationCurveShape string `json:"digging_creation_curve_shape"`
	AttackCurveShape          string `json:"attack_curve_shape"`
	DamageTakenCurveShape     string `json:"damage_taken_curve_shape"`
	ThornsCurveShape          string `json:"thorns_curve_shape"`
	PhToleranceCurveShape     string `json:"ph_tolerance_curve_shape"`
	ChemoPhEffectCurveShape   string `json:"chemo_ph_effect_curve_shape"`
	EatingPhEffectCurveShape  string `json:"eating_ph_effect_curve_shape"`
	EatingCostCurveShape      string `json:"eating_cost_curve_shape"`
	AttackCostCurveShape      string `json:"attack_cost_curve_shape"`

	// --- Physiology --- ChanceToGainFeature is the per-spawn probability that a child gains one new physiological feature (drawn uniformly at random from those whose prerequisites the parent already meets).
	WallCreatedSmall  int `json:"wall_created_small"`
	WallCreatedMedium int `json:"wall_created_medium"`
	WallCreatedLarge  int `json:"wall_created_large"`
	// WallCreatedAtZero is what a dig raises with no Digging at all.
	WallCreatedAtZero int `json:"wall_created_at_zero"`
	// WallBreakMultiplier scales an organism's size × Digging score into the wall strength it can shoulder straight through.
	WallBreakMultiplier float64 `json:"wall_break_multiplier"`

	// WallBreakAtZeroDigging and WallBreakAtMaxDigging are the wall strength an organism shoulders through per unit of its size, at 0 Digging and at the ability cap.
	WallBreakAtZeroDigging float64 `json:"wall_break_at_zero"`
	WallBreakAtMaxDigging  float64 `json:"wall_break_at_max"`

	// BurrowSpoilFraction is how much of a burrowed wall's strength survives as spoil, piled onto the two cells flanking the tunnel.
	BurrowSpoilFraction float64 `json:"burrow_spoil_fraction"`

	// DisabledDecisionNodes names the actions and conditions mutation is not allowed to put into a decision tree, by their decision.Names identifiers.
	MuchBiggerSizeRatio  float64 `json:"much_bigger_size_ratio"`
	MuchSmallerSizeRatio float64 `json:"much_smaller_size_ratio"`

	// MuchFoodPerSize is how many food units per unit of its own size a pile has to hold before an organism reads it as "much" food, for the IsMuchFood conditions.
	MuchFoodPerSize float64 `json:"much_food_per_size"`

	DisabledDecisionNodes []string `json:"disabled_decision_nodes"`

	// BasicOnlyConditionFamilies names the condition families that offer only their basic read.
	BasicOnlyConditionFamilies []string `json:"basic_only_condition_families"`

	// SmartTreeMutation keeps mutation from asking a question whose answer is already settled where it is asking it.
	SmartTreeMutation bool `json:"smart_tree_mutation"`

	// TieredConditionMutation restricts a NEW condition node to the basic reads.
	TieredConditionMutation bool `json:"tiered_condition_mutation"`

	// The relative likelihood of each KIND of tree mutation, once a node has been picked at random.
	MutationWeightSwapAction    float64 `json:"mutation_weight_swap_action"`
	MutationWeightGrowBranch    float64 `json:"mutation_weight_grow_branch"`
	MutationWeightSwapCondition float64 `json:"mutation_weight_swap_condition"`
	MutationWeightPruneBranch   float64 `json:"mutation_weight_prune_branch"`
	// Food a dig roots up in the cell ahead when it isn't a wall or occupied.
	FoodFromDiggingSmall  int `json:"food_from_digging_small"`
	FoodFromDiggingMedium int `json:"food_from_digging_medium"`
	FoodFromDiggingLarge  int `json:"food_from_digging_large"`
	// FoodFromDiggingAtZero is what a dig roots up with no Digging at all.
	FoodFromDiggingAtZero int `json:"food_from_digging_at_zero"`

	PopulationUpdateInterval int `json:"population_update_interval"`
}

func LoadFile(filePath string) io.Reader {
	file, err := os.Open(filePath)
	if err != nil {
		panic("failed to read config file")
	}
	return file
}

// GetDefaultGlobals returns the project-baseline Globals decoded from the embedded settings/default.json.
func GetDefaultGlobals() Globals {
	g := applyGlobalsFromJson(loadEmbeddedDefault(), Globals{})
	platformDefaults(g)
	return *g
}

func LoadGlobals(file io.Reader) *Globals {
	defaults := GetDefaultGlobals()
	g := applyGlobalsFromJson(file, defaults)
	return g
}

func applyGlobalsFromJson(file io.Reader, globals Globals) *Globals {
	g := globals
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&g)
	if err != nil {
		panic("failed to read globals from file")
	}
	return &g
}

func DumpGlobals(g *Globals, file io.Writer) {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		panic("failed to convert globals to json")
	}

	_, err = file.Write(data)
	if err != nil {
		panic("failed to write globals to file")
	}
}
