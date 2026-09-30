package ux

import (
	"math"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
	"github.com/Zebbeni/protozoa/utils"
)

// orgTint is everything the organism colour modes read, captured so the
// colour of an organism can be worked out away from the Grid.
//
// The minimap renders on a background goroutine, so it cannot reach into
// Grid for the ability being coloured by, the oldest organism alive or the
// family memo. It takes one of these on the main goroutine instead and the
// two views then paint from one function, which is what stops them drifting
// apart as modes are added.
type orgTint struct {
	mode        mode
	ability     physiology.Ability
	oldestAlive int
	cycle       int
	recordedEnd int
	globals     *config.Globals
	phAt        func(utils.Point) float64
	// family is this tint's OWN tinter, never the Grid's: its memo is
	// written as organisms are coloured, and two goroutines sharing one
	// would race.
	family *familyTinter
	lookup func(int) *organism.DescendantNode
}

func (g *Grid) organismTint() orgTint {
	t := orgTint{
		mode:        g.orgColor,
		ability:     g.colorAbility,
		oldestAlive: g.oldestAlive,
		cycle:       g.simulation.Cycle(),
		recordedEnd: g.simulation.RecordedEndCycle(),
		globals:     config.GetCurrentGlobals(),
		phAt:        g.simulation.GetPhAtPoint,
		lookup:      g.simulation.GetTreeNodeByID,
	}
	if g.orgColor == orgColorFamily {
		if selID := g.simulation.GetSelected(); selID >= 0 {
			if sel := g.simulation.GetTreeNodeByID(selID); sel != nil {
				t.family = newFamilyTinter(sel, selID, g.simulation.TreesGeneration())
			}
		}
	}
	return t
}

// bodyColor is the colour to paint info. The second result is false for the
// mode that keeps an organism's own identity, where the caller should leave
// its overlays on their secondary colour rather than flattening them.
func (t orgTint) bodyColor(info *organism.Info) (colorful.Color, bool) {
	switch t.mode {
	case orgColorPhEffect:
		return phEffectColor(info.PhPositive, info.PhNegative), true
	case orgColorHealth:
		return healthColor(info.Health, info.Size), true
	case orgColorAbility:
		return gh.AbilityColor(info.Abilities, t.ability), true
	case orgColorTolerance:
		distance := math.Abs(info.IdealPh - t.phAt(info.Location))
		damage := effects.PhDamage(t.globals, info.Abilities[physiology.AbilityTolerance], info.Size, distance)
		return phToleranceColor(damage), true
	case orgColorSuccess:
		return gh.GrayGreenColor(organism.LineageSuccess(info.LineageEndCycle, t.cycle, t.recordedEnd)), true
	case orgColorFamily:
		return t.familyColor(info.ID), true
	case orgColorAge:
		return gh.GrayGreenColor(ageFraction(info.Age, t.oldestAlive)), true
	case orgColorSize:
		return gh.GrayGreenColor(sizeFraction(info.Size)), true
	}
	return info.Color, false
}

func (t orgTint) familyColor(id int) colorful.Color {
	if t.family == nil {
		return familyUnrelatedColor()
	}
	if k, ok := t.family.cached(id); ok {
		return familyColor(k)
	}
	return familyColor(t.family.kinshipOf(t.lookup(id)))
}
