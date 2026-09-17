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

// SetGlobals initializes all globally-referenced constants. It
// normalises the sign-bound settings first, so a file written under an
// older convention still means what it meant.
func SetGlobals(g *Globals) {
	g.NormalizeSigns()
	constants = g
}

// settingSigns lists the settings that only make sense on one side of
// zero: the cost of an action is always a loss, damage is always damage.
// A slider that can cross zero there offers a setting that turns a cost
// into free health or an attack into a heal, which is never what the
// user is reaching for.
//
// Damage is stored as a positive magnitude and subtracted where it
// lands, so the settings read the way they're spoken about ("650 damage"
// rather than "-650 health"). NormalizeSigns is what keeps files written
// under the old negative convention working.
var settingSigns = map[string]int{
	"health_change_from_idle":           -1,
	"health_change_from_turning":        -1,
	"health_change_from_moving":         -1,
	"health_change_from_moving_at_max":  -1,
	"health_change_from_digging_at_max": -1,
	"health_change_from_turning_at_max": -1,
	"health_change_from_eating_attempt": -1,
	"health_change_from_spawning":       -1,
	"health_change_from_attacking":      -1,
	"health_change_from_digging":        -1,
	"health_change_from_blocked_move":   -1,
	"health_change_inflicted_by_attack": 1,
	"health_change_inflicted_by_thorns": 1,
	"max_chemosynthesis_gain":           1,
	"health_per_food_unit":              1,
	"unhealthy_ph_damage":               1,
	"chemo_ph_effect":                   1,
	"eating_ph_effect":                  1,
}

// SettingSign is 1 for a setting that must be positive, -1 for one that
// must be negative, and 0 for one that is genuinely signed.
func SettingSign(jsonTag string) int { return settingSigns[jsonTag] }

// NormalizeSigns puts every sign-bound float on its own side of zero,
// keeping the magnitude. Called on every path that installs settings, so
// a value typed, loaded or replayed from a file under the old convention
// (attack damage was stored as -650) lands as the same amount of damage
// rather than as healing.
func NormalizeSign(v float64, jsonTag string) float64 {
	if sign := SettingSign(jsonTag); sign != 0 {
		return float64(sign) * math.Abs(v)
	}
	return v
}

// NormalizeSigns applies the sign convention to every field that has
// one.
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

// GetCurrentGlobals returns a pointer to the current globals for modification.
func GetCurrentGlobals() *Globals {
	return constants
}

// Accessors are grouped by config-screen section so adding a new
// knob slots into the right neighbourhood for readability. Section
// order matches default.json's grouping and the editor's section
// list (see ux/config_screen.go).

// --- Simulation ---
func Seed() int { return constants.Seed }

// --- Display ---
func GridUnitsWide() int { return constants.GridUnitsWide }
func GridUnitsHigh() int { return constants.GridUnitsHigh }
func ScreenWidth() int   { return constants.ScreenWidth }
func ScreenHeight() int  { return constants.ScreenHeight }

// --- Environment ---
func InitialOrganisms() int        { return constants.InitialOrganisms }
func InitialFood() int             { return constants.InitialFood }
func InitialWalls() int            { return constants.InitialWalls }
func ChanceToAddFoodItem() float64 { return constants.ChanceToAddFoodItem }
func MinFoodValue() int            { return constants.MinFoodValue }
func MaxFoodValue() int            { return constants.MaxFoodValue }

// --- pH ---

// The pH scale is fixed rather than configurable. Colour schemes, the pH
// graph's buckets and every tolerance are all tuned against 0-10, so a
// different range would quietly break them rather than rescale them.
const (
	minPh = 0.0
	maxPh = 10.0
	// initialPh is the pH every cell starts at: the middle of the scale.
	// Not configurable — a world that starts uniform and drifts from
	// whatever its organisms do is the interesting case, and a starting
	// offset only shifts which way it drifts first.
	initialPh = (minPh + maxPh) / 2
)

func MinPh() float64        { return minPh }
func MaxPh() float64        { return maxPh }
func InitialPh() float64    { return initialPh }
func IdealPhRange() float64 { return constants.IdealPhRange }

// MinIdealPh and MaxIdealPh are the ends of the ideal-pH range lineages
// can evolve within: IdealPhRange wide, centred on the middle of the pH
// scale. One setting rather than two ends, since an off-centre range
// would just bias every lineage the same way.
func MinIdealPh() float64          { return initialPh - constants.IdealPhRange/2 }
func MaxIdealPh() float64          { return initialPh + constants.IdealPhRange/2 }
func IdealPhMutationStep() float64 { return constants.IdealPhMutationStep }
func MaxPhToleranceWidth() float64 { return constants.MaxPhToleranceWidth }

func ChemoPhEffect() float64        { return constants.ChemoPhEffect }
func EatingPhEffect() float64       { return constants.EatingPhEffect }
func PhDiffuseFactor() float64      { return constants.PhDiffuseFactor }
func PhIncrementToDisplay() float64 { return constants.PhIncrementToDisplay }

// --- Organisms ---
func MinOrganisms() int                  { return constants.MinOrganisms }
func MaxOrganisms() int                  { return constants.MaxOrganisms }
func GrowthFactor() float64              { return constants.GrowthFactor }
func EatingGrowthFactor() float64        { return constants.EatingGrowthFactor }
func MaximumMaxSize() float64            { return constants.MaximumMaxSize }
func MinimumMaxSize() float64            { return constants.MinimumMaxSize }
func MaximumInitialSize() float64        { return constants.MaximumInitialSize }
func MaximumInitialSpawnHealth() float64 { return constants.MaximumInitialSpawnHealth }
func MaxCyclesBetweenSpawns() int        { return constants.MaxCyclesBetweenSpawns }
func MaxInitialCyclesBetweenSpawns() int { return constants.MaxInitialCyclesBetweenSpawns }
func MinSpawnHealth() float64            { return constants.MinSpawnHealth }
func MaxSpawnHealthPercent() float64     { return constants.MaxSpawnHealthPercent }
func MaxLifespan() int                   { return constants.MaxLifespan }

// --- Decision Trees ---
func InitialDecisionTreeMutations() int   { return constants.InitialDecisionTreeMutations }
func ChanceToMutateDecisionTree() float64 { return constants.ChanceToMutateDecisionTree }
func MaxDecisionTreeSize() int            { return constants.MaxDecisionTreeSize }

// --- Health Changes ---
// Per-action costs/gains paid by the actor; size-scaled at the apply
// site. Signed: negative = cost, positive = gain (only chemo today).
func MaxChemosynthesisGain() float64         { return constants.MaxChemosynthesisGain }
func HealthChangeFromIdle() float64          { return constants.HealthChangeFromIdle }
func HealthChangeFromTurning() float64       { return constants.HealthChangeFromTurning }
func HealthChangeFromMovingAtMax() float64   { return constants.HealthChangeFromMovingAtMax }
func HealthChangeFromTurningAtMax() float64  { return constants.HealthChangeFromTurningAtMax }
func HealthChangeFromDiggingAtMax() float64  { return constants.HealthChangeFromDiggingAtMax }
func HealthChangeFromMoving() float64        { return constants.HealthChangeFromMoving }
func HealthChangeFromEatingAttempt() float64 { return constants.HealthChangeFromEatingAttempt }
func HealthChangeFromSpawning() float64      { return constants.HealthChangeFromSpawning }
func HealthChangeFromAttacking() float64     { return constants.HealthChangeFromAttacking }
func HealthChangeFromDigging() float64       { return constants.HealthChangeFromDigging }
func HealthChangeFromBlockedMove() float64   { return constants.HealthChangeFromBlockedMove }

// Damage delivered to targets — size-scaled positive magnitudes,
// subtracted where they land.
func AttackDamageAtFullAttack() float64 { return constants.AttackDamageAtFullAttack }
func CorpseFoodMultiplier() float64     { return constants.CorpseFoodMultiplier }

// Environmental health changes.
func UnhealthyPhDamage() float64 { return constants.UnhealthyPhDamage }

// --- Ability scores ---
// Every organism distributes a fixed budget (physiology.PointTotal, 200)
// across seven ability scores, each capped at physiology.MaxAbilityScore
// (100), which drive multiplier curves between 0 and 1 across scores 0 to
// 100 (see physiology.CurveFor and the per-shape K settings).
func HealthPerFoodUnit() float64         { return constants.HealthPerFoodUnit }
func InitialAbilityScores() []int        { return constants.InitialAbilityScores }
func RandomInitialAbilities() bool       { return constants.RandomInitialAbilities }
func ChanceToMutateAbilities() float64   { return constants.ChanceToMutateAbilities }
func ThornsDamageAtFullDefense() float64 { return constants.ThornsDamageAtFullDefense }

// --- Appearance thresholds ---
// Sprite overlays are derived from ability scores, not inherited, so
// these decide at what point an organism starts *looking* like what it
// has specialised in. A threshold at or below the initial scores
// gives every founder the overlay from the start.
func ShellBodyThreshold() int     { return constants.ShellBodyThreshold }
func SpikesBodyThreshold() int    { return constants.SpikesBodyThreshold }
func PiliMotorThreshold() int     { return constants.PiliMotorThreshold }
func FlagellaMotorThreshold() int { return constants.FlagellaMotorThreshold }
func TeethMouthThreshold() int    { return constants.TeethMouthThreshold }
func FangsMouthThreshold() int    { return constants.FangsMouthThreshold }
func TusksMouthThreshold() int    { return constants.TusksMouthThreshold }
func SensorMinConditions() int    { return constants.SensorMinConditions }

// --- Physiology ---
func WallStrengthDeltaSmall() int  { return constants.WallStrengthDeltaSmall }
func WallStrengthDeltaMedium() int { return constants.WallStrengthDeltaMedium }
func WallStrengthDeltaLarge() int  { return constants.WallStrengthDeltaLarge }

// Sensors tree

// Defense tree

// Teeth tree

// --- Statistics ---
func PopulationUpdateInterval() int { return constants.PopulationUpdateInterval }

// Theme returns the active GUI theme name. Recognised values: "dark",
// "light". Anything else falls back to dark behaviour at render time.
func Theme() string { return constants.Theme }

// PH colour scheme names. Stored in Globals.PhColorScheme so the
// preference survives across config-screen launches and snapshot
// reloads. The default ("green-pink") matches the original palette;
// "blue-orange" is the colour-blind-safer alternative.
const (
	PhColorSchemeGreenPink  = "green-pink"
	PhColorSchemeBlueOrange = "blue-orange"
)

// PhColorScheme returns the active pH colour palette. Anything other
// than the recognised values is treated as the green-pink default.
func PhColorScheme() string {
	switch constants.PhColorScheme {
	case PhColorSchemeBlueOrange:
		return PhColorSchemeBlueOrange
	default:
		return PhColorSchemeGreenPink
	}
}

// SetPhColorScheme swaps the active pH colour palette at runtime.
// Callers are expected to invalidate any cached pH-coloured artefacts
// (env layer, pH graphs, pre-coloured descendant tree nodes) so the
// new palette is picked up on the next render.
func SetPhColorScheme(name string) {
	switch name {
	case PhColorSchemeGreenPink, PhColorSchemeBlueOrange:
		constants.PhColorScheme = name
	default:
		constants.PhColorScheme = PhColorSchemeGreenPink
	}
}

// IsLightTheme reports whether the theme has a light-valued background.
// Drives whichever code paths need to flip lightness curves (e.g. pH
// colour mapping) so content stays readable against the window fill.
func IsLightTheme() bool {
	return constants.Theme == "light"
}

// ThemeBackgroundRGB returns the window / panel fill colour for the
// active theme as RGB floats in [0, 1]. Kept in the config package so
// deep modules (ux, graph helpers, organism) can blend pH colours
// towards it without having to import ux.
func ThemeBackgroundRGB() (r, g, b float64) {
	switch constants.Theme {
	case "light":
		return 240.0 / 255, 240.0 / 255, 240.0 / 255
	default: // "dark" or unknown
		return 0, 0, 0
	}
}

// PhTargetColorRGB returns the "extreme" colour that a pH cell blends
// towards as it moves away from neutral, in RGB floats [0, 1]. The
// extremes depend on the active pH colour scheme:
//   - green-pink:  acid #A9C218 (yellow-green) / base #E74766 (red-pink)
//   - blue-orange: acid #2C7BB6 (blue)         / base #E66101 (orange)
//
// At exactly neutral the caller should use a weight of 0 so the target
// colour has no effect.
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

// PhEffectHueRange returns the HSLuv hue endpoints for the active pH
// colour scheme. spec=0 (acid) maps to the first value, spec=1 (base)
// to the second. ComputePhEffectColor interpolates between them.
func PhEffectHueRange() (acidHue, baseHue float64) {
	switch PhColorScheme() {
	case PhColorSchemeBlueOrange:
		// Blue (~hue 250°) → orange (~hue 40°). The spectrum
		// blends through purple/pink (going forward through 360°)
		// rather than through green, but at neutral the colour
		// blends to background so the intermediate hue isn't shown.
		return 250.0, 40.0
	default:
		return 100.0, 0.0
	}
}

// SetTheme swaps the active theme at runtime. The UI chrome (background
// fill, text colour, pH blending) recomputes on the next render without
// needing to touch sprite resources — sprites themselves are theme-
// agnostic now and designed to read against any background.
func SetTheme(theme string) {
	switch theme {
	case "dark", "light":
		constants.Theme = theme
	default:
		constants.Theme = "dark"
	}
}

type Globals struct {
	// Seed for the simulation RNG. 0 means "use the CLI --seed flag
	// or the time-based default chosen by the runner". Editable via
	// the config screen so wasm builds (no CLI) can pick a seed.
	// Fields grouped by config-screen section. New knobs should be
	// added under the matching section so default.json stays
	// readable and the editor and source stay aligned.

	// --- Simulation ---
	// Seed for the simulation RNG. 0 means "use the CLI --seed flag
	// or the time-based default chosen by the runner". Editable via
	// the config screen so wasm builds (no CLI) can pick a seed.
	Seed int `json:"seed"`

	// --- Display ---
	GridUnitsWide int `json:"grid_units_wide"`
	GridUnitsHigh int `json:"grid_units_high"`
	ScreenWidth   int `json:"screen_width"`
	ScreenHeight  int `json:"screen_height"`
	// GUI theme: "light" or "dark". Controls the window background
	// and selects between <theme>-prefixed sprite sheets.
	Theme string `json:"theme"`
	// PhColorScheme: "green-pink" (default) or "blue-orange". Drives
	// the pH grid layer, organism PH-effect tints, panel pH stat
	// colours, and the pH/PhEffect graphs. Blue-orange is friendlier
	// to red-green colour blindness.
	PhColorScheme string `json:"ph_color_scheme"`

	// --- Environment ---
	InitialOrganisms    int     `json:"initial_organisms"`
	InitialFood         int     `json:"initial_food"`
	InitialWalls        int     `json:"initial_walls"`
	ChanceToAddFoodItem float64 `json:"chance_to_add_food_item"`
	MinFoodValue        int     `json:"min_food_value"`
	MaxFoodValue        int     `json:"max_food_value"`

	// --- pH ---
	// IdealPhRange is how wide a band of ideal pH values lineages can
	// evolve across, centred on the middle of the pH scale: 9 gives
	// 0.5-9.5. Every cell starts at that middle (config.InitialPh).
	IdealPhRange float64 `json:"ideal_ph_range"`
	// IdealPhMutationStep is the furthest a child's ideal pH can land
	// from its parent's, up or down. It sets how fast a lineage can
	// follow a drifting world: 0 pins every lineage to the pH it started
	// at, so a world that drifts far enough kills them.
	IdealPhMutationStep float64 `json:"ideal_ph_mutation_step"`
	// PhTolerance is the absolute pH distance every organism can sit
	// from its IdealPh without taking unhealthy-pH damage. Global
	// rather than per-organism — variation between organisms now
	// comes from IdealPh alone.
	// MaxPhToleranceWidth is T at 100 Tolerance: the pH offset an
	// organism can bear for one unit of damage. Lower scores bear less,
	// along the pH tolerance curve.
	MaxPhToleranceWidth float64 `json:"max_ph_tolerance_width"`
	// ChemoPhEffect and EatingPhEffect are how much pH an action moves
	// per unit of health it gained: chemosynthesis pushes the local pH
	// down, eating pushes it up. Tied to the health gained rather than to
	// size or food value, so an action that barely paid off barely moves
	// the water — the feedback that stops a population from driving its
	// own pH away without limit. Actions that gain nothing move nothing.
	ChemoPhEffect        float64 `json:"chemo_ph_effect"`
	EatingPhEffect       float64 `json:"eating_ph_effect"`
	PhDiffuseFactor      float64 `json:"ph_diffuse_factor"`
	PhIncrementToDisplay float64 `json:"ph_increment_to_display"`

	// --- Organisms ---
	// MinOrganisms ends a run once the living population falls below it,
	// so a sim doesn't idle for thousands of cycles waiting for the last
	// few stragglers to die. Only armed after the population has reached
	// twice this number, so the small founding population of a run that
	// is just getting started never trips it. 0 disables it, leaving
	// extinction as the only end condition. (This key once topped the
	// population back up with random organisms; that behaviour was
	// removed and the key sat unused until it took on this meaning.)
	MinOrganisms int     `json:"min_organisms"`
	MaxOrganisms int     `json:"max_organisms"`
	GrowthFactor float64 `json:"growth_factor"`
	// MaxFoodPerEatAtFullEating is the most food an organism with 100
	// Eating removes in one eating action, as a multiple of its size.
	// Lower Eating scores remove a fraction of this along the Eating
	// curve. The json tag still says "bite": renaming it would drop the
	// setting from every settings file and replay header already written.
	MaxFoodPerEatAtFullEating float64 `json:"max_bite_at_full_eating"`
	// HealthPerFoodUnit is the health one unit of food is worth to
	// whoever eats it. Flat — how much an organism can swallow is what
	// its Eating score buys, and what the food is worth is a property of
	// the food, so this doesn't ride the Eating curve. It used to be
	// implicitly 1, which made "max food per eat" read as a health
	// number and hid how far above the food supply the cap sat.
	HealthPerFoodUnit float64 `json:"health_per_food_unit"`
	// EatingGrowthFactor is the share of health gained from eating, beyond
	// an organism's current size, that turns into growth. GrowthFactor
	// plays the same role for every other gain. At 1 an eater keeps all of
	// a big meal (up to its max size) instead of losing the overflow.
	EatingGrowthFactor            float64 `json:"eating_growth_factor"`
	MaximumMaxSize                float64 `json:"maximum_max_size"`
	MinimumMaxSize                float64 `json:"minimum_max_size"`
	MaximumInitialSize            float64 `json:"maximum_initial_size"`
	MaximumInitialSpawnHealth     float64 `json:"maximum_initial_spawn_health"`
	MaxCyclesBetweenSpawns        int     `json:"max_cycles_between_spawns"`
	MaxInitialCyclesBetweenSpawns int     `json:"max_initial_cycles_between_spawns"`
	MinSpawnHealth                float64 `json:"min_spawn_health"`
	MaxSpawnHealthPercent         float64 `json:"max_spawn_health_percent"`
	// MaxLifespan is the global lifespan cap (in cycles) every
	// organism dies at when reached. Set to 0 to disable lifespan-
	// based death entirely.
	MaxLifespan int `json:"max_lifespan"`

	// --- Decision Trees ---
	InitialDecisionTreeMutations int     `json:"initial_organism_decision_tree_mutations"`
	ChanceToMutateDecisionTree   float64 `json:"chance_to_mutate_decision_tree"`
	MaxDecisionTreeSize          int     `json:"max_decision_tree_size"`

	// --- Health Changes ---
	// HealthChange* values follow a single naming convention:
	//   - HealthChangeFrom<Action> — size-scaled health delta the
	//     actor pays (negative cost) or gains (positive). One per
	//     action.
	//   - <Thing>Damage — size-scaled damage delivered to a target,
	//     stored as a positive magnitude and subtracted where it lands.
	//   - HealthChangePer<Source> — environmental health delta.
	// The costs are held negative and the damages positive by
	// settingSigns; their json tags predate the split.
	MaxChemosynthesisGain   float64 `json:"max_chemosynthesis_gain"`
	HealthChangeFromIdle    float64 `json:"health_change_from_idle"`
	HealthChangeFromTurning float64 `json:"health_change_from_turning"`
	// The far end of the cost curves: what each action costs at full
	// ability, where the curve alone would have taken it to nothing. No
	// action is ever free — a body still has to move itself, and still
	// has to shift the terrain — so the ability buys a discount, not
	// exemption. The curve carries each cost from the value at ability 0
	// down to the value here.
	HealthChangeFromMovingAtMax   float64 `json:"health_change_from_moving_at_max"`
	HealthChangeFromTurningAtMax  float64 `json:"health_change_from_turning_at_max"`
	HealthChangeFromDiggingAtMax  float64 `json:"health_change_from_digging_at_max"`
	HealthChangeFromMoving        float64 `json:"health_change_from_moving"`
	HealthChangeFromEatingAttempt float64 `json:"health_change_from_eating_attempt"`
	HealthChangeFromSpawning      float64 `json:"health_change_from_spawning"`
	HealthChangeFromAttacking     float64 `json:"health_change_from_attacking"`
	HealthChangeFromDigging       float64 `json:"health_change_from_digging"`
	// HealthChangeFromBlockedMove is charged on top of the move cost,
	// per unit of size, when an organism walks into something that was
	// already there — a wall, food, or another organism — so misreading
	// the world ahead costs more than reading it right. Size-scaled like
	// every other health change, so it keeps biting as an organism grows,
	// but no ability scales it: no score makes a wall passable. It is NOT
	// charged when the cell was empty at decision time and someone else
	// reached it first; losing a race is luck, not a misread.
	HealthChangeFromBlockedMove float64 `json:"health_change_from_blocked_move"`
	// AttackDamageAtFullAttack is the damage an attacker with 100 Attack
	// deals per unit of its size, before the target's Defense. A positive
	// magnitude: applyAttack subtracts it.
	AttackDamageAtFullAttack float64 `json:"health_change_inflicted_by_attack"`
	// CorpseFoodMultiplier scales the food a dead organism leaves behind,
	// as a multiple of its size.
	CorpseFoodMultiplier float64 `json:"corpse_food_multiplier"`
	UnhealthyPhDamage    float64 `json:"unhealthy_ph_damage"`

	// --- Ability scores ---
	// InitialAbilityScores is the ability distribution the simulation's
	// initial organisms start with, in physiology.AllAbilities order
	// (chemosynthesis, eating, movement, digging, attack, defense,
	// tolerance). It must sum to the 200-point budget with no entry over
	// 100; the config screen refuses to start otherwise, and a bad value
	// loaded from a file falls back to the balanced distribution.
	InitialAbilityScores []int `json:"initial_ability_scores"`
	// RandomInitialAbilities gives each initial organism its own random
	// split of the budget instead of InitialAbilityScores.
	RandomInitialAbilities bool `json:"random_initial_abilities"`
	// ChanceToMutateAbilities is the per-spawn probability that a child
	// shifts points between two abilities. Higher than the old feature
	// gain rate because a transfer is a small nudge rather than a whole
	// new capability.
	ChanceToMutateAbilities float64 `json:"chance_to_mutate_abilities"`
	// ThornsDamageAtFullDefense is the damage a defender with 100
	// Defense deals back to each organism that hits it, per unit of the
	// defender's size, along the Thorns curve. Independent of the attack:
	// the counter-attack is the defender's, so a soft hit is answered as
	// hard as a heavy one.
	ThornsDamageAtFullDefense float64 `json:"health_change_inflicted_by_thorns"`

	// --- Appearance thresholds ---
	// Score at which each overlay starts being drawn.
	ShellBodyThreshold     int `json:"shell_body_threshold"`
	SpikesBodyThreshold    int `json:"spikes_body_threshold"`
	PiliMotorThreshold     int `json:"pili_motor_threshold"`
	FlagellaMotorThreshold int `json:"flagella_motor_threshold"`
	TeethMouthThreshold    int `json:"teeth_mouth_threshold"`
	FangsMouthThreshold    int `json:"fangs_mouth_threshold"`
	TusksMouthThreshold    int `json:"tusks_mouth_threshold"`
	// SensorMinConditions is how many conditions of one sense category
	// a decision tree must contain before the matching sensor overlay
	// is drawn. Unlike the others this counts tree nodes, not score:
	// sensing is a behaviour, so the sprite follows what the organism
	// actually checks for.
	SensorMinConditions int `json:"sensor_min_conditions"`

	// Ability curves. Each ability multiplier is a dampened cosine curve
	// between 0 and 1 across scores 0 to 100 — rising for effects, falling
	// for costs — scaling the health change or damage setting that defines
	// its full value. These K values set how strongly each curve suppresses
	// early values, from 0 (a pure cosine S-curve) to 1 (the most). See
	// physiology.Curve.
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

	// Curve shapes. Each names how its curve climbs from 0 to 1 —
	// "linear", "quadratic", "cosine" or "saturating" — so an ability can
	// be made an early buy or a late commitment without touching the
	// values it scales. Anything else falls back to the curve's default.
	// See physiology.ShapeKind.
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

	// --- Physiology ---
	// ChanceToGainFeature is the per-spawn probability that a child
	// gains one new physiological feature (drawn uniformly at random
	// from those whose prerequisites the parent already meets).
	// ChanceToLoseFeature is the per-spawn probability that a child
	// loses the deepest feature from one of its non-empty modality
	// trees (drawn uniformly at random across non-empty trees). Lets
	// lineages back out of a branch so descendants can re-grow down
	// a sibling — without this the population saturates at the same
	// leaf set in every tree and physiological diversity vanishes.
	// Posture-state modifiers applied during the cycle an organism
	// is in the matching posture. Multipliers default to 1.0 = no
	// effect; the additive modifier is a signed delta.
	// WallStrengthDeltaSmall/Medium/Large set how much wall strength
	// a single ActDig adds or removes, bucketed by the
	// organism's size class (thirds of MaximumMaxSize).
	// Wall strength one dig moves, and food one dig roots up, by the
	// digger's size class. Both are whole units — wall strength and food
	// values are ints — so each is the value at 100 Digging, and the
	// *AtZero setting beside it is the value at 0. The Digging strength
	// curve interpolates between the two and the result is rounded, which
	// is what makes these step from one whole unit to the next rather
	// than fading to a fraction nothing can hold.
	WallStrengthDeltaSmall  int `json:"wall_strength_delta_small"`
	WallStrengthDeltaMedium int `json:"wall_strength_delta_medium"`
	WallStrengthDeltaLarge  int `json:"wall_strength_delta_large"`
	// WallStrengthDeltaAtZero is what a dig clears with no Digging at all.
	// 1 keeps a scratch from doing literally nothing, which would read as
	// a bug from the outside: the dig still costs health.
	WallStrengthDeltaAtZero int `json:"wall_strength_delta_at_zero"`
	// Wall strength a dig packs into the cell either side of it, by size
	// class, along its own curve. Separate from what the dig clears ahead
	// so the two can differ: a small organism can tunnel slowly without
	// being able to raise walls at all (set its class to 0), which one
	// shared number could never express.
	WallCreatedSmall  int `json:"wall_created_small"`
	WallCreatedMedium int `json:"wall_created_medium"`
	WallCreatedLarge  int `json:"wall_created_large"`
	// WallCreatedAtZero is what a dig raises with no Digging at all.
	WallCreatedAtZero int `json:"wall_created_at_zero"`
	// Food a dig roots up in the cell ahead when it isn't a wall or
	// occupied. The only food source besides random spawns and corpses,
	// so energy can enter the world without chemosynthesis.
	FoodFromDiggingSmall  int `json:"food_from_digging_small"`
	FoodFromDiggingMedium int `json:"food_from_digging_medium"`
	FoodFromDiggingLarge  int `json:"food_from_digging_large"`
	// FoodFromDiggingAtZero is what a dig roots up with no Digging at
	// all. 0 by default: rooting for nutrients is the ability's payoff,
	// so an organism that hasn't invested in it gets nothing.
	FoodFromDiggingAtZero int `json:"food_from_digging_at_zero"`

	// Sensors tree

	// Defense tree

	// Teeth tree

	// --- Statistics ---
	PopulationUpdateInterval int `json:"population_update_interval"`
}

func LoadFile(filePath string) io.Reader {
	file, err := os.Open(filePath)
	if err != nil {
		panic("failed to read config file")
	}
	return file
}

// GetDefaultGlobals returns the project-baseline Globals decoded from
// the embedded settings/default.json. Used both by the config screen
// (as the starting point) and by anyone who wants a known-good
// configuration without going through user input.
func GetDefaultGlobals() Globals {
	g := applyGlobalsFromJson(loadEmbeddedDefault(), Globals{})
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
