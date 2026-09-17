package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/physiology"
)

// TestLogLineColumnsStayPut: every field is fixed-width, so a pH reading
// of 10.00 doesn't shove the ability columns along and make the log
// unreadable down the page.
func TestLogLineColumnsStayPut(t *testing.T) {
	var scores [physiology.AbilityCount]float64
	for i := range scores {
		scores[i] = 14.2
	}
	narrow := FormatLogLine(100, 5, 6, 7, 4.66, 2.43, 9.58, scores)
	wide := FormatLogLine(100, 5, 6, 7, 10, 0, 10, scores)
	if len(narrow.Text) != len(wide.Text) {
		t.Errorf("pH 10.00 makes the line %d chars against %d for 4.66:\n%s\n%s",
			len(wide.Text), len(narrow.Text), narrow.Text, wide.Text)
	}

	// The same holds for the whole line, ability columns included.
	if len(narrow.String()) != len(wide.String()) {
		t.Errorf("full lines differ in width:\n%s\n%s", narrow.String(), wide.String())
	}

	// And for the counts either side of a jump in magnitude.
	small := FormatLogLine(1, 1, 1, 1, 5, 5, 5, scores)
	big := FormatLogLine(999999, 99999, 999999, 99999, 5, 5, 5, scores)
	if len(small.Text) != len(big.Text) {
		t.Errorf("counts shift the columns:\n%s\n%s", small.Text, big.Text)
	}
}
