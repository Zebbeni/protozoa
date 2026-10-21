package ux

import (
	"os"
	"sort"
	"testing"

	d "github.com/Zebbeni/protozoa/decision"
	r "github.com/Zebbeni/protozoa/resources"
)

// TestLabelProbe prints what the brain labels measure in the face they are
// drawn in, and what the columns leave for the diagram between them, so a
// label can be lengthened against the font rather than against a guess.
// Asserts nothing — TestBrainLabelsFitTheirColumns is what fails.
//
// LABEL_PROBE=1 go test ./ux/ -run TestLabelProbe -v
func TestLabelProbe(t *testing.T) {
	if os.Getenv("LABEL_PROBE") == "" {
		t.Skip("set LABEL_PROBE=1 to measure the brain labels")
	}
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	face := r.FontSourceCodePro8
	t.Logf("8pt: one character %dpx, line height %dpx",
		textAdvance(face, "M"), face.Metrics().Height.Round())

	widest := func(labels []string, column int) {
		sort.Slice(labels, func(i, j int) bool {
			return textAdvance(face, labels[i]) > textAdvance(face, labels[j])
		})
		for _, label := range labels[:min(5, len(labels))] {
			over := ""
			if textAdvance(face, label) > column {
				over = "  OVER"
			}
			t.Logf("  %2d chars %3dpx of %dpx  %q%s",
				len(label), textAdvance(face, label), column, label, over)
		}
	}

	var conditions []string
	for _, label := range brainConditionLabels {
		conditions = append(conditions, label)
	}
	t.Log("longest condition labels:")
	widest(conditions, brainInputLabelW)

	var actions []string
	for _, label := range brainActionLabels {
		actions = append(actions, label)
	}
	t.Log("longest action labels:")
	widest(actions, brainOutputLabelW)

	t.Logf("value column %dpx holds %q at %dpx",
		brainValueW, brainValueSample, textAdvance(face, brainValueSample))
	t.Logf("columns %d + %d + %d and %d gaps leave %dpx of diagram in the panel's %dpx",
		brainInputLabelW, brainOutputLabelW, brainValueW, 3*brainLabelGap,
		brainDiagramW(infoAreaWidth), infoAreaWidth)

	// What the full names would need, which is why there are short forms.
	full := 0
	for _, c := range d.MutableConditions {
		full = max(full, textAdvance(face, d.Map[c]))
	}
	t.Logf("the longest full condition name is %dpx, against the %dpx column", full, brainInputLabelW)

	// How wide a layer can be before its weights stop fitting between the
	// columns, which is what decides whether a net is labelled at all.
	//
	// Measured against brainEdgeFace() rather than each layout's own face,
	// because that is the one the drawing uses whatever the node labels are
	// set in — measuring the layout's face reported a density the diagram
	// does not have.
	numbers := brainEdgeFace()
	slot := brainEdgeLabelW(numbers)
	t.Logf("numbers: %dpx a line, %dpx slot (%q)",
		numbers.Metrics().Height.Round(), slot, brainEdgeSample)
	for _, where := range []struct {
		name      string
		rowH, gap int
	}{
		{"panel, 9-row net", 25, 25},
		{"popup, 4-row net", 239, 710},
		{"popup, 9-row net", 53, 290},
	} {
		widestFit := 0
		for n := 1; n <= 16; n++ {
			if brainEdgeLabelsFit(n, where.rowH, where.gap, slot, numbers) {
				widestFit = n
			}
		}
		t.Logf("%s: %dpx pitch, %dpx between layers -> labels up to a %d-node layer",
			where.name, where.rowH, where.gap, widestFit)
	}
}
