package ux

import (
	"fmt"

	"github.com/Zebbeni/protozoa/physiology"
)

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
	"ideal_ph_range":          "How wide a band of ideal pH values lineages can evolve across, centred on the middle of the pH scale: 9 gives 0.5 to 9.5. Every organism starts at that middle, and the environment does too.",
	"ideal_ph_mutation_step":  "How far a child's ideal pH can shift from its parent's, up or down. Sets how fast a lineage can follow a drifting world: 0 pins every lineage to the pH it started at, so a world that drifts far enough wipes them out.",
	"max_ph_tolerance_width":  "The pH offset an organism with 100 Tolerance bears for one unit of damage. Lower scores bear less, along the pH tolerance curve.",
	"chemo_ph_effect":         "How far chemosynthesis pushes the local pH down at full Chemosynthesis, per unit of health it gained, scaled from there by the Chemosynthesis pH push curve. An attempt that gained nothing moves nothing.",
	"eating_ph_effect":        "How far eating pushes the local pH up at full Eating, per unit of health the meal gave, scaled from there by the Eating pH push curve. Dropped behind the eater rather than where the food was.",
	"ph_diffuse_factor":       "How fast pH spreads between neighbouring cells each cycle. Higher values even out differences sooner.",
	"ph_increment_to_display": "Size of the pH steps the grid redraws at: a cell is redrawn when its pH crosses into a new step. Only affects drawing, not the simulation.",

	// Organisms
	"min_organisms":                     "Ends the run once the population falls below this, after it has first grown to twice this number. 0 disables it, so only extinction ends a run.",
	"max_cycles":                        "Ends the run once it reaches this cycle, so a simulation can be started and left alone. 0 leaves it unlimited.",
	"max_replay_size_mb":                "Ends the run once its replay file is projected to reach this many megabytes, so a sim left running can't fill a disk. Estimated from the bytes written so far plus what the descendant trees will add at the end. No unlimited option: this is the condition that keeps an unattended run in check.",
	"max_organisms":                     "Population cap. Organisms can't reproduce while the population is at this size.",
	"growth_factor":                     "Share of health gained beyond an organism's current size that becomes growth, for every gain except eating.",
	"max_bite_at_full_eating":           "Most food an organism with 100 Eating removes in one eating attempt, as a multiple of its size. Lower Eating removes less along the Eating curve. Food piles hold 5-100, so a cap far above that leaves Eating with nothing to decide.",
	"health_per_food_unit":              "Health one unit of food is worth to whoever eats it. Flat: the Eating score buys how much an organism can swallow, not how much the food nourishes it.",
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
	"max_chemosynthesis_gain":           "The health an organism gains for chemosynthesizing at exactly its ideal pH, per unit of size. Water further off gains less, and past the organism's Chemosynthesis width the attempt costs health instead.",
	"health_change_from_idle":           "Health change for idling, per unit of size.",
	"health_change_from_turning":        "Health cost of turning with 0 Movement, per unit of size. Higher Movement pays less along the Movement cost curve.",
	"health_change_from_moving_at_max":  "Health a move costs at full Movement, per unit of size. The curve runs from the 0-Movement cost down to this, so the ability buys a discount rather than free travel.",
	"health_change_from_turning_at_max": "Health a turn costs at full Movement, per unit of size. The curve runs from the 0-Movement cost down to this.",
	"health_change_from_moving":         "Health cost of moving (or trying to) with 0 Movement, per unit of size. Higher Movement pays less along the Movement cost curve.",
	"health_change_from_eating_attempt": "Health cost of each eating attempt, whether or not there is food, per unit of size.",
	"health_change_from_blocked_move":   "Extra health lost, per unit of size, when an organism moves into a wall, food or another organism that was already there, on top of the move cost. No ability reduces it — nothing makes a wall passable. Not charged when the cell was empty at decision time and another organism reached it first.",
	"health_change_from_spawning":       "Health cost of reproducing, per unit of size, on top of the health given to the child.",
	"health_change_from_attacking":      "Health cost of attacking, per unit of the attacker's size.",
	"health_change_from_digging":        "Health cost of digging with 0 Digging, per unit of size. Higher Digging pays less along the Digging cost curve.",
	"health_change_from_digging_at_max": "Health a dig costs at full Digging, per unit of size. The curve runs from the 0-Digging cost down to this, so the ability buys a discount rather than free digging.",
	"health_change_inflicted_by_attack": "Damage an attack deals with 100 Attack, per unit of the attacker's size, before the target's Defense. Lower Attack deals less along the Attack curve. Damage is a positive amount; it's subtracted from the target.",
	"corpse_food_multiplier":            "Food a dead organism leaves behind, as a multiple of its size.",
	"unhealthy_ph_damage":               "Health lost per cycle, per unit of size, for an environment 1 pH-width from an organism's ideal. The cost grows with the square of the distance and shrinks with Tolerance, but never reaches zero.",

	// Terrain
	"wall_strength_delta_small":   "Wall strength a small organism at full Digging clears from the cell ahead with one dig. Lower Digging clears less, down to the clear @0 value.",
	"wall_strength_delta_medium":  "Wall strength a medium organism at full Digging clears from the cell ahead with one dig. Lower Digging clears less, down to the clear @0 value.",
	"wall_strength_delta_large":   "Wall strength a large organism at full Digging clears from the cell ahead with one dig. Lower Digging clears less, down to the clear @0 value.",
	"wall_strength_delta_at_zero": "Wall strength one dig clears with no Digging at all. The removal curve runs from here up to the size-class value at full Digging, rounded to whole units. 1 keeps a scratch from doing nothing at all.",
	"wall_created_small":          "Wall strength a small organism at full Digging packs into EACH cell beside it with one dig. 0 means it can tunnel but not build.",
	"wall_created_medium":         "Wall strength a medium organism at full Digging packs into EACH cell beside it with one dig. 0 means it can tunnel but not build.",
	"wall_created_large":          "Wall strength a large organism at full Digging packs into EACH cell beside it with one dig. 0 means it can tunnel but not build.",
	"wall_created_at_zero":        "Wall strength one dig raises beside it with no Digging at all. The creation curve runs from here up to the size-class value at full Digging.",
	"wall_break_multiplier":       "Scales size x Digging score into the wall strength an organism can shoulder straight through, destroying the wall and taking its cell in one move. Wall strengths run 1-100. 0 switches burrowing off, leaving digging as the only way through.",
	"food_from_digging_small":     "Food a small organism with 100 Digging roots up in the cell ahead (if it isn't a wall or occupied), as whole units. Lower Digging roots up less, down to the food @0 value.",
	"food_from_digging_medium":    "Food a medium organism with 100 Digging roots up in the cell ahead (if it isn't a wall or occupied), as whole units. Lower Digging roots up less, down to the food @0 value.",
	"food_from_digging_large":     "Food a large organism with 100 Digging roots up in the cell ahead (if it isn't a wall or occupied), as whole units. Lower Digging roots up less, down to the food @0 value.",
	"food_from_digging_at_zero":   "Food one dig roots up with no Digging at all. The curve runs from here up to the size-class value at 100 Digging, and the result is rounded to whole units. 0 means rooting for nutrients pays only what the ability is worth.",

	// Initial abilities
	"random_initial_abilities": "Give each initial organism its own random split of the 200 ability points (100 max per ability), instead of the scores below.",

	// Ability scores
	"chance_to_mutate_abilities":        "Chance that a child shifts some ability points from one ability to another.",
	"health_change_inflicted_by_thorns": "Damage an organism with 100 Defense deals back to whoever hits it, per unit of its own size, as a positive amount. Independent of how hard the hit was.",
	"chemosynthesis_cosine_k":           "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine, symmetric about score 50) to 1 (the most suppression). Open the graphs to see it.",
	"chemosynthesis_saturating_k":       "K: how early the saturating shape delivers its gains, from 1 to 100. Half the benefit arrives by score √K — about 1 at K 1, 5 at K 25, 10 at K 100. Open the graphs to see it.",
	"eating_cosine_k":                   "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine, symmetric about score 50) to 1 (the most suppression). Open the graphs to see it.",
	"eating_saturating_k":               "K: how early the saturating shape delivers its gains, from 1 to 100. Half the benefit arrives by score √K — about 1 at K 1, 5 at K 25, 10 at K 100. Open the graphs to see it.",
	"movement_cost_cosine_k":            "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine, symmetric about score 50) to 1 (the most suppression). Open the graphs to see it.",
	"movement_cost_saturating_k":        "K: how early the saturating shape delivers its gains, from 1 to 100. Half the benefit arrives by score √K — about 1 at K 1, 5 at K 25, 10 at K 100. Open the graphs to see it.",
	"digging_cost_cosine_k":             "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine, symmetric about score 50) to 1 (the most suppression). Open the graphs to see it.",
	"digging_cost_saturating_k":         "K: how early the saturating shape delivers its gains, from 1 to 100. Half the benefit arrives by score √K — about 1 at K 1, 5 at K 25, 10 at K 100. Open the graphs to see it.",
	"digging_strength_cosine_k":         "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine, symmetric about score 50) to 1 (the most suppression). Open the graphs to see it.",
	"digging_strength_saturating_k":     "K: how early the saturating shape delivers its gains, from 1 to 100. Half the benefit arrives by score √K — about 1 at K 1, 5 at K 25, 10 at K 100. Open the graphs to see it.",
	"digging_creation_cosine_k":         "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine) to 1 (the most suppression). Open the graphs to see it.",
	"digging_creation_saturating_k":     "K: how early the saturating shape delivers its gains. Half the benefit arrives by score √K. Open the graphs to see it.",
	"attack_cosine_k":                   "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine, symmetric about score 50) to 1 (the most suppression). Open the graphs to see it.",
	"attack_saturating_k":               "K: how early the saturating shape delivers its gains, from 1 to 100. Half the benefit arrives by score √K — about 1 at K 1, 5 at K 25, 10 at K 100. Open the graphs to see it.",
	"damage_taken_cosine_k":             "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine, symmetric about score 50) to 1 (the most suppression). Open the graphs to see it.",
	"damage_taken_saturating_k":         "K: how early the saturating shape delivers its gains, from 1 to 100. Half the benefit arrives by score √K — about 1 at K 1, 5 at K 25, 10 at K 100. Open the graphs to see it.",
	"thorns_cosine_k":                   "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine, symmetric about score 50) to 1 (the most suppression). Open the graphs to see it.",
	"thorns_saturating_k":               "K: how early the saturating shape delivers its gains, from 1 to 100. Half the benefit arrives by score √K — about 1 at K 1, 5 at K 25, 10 at K 100. Open the graphs to see it.",
	"chemo_ph_effect_cosine_k":          "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine) to 1 (the most suppression). Open the graphs to see it.",
	"chemo_ph_effect_saturating_k":      "K: how early the curve pays off, from 1 (almost all of it by a low score) to 100 (only specialists see much). Open the graphs to see it.",
	"eating_ph_effect_cosine_k":         "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine) to 1 (the most suppression). Open the graphs to see it.",
	"eating_ph_effect_saturating_k":     "K: how early the curve pays off, from 1 (almost all of it by a low score) to 100 (only specialists see much). Open the graphs to see it.",
	"ph_tolerance_cosine_k":             "K: how strongly the cosine shape holds back low and middle scores, from 0 (a pure cosine, symmetric about score 50) to 1 (the most suppression). Open the graphs to see it.",
	"ph_tolerance_saturating_k":         "K: how early the saturating shape delivers its gains, from 1 to 100. Half the benefit arrives by score √K — about 1 at K 1, 5 at K 25, 10 at K 100. Open the graphs to see it.",

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
	return fmt.Sprintf("Starting %s score for the initial organisms. Click - / + to change it by 1, "+
		"or shift-click for 10. Each score caps at %d, and all seven must add up to %d.",
		name, physiology.MaxAbilityScore, physiology.PointTotal)
}

// abilityTotalTooltip explains the INITIAL ABILITIES total row.
const abilityTotalTooltip = "Sum of the starting ability scores. The simulation won't start until it's exactly 100, unless Random Initial Abilities is on."

// tooltipFor returns the tooltip text for a config row, or "" if none.
// curveShapeTooltip explains the shape picker, which is one control
// repeated per curve rather than a setting of its own.
const curveShapeTooltip = "How this curve climbs from nothing at score 0 to the full value at 100. " +
	"saturating: most of the benefit arrives in the first few points (K sets how early). " +
	"linear: every point buys the same amount. " +
	"quadratic: slow at first, then accelerating, so the ability rewards committing to it. " +
	"cosine: an S-curve, slow at both ends and fastest in the middle (K holds back low scores). " +
	"Linear and quadratic ignore K. Open the graphs to see the shape."

// designRowTooltip explains the INITIAL DESIGNS rows. One tooltip for
// every row: what differs between them is which design, and the row
// already says that.
const designRowTooltip = "Found the simulation with this saved organism design. Tick several and they are dealt round-robin across the initial organisms, so two designs can be pitted against each other. With none ticked, the founders are random as usual."

func tooltipFor(field configField) string {
	switch field.row {
	case rowAbilityScore:
		return abilityScoreTooltip(field.label)
	case rowAbilityTotal:
		return abilityTotalTooltip
	case rowCurveGraph:
		return ""
	case rowCurveHeader:
		return curveShapeTooltip
	case rowDesign:
		return designRowTooltip
	}
	if field.textOnly && field.jsonTag == "" {
		// A section's explanatory line, e.g. the note shown when no
		// designs have been saved yet. It is its own tooltip.
		return field.label
	}
	if field.graphToggle {
		return configTooltips[field.jsonTag] + " Click the arrow to show or hide graphs of how the score changes these actions."
	}
	return configTooltips[field.jsonTag]
}
