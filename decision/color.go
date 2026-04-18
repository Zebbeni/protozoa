package decision

import (
	"math"

	"github.com/lucasb-eyer/go-colorful"
)

// Fixed hue assignments for each action, spread around the color wheel.
var actionHues = map[Action]float64{
	ActChemosynthesis: 120, // green
	ActMove:           200, // blue
	ActEat:            40,  // orange
	ActAttack:         0,   // red
	ActTurnLeft:       270, // purple
	ActTurnRight:      300, // pink
	ActSpawn:          160, // teal
	ActIdle:           90,  // yellow-green (unused gap)
}

// Fixed hue assignments for conditions. Spread so that organisms checking
// different environmental cues get visually distinct colors.
var conditionHues = map[Condition]float64{
	CanMove:                   180, // cyan
	IsFoodAhead:               60,  // yellow
	IsFoodLeft:                50,  // gold
	IsFoodRight:               70,  // yellow-green
	IsOrganismAhead:           0,   // red
	IsBiggerOrganismAhead:     10,  // red-orange
	IsRelatedOrganismAhead:    20,  // orange
	IsOrganismLeft:            340, // rose
	IsRelatedOrganismLeft:     350, // pink-red
	IsOrganismRight:           330, // magenta
	IsRelatedOrganismRight:    320, // purple-pink
	IsHealthAboveFiftyPercent: 140, // green-teal
	IsHealthyPhHere:           100, // lime
	IsHealthierPhAhead:        110, // green-lime
	IsAgeMultipleOfTwo:        220, // blue
	IsAgeMultipleOfTen:        240, // indigo
}

// circularWeightedHue computes the circular weighted average of hues.
func circularWeightedHue(hueMap map[float64]float64) float64 {
	var sinSum, cosSum float64
	for hue, weight := range hueMap {
		rad := hue * math.Pi / 180
		sinSum += weight * math.Sin(rad)
		cosSum += weight * math.Cos(rad)
	}
	avg := math.Atan2(sinSum, cosSum) * 180 / math.Pi
	if avg < 0 {
		avg += 360
	}
	return avg
}

// TreeColor derives a deterministic color from a decision tree.
// Hue is a blend of action hues and condition hues (conditions weighted more
// since they're more distinctive between similar trees).
// Saturation reflects how concentrated the action distribution is.
// Lightness is fixed for visual clarity.
func TreeColor(t *Tree) colorful.Color {
	actionWeights := t.ActionWeights()
	conditionWeights := t.ConditionWeights()

	// Compute action hue component
	actionHueInputs := make(map[float64]float64)
	for action, weight := range actionWeights {
		if hue, ok := actionHues[action]; ok {
			actionHueInputs[hue] += weight
		}
	}
	actionHue := circularWeightedHue(actionHueInputs)

	// Compute condition hue component
	conditionHueInputs := make(map[float64]float64)
	totalCondWeight := 0.0
	for cond, weight := range conditionWeights {
		if hue, ok := conditionHues[cond]; ok {
			conditionHueInputs[hue] += weight
			totalCondWeight += weight
		}
	}
	conditionHue := circularWeightedHue(conditionHueInputs)

	// Blend: conditions contribute more to hue distinction.
	// If tree has no conditions (single action node), use action hue only.
	var finalHue float64
	if totalCondWeight > 0 {
		// Blend 30% action hue + 70% condition hue via circular average
		blendInputs := map[float64]float64{
			actionHue:    0.3,
			conditionHue: 0.7,
		}
		finalHue = circularWeightedHue(blendInputs)
	} else {
		finalHue = actionHue
	}

	// Saturation from action concentration
	maxWeight := 0.0
	for _, w := range actionWeights {
		if w > maxWeight {
			maxWeight = w
		}
	}
	sat := 0.5 + 0.5*maxWeight

	light := 0.5

	return colorful.HSLuv(finalHue, sat, light)
}
