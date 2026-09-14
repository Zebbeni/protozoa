package config

import (
	"encoding/json"
	"io"
	"os"
)

var defaultFilePath = "settings/default.json"
var constants *Globals

// SetGlobals initializes all globally-referenced constants.
func SetGlobals(g *Globals) {
	constants = g
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
)

func MinPh() float64                   { return minPh }
func MaxPh() float64                   { return maxPh }
func MinInitialPh() float64            { return constants.MinInitialPh }
func MaxInitialPh() float64            { return constants.MaxInitialPh }
func MinIdealPh() float64              { return constants.MinIdealPh }
func MaxIdealPh() float64              { return constants.MaxIdealPh }
func PhTolerance() float64             { return constants.PhTolerance }
func ChemosynthesisTolerance() float64 { return constants.ChemosynthesisTolerance }

// ChemosynthesisPhWindow is how far an organism's cell pH may sit from its
// ideal and still allow chemosynthesis. The single source for that window:
// the chemosynthesis action and the CanChemosynthesizeHere condition both
// read it, so what a decision tree tests can never disagree with what the
// action does. Wider than PhTolerance when ChemosynthesisTolerance > 1, in
// which case chemosynthesis works in water that is also damaging.
func ChemosynthesisPhWindow() float64 { return PhTolerance() * ChemosynthesisTolerance() }
func ChemoPhFalloff() float64         { return constants.ChemoPhFalloff }
func ChemoCurveExponent() float64     { return constants.ChemoCurveExponent }
func ChemoPhEffectPerSize() float64   { return constants.ChemoPhEffectPerSize }
func EatingPhEffectPerFood() float64  { return constants.EatingPhEffectPerFood }
func PhDiffuseFactor() float64        { return constants.PhDiffuseFactor }
func PhIncrementToDisplay() float64   { return constants.PhIncrementToDisplay }

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
func HealthChangeFromChemosynthesis() float64 { return constants.HealthChangeFromChemosynthesis }
func HealthChangeFromFailedChemosynthesis() float64 {
	return constants.HealthChangeFromFailedChemosynthesis
}
func HealthChangeFromIdle() float64          { return constants.HealthChangeFromIdle }
func HealthChangeFromTurning() float64       { return constants.HealthChangeFromTurning }
func HealthChangeFromMoving() float64        { return constants.HealthChangeFromMoving }
func HealthChangeFromEatingAttempt() float64 { return constants.HealthChangeFromEatingAttempt }
func HealthChangeFromSpawning() float64      { return constants.HealthChangeFromSpawning }
func HealthChangeFromAttacking() float64     { return constants.HealthChangeFromAttacking }
func HealthChangeFromDigging() float64       { return constants.HealthChangeFromDigging }

// Damage delivered to targets — size-scaled, always negative.
func HealthChangeInflictedByAttack() float64  { return constants.HealthChangeInflictedByAttack }
func HealthChangeInflictedByShoving() float64 { return constants.HealthChangeInflictedByShoving }
func AttackHealthGain() float64               { return constants.AttackHealthGain }
func ChemoCrowdingPenalty() float64           { return constants.ChemoCrowdingPenalty }
func CorpseFoodMultiplier() float64           { return constants.CorpseFoodMultiplier }

// Environmental health changes.
func HealthChangePerUnhealthyPh() float64 { return constants.HealthChangePerCycleUnhealthyPh }

// --- Ability scores ---
// Every organism distributes a fixed budget (physiology.PointTotal)
// across six ability scores. Each score maps to an effect multiplier
// via two linear segments pinned to 1.0 at the even-split score, so
// the *AtZero / *AtMax pairs below are the full balance surface.
//
// Cost-style abilities (movement, and the cost half of digging) invert:
// their AtZero is above 1 and AtMax below, so a high score means the
// action is cheaper rather than stronger.
func GenesisMinorAbilityScore() int    { return constants.GenesisMinorAbilityScore }
func InitialAbilityScores() []int      { return constants.InitialAbilityScores }
func RandomInitialAbilities() bool     { return constants.RandomInitialAbilities }
func ChanceToMutateAbilities() float64 { return constants.ChanceToMutateAbilities }
func MaxAbilityShift() int             { return constants.MaxAbilityShift }
func AbilitySpecializationSpan() int   { return constants.AbilitySpecializationSpan }
func ThornsThreshold() int             { return constants.ThornsThreshold }
func ThornsDamagePerPoint() float64    { return constants.ThornsDamagePerPoint }
func DefensePhProtection() float64     { return constants.DefensePhProtection }

// --- Appearance thresholds ---
// Sprite overlays are derived from ability scores, not inherited, so
// these decide at what point an organism starts *looking* like what it
// has specialised in. Genesis organisms sit at GenesisMinorAbilityScore
// in every non-chemo ability, so a threshold at or below that would
// give every newborn the overlay for free.
func ShellBodyThreshold() int     { return constants.ShellBodyThreshold }
func SpikesBodyThreshold() int    { return constants.SpikesBodyThreshold }
func PiliMotorThreshold() int     { return constants.PiliMotorThreshold }
func FlagellaMotorThreshold() int { return constants.FlagellaMotorThreshold }
func TeethMouthThreshold() int    { return constants.TeethMouthThreshold }
func FangsMouthThreshold() int    { return constants.FangsMouthThreshold }
func TusksMouthThreshold() int    { return constants.TusksMouthThreshold }
func SensorMinConditions() int    { return constants.SensorMinConditions }
func ChemoMultAtZero() float64    { return constants.ChemoMultAtZero }
func ChemoMultAtMax() float64     { return constants.ChemoMultAtMax }
func EatingMultAtZero() float64   { return constants.EatingMultAtZero }
func EatingMultAtMax() float64    { return constants.EatingMultAtMax }
func MovementMultAtZero() float64 { return constants.MovementMultAtZero }
func MovementMultAtMax() float64  { return constants.MovementMultAtMax }
func DiggingMultAtZero() float64  { return constants.DiggingMultAtZero }
func DiggingMultAtMax() float64   { return constants.DiggingMultAtMax }
func AttackMultAtZero() float64   { return constants.AttackMultAtZero }
func AttackMultAtMax() float64    { return constants.AttackMultAtMax }
func DefenseMultAtZero() float64  { return constants.DefenseMultAtZero }
func DefenseMultAtMax() float64   { return constants.DefenseMultAtMax }

// --- Physiology ---
func WallStrengthDeltaSmall() int  { return constants.WallStrengthDeltaSmall }
func WallStrengthDeltaMedium() int { return constants.WallStrengthDeltaMedium }
func WallStrengthDeltaLarge() int  { return constants.WallStrengthDeltaLarge }
func WallBreakScoreSmall() int     { return constants.WallBreakScoreSmall }
func WallBreakScoreMedium() int    { return constants.WallBreakScoreMedium }
func WallBreakScoreLarge() int     { return constants.WallBreakScoreLarge }

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
	MinInitialPh float64 `json:"min_initial_ph"`
	MaxInitialPh float64 `json:"max_initial_ph"`
	MinIdealPh   float64 `json:"min_ideal_ph"`
	MaxIdealPh   float64 `json:"max_ideal_ph"`
	// PhTolerance is the absolute pH distance every organism can sit
	// from its IdealPh without taking unhealthy-pH damage. Global
	// rather than per-organism — variation between organisms now
	// comes from IdealPh alone.
	PhTolerance float64 `json:"ph_tolerance"`
	// ChemosynthesisTolerance multiplies PhTolerance to define the
	// pH window in which chemosynthesis succeeds. Values below 1
	// narrow chemo-viable pH relative to general survival tolerance;
	// values above 1 widen it.
	ChemosynthesisTolerance float64 `json:"chemosynthesis_tolerance"`
	// ChemoPhFalloff shapes how chemosynthesis efficiency drops as a
	// cell's pH moves away from the organism's ideal, inside the
	// chemosynthesis window: efficiency = 1 - (distance/window)^k. Both
	// the health gained and the acid produced scale with it, so a
	// population chemosynthesizing in increasingly poor water also slows
	// the acidification that made it poor. 1 is a linear decline, 2
	// forgives small offsets and drops steeply near the edge, 0.5 drops
	// quickly from the start. 0 disables it: full efficiency anywhere
	// inside the window, the original flat behaviour.
	ChemoPhFalloff float64 `json:"chemo_ph_falloff"`
	// ChemoCurveExponent shapes the Chemosynthesis ability curve above an
	// organism's genesis allocation. 1 is linear. Below 1 gives
	// diminishing returns: the first points invested past genesis pay
	// most, and stacking chemosynthesis toward the top of the budget pays
	// progressively less, leaving more reason to spend points elsewhere.
	ChemoCurveExponent float64 `json:"chemo_curve_exponent"`
	// Chemosynthesis pushes the local pH down by ChemoPhEffectPerSize
	// * organism.Size on each successful chemo cycle. Eating pushes
	// the local pH up by EatingPhEffectPerFood * food_amount_eaten
	// on each successful eat.
	ChemoPhEffectPerSize  float64 `json:"chemosynthesis_ph_effect_per_size"`
	EatingPhEffectPerFood float64 `json:"eating_ph_effect_per_food"`
	PhDiffuseFactor       float64 `json:"ph_diffuse_factor"`
	PhIncrementToDisplay  float64 `json:"ph_increment_to_display"`

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
	//   - HealthChangeInflictedBy<Action> — size-scaled damage
	//     delivered to targets (always negative).
	//   - HealthChangePer<Source> — environmental health delta.
	HealthChangeFromChemosynthesis       float64 `json:"health_change_from_chemosynthesis"`
	HealthChangeFromFailedChemosynthesis float64 `json:"health_change_from_failed_chemosynthesis"`
	HealthChangeFromIdle                 float64 `json:"health_change_from_idle"`
	HealthChangeFromTurning              float64 `json:"health_change_from_turning"`
	HealthChangeFromMoving               float64 `json:"health_change_from_moving"`
	HealthChangeFromEatingAttempt        float64 `json:"health_change_from_eating_attempt"`
	HealthChangeFromSpawning             float64 `json:"health_change_from_spawning"`
	HealthChangeFromAttacking            float64 `json:"health_change_from_attacking"`
	HealthChangeFromDigging              float64 `json:"health_change_from_digging"`
	HealthChangeInflictedByAttack        float64 `json:"health_change_inflicted_by_attack"`
	// HealthChangeInflictedByShoving is the damage per unit of size a
	// large organism deals when it tries to move into an occupied cell,
	// scaled by its Digging multiplier and reduced by the target's
	// Defense like any other damage. See manager.shoveEffect.
	HealthChangeInflictedByShoving float64 `json:"health_change_inflicted_by_shoving"`
	// AttackHealthGain is the share of attack damage landed on a victim
	// that the attacker gains as health, capped at the health the victim
	// had left: predation. 0 disables it, leaving corpses as the only
	// payoff from an attack.
	AttackHealthGain float64 `json:"attack_health_gain"`
	// ChemoCrowdingPenalty is the share of chemosynthesis efficiency lost
	// per adjacent organism (up to four), modelling competition for the
	// same dissolved nutrients. 0 disables it.
	ChemoCrowdingPenalty float64 `json:"chemo_crowding_penalty"`
	// CorpseFoodMultiplier scales the food a dead organism leaves behind,
	// as a multiple of its size.
	CorpseFoodMultiplier            float64 `json:"corpse_food_multiplier"`
	HealthChangePerCycleUnhealthyPh float64 `json:"health_change_per_unhealthy_ph"`

	// --- Ability scores ---
	// GenesisMinorAbilityScore is the score every non-chemosynthesis
	// ability starts at for a genesis organism; chemosynthesis takes
	// the whole remainder of the budget. Genesis organisms are pure
	// chemosynthesizers, and every point of movement / attack / defense
	// a lineage later evolves is a point taken off self-feeding.
	GenesisMinorAbilityScore int `json:"genesis_minor_ability_score"`
	// InitialAbilityScores is the ability distribution the simulation's
	// initial organisms start with, in physiology.AllAbilities order
	// (chemosynthesis, eating, movement, digging, attack, defense). It
	// must sum to the 100-point budget; the config screen refuses to
	// start otherwise, and a bad value loaded from a file falls back to
	// the genesis distribution. It sets only where organisms start —
	// the multiplier curves still pivot on the genesis distribution
	// from GenesisMinorAbilityScore.
	InitialAbilityScores []int `json:"initial_ability_scores"`
	// RandomInitialAbilities gives each initial organism its own random
	// split of the budget instead of InitialAbilityScores.
	RandomInitialAbilities bool `json:"random_initial_abilities"`
	// ChanceToMutateAbilities is the per-spawn probability that a child
	// shifts points between two abilities. Higher than the old feature
	// gain rate because a transfer is a small nudge rather than a whole
	// new capability.
	ChanceToMutateAbilities float64 `json:"chance_to_mutate_abilities"`
	// MaxAbilityShift caps how many points one mutation can move. The
	// actual shift is 1..MaxAbilityShift, clamped to the donor balance.
	MaxAbilityShift int `json:"max_ability_shift"`
	// AbilitySpecializationSpan is how many points above its genesis
	// allocation an ability must gain to reach its full multiplier.
	// Fixed across abilities on purpose: anchoring the top of the curve
	// at the 100-point budget instead made non-chemo abilities nine
	// times harder to grow into than to abandon, which put every
	// specialist behind a fitness valley no lineage could cross.
	AbilitySpecializationSpan int `json:"ability_specialization_span"`
	// ThornsThreshold is the Defense score above which a
	// defender starts returning damage to its attacker. Gated rather
	// than scaled from zero so light armour absorbs damage while heavy
	// armour punishes — the counter-attack should read as its own
	// strategy, not as a small bonus everyone gets.
	ThornsThreshold int `json:"thorns_threshold"`
	// ThornsDamagePerPoint is the damage a defender deals back to each
	// organism that hits it, per point of Defense above ThornsThreshold
	// and per unit of the defender's size. Independent of the attack: at
	// 0.005 a size-50 defender with 80 Defense deals 10 per hit taken,
	// however hard or soft the hit was.
	ThornsDamagePerPoint float64 `json:"thorns_damage_per_point"`
	// DefensePhProtection is how much of Defense's protection against
	// attacks also applies to unhealthy-pH damage, from 0 (none — pH
	// damage ignores Defense) to 1 (pH damage is reduced exactly as much
	// as attack damage).
	DefensePhProtection float64 `json:"defense_ph_protection"`

	// --- Appearance thresholds ---
	// Score at which each overlay starts being drawn. Must sit above
	// GenesisMinorAbilityScore or newborns get the sprite for nothing.
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

	// Per-ability multiplier curve endpoints. Each pair is
	// (multiplier at score 0, multiplier at score PointTotal); the
	// curve passes through exactly 1.0 at the even-split score.
	ChemoMultAtZero    float64 `json:"chemo_mult_at_zero"`
	ChemoMultAtMax     float64 `json:"chemo_mult_at_max"`
	EatingMultAtZero   float64 `json:"eating_mult_at_zero"`
	EatingMultAtMax    float64 `json:"eating_mult_at_max"`
	MovementMultAtZero float64 `json:"movement_mult_at_zero"`
	MovementMultAtMax  float64 `json:"movement_mult_at_max"`
	DiggingMultAtZero  float64 `json:"digging_mult_at_zero"`
	DiggingMultAtMax   float64 `json:"digging_mult_at_max"`
	AttackMultAtZero   float64 `json:"attack_mult_at_zero"`
	AttackMultAtMax    float64 `json:"attack_mult_at_max"`
	DefenseMultAtZero  float64 `json:"defense_mult_at_zero"`
	DefenseMultAtMax   float64 `json:"defense_mult_at_max"`

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
	WallStrengthDeltaSmall  int `json:"wall_strength_delta_small"`
	WallStrengthDeltaMedium int `json:"wall_strength_delta_medium"`
	WallStrengthDeltaLarge  int `json:"wall_strength_delta_large"`
	// WallBreakScoreSmall/Medium/Large calibrate how hard organisms wear
	// down walls by moving into them (Digging) or attacking them
	// (Attack): the ability score at which an organism of that size
	// class removes exactly one point of wall strength per hit. Higher
	// scores remove proportionally more, following the ability's
	// multiplier curve. See manager.wallDamage.
	WallBreakScoreSmall  int `json:"wall_break_score_small"`
	WallBreakScoreMedium int `json:"wall_break_score_medium"`
	WallBreakScoreLarge  int `json:"wall_break_score_large"`

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
