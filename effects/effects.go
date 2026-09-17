// Package effects holds every formula that turns an organism's ability
// scores into what its actions actually do: health costs, gains, damage,
// health gained from eating, and terrain changes.
//
// It is the single source for that math. The simulation calls these with
// the live configuration, and the config screen's ability graphs call them
// with the values being edited, so a formula changed here changes both.
// Every function takes the configuration it reads explicitly rather than
// reading global state, and none of them has side effects.
//
// Keep floating-point operation order stable when editing these: replays
// depend on bit-identical results.
package effects

import (
	"math"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// Multiplier is curve id's multiplier at score under g. See
// physiology.CurveFor for the curve's shape.
func Multiplier(g *config.Globals, id physiology.CurveID, score int) float64 {
	return physiology.CurveFor(g, id).At(float64(score))
}

// SizeBracket returns 0, 1 or 2 for small, medium or large organisms:
// thirds of MaximumMaxSize, matching the renderer's size buckets.
func SizeBracket(g *config.Globals, size float64) int {
	maxSize := g.MaximumMaxSize
	switch {
	case size < maxSize*(1.0/3.0):
		return 0
	case size < maxSize*(2.0/3.0):
		return 1
	default:
		return 2
	}
}

// ChemosynthesisGain is the health an organism gains by chemosynthesizing
// in water D from its ideal pH:
//
//	H = size × MaxChemosynthesisGain × (1 − (D/C)²)
//
// C is the organism's Chemosynthesis multiplier, so the ability buys the
// width of the pH band it can feed in rather than the amount it takes.
// The same curve covers success and failure: inside C the organism
// gains, at C it breaks even, and past C it loses health, which is why
// there is no separate failed-chemosynthesis setting. The loss is
// floored at what a perfect attempt would have gained, so water far from
// ideal is ruinous without being instantly fatal.
func ChemosynthesisGain(g *config.Globals, score int, size, distance float64) float64 {
	full := size * g.MaxChemosynthesisGain
	width := ChemoWidth(g, score)
	if width <= 0 {
		if distance == 0 {
			return full
		}
		return -full
	}
	ratio := distance / width
	return full * max(-1, 1-ratio*ratio)
}

// ChemoWidth is C: how far from its ideal pH an organism can chemosynthesize
// before the attempt costs more than it gains, from its Chemosynthesis
// score. Runs 0 to 1 along the Chemosynthesis curve.
func ChemoWidth(g *config.Globals, score int) float64 {
	return Multiplier(g, physiology.CurveChemosynthesis, score)
}

// MaxFoodPerEat is the most food one eating action removes:
// size × MaxFoodPerEatAtFullEating × Eating multiplier. What that food
// is worth to the eater is HealthFromFood.
func MaxFoodPerEat(g *config.Globals, score int, size float64) float64 {
	return size * g.MaxFoodPerEatAtFullEating * Multiplier(g, physiology.CurveEating, score)
}

// HealthFromFood is the health a quantity of food gives whoever ate it.
// Flat per unit: the Eating score buys capacity, not nourishment.
func HealthFromFood(g *config.Globals, food float64) float64 {
	return food * g.HealthPerFoodUnit
}

// costBetween is a cost whose two ends are both configured: atZero at 0
// Movement and atMax at full, with the cost curve's multiplier (1 at 0,
// 0 at full) carrying it between them.
//
// A cost curve on its own runs to nothing, which would make a maxed-out
// ability exempt from the action entirely. Naming both ends instead lets
// the ability buy a discount without ever buying a free action.
func costBetween(atZero, atMax, multiplier float64) float64 {
	return atMax + (atZero-atMax)*multiplier
}

// MoveCost is the health change for moving (or trying to): size-scaled,
// from HealthChangeFromMoving at 0 Movement to HealthChangeFromMovingAtMax
// at full, along the Movement cost curve.
func MoveCost(g *config.Globals, score int, size float64) float64 {
	return size * costBetween(g.HealthChangeFromMoving, g.HealthChangeFromMovingAtMax,
		Multiplier(g, physiology.CurveMovementCost, score))
}

// TurnCost is the health change for turning: size-scaled, from
// HealthChangeFromTurning at 0 Movement to HealthChangeFromTurningAtMax at
// full, along the Movement cost curve.
func TurnCost(g *config.Globals, score int, size float64) float64 {
	return size * costBetween(g.HealthChangeFromTurning, g.HealthChangeFromTurningAtMax,
		Multiplier(g, physiology.CurveMovementCost, score))
}

// DigCost is the health change for digging: size-scaled, from
// HealthChangeFromDigging at 0 Digging to HealthChangeFromDiggingAtMax at
// full, along the Digging cost curve. A better digger pays less, never
// nothing.
func DigCost(g *config.Globals, score int, size float64) float64 {
	return size * costBetween(g.HealthChangeFromDigging, g.HealthChangeFromDiggingAtMax,
		Multiplier(g, physiology.CurveDiggingCost, score))
}

// SizeFoodFromDigging is the food a dig roots up for an organism of this
// size with 100 Digging, by size class.
func SizeFoodFromDigging(g *config.Globals, size float64) int {
	switch SizeBracket(g, size) {
	case 0:
		return g.FoodFromDiggingSmall
	case 1:
		return g.FoodFromDiggingMedium
	default:
		return g.FoodFromDiggingLarge
	}
}

// digStep interpolates a whole-unit dig effect from its value at 0
// Digging to its value at 100 along the Digging strength curve, rounded.
//
// Wall strength and food are both integers, so an effect of theirs has
// to land on a whole unit somewhere. Rounding the interpolation is what
// puts the steps where the user can see them on the graph, rather than
// hiding a fraction behind a dice roll — food used to be rolled for, so
// a small digger scratched four times for nothing and then produced an
// item, with no way to tell the two outcomes apart from the settings.
func digStep(atZero, atFull int, multiplier float64) int {
	return int(math.Round(float64(atZero) + float64(atFull-atZero)*multiplier))
}

// SizeWallCreated is the wall strength a dig raises either side of it
// for an organism of this size at full Digging, by size class.
func SizeWallCreated(g *config.Globals, size float64) int {
	switch SizeBracket(g, size) {
	case 0:
		return g.WallCreatedSmall
	case 1:
		return g.WallCreatedMedium
	default:
		return g.WallCreatedLarge
	}
}

// DigWallCreated is the wall strength one dig packs into each of the two
// cells beside it: a whole-unit step from WallCreatedAtZero to the
// digger's size-class value along the Digging creation curve. Zero means
// this organism shifts terrain without being able to build any.
func DigWallCreated(g *config.Globals, score int, size float64) int {
	return digStep(g.WallCreatedAtZero, SizeWallCreated(g, size),
		Multiplier(g, physiology.CurveDiggingCreate, score))
}

// SizeStrengthDelta is the wall strength a dig clears ahead for an
// organism of this size at full Digging: WallStrengthDeltaSmall, Medium
// or Large by size class.
func SizeStrengthDelta(g *config.Globals, size float64) int {
	switch SizeBracket(g, size) {
	case 0:
		return g.WallStrengthDeltaSmall
	case 1:
		return g.WallStrengthDeltaMedium
	default:
		return g.WallStrengthDeltaLarge
	}
}

// DigWallRemoved is the wall strength one dig clears from the cell
// ahead: a whole-unit step from WallStrengthDeltaAtZero to the size-class
// delta along the Digging removal curve. What the same dig raises beside
// it is DigWallCreated, on its own curve.
func DigWallRemoved(g *config.Globals, score int, size float64) int {
	return digStep(g.WallStrengthDeltaAtZero, SizeStrengthDelta(g, size),
		Multiplier(g, physiology.CurveDiggingStrength, score))
}

// DigFood is the food one dig roots up in front of the digger: a
// whole-unit step from FoodFromDiggingAtZero to the digger's size-class
// value along the Digging strength curve, like DigStrengthDelta.
func DigFood(g *config.Globals, score int, size float64) int {
	return digStep(g.FoodFromDiggingAtZero, SizeFoodFromDigging(g, size),
		Multiplier(g, physiology.CurveDiggingStrength, score))
}

// AttackDamage is the damage an attack deals before the target's
// Defense: AttackDamageAtFullAttack (the damage at 100 Attack) × attacker
// size × Attack multiplier. A positive magnitude — the caller subtracts
// it from the target — so the setting reads as damage rather than as a
// negative health change that only hurts if its sign is right.
func AttackDamage(g *config.Globals, score int, size float64) float64 {
	return g.AttackDamageAtFullAttack * size * Multiplier(g, physiology.CurveAttack, score)
}

// DamageTakenMultiplier scales incoming attack damage by the defender's
// Damage taken curve: 1 (full damage) at 0 Defense down to 0 (immune) at
// 100. Floored at zero, since a negative multiplier would turn damage into
// healing.
func DamageTakenMultiplier(g *config.Globals, defense int) float64 {
	return max(0, Multiplier(g, physiology.CurveDamageTaken, defense))
}

// ThornsDamage is the damage a defender deals back to each organism
// that hits it: ThornsDamageAtFullDefense (the damage at 100 Defense)
// scaled by the defender's size and its Thorns curve. Positive, like
// AttackDamage.
// Independent of the attack's strength — the counter-attack is the
// defender's, so a soft hit is answered as hard as a heavy one.
func ThornsDamage(g *config.Globals, defense int, defenderSize float64) float64 {
	return g.ThornsDamageAtFullDefense * defenderSize * Multiplier(g, physiology.CurveThorns, defense)
}

// PhDamage is the (negative) health change an organism takes each cycle
// for sitting D from its ideal pH:
//
//	H = −size × UnhealthyPhDamage × (D/T)²
//
// The mirror of ChemosynthesisGain: squared, so bad water costs little at
// first and compounds as it gets worse, and divided by the organism's
// Tolerance multiplier T, which widens the bearable band rather than
// capping the cost. Only water at exactly its ideal pH is free.
func PhDamage(g *config.Globals, toleranceScore int, size, distance float64) float64 {
	if distance == 0 {
		return 0
	}
	width := PhToleranceWidth(g, toleranceScore)
	if width <= 0 {
		width = 1
	}
	ratio := distance / width
	return -size * g.UnhealthyPhDamage * ratio * ratio
}

// PhToleranceWidth is T: the pH offset an organism's Tolerance lets it
// bear for one unit of damage. Runs 1 to MaxPhToleranceWidth along the pH
// tolerance curve, so every point widens the bearable band and no score
// escapes damage entirely.
func PhToleranceWidth(g *config.Globals, toleranceScore int) float64 {
	return 1 + (g.MaxPhToleranceWidth-1)*Multiplier(g, physiology.CurvePhTolerance, toleranceScore)
}
