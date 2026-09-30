// Package effects holds every formula that turns an organism's ability scores into what its actions actually do.
package effects

import (
	"math"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

func Multiplier(g *config.Globals, id physiology.CurveID, score int) float64 {
	return physiology.CurveFor(g, id).At(float64(score))
}

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

// ChemosynthesisGain is the health an organism gains by chemosynthesizing in water D from its ideal pH.
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

// ChemoWidth is C: how far from its ideal pH an organism can chemosynthesize before the attempt costs more than it gains, from its Chemosynthesis score.
func ChemoWidth(g *config.Globals, score int) float64 {
	return g.MaxChemosynthesisPhWidth * Multiplier(g, physiology.CurveChemosynthesis, score)
}

func MaxFoodPerEat(g *config.Globals, score int, size float64) float64 {
	// Both ends scaled by size BEFORE interpolating, not after.
	return effectBetween(size*g.MaxFoodPerEatAtZeroEating, size*g.MaxFoodPerEatAtFullEating,
		Multiplier(g, physiology.CurveEating, score))
}

func HealthFromFood(g *config.Globals, food float64) float64 {
	return food * g.HealthPerFoodUnit
}

func effectBetween(atZero, atMax, multiplier float64) float64 {
	return atZero + (atMax-atZero)*multiplier
}

func costBetween(atZero, atMax, multiplier float64) float64 {
	return atMax + (atZero-atMax)*multiplier
}

func MoveCost(g *config.Globals, score int, size float64) float64 {
	return size * costBetween(g.HealthChangeFromMoving, g.HealthChangeFromMovingAtMax,
		Multiplier(g, physiology.CurveMovementCost, score))
}

func TurnCost(g *config.Globals, score int, size float64) float64 {
	return size * costBetween(g.HealthChangeFromTurning, g.HealthChangeFromTurningAtMax,
		Multiplier(g, physiology.CurveMovementCost, score))
}

func DigCost(g *config.Globals, score int, size float64) float64 {
	return size * costBetween(g.HealthChangeFromDigging, g.HealthChangeFromDiggingAtMax,
		Multiplier(g, physiology.CurveDiggingCost, score))
}

func EatCost(g *config.Globals, score int, size float64) float64 {
	return size * costBetween(g.HealthChangeFromEatingAttempt, g.HealthChangeFromEatingAttemptAtMax,
		Multiplier(g, physiology.CurveEatingCost, score))
}

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

// digStep interpolates a whole-unit dig effect from its value at 0 Digging to its value at 100 along the Digging strength curve, rounded.
func digStep(atZero, atFull int, multiplier float64) int {
	return int(math.Round(float64(atZero) + float64(atFull-atZero)*multiplier))
}

// SizeWallCreated is the wall strength a dig raises either side of it for an organism of this size at full Digging, by size class.
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

func DigWallCreated(g *config.Globals, score int, size float64) int {
	return digStep(g.WallCreatedAtZero, SizeWallCreated(g, size),
		Multiplier(g, physiology.CurveDiggingCreate, score))
}

// DigFood is how much buried food one dig can bring back up under the digger.
func DigFood(g *config.Globals, score int, size float64) int {
	return digStep(g.FoodFromDiggingAtZero, SizeFoodFromDigging(g, size),
		Multiplier(g, physiology.CurveDiggingStrength, score))
}

// AttackDamage is the damage an attack deals before the target's Defense.
func AttackDamage(g *config.Globals, score int, size float64) float64 {
	return effectBetween(g.AttackDamageAtZeroAttack*size, g.AttackDamageAtFullAttack*size,
		Multiplier(g, physiology.CurveAttack, score))
}

// DamageTakenMultiplier scales incoming attack damage by the defender's Damage taken curve.
func DamageTakenMultiplier(g *config.Globals, defense int) float64 {
	return max(0, Multiplier(g, physiology.CurveDamageTaken, defense))
}

// ThornsDamage is the damage a defender deals back to each organism that hits it.
func ThornsDamage(g *config.Globals, defense int, defenderSize float64) float64 {
	return effectBetween(g.ThornsDamageAtZeroDefense*defenderSize, g.ThornsDamageAtFullDefense*defenderSize,
		Multiplier(g, physiology.CurveThorns, defense))
}

// WallBreakStrength is the wall strength an organism can shoulder straight through, destroying the wall and taking its cell in one move.
func WallBreakStrength(g *config.Globals, score int, size float64) float64 {
	// Linear in the raw score between the two configured ends, scaled by the multiplier.
	p := float64(score) / float64(physiology.MaxAbilityScore)
	return size * effectBetween(g.WallBreakAtZeroDigging, g.WallBreakAtMaxDigging, p) *
		g.WallBreakMultiplier
}

// BurrowSpoilPerSide is the wall strength a burrow piles onto EACH of the two cells flanking the tunnel, out of the strength of the wall it just destroyed.
func BurrowSpoilPerSide(g *config.Globals, strength int) int {
	if strength <= 0 || g.BurrowSpoilFraction <= 0 {
		return 0
	}
	return int(math.Round(float64(strength) * g.BurrowSpoilFraction / 2))
}

// CanBreakWall reports whether an organism of this size and Digging score gets through a wall of the given strength.
func CanBreakWall(g *config.Globals, score int, size float64, wallStrength int) bool {
	return WallBreakStrength(g, score, size) > float64(wallStrength)
}

// MaxBreakableWall is the strongest wall an organism of this size and Digging score gets through.
func MaxBreakableWall(g *config.Globals, score int, size float64) int {
	strongest := int(math.Ceil(WallBreakStrength(g, score, size))) - 1
	return max(0, strongest)
}

// ChemoPhPush is how far down an organism drives the pH around it for a chemosynthesis attempt that gained it `gain` health.
func ChemoPhPush(g *config.Globals, score int, gain float64) float64 {
	return gain * g.ChemoPhEffect * Multiplier(g, physiology.CurveChemoPhEffect, score)
}

func EatingPhPush(g *config.Globals, score int, gain float64) float64 {
	return gain * g.EatingPhEffect * Multiplier(g, physiology.CurveEatingPhEffect, score)
}

// PhDamage is the (negative) health change an organism takes each cycle for sitting D from its ideal pH.
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

// PhToleranceWidth is T: the pH offset an organism's Tolerance lets it bear for one unit of damage.
func PhToleranceWidth(g *config.Globals, toleranceScore int) float64 {
	return 1 + (g.MaxPhToleranceWidth-1)*Multiplier(g, physiology.CurvePhTolerance, toleranceScore)
}
