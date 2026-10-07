package ux

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
)

// The end-condition list: what will stop this run, shown in the running popup's footer beside the Stop button.
type endConditionLine struct {
	label string
	// met is true for the condition that ended the run.
	met bool
}

// endConditionLines describes the conditions in force, in the order they are checked.
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
	// The replay size is the one condition that carries a running value as well as its limit.
	if g.MaxReplaySizeMb > 0 {
		label := fmt.Sprintf("replay size: %s / %dMB", formatBytes(replayBytes), g.MaxReplaySizeMb)
		lines = append(lines, endConditionLine{
			label: label,
			met:   fired == simulation.EndMaxReplaySize,
		})
	}
	return lines
}

func formatBytes(b int64) string {
	const mb = 1 << 20
	if b >= mb {
		return fmt.Sprintf("%dMB", b/mb)
	}
	return fmt.Sprintf("%.1fMB", float64(b)/mb)
}

// endConditionLineH is the step between lines, from the face they are drawn in so the block can't drift out of step with the type.
func endConditionLineH() int { return r.FontSourceCodePro8.Metrics().Height.Round() }

func endConditionsHeight(lines []endConditionLine) int {
	return len(lines) * endConditionLineH()
}

// drawEndConditions paints the list with its left edge at x and its bottom at bottomY.
func drawEndConditions(dst *ebiten.Image, lines []endConditionLine, x, bottomY int) {
	lineH := endConditionLineH()
	y := bottomY - len(lines)*lineH
	for _, line := range lines {
		y += lineH
		// The met condition is the loudest thing in the footer.
		ink := themedMuted()
		if line.met {
			ink = themedBad()
		}
		text.Draw(dst, line.label, r.FontSourceCodePro8, x, y, ink)
	}
}
