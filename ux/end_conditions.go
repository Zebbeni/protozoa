package ux

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
)

// The end-condition list: what will stop this run, shown in the running
// popup's footer beside the Stop button.
//
// A simulation that can run unattended needs to say what it is waiting
// for. Before this the only way to know whether a run would ever stop on
// its own was to remember what min_organisms was set to — and max_cycles
// did not exist at all, so the honest answer was usually "it won't".
//
// Each condition is dim until it is the one that has fired, which is the
// same list doing two jobs: while the run goes, it is the terms it is
// running under; when it stops, it is why.
type endConditionLine struct {
	label string
	// met is true for the condition that ended the run.
	met bool
}

// endConditionLines describes the conditions in force, in the order they
// are checked. Conditions that are switched off are left out rather than
// shown as "off": the list is what will stop this run, and a line saying
// something won't is noise in a footer this small.
//
// Extinction is always listed, because it is always in force and it is
// the answer a user most wants to see confirmed when a run ends early.
func endConditionLines(g *config.Globals, fired simulation.EndCondition, replayBytes int64) []endConditionLine {
	lines := []endConditionLine{
		{label: "extinction", met: fired == simulation.EndExtinct},
	}
	if g.MinOrganisms > 0 {
		lines = append(lines, endConditionLine{
			label: fmt.Sprintf("min organisms: %d", g.MinOrganisms),
			met:   fired == simulation.EndBelowMinimum,
		})
	}
	if g.MaxCycles > 0 {
		lines = append(lines, endConditionLine{
			label: fmt.Sprintf("max cycles: %d", g.MaxCycles),
			met:   fired == simulation.EndMaxCycles,
		})
	}
	// The replay size is the one condition that carries a running value
	// as well as its limit: it is the number the user is actually
	// watching, and "250MB" alone doesn't say how close the run is.
	if g.MaxReplaySizeMb > 0 {
		label := fmt.Sprintf("replay size: %s / %dMB", formatBytes(replayBytes), g.MaxReplaySizeMb)
		lines = append(lines, endConditionLine{
			label: label,
			met:   fired == simulation.EndMaxReplaySize,
		})
	}
	return lines
}

// formatBytes is a compact size for the footer: whole MB once there is
// a megabyte to show, one decimal below that, so a run that has barely
// started doesn't read as a flat 0MB for its first minute.
func formatBytes(b int64) string {
	const mb = 1 << 20
	if b >= mb {
		return fmt.Sprintf("%dMB", b/mb)
	}
	return fmt.Sprintf("%.1fMB", float64(b)/mb)
}

// endConditionLineH is the step between lines, from the face they are
// drawn in so the block can't drift out of step with the type.
func endConditionLineH() int { return r.FontSourceCodePro8.Metrics().Height.Round() }

// endConditionsHeight is how tall the list is, so a caller can bottom-
// align it against a button of a different height.
func endConditionsHeight(lines []endConditionLine) int {
	return len(lines) * endConditionLineH()
}

// drawEndConditions paints the list with its left edge at x and its
// bottom at bottomY, so it sits on the same baseline as the button
// beside it however many conditions are in force.
func drawEndConditions(dst *ebiten.Image, lines []endConditionLine, x, bottomY int) {
	lineH := endConditionLineH()
	y := bottomY - len(lines)*lineH
	for _, line := range lines {
		y += lineH
		// The met condition is the loudest thing in the footer, because
		// at that moment it is the answer to "why did this stop".
		ink := themedMuted()
		if line.met {
			ink = themedBad()
		}
		text.Draw(dst, line.label, r.FontSourceCodePro8, x, y, ink)
	}
}
