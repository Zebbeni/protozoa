package ux

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

// TestLedgerLinesFitThePortraitColumn measures against the biggest amount the
// simulation can actually produce — a full-Attack hit on an organism at
// config.MaximumMaxSize — rather than an invented large number, which would
// demand headroom for values nothing can reach and buy it with a smaller
// font. The column is the portrait's own width: a line wider than that runs
// off the panel.
func TestLedgerLinesFitThePortraitColumn(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	g := config.GetCurrentGlobals()
	size := config.MaximumMaxSize()
	biggest := effects.AttackDamage(g, physiology.MaxAbilityScore, size)
	// The widest line is the widest amount beside the widest label.
	widestLabel := ""
	for _, src := range organism.AllHealthSources {
		if len(src.Label()) > len(widestLabel) {
			widestLabel = src.Label()
		}
	}
	// The net can be wider than any single line, being their sum.
	pad := len(formatLedgerAmount(-biggest * float64(len(organism.AllHealthSources))))
	line := ledgerLine(-biggest, widestLabel, pad)
	if w := boundString(r.FontSourceCodePro8, line).Dx(); w > portraitSize {
		t.Errorf("the widest ledger line %q is %dpx, the portrait column is %d", line, w, portraitSize)
	}
}

// TestLedgerNeverDrawsBelowWhatItReserves: the reserved height is what the
// info block measures from, so a ledger that paints past it writes over the
// DECISION / DESCENDANT tabs below.
func TestLedgerNeverDrawsBelowWhatItReserves(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	for rows := 0; rows <= len(organism.AllHealthSources); rows++ {
		if got, reserved := ledgerDrawnHeight(rows), healthLedgerHeight(); got > reserved {
			t.Errorf("%d rows paint %dpx into %dpx of reserved space", rows, got, reserved)
		}
	}
}

// TestOnlyTheSourcesThatMovedAreListed: a column of zeroes for everything
// that did not happen buries the two or three lines that did.
func TestOnlyTheSourcesThatMovedAreListed(t *testing.T) {
	var l organism.HealthLedger
	l.Recorded = true
	l.Amounts[organism.HealthFromPh] = -0.12
	l.Amounts[organism.HealthFromAction] = -0.11
	l.Amounts[organism.HealthFromChemo] = 4.61

	rows := healthLedgerRows(l)
	if len(rows) != 3 {
		t.Fatalf("listed %d rows for three sources: %v", len(rows), rows)
	}
	// AllHealthSources order, not map order, so the lines don't reshuffle
	// between frames.
	want := []string{"ph", "act", "chemo"}
	for i, row := range rows {
		if row.label != want[i] {
			t.Errorf("row %d is %q, want %q", i, row.label, want[i])
		}
	}
	if got := l.Total(); fmt.Sprintf("%+.2f", got) != "+4.38" {
		t.Errorf("net %+.2f, want +4.38", got)
	}
}

// TestAmountsAreRightAligned: the panel's type is monospaced, so padding to
// a common width is all it takes to flush the amounts against their labels.
// Decimal points line up only among rows in the same magnitude band, which
// is what mixed precision costs and is the ordinary way numbers are set.
func TestAmountsAreRightAligned(t *testing.T) {
	var l organism.HealthLedger
	l.Recorded = true
	l.Amounts[organism.HealthFromAttack] = -1204.5
	l.Amounts[organism.HealthFromChemo] = 4.5

	rows := healthLedgerRows(l)
	pad := ledgerAmountWidth(rows, l.Total())
	var lines []string
	for _, row := range rows {
		lines = append(lines, ledgerLine(row.amount, row.label, pad))
	}
	lines = append(lines, ledgerLine(l.Total(), "net", pad))
	for _, line := range lines {
		if at := strings.Index(line, " ("); at != pad {
			t.Errorf("the amount in %q ends at %d, want every amount flush at %d; lines: %v",
				line, at, pad, lines)
		}
	}
}

// TestDecimalsThinOutAsTheMagnitudeGrows: a hit is damage times body size, so
// four and five figures are ordinary, and two decimals on one of those is
// precision the number does not carry, in a column one portrait wide.
func TestDecimalsThinOutAsTheMagnitudeGrows(t *testing.T) {
	for _, tc := range []struct {
		amount float64
		want   string
	}{
		{4.61, "+4.61"},
		{-0.12, "-0.12"},
		{99.994, "+99.99"},
		{-114.17, "-114.2"},
		{-10000, "-10000"},
		// Go rounds a tie to even, so pick a value that is not one.
		{1234.6, "+1235"},
	} {
		if got := formatLedgerAmount(tc.amount); got != tc.want {
			t.Errorf("%v formats as %q, want %q", tc.amount, got, tc.want)
		}
	}
}

// TestGainsAreGreenAndCostsAreRed is the whole point of the colouring, and
// both inks are already contrast-checked in both themes by
// TestThemedInksStayLegible.
func TestGainsAreGreenAndCostsAreRed(t *testing.T) {
	loadKeyGlobals(t)
	gain, cost := ledgerInk(1, false), ledgerInk(-1, false)
	if gain != themedOK() {
		t.Errorf("a gain is inked %v, want the OK green %v", gain, themedOK())
	}
	if cost != themedBad() {
		t.Errorf("a cost is inked %v, want the bad red %v", cost, themedBad())
	}
	if ledgerInk(1, true) != ledgerInk(-1, true) {
		t.Error("a dead organism's ledger should be uniformly dimmed, not still coloured")
	}
}

// TestAnAmountThatRoundsAwayIsNeitherGreenNorRed: the sign is the loudest
// thing in a narrow column, and a net of -4e-05 shows as "+0.00" — inking
// that red makes the colour contradict the number beside it. Same rule as
// the STATS tab's TestNoNegativeZeroes.
func TestAnAmountThatRoundsAwayIsNeitherGreenNorRed(t *testing.T) {
	loadKeyGlobals(t)
	for _, tiny := range []float64{-4e-05, 4e-05, 0, -0.0009} {
		if got := formatLedgerAmount(tiny); got != "+0.00" {
			t.Errorf("%v formats as %q, want +0.00", tiny, got)
		}
		if ledgerInk(tiny, false) != themedMuted() {
			t.Errorf("%v is inked as a gain or a cost though it shows as +0.00", tiny)
		}
	}
	// A magnitude that survives the rounding keeps its sign and its colour.
	if got := formatLedgerAmount(-0.005); got != "-0.01" {
		t.Errorf("-0.005 formats as %q", got)
	}
}
