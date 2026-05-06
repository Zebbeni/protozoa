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

func GridUnitsWide() int                     { return constants.GridUnitsWide }
func GridUnitsHigh() int                     { return constants.GridUnitsHigh }
func ScreenWidth() int                       { return constants.ScreenWidth }
func ScreenHeight() int                      { return constants.ScreenHeight }
func PopulationUpdateInterval() int          { return constants.PopulationUpdateInterval }
func InitialOrganisms() int                  { return constants.InitialOrganisms }
func InitialFood() int                       { return constants.InitialFood }
func ChanceToAddFoodItem() float64           { return constants.ChanceToAddFoodItem }
func MaxFoodValue() int                      { return constants.MaxFoodValue }
func MinFoodValue() int                      { return constants.MinFoodValue }
func MinPh() float64                         { return constants.MinPh }
func MaxPh() float64                         { return constants.MaxPh }
func MinInitialPh() float64                  { return constants.MinInitialPh }
func MaxInitialPh() float64                  { return constants.MaxInitialPh }
func MaxCyclesBetweenSpawns() int            { return constants.MaxCyclesBetweenSpawns }
func MinSpawnHealth() float64                { return constants.MinSpawnHealth }
func MaxSpawnHealthPercent() float64         { return constants.MaxSpawnHealthPercent }
func InitialDecisionTreeMutations() int      { return constants.InitialDecisionTreeMutations }
func MinChanceToMutateDecisionTree() float64 { return constants.MinChanceToMutateDecisionTree }
func MaxChanceToMutateDecisionTree() float64 { return constants.MaxChanceToMutateDecisionTree }
func MinOrganisms() int                      { return constants.MinOrganisms }
func MaxOrganisms() int                      { return constants.MaxOrganisms }
func GrowthFactor() float64                  { return constants.GrowthFactor }
func MaximumMaxSize() float64                { return constants.MaximumMaxSize }
func MinimumMaxSize() float64                { return constants.MinimumMaxSize }
func MaximumInitialSize() float64            { return constants.MaximumInitialSize }
func MaximumInitialSpawnHealth() float64     { return constants.MaximumInitialSpawnHealth }
func MaxInitialCyclesBetweenSpawns() int     { return constants.MaxInitialCyclesBetweenSpawns }
func MinIdealPh() float64                    { return constants.MinIdealPh }
func MaxIdealPh() float64                    { return constants.MaxIdealPh }
func MinPhToleranceRange() float64           { return constants.MinPhToleranceRange }
func MaxPhToleranceRange() float64           { return constants.MaxPhToleranceRange }
func ChemosynthesisTolerance() float64       { return constants.ChemosynthesisTolerance }
func MaxOrganismPhGrowthEffect() float64     { return constants.MaxOrganismPhGrowthEffect }
func MaxPhEffectChange() float64             { return constants.MaxPhEffectChange }
func MinMaxLifespan() int                    { return constants.MinMaxLifespan }
func MaxMaxLifespan() int                    { return constants.MaxMaxLifespan }
func MaxMaxLifespanChange() int              { return constants.MaxMaxLifespanChange }
func PhIncrementToDisplay() float64          { return constants.PhIncrementToDisplay }
func PhDiffuseFactor() float64               { return constants.PhDiffuseFactor }
func UsePools() bool                         { return constants.UsePools }
func PoolWidth() int                         { return constants.PoolWidth }
func PoolHeight() int                        { return constants.PoolHeight }

// Theme returns the active GUI theme name. Recognised values: "dark",
// "light". Anything else falls back to dark behaviour at render time.
func Theme() string { return constants.Theme }

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
// towards as it moves away from neutral, in RGB floats [0, 1].
//   - Sub-neutral (acidic) pH → #A9C218 (yellow-green)
//   - Supra-neutral (basic) pH → #E74766 (red-pink)
//
// At exactly neutral the caller should use a weight of 0 so the target
// colour has no effect.
func PhTargetColorRGB(ph float64) (r, g, b float64) {
	neutral := (constants.MaxPh + constants.MinPh) / 2.0
	if ph < neutral {
		return 0xA9 / 255.0, 0xC2 / 255.0, 0x18 / 255.0
	}
	return 0xE7 / 255.0, 0x47 / 255.0, 0x66 / 255.0
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
func HealthChangeFromChemosynthesis() float64 { return constants.HealthChangeFromChemosynthesis }
func HealthChangeFromFailedChemosynthesis() float64 {
	return constants.HealthChangeFromFailedChemosynthesis
}
func HealthChangeFromTurning() float64        { return constants.HealthChangeFromTurning }
func HealthChangeFromMoving() float64         { return constants.HealthChangeFromMoving }
func HealthChangeFromEatingAttempt() float64  { return constants.HealthChangeFromEatingAttempt }
func HealthChangeFromAttacking() float64      { return constants.HealthChangeFromAttacking }
func HealthChangeFromSpawning() float64       { return constants.HealthChangeFromSpawning }
func HealthChangeFromIdle() float64           { return constants.HealthChangeFromIdle }
func HealthChangeInflictedByAttack() float64  { return constants.HealthChangeInflictedByAttack }

func HealthChangePerUnhealthyPh() float64 { return constants.HealthChangePerCycleUnhealthyPh }
func MaxDecisionTreeSize() int            { return constants.MaxDecisionTreeSize }
func Seed() int                           { return constants.Seed }

type Globals struct {
	// Seed for the simulation RNG. 0 means "use the CLI --seed flag
	// or the time-based default chosen by the runner". Editable via
	// the config screen so wasm builds (no CLI) can pick a seed.
	Seed int `json:"seed"`

	// Drawing parameters
	GridUnitsWide int `json:"grid_units_wide"`
	GridUnitsHigh int `json:"grid_units_high"`
	ScreenWidth   int `json:"screen_width"`
	ScreenHeight  int `json:"screen_height"`

	// Statistics parameters
	PopulationUpdateInterval int `json:"population_update_interval"`

	// Environment parameters
	InitialOrganisms    int     `json:"initial_organisms"`
	InitialFood         int     `json:"initial_food"`
	ChanceToAddFoodItem float64 `json:"chance_to_add_food_item"`
	MaxFoodValue        int     `json:"max_food_value"`
	MinFoodValue        int     `json:"min_food_value"`
	MinPh               float64 `json:"min_ph"`
	MaxPh               float64 `json:"max_ph"`
	MinInitialPh        float64 `json:"min_initial_ph"`
	MaxInitialPh        float64 `json:"max_initial_ph"`

	// Organism parameters
	MaxCyclesBetweenSpawns        int     `json:"max_cycles_between_spawns"`
	MinSpawnHealth                float64 `json:"min_spawn_health"`
	MaxSpawnHealthPercent         float64 `json:"max_spawn_health_percent"`
	MinOrganisms                  int     `json:"min_organisms"`
	MaxOrganisms                  int     `json:"max_organisms"`
	GrowthFactor                  float64 `json:"growth_factor"`
	MaximumMaxSize                float64 `json:"maximum_max_size"`
	MinimumMaxSize                float64 `json:"minimum_max_size"`
	MaximumInitialSize            float64 `json:"maximum_initial_size"`
	MaximumInitialSpawnHealth     float64 `json:"maximum_initial_spawn_health"`
	MaxInitialCyclesBetweenSpawns int     `json:"max_initial_cycles_between_spawns"`
	InitialDecisionTreeMutations  int     `json:"initial_organism_decision_tree_mutations"`
	MinChanceToMutateDecisionTree float64 `json:"min_chance_to_mutate_decision_tree"`
	MaxChanceToMutateDecisionTree float64 `json:"max_chance_to_mutate_decision_tree"`
	MaxDecisionTreeSize           int     `json:"max_decision_tree_size"`
	MinIdealPh                    float64 `json:"min_ideal_ph"`
	MaxIdealPh                    float64 `json:"max_ideal_ph"`
	MinPhToleranceRange           float64 `json:"min_ph_tolerance_range"`
	MaxPhToleranceRange           float64 `json:"max_ph_tolerance_range"`
	// ChemosynthesisTolerance multiplies an organism's PhTolerance to
	// define the pH window in which chemosynthesis succeeds. Values
	// below 1 narrow chemo-viable pH relative to the organism's
	// general survival tolerance; values above 1 widen it.
	ChemosynthesisTolerance       float64 `json:"chemosynthesis_tolerance"`
	MaxOrganismPhGrowthEffect     float64 `json:"max_organism_ph_growth_effect"`
	MaxPhEffectChange             float64 `json:"max_ph_effect_change"`
	// Max lifespan (in cycles) range and mutation step. Set
	// MaxMaxLifespan to 0 to disable lifespan-based death entirely.
	MinMaxLifespan       int     `json:"min_max_lifespan"`
	MaxMaxLifespan       int     `json:"max_max_lifespan"`
	MaxMaxLifespanChange int     `json:"max_max_lifespan_change"`
	MinChangeToPh        float64 `json:"min_change_to_ph"`
	MaxChangeToPh        float64 `json:"max_change_to_ph"`
	PhIncrementToDisplay float64 `json:"ph_increment_to_display"`
	PhDiffuseFactor      float64 `json:"ph_diffuse_factor"`
	UsePools             bool    `json:"use_pools"`
	PoolWidth            int     `json:"pool_width"`
	PoolHeight           int     `json:"pool_height"`

	// GUI theme: "light" or "dark". Controls the window background and
	// selects between <theme>-prefixed sprite sheets.
	Theme string `json:"theme"`

	// Health parameters (percent of organism size)
	HealthChangeFromChemosynthesis       float64 `json:"health_change_from_chemosynthesis"`
	HealthChangeFromFailedChemosynthesis float64 `json:"health_change_from_failed_chemosynthesis"`
	HealthChangeFromTurning              float64 `json:"health_change_from_turning"`
	HealthChangeFromMoving          float64 `json:"health_change_from_moving"`
	HealthChangeFromEatingAttempt   float64 `json:"health_change_from_eating_attempt"`
	HealthChangeFromAttacking       float64 `json:"health_change_from_attacking"`
	HealthChangeFromSpawning        float64 `json:"health_change_from_spawning"`
	HealthChangeFromIdle            float64 `json:"health_change_from_idle"`
	HealthChangeInflictedByAttack   float64 `json:"health_change_inflicted_by_attack"`
	HealthChangePerCycleUnhealthyPh float64 `json:"health_change_per_unhealthy_ph"`
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
