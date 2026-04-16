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
}

// ColorFromWeights derives a deterministic color from an action weight distribution.
// Hue = weighted circular average of action hues.
// Saturation = how concentrated the distribution is (one dominant action = high sat).
// Lightness = fixed range for visual clarity.
func ColorFromWeights(weights map[Action]float64) colorful.Color {
	// Weighted circular average for hue
	var sinSum, cosSum float64
	for action, weight := range weights {
		hue, ok := actionHues[action]
		if !ok {
			continue
		}
		rad := hue * math.Pi / 180
		sinSum += weight * math.Sin(rad)
		cosSum += weight * math.Cos(rad)
	}
	avgHue := math.Atan2(sinSum, cosSum) * 180 / math.Pi
	if avgHue < 0 {
		avgHue += 360
	}

	// Concentration: how dominant the top action is.
	// maxWeight=1.0 → fully concentrated, even spread → low concentration.
	maxWeight := 0.0
	for _, w := range weights {
		if w > maxWeight {
			maxWeight = w
		}
	}
	// Map concentration to saturation: [0.3, 1.0]
	sat := 0.5 + 0.5*maxWeight

	// Fixed lightness in a visible range
	light := 0.5

	return colorful.HSLuv(avgHue, sat, light)
}

// TreeColor returns a deterministic color derived from a decision tree's
// action weight distribution.
func TreeColor(t *Tree) colorful.Color {
	return ColorFromWeights(t.ActionWeights())
}
