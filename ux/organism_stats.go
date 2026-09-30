package ux

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

type statRow struct {
	label      string
	value      string
	valueColor color.Color
	swatches   []colorful.Color
}

// traitRows is the TRAITS tab's text column.
func traitRows(traits organism.Traits) []statRow {
	return []statRow{
		{label: "IDEAL PH:", value: fmt.Sprintf("%.1f", traits.IdealPh)},
		{label: "MAX SIZE:", value: fmt.Sprintf("%.1f", traits.MaxSize)},
		{label: "SPAWN HP:", value: fmt.Sprintf("%.2f", traits.MinHealthToSpawn)},
		{label: "CHILD HP:", value: fmt.Sprintf("%.2f", traits.SpawnHealth)},
		{label: "SPAWN GAP:", value: fmt.Sprintf("%d", traits.MinCyclesBetweenSpawns)},
		{label: "COLORS:", swatches: []colorful.Color{traits.OrganismColor, traits.SecondaryColor}},
	}
}

func statColumns(g *config.Globals, info *organism.Info, traits organism.Traits, phHere float64) [2][]statRow {
	scores := traits.Abilities
	size := info.Size

	// Distance from ideal drives both pH effects.
	distance := math.Abs(traits.IdealPh - phHere)
	band := effects.PhToleranceWidth(g, scores[physiology.AbilityTolerance])
	phDamage := effects.PhDamage(g, scores[physiology.AbilityTolerance], size, distance)
	chemo := effects.ChemosynthesisGain(g, scores[physiology.AbilityChemosynthesis], size, distance)

	maxEat := effects.MaxFoodPerEat(g, scores[physiology.AbilityEating], size)
	digScore := scores[physiology.AbilityDigging]

	left := []statRow{
		{label: "AGE:", value: fmt.Sprintf("%d", info.Age)},
		{label: "SIZE:", value: fmt.Sprintf("%.1f", size)},
		{label: "CHILDREN:", value: fmt.Sprintf("%d", info.Children)},
		{label: "TRAVELED:", value: fmt.Sprintf("%d", info.TraveledDist)},
		{label: "ATTACKS:", value: fmt.Sprintf("%d", info.AttackTotal)},
		{label: "HITS:", value: fmt.Sprintf("%d", info.AttackHits)},
		{
			label:      "PH EFF:",
			value:      signedValue("%+.2f", info.PhPositive-info.PhNegative),
			valueColor: phEffectTextColor(info.PhPositive, info.PhNegative),
		},
		{label: "PH DIST:", value: fmt.Sprintf("%.2f", distance)},
		{label: "PH BAND:", value: fmt.Sprintf("%.2f", band)},
		{
			label: "PH DMG:",
			value: signedValue("%+.3f", phDamage),
			// The same ramp the grid's TOLERANCE view paints, so a red row here and a red organism out there mean one thing.
			valueColor: phToleranceColor(phDamage),
		},
		// Chemosynthesis closes the pH group rather than opening the action column.
		{label: "CHEMO HP:", value: signedValue("%+.2f", chemo), valueColor: gainColor(chemo)},
	}

	right := []statRow{
		// Two numbers, because eating is two quantities.
		{label: "FOOD/EAT:", value: fmt.Sprintf("%.1f", maxEat)},
		{label: "HP/EAT:", value: signedValue("%+.2f", effects.HealthFromFood(g, maxEat))},
		// Both are damage this organism deals, already multiplied by its own size.
		{label: "ATK DMG:", value: fmt.Sprintf("%.2f", effects.AttackDamage(g, scores[physiology.AbilityAttack], size))},
		{label: "DEF DMG:", value: fmt.Sprintf("%.2f", effects.ThornsDamage(g, scores[physiology.AbilityDefense], size))},
		// A share of incoming damage rather than a multiplier.
		{label: "DMG TAKEN:", value: fmt.Sprintf("%.0f%%", 100*effects.DamageTakenMultiplier(g, scores[physiology.AbilityDefense]))},
		{label: "MOVE HP:", value: signedValue("%+.2f", effects.MoveCost(g, scores[physiology.AbilityMovement], size))},
		{label: "TURN HP:", value: signedValue("%+.2f", effects.TurnCost(g, scores[physiology.AbilityMovement], size))},
		{label: "DIG HP:", value: signedValue("%+.2f", effects.DigCost(g, scores[physiology.AbilityDigging], size))},
		// What this organism does to terrain, on the 1-100 scale a wall's own strength is read on.
		{label: "WALL THRU:", value: fmt.Sprintf("%d", min(manager.MaxWallStrength,
			effects.MaxBreakableWall(g, scores[physiology.AbilityDigging], size)))},
		{label: "WALL MADE:", value: fmt.Sprintf("%d", effects.DigWallCreated(g, digScore, size))},
		// The food a dig roots up when there is no wall ahead to wear down.
		{label: "DIG FOOD:", value: fmt.Sprintf("%d", effects.DigFood(g, digScore, size))},
	}

	return [2][]statRow{left, right}
}

// signedValue formats a signed figure, but never as a negative zero.
func signedValue(format string, v float64) string {
	s := fmt.Sprintf(format, v)
	if strings.Trim(s, "+-0.") == "" {
		return fmt.Sprintf(format, 0.0)
	}
	return s
}

// gainColor tints an action's yield by whether it is one.
func gainColor(gain float64) color.Color {
	if gain < 0 {
		return themedBad()
	}
	return themedOK()
}

func statRowsHigh(cols [2][]statRow) int {
	return max(len(cols[0]), len(cols[1]))
}
