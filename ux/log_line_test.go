package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

func TestLogLineColumnsStayPut(t *testing.T) {
	var scores [physiology.AbilityCount]float64
	for i := range scores {
		scores[i] = 14.2
	}
	narrow := FormatLogLine(100, 5, 6, 7, 8, 4.66, 2.43, 9.58, scores)
	wide := FormatLogLine(100, 5, 6, 7, 8, 10, 0, 10, scores)
	if len(narrow.Text) != len(wide.Text) {
		t.Errorf("pH 10.00 makes the line %d chars against %d for 4.66:\n%s\n%s",
			len(wide.Text), len(narrow.Text), narrow.Text, wide.Text)
	}

	// The same holds for the whole line, ability columns included.
	if len(narrow.String()) != len(wide.String()) {
		t.Errorf("full lines differ in width:\n%s\n%s", narrow.String(), wide.String())
	}

	small := FormatLogLine(1, 1, 1, 1, 1, 5, 5, 5, scores)
	big := FormatLogLine(999999, 99999, 999999, 999999, 99999, 5, 5, 5, scores)
	if len(small.Text) != len(big.Text) {
		t.Errorf("counts shift the columns:\n%s\n%s", small.Text, big.Text)
	}
}

func TestLogLineFitsTheRunningPopup(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	var scores [physiology.AbilityCount]float64
	for i := range scores {
		scores[i] = 4.25 // widest ability rendering: two digits and a decimal
	}
	widest := FormatLogLine(999999, 99999, 999999, 999999, 99999, 10, 0, 10, scores)

	got := textAdvance(r.FontSourceCodePro10, widest.String())
	usable := popupMaxW - popupPad - 1
	if got > usable {
		t.Errorf("the widest log line is %dpx in %dpx of popup: the ability columns "+
			"fall off the right edge.\n%s", got, usable, widest.String())
	}
}
