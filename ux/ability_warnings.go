package ux

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

// The config screen's check on the two abilities that FEED an organism.
//
// An action that returns less health than it costs is not a weak strategy, it
// is a trap: a lineage drifting into it loses health every time it does the
// thing it is built for, and nothing in the settings screen says so. The
// shipped defaults reached exactly that state — a SUCCESSFUL eat at Eating 0
// or 1 ended the cycle with less health than it started, so eating died out
// of every run and the only force that pushes pH back up went with it.
//
// Each check is the BEST case for the action: a cell holding more food than
// the bite can take, and water at exactly the organism's ideal pH. A worse
// case can only be worse, so a warning here is a statement about every case.
// The figures come from effects, like the ability graphs and the STATS tab —
// a second copy of these formulas in the UI would read correctly the day it
// was written and drift on the first change to the real one.

// warnSize is the size the checks are run at. Every quantity in them scales
// with size, so which one is used decides the figures quoted and not whether
// a warning fires; 1 states them per unit of size, which is the unit the
// settings themselves are in.
const warnSize = 1.0

// abilityWarnings is what is wrong with this ability's settings, in the order
// it should be read. Empty is the ordinary case.
func abilityWarnings(g *c.Globals, a physiology.Ability) []string {
	switch a {
	case physiology.AbilityEating:
		return eatingWarnings(g)
	case physiology.AbilityChemosynthesis:
		return chemoWarnings(g)
	}
	return nil
}

// eatingWarnings: a successful eat has to beat its own attempt cost at every
// score, or the first points a lineage spends on Eating lose it health.
func eatingWarnings(g *c.Globals) []string {
	net := func(score int) float64 {
		// The best case is a cell with more food in it than the bite can
		// take, so what is left is the bite against the attempt.
		return effects.HealthFromFood(g, score, effects.MaxFoodPerEat(g, score, warnSize)) +
			effects.EatCost(g, score, warnSize)
	}
	if net(0) >= 0 {
		return nil
	}
	highest := 0
	for score := 1; score <= physiology.MaxAbilityScore; score++ {
		if net(score) < 0 {
			highest = score
		}
	}
	span := fmt.Sprintf("Eating %d", highest)
	if highest > 0 {
		span = fmt.Sprintf("Eating 0-%d", highest)
	}
	return []string{fmt.Sprintf(
		"%s: a SUCCESSFUL eat loses %.3f health per unit of size", span, -net(0))}
}

// chemoWarnings: the gain at the organism's own ideal pH is the best case
// chemosynthesis has, and the band is what it has to find that pH in.
func chemoWarnings(g *c.Globals) []string {
	var out []string
	if gain := effects.ChemosynthesisGain(g, 0, warnSize, 0); gain <= 0 {
		out = append(out, fmt.Sprintf(
			"Chemosynthesis 0: gains %.3f health per unit of size at its own ideal pH", gain))
	}
	// A band of zero width at the TOP of the range is a different fault from
	// one at the bottom: no score opens a band at all, so every attempt but
	// an exact pH match costs the full gain, for every organism in the world.
	if effects.ChemoWidth(g, physiology.MaxAbilityScore) <= 0 {
		out = append(out, fmt.Sprintf(
			"No Chemosynthesis score opens a feeding band: any pH but the exact ideal costs %.3f",
			effects.ChemosynthesisGain(g, 0, warnSize, 1)))
	}
	return out
}

// warnFace is the face the warnings are set in, named so what is measured
// and what is drawn cannot drift.
func warnFace() font.Face { return r.FontSourceCodePro8 }

// drawAbilityWarningRow writes the warnings under their ability's header, in
// the theme's bad ink — the same red a failed check uses everywhere else on
// the screen.
func (cs *ConfigScreen) drawAbilityWarningRow(screen *ebiten.Image, px, py int, field configField) {
	face := warnFace()
	for i, line := range abilityWarnings(cs.globals, field.curveAbility) {
		text.Draw(screen, line, face, px+graphToggleW+2, py+i*cfgWarnLineH+9, themedBad())
	}
}
