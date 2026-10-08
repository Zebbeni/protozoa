package ux

import (
	"fmt"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/physiology"
)

// plainLook is the sample organism the action strip uses: no overlays, so
// the only thing changing between cells is the action being performed.
var plainLook = physiology.Appearance{}

// actionNames is the full name of every animation the action list shows.
// TestEveryAnimationIsExplained walks animation.AllAnimations against this
// and actionsNotShown, so a new animation cannot arrive without a name.
var actionNames = map[animation.Animation]string{
	animation.AnimMove:       "Move",
	animation.AnimBlocked:    "Blocked",
	animation.AnimTurnLeft:   "Turn Left",
	animation.AnimTurnRight:  "Turn Right",
	animation.AnimChemo:      "Chemosynthesis",
	animation.AnimChemoFail:  "Chemosynthesis Failure",
	animation.AnimEat:        "Eat",
	animation.AnimEatFail:    "Eat Failure",
	animation.AnimAttack:     "Attack",
	animation.AnimAttackMove: "Attack & Move",
	animation.AnimDig:        "Dig",
	animation.AnimDie:        "Dying",
	animation.AnimSpawn:      "Spawn",
}

// actionsNotShown is left out of the list deliberately. Idle is an organism
// doing nothing, which needs no illustration.
var actionsNotShown = map[animation.Animation]bool{
	animation.AnimIdle: true,
}

// actionOrder is the order the two-column list reads in: each pair shares a
// row, so an action and its failure, and the two turns, sit side by side.
var actionOrder = []animation.Animation{
	animation.AnimMove, animation.AnimBlocked,
	animation.AnimTurnLeft, animation.AnimTurnRight,
	animation.AnimChemo, animation.AnimChemoFail,
	animation.AnimEat, animation.AnimEatFail,
	animation.AnimAttack, animation.AnimAttackMove,
	animation.AnimDig, animation.AnimSpawn,
	animation.AnimDie,
}

func actionCells() []rulesSpriteCellSpec {
	out := make([]rulesSpriteCellSpec, 0, len(actionOrder))
	for _, anim := range actionOrder {
		name, ok := actionNames[anim]
		if !ok {
			continue
		}
		out = append(out, rulesSpriteCellSpec{label: name, anim: anim, look: plainLook})
	}
	return out
}

// appearanceCells shows every class of one appearance dimension on the same
// idle organism, so the sprite is the only thing differing between cells.
func appearanceCells(row int) []rulesSpriteCellSpec {
	spec := appearanceRows[row]
	out := make([]rulesSpriteCellSpec, 0, len(spec.options))
	for v, label := range spec.options {
		look := physiology.Appearance{}
		spec.set(&look, v)
		out = append(out, rulesSpriteCellSpec{label: label, anim: animation.AnimIdle, look: look})
	}
	return out
}

// abilityEffects is what each ability buys, written against effects/ rather
// than from memory. TestEveryAbilityIsExplained walks AllAbilities.
var abilityEffects = map[physiology.Ability][2]string{
	physiology.AbilityChemosynthesis: {"CHEMOSYNTHESIS", "wider band of pH it can feed in"},
	physiology.AbilityEating:         {"EATING", "more food per bite, lower health cost per action"},
	physiology.AbilityMovement:       {"MOVEMENT", "lower health cost per action"},
	physiology.AbilityDigging:        {"DIGGING", "more food dug up, stronger walls dug through, lower health cost per action"},
	physiology.AbilityAttack:         {"ATTACK", "more damage per hit, lower health cost per action"},
	physiology.AbilityDefense:        {"DEFENSE", "less damage taken, more damage returned to attackers"},
	physiology.AbilityTolerance:      {"TOLERANCE", "less health lost to the wrong pH"},
}

func abilityTable() rulesTable {
	rows := make([][2]string, 0, len(physiology.AllAbilities))
	for _, a := range physiology.AllAbilities {
		if e, ok := abilityEffects[a]; ok {
			rows = append(rows, e)
		}
	}
	return rulesTable{rows: rows, nameW: 170}
}

func budgetParagraph() rulesText {
	return rulesText{fmt.Sprintf(
		"An organism's abilities are defined by distributing %d ability points across %d "+
			"categories, each with a maximum value of %d. Every point moved into one category is "+
			"a point taken from another.",
		physiology.PointTotal, len(physiology.AllAbilities), physiology.MaxAbilityScore)}
}

// The document, split into the tabs the About screen shows. The summary and
// the organism crossing the page sit above the tabs and stay put; everything
// else belongs to one of them.
//
// Every claim is checkable in the package it describes. The version this
// replaced listed an action that does not exist, called pH tolerance global
// when it comes from an ability score, and described a per-node health drain
// that was never implemented.

// aboutTab is one tab's name and the blocks under it.
type aboutTab struct {
	label  string
	blocks []rulesBlock
}

// aboutHeader is drawn above the tab strip on every tab.
func aboutHeader() []rulesBlock {
	return []rulesBlock{
		rulesText{"Protozoa is a simulation of organisms navigating a simple environment according " +
			"to inherited traits and decision trees. Successful organisms pass these down to their " +
			"offspring with occasional slight mutations, creating evolutionary branches " +
			"characterized by different abilities and behaviors."},
		newRulesWalker(),
	}
}

func aboutTabs() []aboutTab {
	return []aboutTab{
		{label: "WORLD", blocks: worldBlocks()},
		{label: "ORGANISMS", blocks: organismBlocks()},
		{label: "ACTIONS", blocks: actionBlocks()},
		{label: "VIEWER", blocks: viewerBlocks()},
	}
}

// rulesContent is the whole document in reading order, which is what the
// tests that are about the writing rather than the layout walk.
func rulesContent() []rulesBlock {
	blocks := aboutHeader()
	for _, tab := range aboutTabs() {
		blocks = append(blocks, tab.blocks...)
	}
	return blocks
}

func worldBlocks() []rulesBlock {
	return []rulesBlock{
		rulesHeading{"THE WORLD"},
		rulesText{"The world is a 2D grid that wraps around at every edge, so an organism walking " +
			"off the right-hand side comes back on the left. Time is measured in cycles, with every " +
			"organism performing one action each cycle. Every cell holds a pH value between 0 and " +
			"10, and in a new world every cell starts at 5. Nothing in the world moves pH except " +
			"the organisms living on it."},
		rulesText{"Each cycle, pH spreads between neighbouring cells, so a world left alone " +
			"eventually equalizes toward one single pH value."},
		newRulesPhGrid(),
		rulesText{"Walls are objects created by digging organisms that slow down pH diffusion and " +
			"restrict organism movement. Walls of higher strength have a greater slowing effect on " +
			"pH and block a wider range of organisms."},
		newRulesWallPhGrid(),
		rulesText{"Food items are objects in the world that can be eaten by an organism sharing " +
			"the same space. Over time, uneaten food items slowly become buried underground, " +
			"appearing fainter. Buried food items may amass more value than surface-level food " +
			"items, but they must be dug out by an organism in order to be consumed."},
		newRulesFoodGrid(),

		rulesHeading{"HOW pH CHANGES"},
		rulesText{"Chemosynthesis pushes the pH at the organism's own cell down. Eating pushes it " +
			"up. Both in proportion to the health the action gained, so a marginal action barely " +
			"moves the pH and a failed one does not move it at all."},
		rulesText{"Nothing else in the world moves pH, which is what makes it worth watching. A " +
			"colony that chemosynthesises hard lowers the pH it is standing in, which cuts what " +
			"chemosynthesis pays there and makes eating the better option; eating pushes the pH " +
			"back up again. A population that never learns to eat has no way of turning the drift " +
			"around."},
	}
}

func organismBlocks() []rulesBlock {
	return []rulesBlock{
		rulesHeading{"ORGANISM"},
		rulesText{"An organism has a mix of traits, abilities, and a decision tree that dictates " +
			"its behaviour each cycle. Traits define " +
			"an organism's color, its ideal pH, its maximum size, the health it needs before it " +
			"can spawn offspring, the health it hands each child, and how many cycles it waits " +
			"between one child and the next."},
		budgetParagraph(),
		rulesText{"Every child is its parent with small mutations to all three, so a lineage " +
			"drifts a little at a time rather than jumping."},

		rulesHeading{"WHAT EACH ABILITY BUYS"},
		abilityTable(),
		rulesText{"An ability never unlocks an action. Every organism can do everything at any " +
			"score; the score only decides how well it goes and what it costs. Each one runs along " +
			"a curve you can tune, so whether an ability pays off early or only once an organism " +
			"has specialised is a setting rather than a fact about the ability."},

		rulesHeading{"HEALTH"},
		rulesText{"An organism's health may be thought of as its energy. An organism uses health " +
			"to perform actions and dies the moment its health reaches zero, leaving food worth a " +
			"share of its size at the time of death."},
		rulesText{"An organism's health value may never surpass its size. If an organism would " +
			"gain more health than its size allows, the organism grows in size, up to the value " +
			"defined by its maximum size."},
		newRulesPortrait(),
		rulesText{"There are two ways to gain health. Chemosynthesis converts some portion of the " +
			"environment's pH to health, paying most where the pH is closest to the organism's " +
			"ideal and costing health once it is too far off. Eating converts food to health at a " +
			"flat rate per unit. Health is lost on every action an organism takes, on being " +
			"attacked, and on standing where the pH is away from its ideal. That last one is " +
			"charged every cycle whatever it is doing, and no choice of action avoids it."},
		rulesText{"A bigger body gains and loses proportionally more at the same ability scores, " +
			"so growing changes how fast everything happens rather than whether it pays."},
	}
}

func actionBlocks() []rulesBlock {
	return []rulesBlock{
		rulesHeading{"ACTIONS, AND WHAT THEY LOOK LIKE"},
		rulesText{"Each cycle an organism walks its tree and performs exactly one action. The " +
			"sprite shows which one, so you can read what a crowd is doing without clicking on " +
			"anything. Here is the same organism performing them."},
		rulesActionList{cells: actionCells()},
		rulesText{"Chemosynthesis converts some portion of the environment's pH to health, " +
			"decreasing the pH at the chemosynthesizing organism's location. It pays most where the " +
			"pH is closest to the organism's ideal, and costs health once the pH is too far off, " +
			"which is what Chemosynthesis Failure shows. Eating consumes food at the " +
			"organism's own cell and turns it into health; an attempt that finds nothing there " +
			"still costs something, so eating at an empty cell is a real mistake."},
		rulesText{"Digging brings up food buried under the organism's own cell, as much as is " +
			"actually down there, and raises wall strength on the two cells either side of it. " +
			"Attacking damages whatever is in the cell ahead and hands the attacker a share of the " +
			"health it removed. The attacker carries forward into that cell if nothing is left to " +
			"stop it, either because the blow killed what was in the way or because it swung at an " +
			"empty cell."},

		rulesHeading{"APPEARANCE"},
		rulesText{"A sprite is assembled from the organism's ability scores and its decision tree, " +
			"so you can see what something is built for before you click on it. None of it is " +
			"inherited directly: it is worked out from the scores and the tree, and worked out " +
			"again for every child."},

		newRulesAppearanceGrid(),
		rulesText{"Sensors are the one class that comes from the tree rather than from a score. " +
			"Sensor types provide a visual way to see what type of conditions an organism most " +
			"considers in its decision making: antennae for food and the organisms around it, " +
			"feelers for walls and relative size, tasters for the pH of its environment. A tree " +
			"that only ever checks its own health and age grows none at all. A mouth follows " +
			"whichever of Eating, Attack and Digging leads."},

		rulesHeading{"DECISION TREES"},
		rulesText{"A tree is a set of questions with actions at the ends of them. The questions " +
			"ask about the cell ahead, the cells to either side, the organism's own cell, or the " +
			"organism itself. Here is a small one, printed the way the panel prints the tree of " +
			"whichever organism you have selected."},
		newRulesTreeBlock(),
		rulesText{"The arrows mark the path it took last cycle: no food underneath it, the way " +
			"ahead was clear, so it moved. Those lines are the brightest. Mid-grey is a branch the " +
			"organism has gone down at some point in its life, and the faintest lines are branches " +
			"it has never reached — a tree full of faint lines is carrying a lot of dead weight."},
		rulesText{"Mutation can swap an action, swap a question, grow a new branch or prune one " +
			"away, so trees both grow and shrink along a lineage. Early trees have plenty in them " +
			"that does nothing useful. Nothing selects for a better tree except surviving long " +
			"enough to spawn, and a branch that never pays off is only removed by chance, so " +
			"lineages improve slowly and unevenly."},
	}
}

func viewerBlocks() []rulesBlock {
	return []rulesBlock{
		rulesHeading{"LINEAGES AND THE REPLAY"},
		rulesText{"Every spawn is recorded against its parent, which builds a family tree under " +
			"each of the organisms the world started with. Every run also writes a replay: the " +
			"same seed gives the same world back, so a run can be watched again from any point " +
			"rather than only as it happens."},
		rulesText{"A replay is re-run rather than played back as pictures, which is why the " +
			"viewer can change what it draws while it goes. Scrub the timeline or click a graph " +
			"to jump to a cycle, play forward at a chosen speed, and pause anywhere."},

		rulesHeading{"WHAT THE VIEWER SHOWS"},
		viewerTable(),
		rulesText{"Clicking an organism opens its panel: its traits, the stats those traits work " +
			"out to at its current size and pH, what this cycle cost and paid it, its decision " +
			"tree with the branch it took marked, and its place in its family's tree. The FIND " +
			"MOST buttons pick one out for you — the oldest, the one with the most children, the " +
			"furthest travelled, the most aggressive, the most successful."},
	}
}

// viewerFeatures is what the viewer offers, named the way its own controls
// are so the page can be read with the panel open beside it.
var viewerFeatures = [][2]string{
	{"GRID DISPLAY", "turn the organism, pH, food, buried food and wall layers on and off"},
	{"ORGANISM COLOR", "paint them by ability, age, size, health, tolerance, lineage, family or action"},
	{"GRAPHS", "population by lineage or ability, food, walls, buried food and pH, over the whole run"},
	{"ZOOM", "four sprite sizes, with a minimap when the world runs past the window"},
	{"MENU", "run the same settings again, edit them, read the ones this run used, or save the recording"},
}

func viewerTable() rulesTable {
	rows := make([][2]string, len(viewerFeatures))
	copy(rows, viewerFeatures)
	return rulesTable{rows: rows, nameW: 170}
}
