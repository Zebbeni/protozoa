package ux

import (
	"math"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
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
	action      d.Action
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
		action:      g.colorAction,
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
	case orgColorAction:
		return actionColor(info, t.action), true
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

// actionWeightCeiling is the share of a tree at which the gray-to-green ramp
// is full. A tree is rarely more than about a third one action, so running
// the ramp to 1 would spend most of the colour on shares nothing reaches.
const actionWeightCeiling = 0.34

// actionBrightest is the colour for an organism taking the action right now.
// Past the end of the weight ramp, so "doing it" always reads brighter than
// any amount of "built for it".
func actionBrightest() colorful.Color { return gh.GrayGreenColor(1) }

// actionColor is brightest for an organism performing the action this cycle,
// and gray-to-mid green by that action's share of its tree otherwise.
//
// The two answer different questions — what it is doing now, and what it is
// built to do — and the mode shows both at once because an action a tree is
// full of but never takes is the interesting case.
func actionColor(info *organism.Info, action d.Action) colorful.Color {
	if info.Action == action {
		return actionBrightest()
	}
	return gh.GrayGreenColor(actionWeightFraction(info, action) * midActionGreen)
}

// midActionGreen caps the weight ramp below the brightest, so the two halves
// of the mode stay apart at a glance.
const midActionGreen = 0.7

// actionWeightFraction is the action's share of the tree against the ceiling,
// clamped. An organism restored from another configuration can carry weights
// for actions this build does not have.
func actionWeightFraction(info *organism.Info, action d.Action) float64 {
	idx := int(action)
	if idx < 0 || idx >= len(info.ActionWeights) {
		return 0
	}
	f := info.ActionWeights[idx] / actionWeightCeiling
	return math.Max(0, math.Min(1, f))
}
