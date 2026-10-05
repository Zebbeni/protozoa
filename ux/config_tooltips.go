package ux

import (
	"fmt"

	"github.com/Zebbeni/protozoa/physiology"
)

// configTooltips explains each config-screen setting, keyed by JSON tag.
var configTooltips = map[string]string{
	"seed": "Random seed for the simulation. The same seed and settings reproduce the same run. 0 picks a new random seed each time.",

	"grid_units_wide": "Width of the world, in grid cells.",
	"grid_units_high": "Height of the world, in grid cells.",
	"screen_width":    "Window width in pixels.",
	"screen_height":   "Window height in pixels.",

	"initial_organisms":       "How many organisms the simulation starts with.",
	"initial_food":            "How many food items are scattered across the world at the start.",
	"initial_walls":           "How many walls are placed at random at the start, with random strengths.",
	"chance_to_add_food_item": "Chance each cycle that a new food item appears somewhere in the world.",
	"min_food_value":          "Food items worth this much or less disappear.",
	"max_food_value":          "Largest amount a food item can be worth. New food gets a random value up to this.",
	"max_buried_food_value":   "The most that can sit buried under one cell, separate from the surface limit so the ground can hold far more than a pile can. A high value lets the world bank potential energy underground for digging to recover later; the surface limit still applies when it comes back up, so a deep store surfaces over many digs rather than all at once. 0 falls back to Max Food Value.",

	"ideal_ph_range":              "How wide a band of ideal pH values lineages can evolve across, centred on the middle of the pH scale: 9 gives 0.5 to 9.5. Every organism starts at that middle, and the environment does too.",
	"ideal_ph_mutation_step":      "How far a child's ideal pH can shift from its parent's, up or down. Sets how fast a lineage can follow a drifting world: 0 pins every lineage to the pH it started at, so a world that drifts far enough wipes them out.",
	"max_ph_tolerance_width":      "The pH offset an organism with 100 Tolerance bears for one unit of damage. Lower scores bear less, along the pH tolerance curve.",
	"max_chemosynthesis_ph_width": "The pH offset an organism at full Chemosynthesis can still gain health at. Lower scores reach less of it along the chemosynthesis curve, and at 0 an organism only breaks even at exactly its ideal pH. This is the width of the band a lineage can feed in, so it bounds how far the world's pH can drift before chemosynthesis stops paying anywhere. The tolerance counterpart is Max pH Tolerance Width.",
	"chemo_ph_effect":             "How far chemosynthesis pushes the local pH down at full Chemosynthesis, per unit of health it gained, scaled from there by the Chemosynthesis pH push curve. An attempt that gained nothing moves nothing.",
	"eating_ph_effect":            "How far eating pushes the local pH up at full Eating, per unit of health the meal gave, scaled from there by the Eating pH push curve. Dropped behind the eater rather than where the food was.",
	"ph_diffuse_factor":           "How fast pH spreads between neighbouring cells each cycle. Higher values even out differences sooner.",
	"chemo_crowding_penalty":      "How strongly chemosynthesisers compete for the same ground. A gain is drawn in equal shares from five cells — the organism's own and its four neighbours — and every chemosynthesiser within one step of a cell draws on it, so a cell worked by three of them yields each a third. An organism alone keeps its whole gain; one boxed in on four sides keeps a fifth. At 0 there is no competition, which is how the simulation behaved before this existed, and at 1 the split applies in full. A FAILED attempt is never shared: its cost is the organism's own wasted effort, so being surrounded does not soften it.",
	"attack_health_gain":          "The share of the health an attack actually removes that the attacker feeds back. At 0 a kill pays only in the corpse it leaves, which whoever eats it collects — so a predator needs Attack AND Eating AND to be standing on the body, where a chemosynthesiser needs one ability and no positioning. Above 0 killing feeds the killer directly, which is what lets carnivory stand on its own. Paid on health REMOVED, not damage dealt: attack damage runs far past what an organism holds, so overkill earns nothing extra.",
	"ph_smoothing":                "Whether the pH layer is blended across cell boundaries when it is enlarged. Off draws every cell as one flat colour, which is what the simulation actually holds — pH is a per-cell reading. On blends neighbours, which reads as a continuous field and hides where one cell ends. Rendering only: it changes nothing the simulation does, so a recording replays the same either way.",
	"wall_ph_block_at_max":        "How much of the pH diffusion a full-strength wall takes away. 1 seals it completely; 0 leaves every wall as open as water. Weaker walls block proportionally less, so digging one down opens it gradually.",
	"wall_ph_block_curve":         "How a wall's blocking ramps up with its strength. 1 is a straight line. Above 1 the blocking bunches up near full strength, so wearing a wall down opens it early. Below 1 even a flimsy wall is a real barrier, and 0 makes any wall block completely whatever its strength.",
	"ph_increment_to_display":     "Size of the pH steps the grid redraws at: a cell is redrawn when its pH crosses into a new step. Only affects drawing, not the simulation.",

	"min_organisms":                     "Ends the run once the population falls below this, after it has first grown to twice this number. 0 disables it, so only extinction ends a run.",
	"max_cycles":                        "Ends the run once it reaches this cycle, so a simulation can be started and left alone. 0 leaves it unlimited.",
	"max_replay_size_mb":                "Ends the run once its replay file is projected to reach this many megabytes, so a sim left running can't fill a disk. Estimated from the bytes written so far plus what the descendant trees will add at the end. No unlimited option: this is the condition that keeps an unattended run in check.",
	"max_organisms":                     "Population cap. Organisms can't reproduce while the population is at this size.",
	"growth_factor":                     "Share of health gained beyond an organism's current size that becomes growth, for every gain except eating.",
	"max_bite_at_full_eating":           "Units of food one eating attempt removes from the cell ahead, per unit of the eater's size, at maximum Eating. An organism of size 10 with this at 10 and full Eating clears 100 units in one attempt. Lower Eating removes less, down to the 0-Eating value, along the Eating curve. What that food is then worth in health is a separate setting. Food piles hold 5-100 units, so a value far above that leaves the Eating score with nothing to decide.",
	"health_per_food_unit":              "Health one unit of food is worth to whoever eats it. Flat, and separate from how much food an attempt removes: the Eating score buys how many units an organism can take, not what each unit nourishes it by.",
	"eating_growth_factor":              "Share of health gained from eating, beyond an organism's current size, that becomes growth. At 1 an eater keeps a whole meal instead of losing the overflow.",
	"maximum_max_size":                  "Largest max size any organism can evolve. Also sets the small / medium / large size classes (thirds of this).",
	"minimum_max_size":                  "Smallest max size an organism's descendants can evolve.",
	"maximum_initial_size":              "Initial organisms get a random max size up to this.",
	"initial_organism_size_fraction":    "The size and health the first organisms start at, as a fraction of their own max size. 0 starts them at their spawn health — the size anything born in the simulation starts at — which can be a few hundred cycles of growing before the first one is big enough to reproduce. 1 starts them full grown and able to spawn on their first cycle. Only affects the organisms the world begins with, never anything born later. It cannot start a founder smaller than its spawn health.",
	"maximum_initial_spawn_health":      "Initial organisms give each child a random amount of health up to this (and never more than their max spawn share).",
	"max_cycles_between_spawns":         "Longest wait between reproductions an organism can evolve.",
	"max_initial_cycles_between_spawns": "Initial organisms wait a random number of cycles between reproductions, up to this.",
	"min_spawn_health":                  "Smallest amount of health a parent can evolve to give each child.",
	"max_spawn_health_percent":          "The largest share of its spawn threshold a parent may hand a child, so it always keeps the rest. At 0.25 an organism that spawns at 40 health gives at most 10 and keeps 30. It is measured against the health the parent must REACH to spawn, not against its max size: a share of max size says nothing about what the parent is holding when it actually spawns, which is how an organism could give away everything and die doing it. The spawn threshold is floored so a child is still worth producing — at a small share that floor is high.",
	"max_lifespan":                      "Organisms die when they reach this age, in cycles. 0 turns lifespan-based death off.",

	"initial_organism_decision_tree_mutations": "How many random mutations the initial organisms' decision trees get before the run starts. 0 starts every organism just chemosynthesizing.",
	"chance_to_mutate_decision_tree":           "Chance that a child's decision tree mutates from its parent's.",
	"max_decision_tree_size":                   "Largest number of nodes a decision tree can grow to.",
	"smart_tree_mutation":                      "Keeps mutation from asking a question whose answer is already settled. A condition is only offered at a spot in a tree if the branches above it haven't already answered it, and if it wouldn't settle the answer to something asked below it — so no branch is grown that can never be taken. Worth turning on when many conditions are enabled, since every dead branch costs a node against Max Tree Size and dilutes the conditions that do something. It changes which condition each mutation lands on, so a run with it on is a different run. Measured over 150,000 cycles it neither helps nor hurts survival, population or the pH swings — what it buys is trees with no wasted nodes.",
	"tiered_condition_mutation":                "Limits a BRAND NEW condition node to the basic reads — can move, food here or ahead, buried food, a wall, safe pH, an organism nearby — and lets the finer ones (bigger organism, much bigger, much food) in only by mutating a condition that is already there. Conditions are arranged in families: an organism read refines to bigger, which refines to much bigger, one step per mutation in either direction, plus a lateral move to any other basic read. Without it a lineage's first question about its surroundings is as likely to be a narrow one as a coarse one, and a narrow read is almost always false, so the branch under it is dead weight that only chance removes. Independent of Smart Mutation, which asks whether a read is already settled at that spot rather than how fine it is.", "initial_buried_food": "How many cells start with food already buried under them. Gives digging something to find in the first few hundred cycles: a tree only keeps the Dig action if digging pays, and with an empty buried layer the first dig of every run returns nothing, so the ability is selected against before burial has stocked the ground. A count of cells rather than a quantity, because what an early organism needs is for a dig where it happens to be standing to pay off.",
	"mutation_weight_swap_action":    "How often a mutation that lands on an ACTION node swaps it for a different action, against growing it into a conditional branch. A weight, not a probability: it is normalised against Grow Branch, so 1 and 1 is an even split and 3 and 1 makes swapping three times as likely.",
	"mutation_weight_grow_branch":    "How often a mutation that lands on an ACTION node turns it into a condition with two action branches — the only way a tree gains structure. Normalised against Swap Action. Ignored when the tree is within two nodes of Max Tree Size, since growing needs the room.",
	"mutation_weight_swap_condition": "How often a mutation that lands on a CONDITION node swaps it for a different condition, against collapsing it back to an action. Normalised against Prune Branch. With Tiered Conditions on, a swap is also how a read gets refined or coarsened by one step within its family.",
	"mutation_weight_prune_branch":   "How often a mutation that lands on a CONDITION node collapses it back to a single action, removing the two nodes a grow added. Only offered where it is the exact inverse of growing — a condition whose two branches are both actions — so one mutation can never discard a large subtree. Set to 0 for trees that can only ever grow, which is what the simulation did before this existed.",
	"min_initial_buried_value":       "Smallest amount of food a cell can start with buried under it.",
	"max_initial_buried_value":       "Largest amount of food a cell can start with buried under it, capped by Max Food Value like any other pile.",
	"burial_amount":                  "How much food each pile loses to the buried layer every burial interval. A flat amount rather than a fraction, so a small pile vanishes in a knowable number of cycles while a large one only thins. Nothing is destroyed: the cell's buried store gains exactly what the pile lost, and digging is the only way back.",
	"burial_interval":                "How many cycles between burials. 0 switches burial off entirely, which is what a settings file written before burial existed does. Lower means food settles out of reach faster, which makes Digging worth more and standing food scarcer.",
	"much_food_per_size":             "How many food units per unit of its own size a pile must hold before an organism reads it as \"much\" food, for the Is Much Food Here and Is Much Food Buried Here conditions. Relative to size because an absolute number would mean a different thing to every organism: a pile of 30 is a windfall at size 5 and a mouthful at size 50. 1 is \"as much food as I am big\". Only matters if those conditions are enabled below.",
	"much_bigger_size_ratio":         "How many times its own size a neighbour must be before an organism reads it as \"much bigger\", for the Is Much Bigger Organism conditions. 4 means four times its size. Only matters if those conditions are enabled below.",
	"much_smaller_size_ratio":        "What fraction of its own size a neighbour must be under before an organism reads it as \"much smaller\", for the Is Much Smaller Organism conditions. 0.25 means a quarter its size, the reciprocal of the much-bigger ratio, so the two describe one relationship from both sides. Only matters if those conditions are enabled below.",

	"max_chemosynthesis_gain":                  "The health an organism gains for chemosynthesizing at exactly its ideal pH, per unit of size. Water further off gains less, and past the organism's Chemosynthesis width the attempt costs health instead.",
	"health_change_from_idle":                  "Health change for idling, per unit of size.",
	"health_change_from_turning":               "Health cost of turning with 0 Movement, per unit of size. Higher Movement pays less along the Movement cost curve.",
	"health_change_from_moving_at_max":         "Health a move costs at full Movement, per unit of size. The curve runs from the 0-Movement cost down to this, so the ability buys a discount rather than free travel.",
	"health_change_from_turning_at_max":        "Health a turn costs at full Movement, per unit of size. The curve runs from the 0-Movement cost down to this.",
	"health_change_from_moving":                "Health cost of moving (or trying to) with 0 Movement, per unit of size. Higher Movement pays less along the Movement cost curve.",
	"health_change_from_eating_attempt":        "Health cost of each eating attempt, whether or not there is food, per unit of size.",
	"health_change_from_eating_attempt_at_max": "What one eating attempt costs an organism at full Eating, per unit of its size. The cost runs from the 0-Eating value down to this one along the Eating cost curve, so specialising buys a cheaper miss. Never reaches zero: the ability buys a discount on a wasted attempt, not an exemption.",
	"max_bite_at_zero_eating":                  "Units of food one eating attempt removes from the cell ahead, per unit of the eater's size, at 0 Eating. The amount runs from this up to the maximum-Eating value along the Eating curve. 0 means an organism with no investment in eating removes nothing at all, however much food is in front of it.",
	"attack_damage_at_zero":                    "The damage an attack deals at 0 Attack, per unit of the attacker's size. 0 means an un-invested organism's attack does nothing.",
	"thorns_damage_at_zero":                    "The damage a defender with 0 Defense returns per hit taken, per unit of its size. 0 means no armour and no answer.",
	"wall_break_at_zero":                       "The wall strength an organism with 0 Digging shoulders straight through, per unit of its size. 0 means no burrowing without the ability. The score carries it linearly up to the at-max value.",
	"wall_break_at_max":                        "The wall strength a full-Digging organism shoulders straight through, per unit of its size. Wall strengths run 1-100, so this reads against a wall's own strength directly. Use the multiplier to switch burrowing off entirely.",
	"burrow_spoil_fraction":                    "How much of a burrowed wall's strength survives as spoil, piled onto the two cells either side of the tunnel instead of vanishing. Half goes to each side, relative to the burrower's direction. 0 means the wall is simply destroyed; 1 means a burrower can only move wall aside, never reduce it. Spoil is never piled onto a cell holding another organism.",
	"eating_cost_cosine_k":                     "Holds back the eating cost discount at low scores when the cosine shape is in use.",
	"eating_cost_saturating_k":                 "Where the eating cost discount reaches half its span when the saturating shape is in use.",
	"health_change_from_blocked_move":          "Extra health lost, per unit of size, when an organism moves into a wall, food or another organism that was already there, on top of the move cost. No ability reduces it — nothing makes a wall passable. Not charged when the cell was empty at decision time and another organism reached it first.",
	"health_change_from_spawning":              "Health cost of reproducing, per unit of size, on top of the health given to the child.",
	"health_change_from_attacking":             "Health cost of attacking, per unit of the attacker's size.",
	"attack_cost_cosine_k":                     "Coefficient for the attack cost curve's cosine shape.",
	"attack_cost_saturating_k":                 "Coefficient for the attack cost curve's saturating shape.",
	"health_change_from_attacking_at_max":      "What one attack costs an organism with a full Attack score, per unit of its size. The cost at 0 Attack is the setting above, and the attack cost curve carries it between the two, so specialising makes attacking cheaper without making it free. Set both ends equal for the flat cost attacking had before the curve existed.",
	"health_change_from_digging":               "Health cost of digging with 0 Digging, per unit of size. Higher Digging pays less along the Digging cost curve.",
	"health_change_from_digging_at_max":        "Health a dig costs at full Digging, per unit of size. The curve runs from the 0-Digging cost down to this, so the ability buys a discount rather than free digging.",
	"health_change_inflicted_by_attack":        "Damage an attack deals with 100 Attack, per unit of the attacker's size, before the target's Defense. Lower Attack deals less along the Attack curve. Damage is a positive amount; it's subtracted from the target.",
	"corpse_food_multiplier":                   "Food a dead organism leaves behind, as a multiple of its size.",
	"unhealthy_ph_damage":                      "Health lost per cycle, per unit of size, for an environment 1 pH-width from an organism's ideal. The cost grows with the square of the distance and shrinks with Tolerance, but never reaches zero.",

	"wall_created_small":        "Wall strength a small organism at full Digging packs into EACH cell beside it with one dig. 0 means it can tunnel but not build.",
	"wall_created_medium":       "Wall strength a medium organism at full Digging packs into EACH cell beside it with one dig. 0 means it can tunnel but not build.",
	"wall_created_large":        "Wall strength a large organism at full Digging packs into EACH cell beside it with one dig. 0 means it can tunnel but not build.",
	"wall_created_at_zero":      "Wall strength one dig raises beside it with no Digging at all. The creation curve runs from here up to the size-class value at full Digging.",
	"wall_break_multiplier":     "Scales size x Digging score into the wall strength an organism can shoulder straight through, destroying the wall and taking its cell in one move. Wall strengths run 1-100. 0 switches burrowing off, leaving digging as the only way through.",
	"food_from_digging_small":   "Food a small organism with 100 Digging roots up in the cell ahead (if it isn't a wall or occupied), as whole units. Lower Digging roots up less, down to the food @0 value.",
	"food_from_digging_medium":  "Food a medium organism with 100 Digging roots up in the cell ahead (if it isn't a wall or occupied), as whole units. Lower Digging roots up less, down to the food @0 value.",
	"food_from_digging_large":   "Food a large organism with 100 Digging roots up in the cell ahead (if it isn't a wall or occupied), as whole units. Lower Digging roots up less, down to the food @0 value.",
	"food_from_digging_at_zero": "Food one dig roots up with no Digging at all. The curve runs from here up to the size-class value at 100 Digging, and the result is rounded to whole units. 0 means rooting for nutrients pays only what the ability is worth.",

	"random_initial_abilities": "Give each initial organism its own random split of the 200 ability points (100 max per ability), instead of the scores below.",

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

	"shell_body_threshold":     "Defense score at which an organism is drawn with a shell.",
	"spikes_body_threshold":    "Defense score at which an organism is drawn with spikes.",
	"pili_motor_threshold":     "Movement score at which an organism is drawn with pili.",
	"flagella_motor_threshold": "Movement score at which an organism is drawn with flagella.",
	"teeth_mouth_threshold":    "Eating score at which an organism is drawn with teeth (if Eating is its strongest mouth ability).",
	"fangs_mouth_threshold":    "Attack score at which an organism is drawn with fangs (if Attack is its strongest mouth ability).",
	"tusks_mouth_threshold":    "Digging score at which an organism is drawn with tusks (if Digging is its strongest mouth ability).",
	"sensor_min_conditions":    "How many checks of one sense a decision tree needs before the organism is drawn with that sensor: antennae for food and organisms, feelers for walls, tasters for pH.",

	"population_update_interval": "Cycles between graph samples. Each bar on the graphs covers this many cycles.",
}

func abilityScoreTooltip(name string) string {
	return fmt.Sprintf("Starting %s score for the initial organisms. Click - / + to change it by 1, "+
		"or shift-click for 10. Each score caps at %d, and all seven must add up to %d.",
		name, physiology.MaxAbilityScore, physiology.PointTotal)
}

const abilityTotalTooltip = "Sum of the starting ability scores. The simulation won't start until it's exactly 100, unless Random Initial Abilities is on."

// tooltipFor returns the tooltip text for a config row, or "" if none.
const curveShapeTooltip = "How this curve climbs from nothing at score 0 to the full value at 100. " +
	"saturating: most of the benefit arrives in the first few points (K sets how early). " +
	"linear: every point buys the same amount. " +
	"quadratic: slow at first, then accelerating, so the ability rewards committing to it. " +
	"cosine: an S-curve, slow at both ends and fastest in the middle (K holds back low scores). " +
	"Linear and quadratic ignore K. Open the graphs to see the shape."

const designRowTooltip = "Found the simulation with this saved organism design. Tick several and they are dealt round-robin across the initial organisms, so two designs can be pitted against each other. With none ticked, the founders are random as usual."

const conditionFamilyRowTooltip = "Whether mutation may REFINE this family's basic read into its finer " +
	"versions: organism ahead into bigger and then much bigger, food here into much food here, " +
	"healthy into very healthy or very unhealthy. The basic read itself is unaffected — that is " +
	"the tick above. With Tiered Conditions on, a brand new condition node can only ever ask a basic " +
	"read, so the finer ones enter a tree only by mutating a node that already asks their parent, one " +
	"step at a time. Switching a family off here therefore removes what a lineage can learn to sense " +
	"rather than reducing mutation noise."

const nodeTypeRowTooltip = "Ticked means decision-tree mutation may put this action or condition " +
	"into a tree. Unticking it does not remove it from the simulation: trees that already " +
	"use it still load and still run, and a saved design can still be built with it — only " +
	"new mutations stop reaching for it. Enabling many at once makes each one rarer in any " +
	"given tree, so the ones lineages already rely on get thinner."

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
	case rowNodeType:
		return nodeTypeRowTooltip
	case rowConditionFamily:
		return conditionFamilyRowTooltip
	}
	if field.textOnly && field.jsonTag == "" {
		// A section's explanatory line, e.g. the note shown when no designs have been saved yet.
		return field.label
	}
	if field.graphToggle {
		return configTooltips[field.jsonTag] + " Click the arrow to show or hide graphs of how the score changes these actions."
	}
	return configTooltips[field.jsonTag]
}
