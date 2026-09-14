package ux

// configTooltips explains each config-screen setting, keyed by JSON tag.
// Shown when the mouse rests on a row. TestEveryConfigFieldHasTooltip
// fails if a field on the screen is missing one, so add an entry here
// whenever a field is added to buildSections.
var configTooltips = map[string]string{
	// Simulation
	"seed": "Random seed for the simulation. The same seed and settings reproduce the same run. 0 picks a new random seed each time.",

	// Display
	"grid_units_wide": "Width of the world, in grid cells.",
	"grid_units_high": "Height of the world, in grid cells.",
	"screen_width":    "Window width in pixels.",
	"screen_height":   "Window height in pixels.",

	// Environment
	"initial_organisms":       "How many organisms the simulation starts with.",
	"initial_food":            "How many food items are scattered across the world at the start.",
	"initial_walls":           "How many walls are placed at random at the start, with random strengths.",
	"chance_to_add_food_item": "Chance each cycle that a new food item appears somewhere in the world.",
	"min_food_value":          "Food items worth this much or less disappear.",
	"max_food_value":          "Largest amount a food item can be worth. New food gets a random value up to this.",

	// pH
	"min_initial_ph":                    "Together with Max Initial pH, sets the starting pH of the world: every cell starts at the midpoint of the two.",
	"max_initial_ph":                    "Together with Min Initial pH, sets the starting pH of the world: every cell starts at the midpoint of the two.",
	"min_ideal_ph":                      "Lowest ideal pH an organism can evolve. Initial organisms start at the midpoint of the min and max.",
	"max_ideal_ph":                      "Highest ideal pH an organism can evolve. Initial organisms start at the midpoint of the min and max.",
	"ph_tolerance":                      "How far a cell's pH can be from an organism's ideal pH before the organism starts taking damage.",
	"chemosynthesis_tolerance":          "Sets the pH range where chemosynthesis works, as a multiple of pH Tolerance. Above 1, organisms can chemosynthesize in water that is also hurting them.",
	"chemo_ph_falloff":                  "How quickly chemosynthesis weakens as pH moves away from ideal: efficiency = 1 - (distance / window)^falloff. 1 is a straight-line decline; 0 turns it off (full strength anywhere in the window).",
	"chemo_curve_exponent":              "Shapes the Chemosynthesis ability curve above its starting score. 1 is linear; below 1, extra points pay off less and less.",
	"chemosynthesis_ph_effect_per_size": "How much a successful chemosynthesis lowers the pH of the organism's cell, per unit of the organism's size.",
	"eating_ph_effect_per_food":         "How much eating raises pH, per unit of food eaten. The waste lands in the cell behind the eater.",
	"ph_diffuse_factor":                 "How fast pH spreads between neighbouring cells each cycle. Higher values even out differences sooner.",
	"ph_increment_to_display":           "Size of the pH steps the grid redraws at: a cell is redrawn when its pH crosses into a new step. Only affects drawing, not the simulation.",

	// Organisms
	"min_organisms":                     "Ends the run once the population falls below this, after it has first grown to twice this number. 0 disables it, so only extinction ends a run.",
	"max_organisms":                     "Population cap. Organisms can't reproduce while the population is at this size.",
	"growth_factor":                     "Share of health gained beyond an organism's current size that becomes growth, for every gain except eating.",
	"eating_growth_factor":              "Share of health gained from eating, beyond an organism's current size, that becomes growth. At 1 an eater keeps a whole meal instead of losing the overflow.",
	"maximum_max_size":                  "Largest max size any organism can evolve. Also sets the small / medium / large size classes (thirds of this).",
	"minimum_max_size":                  "Smallest max size an organism's descendants can evolve.",
	"maximum_initial_size":              "Initial organisms get a random max size up to this.",
	"maximum_initial_spawn_health":      "Initial organisms give each child a random amount of health up to this (and never more than their max spawn share).",
	"max_cycles_between_spawns":         "Longest wait between reproductions an organism can evolve.",
	"max_initial_cycles_between_spawns": "Initial organisms wait a random number of cycles between reproductions, up to this.",
	"min_spawn_health":                  "Smallest amount of health a parent can evolve to give each child.",
	"max_spawn_health_percent":          "Largest share of its max size a parent can give each child as health.",
	"max_lifespan":                      "Organisms die when they reach this age, in cycles. 0 turns lifespan-based death off.",

	// Decision trees
	"initial_organism_decision_tree_mutations": "How many random mutations the initial organisms' decision trees get before the run starts. 0 starts every organism just chemosynthesizing.",
	"chance_to_mutate_decision_tree":           "Chance that a child's decision tree mutates from its parent's.",
	"max_decision_tree_size":                   "Largest number of nodes a decision tree can grow to.",

	// Health changes
	"health_change_from_chemosynthesis":        "Health gained from a successful chemosynthesis, per unit of size, before the Chemosynthesis ability and pH efficiency scale it.",
	"health_change_from_failed_chemosynthesis": "Health change when chemosynthesis fails because the pH is outside the chemosynthesis range, per unit of size.",
	"health_change_from_idle":                  "Health change for idling, per unit of size.",
	"health_change_from_turning":               "Health cost of turning, per unit of size, scaled by the Movement ability.",
	"health_change_from_moving":                "Health cost of moving (or trying to), per unit of size, scaled by the Movement ability.",
	"health_change_from_eating_attempt":        "Health cost of each eating attempt, whether or not there is food, per unit of size.",
	"health_change_from_spawning":              "Health cost of reproducing, per unit of size, on top of the health given to the child.",
	"health_change_from_attacking":             "Health cost of attacking, per unit of the attacker's size.",
	"health_change_from_digging":               "Health cost of digging, per unit of size. Better diggers pay less.",
	"health_change_inflicted_by_attack":        "Damage an attack deals, per unit of the attacker's size, scaled by its Attack ability and reduced by the target's Defense.",
	"health_change_inflicted_by_shoving":       "Damage a large organism deals when it pushes into an occupied cell, per unit of its size, scaled by its Digging ability and reduced by the target's Defense.",
	"attack_health_gain":                       "Share of attack damage the attacker gains as health, up to the victim's remaining health. 0 means attackers only benefit by eating the corpse.",
	"chemo_crowding_penalty":                   "Share of chemosynthesis lost for each neighbouring organism (up to four). 0 turns crowding off.",
	"corpse_food_multiplier":                   "Food a dead organism leaves behind, as a multiple of its size.",
	"health_change_per_unhealthy_ph":           "Damage per cycle for each pH unit a cell is beyond an organism's tolerance, per unit of size. Reduced by Defense, depending on Defense pH Protection.",

	// Terrain
	"wall_strength_delta_small":  "Wall strength a small organism adds or removes with one dig, before its Digging ability scales it.",
	"wall_strength_delta_medium": "Wall strength a medium organism adds or removes with one dig, before its Digging ability scales it.",
	"wall_strength_delta_large":  "Wall strength a large organism adds or removes with one dig, before its Digging ability scales it.",
	"wall_break_score_small":     "Digging or Attack score at which a small organism removes exactly 1 point of wall strength per hit when it pushes into or attacks a wall. Higher scores hit harder.",
	"wall_break_score_medium":    "Digging or Attack score at which a medium organism removes exactly 1 point of wall strength per hit when it pushes into or attacks a wall. Higher scores hit harder.",
	"wall_break_score_large":     "Digging or Attack score at which a large organism removes exactly 1 point of wall strength per hit when it pushes into or attacks a wall. Higher scores hit harder.",

	// Initial abilities
	"random_initial_abilities": "Give each initial organism its own random split of the 100 ability points, instead of the scores below.",

	// Ability scores
	"genesis_minor_ability_score": "Score where each non-chemosynthesis ability's multiplier is exactly 1; chemosynthesis pivots on the remainder of the 100 points. Starting scores are set under Initial Abilities.",
	"chance_to_mutate_abilities":  "Chance that a child shifts some ability points from one ability to another.",
	"max_ability_shift":           "Most ability points a single mutation can move.",
	"ability_specialization_span": "Points above its pivot an ability needs to reach its full '@100' multiplier. The multiplier keeps growing in a straight line past that.",
	"thorns_threshold":            "Defense score above which an organism hurts whoever attacks it.",
	"thorns_damage_per_point":     "Damage dealt back to an attacker per hit, for each Defense point above the threshold, per unit of the defender's size.",
	"defense_ph_protection":       "How much of Defense's protection against attacks also applies to unhealthy-pH damage: 0 none, 1 the same.",
	"chemo_mult_at_zero":          "Chemosynthesis multiplier with 0 points in it.",
	"chemo_mult_at_max":           "Chemosynthesis multiplier once the score is Specialization Span points above its pivot.",
	"eating_mult_at_zero":         "Eating multiplier (bite size) with 0 points in it.",
	"eating_mult_at_max":          "Eating multiplier (bite size) once the score is Specialization Span points above its pivot.",
	"movement_mult_at_zero":       "Cost multiplier for moving and turning with 0 Movement points. Above 1 makes moving more expensive.",
	"movement_mult_at_max":        "Cost multiplier for moving and turning once Movement is Specialization Span points above its pivot. Below 1 makes moving cheaper.",
	"digging_mult_at_zero":        "Digging multiplier with 0 points in it: how much terrain a dig moves and how cheap it is.",
	"digging_mult_at_max":         "Digging multiplier once the score is Specialization Span points above its pivot.",
	"attack_mult_at_zero":         "Attack damage multiplier with 0 Attack points.",
	"attack_mult_at_max":          "Attack damage multiplier once Attack is Specialization Span points above its pivot.",
	"defense_mult_at_zero":        "Damage-taken multiplier with 0 Defense points. Above 1 means taking extra damage.",
	"defense_mult_at_max":         "Damage-taken multiplier once Defense is Specialization Span points above its pivot. Below 1 means taking less damage.",

	// Appearance
	"shell_body_threshold":     "Defense score at which an organism is drawn with a shell.",
	"spikes_body_threshold":    "Defense score at which an organism is drawn with spikes.",
	"pili_motor_threshold":     "Movement score at which an organism is drawn with pili.",
	"flagella_motor_threshold": "Movement score at which an organism is drawn with flagella.",
	"teeth_mouth_threshold":    "Eating score at which an organism is drawn with teeth (if Eating is its strongest mouth ability).",
	"fangs_mouth_threshold":    "Attack score at which an organism is drawn with fangs (if Attack is its strongest mouth ability).",
	"tusks_mouth_threshold":    "Digging score at which an organism is drawn with tusks (if Digging is its strongest mouth ability).",
	"sensor_min_conditions":    "How many checks of one sense a decision tree needs before the organism is drawn with that sensor: antennae for food and organisms, feelers for walls, tasters for pH.",

	// Statistics
	"population_update_interval": "Cycles between graph samples. Each bar on the graphs covers this many cycles.",
}

// abilityScoreTooltip explains one of the INITIAL ABILITIES score rows.
func abilityScoreTooltip(name string) string {
	return "Starting " + name + " score for the initial organisms. Click - / + to change it by 1, or shift-click for 10. All six scores must add up to 100."
}

// abilityTotalTooltip explains the INITIAL ABILITIES total row.
const abilityTotalTooltip = "Sum of the starting ability scores. The simulation won't start until it's exactly 100, unless Random Initial Abilities is on."

// tooltipFor returns the tooltip text for a config row, or "" if none.
func tooltipFor(field configField) string {
	switch field.row {
	case rowAbilityScore:
		return abilityScoreTooltip(field.label)
	case rowAbilityTotal:
		return abilityTotalTooltip
	}
	return configTooltips[field.jsonTag]
}
