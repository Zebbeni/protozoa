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
func ChanceToAddFoodItem() float64 { return constants.ChanceToAddFoodItem }
func MinFoodValue() int            { return constants.MinFoodValue }
func MaxFoodValue() int            { return constants.MaxFoodValue }

// --- pH ---
func MinPh() float64                   { return constants.MinPh }
func MaxPh() float64                   { return constants.MaxPh }
func MinInitialPh() float64            { return constants.MinInitialPh }
func MaxInitialPh() float64            { return constants.MaxInitialPh }
func MinIdealPh() float64              { return constants.MinIdealPh }
func MaxIdealPh() float64              { return constants.MaxIdealPh }
func PhTolerance() float64             { return constants.PhTolerance }
func ChemosynthesisTolerance() float64 { return constants.ChemosynthesisTolerance }
func ChemoPhEffectPerSize() float64    { return constants.ChemoPhEffectPerSize }
func EatingPhEffectPerFood() float64   { return constants.EatingPhEffectPerFood }
func PhDiffuseFactor() float64         { return constants.PhDiffuseFactor }
func PhIncrementToDisplay() float64    { return constants.PhIncrementToDisplay }

// --- Organisms ---
func MinOrganisms() int                  { return constants.MinOrganisms }
func MaxOrganisms() int                  { return constants.MaxOrganisms }
func GrowthFactor() float64              { return constants.GrowthFactor }
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
func HealthChangeFromStinging() float64      { return constants.HealthChangeFromStinging }
func HealthChangeFromDigging() float64       { return constants.HealthChangeFromDigging }
func HealthChangeFromHunkering() float64     { return constants.HealthChangeFromHunkering }
func HealthChangeFromFlaring() float64       { return constants.HealthChangeFromFlaring }
func HealthChangeFromHiding() float64        { return constants.HealthChangeFromHiding }

// Damage delivered to targets — size-scaled, always negative.
func HealthChangeInflictedByAttack() float64 { return constants.HealthChangeInflictedByAttack }
func HealthChangeInflictedBySting() float64  { return constants.HealthChangeInflictedBySting }

// Environmental health changes.
func HealthChangePerUnhealthyPh() float64 { return constants.HealthChangePerCycleUnhealthyPh }

// --- Physiology ---
func ChanceToGainFeature() float64   { return constants.ChanceToGainFeature }
func ChanceToLoseFeature() float64   { return constants.ChanceToLoseFeature }
func HunkerDamageTakenMult() float64 { return constants.HunkerDamageTakenMult }
func FlareDamageDealtMult() float64  { return constants.FlareDamageDealtMult }
func FlarePerceivedSizeAdd() float64 { return constants.FlarePerceivedSizeAdd }
func WallStrengthDeltaSmall() int    { return constants.WallStrengthDeltaSmall }
func WallStrengthDeltaMedium() int   { return constants.WallStrengthDeltaMedium }
func WallStrengthDeltaLarge() int    { return constants.WallStrengthDeltaLarge }

// Per-feature passive tradeoffs. Each feature contributes its own
// set; only the deepest-held feature in each tree applies (body-type
// exclusivity — see physiology.Set.Combined). Unitless multipliers
// default to 1.0 (no effect); the additive modifier defaults to 0.

// Flagellae tree
func FlagellaeChemoEfficiencyMult() float64 { return constants.FlagellaeChemoEfficiencyMult }
func CiliaChemoEfficiencyMult() float64     { return constants.CiliaChemoEfficiencyMult }
func CiliaMoveCostMult() float64            { return constants.CiliaMoveCostMult }
func StingerChemoEfficiencyMult() float64   { return constants.StingerChemoEfficiencyMult }

// Sensors tree
func AntennaeChemoEfficiencyMult() float64 { return constants.AntennaeChemoEfficiencyMult }
func FeelersChemoEfficiencyMult() float64  { return constants.FeelersChemoEfficiencyMult }
func TastersChemoEfficiencyMult() float64  { return constants.TastersChemoEfficiencyMult }

// Defense tree
func ShellChemoEfficiencyMult() float64      { return constants.ShellChemoEfficiencyMult }
func ShellMoveCostMult() float64             { return constants.ShellMoveCostMult }
func ShellDamageTakenMult() float64          { return constants.ShellDamageTakenMult }
func SpikesChemoEfficiencyMult() float64     { return constants.SpikesChemoEfficiencyMult }
func SpikesMoveCostMult() float64            { return constants.SpikesMoveCostMult }
func SpikesDamageTakenMult() float64         { return constants.SpikesDamageTakenMult }
func SpikesDamageDealtMult() float64         { return constants.SpikesDamageDealtMult }
func SpikesPerceivedSizeAdd() float64        { return constants.SpikesPerceivedSizeAdd }
func CamouflageChemoEfficiencyMult() float64 { return constants.CamouflageChemoEfficiencyMult }
func CamouflageMoveCostMult() float64        { return constants.CamouflageMoveCostMult }
func CamouflageDamageTakenMult() float64     { return constants.CamouflageDamageTakenMult }

// Teeth tree
func TeethChemoEfficiencyMult() float64 { return constants.TeethChemoEfficiencyMult }
func FangsChemoEfficiencyMult() float64 { return constants.FangsChemoEfficiencyMult }
func FangsDamageDealtMult() float64     { return constants.FangsDamageDealtMult }
func TusksChemoEfficiencyMult() float64 { return constants.TusksChemoEfficiencyMult }
func TusksMoveCostMult() float64        { return constants.TusksMoveCostMult }
func TusksDamageDealtMult() float64     { return constants.TusksDamageDealtMult }

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
	neutral := (constants.MaxPh + constants.MinPh) / 2.0
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
	GridUnitsWide int    `json:"grid_units_wide"`
	GridUnitsHigh int    `json:"grid_units_high"`
	ScreenWidth   int    `json:"screen_width"`
	ScreenHeight  int    `json:"screen_height"`
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
	ChanceToAddFoodItem float64 `json:"chance_to_add_food_item"`
	MinFoodValue        int     `json:"min_food_value"`
	MaxFoodValue        int     `json:"max_food_value"`

	// --- pH ---
	MinPh        float64 `json:"min_ph"`
	MaxPh        float64 `json:"max_ph"`
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
	// Chemosynthesis pushes the local pH down by ChemoPhEffectPerSize
	// * organism.Size on each successful chemo cycle. Eating pushes
	// the local pH up by EatingPhEffectPerFood * food_amount_eaten
	// on each successful eat.
	ChemoPhEffectPerSize  float64 `json:"chemosynthesis_ph_effect_per_size"`
	EatingPhEffectPerFood float64 `json:"eating_ph_effect_per_food"`
	PhDiffuseFactor       float64 `json:"ph_diffuse_factor"`
	PhIncrementToDisplay  float64 `json:"ph_increment_to_display"`

	// --- Organisms ---
	MinOrganisms                  int     `json:"min_organisms"`
	MaxOrganisms                  int     `json:"max_organisms"`
	GrowthFactor                  float64 `json:"growth_factor"`
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
	HealthChangeFromStinging             float64 `json:"health_change_from_stinging"`
	HealthChangeFromDigging              float64 `json:"health_change_from_digging"`
	HealthChangeFromHunkering            float64 `json:"health_change_from_hunkering"`
	HealthChangeFromFlaring              float64 `json:"health_change_from_flaring"`
	HealthChangeFromHiding               float64 `json:"health_change_from_hiding"`
	HealthChangeInflictedByAttack        float64 `json:"health_change_inflicted_by_attack"`
	HealthChangeInflictedBySting         float64 `json:"health_change_inflicted_by_sting"`
	HealthChangePerCycleUnhealthyPh      float64 `json:"health_change_per_unhealthy_ph"`

	// --- Physiology ---
	// ChanceToGainFeature is the per-spawn probability that a child
	// gains one new physiological feature (drawn uniformly at random
	// from those whose prerequisites the parent already meets).
	ChanceToGainFeature float64 `json:"chance_to_gain_feature"`
	// ChanceToLoseFeature is the per-spawn probability that a child
	// loses the deepest feature from one of its non-empty modality
	// trees (drawn uniformly at random across non-empty trees). Lets
	// lineages back out of a branch so descendants can re-grow down
	// a sibling — without this the population saturates at the same
	// leaf set in every tree and physiological diversity vanishes.
	ChanceToLoseFeature float64 `json:"chance_to_lose_feature"`
	// Posture-state modifiers applied during the cycle an organism
	// is in the matching posture. Multipliers default to 1.0 = no
	// effect; the additive modifier is a signed delta.
	HunkerDamageTakenMult float64 `json:"hunker_damage_taken_mult"`
	FlareDamageDealtMult  float64 `json:"flare_damage_dealt_mult"`
	FlarePerceivedSizeAdd float64 `json:"flare_perceived_size_add"`
	// WallStrengthDeltaSmall/Medium/Large set how much wall strength
	// a single dig or burrow action adds or removes, bucketed by the
	// organism's size class (thirds of MaximumMaxSize).
	WallStrengthDeltaSmall  int `json:"wall_strength_delta_small"`
	WallStrengthDeltaMedium int `json:"wall_strength_delta_medium"`
	WallStrengthDeltaLarge  int `json:"wall_strength_delta_large"`

	// Per-feature passive Tradeoffs. Body-type exclusivity means
	// only the deepest-held feature in each tree contributes — see
	// physiology.Set.Combined. Multipliers default to 1.0; additive
	// modifiers default to 0. Fields kept in feature-then-aspect
	// order so the source layout mirrors default.json and the
	// settings editor's per-tree subsections.

	// Flagellae tree
	FlagellaeChemoEfficiencyMult float64 `json:"flagellae_chemo_efficiency_mult"`
	CiliaChemoEfficiencyMult     float64 `json:"cilia_chemo_efficiency_mult"`
	CiliaMoveCostMult            float64 `json:"cilia_move_cost_mult"`
	StingerChemoEfficiencyMult   float64 `json:"stinger_chemo_efficiency_mult"`

	// Sensors tree
	AntennaeChemoEfficiencyMult float64 `json:"antennae_chemo_efficiency_mult"`
	FeelersChemoEfficiencyMult  float64 `json:"feelers_chemo_efficiency_mult"`
	TastersChemoEfficiencyMult  float64 `json:"tasters_chemo_efficiency_mult"`

	// Defense tree
	ShellChemoEfficiencyMult      float64 `json:"shell_chemo_efficiency_mult"`
	ShellMoveCostMult             float64 `json:"shell_move_cost_mult"`
	ShellDamageTakenMult          float64 `json:"shell_damage_taken_mult"`
	SpikesChemoEfficiencyMult     float64 `json:"spikes_chemo_efficiency_mult"`
	SpikesMoveCostMult            float64 `json:"spikes_move_cost_mult"`
	SpikesDamageTakenMult         float64 `json:"spikes_damage_taken_mult"`
	SpikesDamageDealtMult         float64 `json:"spikes_damage_dealt_mult"`
	SpikesPerceivedSizeAdd        float64 `json:"spikes_perceived_size_add"`
	CamouflageChemoEfficiencyMult float64 `json:"camouflage_chemo_efficiency_mult"`
	CamouflageMoveCostMult        float64 `json:"camouflage_move_cost_mult"`
	CamouflageDamageTakenMult     float64 `json:"camouflage_damage_taken_mult"`

	// Teeth tree
	TeethChemoEfficiencyMult float64 `json:"teeth_chemo_efficiency_mult"`
	FangsChemoEfficiencyMult float64 `json:"fangs_chemo_efficiency_mult"`
	FangsDamageDealtMult     float64 `json:"fangs_damage_dealt_mult"`
	TusksChemoEfficiencyMult float64 `json:"tusks_chemo_efficiency_mult"`
	TusksMoveCostMult        float64 `json:"tusks_move_cost_mult"`
	TusksDamageDealtMult     float64 `json:"tusks_damage_dealt_mult"`

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
