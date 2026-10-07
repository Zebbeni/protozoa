package ux

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/organism"
	r "github.com/Zebbeni/protozoa/resources"
	"golang.org/x/image/font"
)

// The per-cycle health ledger under the portrait: what this cycle cost and
// paid the selected organism, by source, with a net at the bottom.
//
// An ability score says what an organism is built for and the STATS tab says
// what one action is worth; neither says what actually happened to it on the
// cycle being looked at, which is the question a dwindling health bar asks.

const (
	ledgerGap = 6
	// Clear of the descenders on the line above, which a 1px rule sits in.
	ledgerRuleGap = 5
)

func ledgerFace() font.Face { return r.FontSourceCodePro10 }

func ledgerRowHeight() int { return ledgerFace().Metrics().Height.Round() }

// healthLedgerHeight reserves room for every source plus the net, so the
// info block's height does not move with what an organism did this cycle.
func healthLedgerHeight() int {
	return ledgerDrawnHeight(len(organism.AllHealthSources))
}

// portraitColumnHeight is the portrait, its health bar and the ledger under it.
func portraitColumnHeight() int {
	return portraitSize + healthBarGap + healthBarH + healthLedgerHeight()
}

type ledgerRow struct {
	amount float64
	label  string
}

// healthLedgerRows is the sources that moved this cycle, in
// AllHealthSources order. Only those: a column of zeroes for everything
// that did not happen buries the two or three lines that did.
func healthLedgerRows(l organism.HealthLedger) []ledgerRow {
	var rows []ledgerRow
	for _, src := range organism.AllHealthSources {
		if amount := l.Amounts[src]; amount != 0 {
			rows = append(rows, ledgerRow{amount: amount, label: src.Label()})
		}
	}
	return rows
}

// ledgerSign is the sign of the amount AS SHOWN. A magnitude that rounds
// away has no sign left to carry, and inking "+0.00" red because the raw
// value was -4e-05 makes the colour contradict the number beside it.
func ledgerSign(amount float64) int {
	shown := formatLedgerAmount(amount)
	if strings.Trim(shown, "+-0.") == "" {
		return 0
	}
	if strings.HasPrefix(shown, "-") {
		return -1
	}
	return 1
}

func ledgerInk(amount float64, dim bool) color.Color {
	if dim {
		return themedForegroundDim()
	}
	switch ledgerSign(amount) {
	case 1:
		return themedOK()
	case -1:
		return themedBad()
	}
	return themedMuted()
}

// formatLedgerAmount keeps the sign only while there is a magnitude left to
// carry it: the minus is the loudest thing in a narrow column, and "-0.00"
// points it at nothing. Same rule as the STATS tab's signedValue.
//
// Decimals thin out as the magnitude grows, because the column is one
// portrait wide and two of them on a four-figure hit are precision the
// number does not carry. A hit is damage times body size, so four and five
// figures are ordinary for a large organism.
func formatLedgerAmount(amount float64) string {
	format := "%+.2f"
	switch mag := math.Abs(amount); {
	case mag >= 1000:
		format = "%+.0f"
	case mag >= 100:
		format = "%+.1f"
	}
	return signedValue(format, amount)
}

// ledgerAmountWidth is the character width to right-align the amounts in, so
// the decimal points line up in the panel's monospaced type.
func ledgerAmountWidth(rows []ledgerRow, total float64) int {
	w := len(formatLedgerAmount(total))
	for _, row := range rows {
		if n := len(formatLedgerAmount(row.amount)); n > w {
			w = n
		}
	}
	return w
}

// ledgerLine is one row as drawn, amounts padded to a common width.
func ledgerLine(amount float64, label string, pad int) string {
	return fmt.Sprintf("%*s (%s)", pad, formatLedgerAmount(amount), label)
}

// ledgerDrawnHeight is the space the ledger actually paints for this many
// source rows, which must never exceed what healthLedgerHeight reserves.
func ledgerDrawnHeight(rows int) int {
	return ledgerGap + (rows+1)*ledgerRowHeight() + 2*ledgerRuleGap + 1
}

// drawHealthLedger draws the selected organism's per-cycle health changes
// under the health bar, and returns the bottom of the space it reserves.
func drawHealthLedger(img *ebiten.Image, l organism.HealthLedger, dim bool, x, y, width int) int {
	bottom := y + healthLedgerHeight()
	lineH := ledgerRowHeight()
	if !l.Recorded {
		// A restored snapshot carries no ledger, which is a different thing
		// from a cycle that cost nothing.
		text.Draw(img, "(no cycle yet)", ledgerFace(), x, y+ledgerGap+lineH, themedMuted())
		return bottom
	}

	rows := healthLedgerRows(l)
	total := l.Total()
	pad := ledgerAmountWidth(rows, total)
	cursor := y + ledgerGap + lineH

	for _, row := range rows {
		text.Draw(img, ledgerLine(row.amount, row.label, pad), ledgerFace(), x, cursor,
			ledgerInk(row.amount, dim))
		cursor += lineH
	}

	// The rule follows the last row drawn rather than sitting at the bottom
	// of the reserved space, so the net reads as the sum of the lines above
	// it instead of floating clear of them.
	cursor += ledgerRuleGap
	var rule color.Color = themedMuted()
	if dim {
		rule = themedForegroundDim()
	}
	ebitenutil.DrawRect(img, float64(x), float64(cursor), float64(width), 1, rule)
	cursor += 1 + ledgerRuleGap + lineH

	text.Draw(img, ledgerLine(total, "net", pad), ledgerFace(), x, cursor, ledgerInk(total, dim))
	return bottom
}
