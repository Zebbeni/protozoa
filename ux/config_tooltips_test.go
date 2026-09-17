package ux

import (
	"strings"
	"testing"
)

// TestEveryConfigFieldHasTooltip: each row on the config screen explains
// itself, and no tooltip is left behind for a field that was removed.
func TestEveryConfigFieldHasTooltip(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	used := map[string]bool{}
	for _, section := range cs.sections {
		for _, field := range section.fields {
			if field.row == rowCurveGraph {
				continue // graph blocks explain themselves
			}
			if tooltipFor(field) == "" {
				t.Errorf("%s: %q has no tooltip", section.title, field.label)
			}
			used[field.jsonTag] = true
		}
	}
	for tag := range configTooltips {
		if !used[tag] {
			t.Errorf("tooltip for %q, which isn't on the config screen", tag)
		}
	}
}

func TestWrapText(t *testing.T) {
	width := func(s string) int { return len(s) }
	lines := wrapText("the quick brown fox jumps over", 10, width)
	for _, l := range lines {
		if len(l) > 10 {
			t.Errorf("line %q is wider than 10", l)
		}
	}
	if got := strings.Join(lines, " "); got != "the quick brown fox jumps over" {
		t.Errorf("wrapping lost or reordered words: %q", got)
	}
	if lines := wrapText("supercalifragilistic word", 10, width); lines[0] != "supercalifragilistic" {
		t.Errorf("an over-long word should get its own line, got %q", lines)
	}
}
