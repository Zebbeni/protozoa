package ux

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

func statTestTraits() organism.Traits {
	var scores physiology.Scores
	for i, a := range physiology.AllAbilities {
		scores[a] = 2 + i%3
	}
	return organism.Traits{
		OrganismColor:          colorful.Color{R: 0.2, G: 0.7, B: 0.4},
		SecondaryColor:         colorful.Color{R: 0.8, G: 0.3, B: 0.1},
		MaxSize:                40,
		SpawnHealth:            6,
		MinHealthToSpawn:       18,
		MinCyclesBetweenSpawns: 12,
		IdealPh:                4.5,
		Abilities:              scores,
	}
}

func statTestInfo() *organism.Info {
	return &organism.Info{
		ID:           7,
		Health:       14,
		Size:         12,
		Location:     utils.Point{X: 3, Y: 4},
		Age:          520,
		Children:     3,
		TraveledDist: 188,
		AttackHits:   11,
		AttackTotal:  40,
		PhPositive:   2.5,
		PhNegative:   1.25,
	}
}

func rowByLabel(rows []statRow, label string) (statRow, bool) {
	for _, row := range rows {
		if row.label == label {
			return row, true
		}
	}
	return statRow{}, false
}

func allStatRows(cols [2][]statRow) []statRow {
	return append(append([]statRow{}, cols[0]...), cols[1]...)
}

func TestInfoTabRowCountsMatchTheirRows(t *testing.T) {
	loadKeyGlobals(t)

	if got := len(traitRows(statTestTraits())); got != traitRowCount {
		t.Errorf("traitRows has %d rows, traitRowCount reserves %d", got, traitRowCount)
	}

	cols := statColumns(config.GetCurrentGlobals(), statTestInfo(), statTestTraits(), 5.0)
	if got := statRowsHigh(cols); got != statsTabRowCount {
		t.Errorf("the taller stat column has %d rows, statsTabRowCount reserves %d", got, statsTabRowCount)
	}
}

// TestStatsMoveWithSize is the point of the STATS tab: these are the figures an ability score does not give you.
func TestStatsMoveWithSize(t *testing.T) {
	loadKeyGlobals(t)
	g := config.GetCurrentGlobals()
	traits := statTestTraits()

	small := statTestInfo()
	small.Size = 4
	large := statTestInfo()
	large.Size = 28

	before := allStatRows(statColumns(g, small, traits, 6.5))
	after := allStatRows(statColumns(g, large, traits, 6.5))

	sizeScaled := []string{
		"SIZE:", "PH DMG:", "CHEMO HP:", "FOOD/EAT:", "HP/EAT:",
		"ATK DMG:", "DEF DMG:", "MOVE HP:", "TURN HP:", "DIG HP:",
		// What an organism can shoulder through is size x score.
		"WALL THRU:",
	}
	for _, label := range sizeScaled {
		b, ok := rowByLabel(before, label)
		if !ok {
			t.Errorf("no %s row at all", label)
			continue
		}
		a, _ := rowByLabel(after, label)
		if a.value == b.value {
			t.Errorf("%s reads %q at size 4 and size 28 — it isn't scaled by size", label, b.value)
		}
	}
}

func TestStatsMoveWithTheEnvironment(t *testing.T) {
	loadKeyGlobals(t)
	g := config.GetCurrentGlobals()
	info, traits := statTestInfo(), statTestTraits()

	atIdeal := allStatRows(statColumns(g, info, traits, traits.IdealPh))
	farOff := allStatRows(statColumns(g, info, traits, traits.IdealPh+3))

	for _, label := range []string{"PH DIST:", "PH DMG:", "CHEMO HP:"} {
		b, _ := rowByLabel(atIdeal, label)
		a, ok := rowByLabel(farOff, label)
		if !ok {
			t.Errorf("no %s row at all", label)
			continue
		}
		if a.value == b.value {
			t.Errorf("%s reads %q both at the ideal pH and 3 away from it", label, b.value)
		}
	}

	for _, label := range []string{
		"ATK DMG:", "DEF DMG:", "MOVE HP:", "DIG HP:", "FOOD/EAT:",
		"WALL THRU:", "WALL DUG:", "WALL MADE:", "DIG FOOD:",
	} {
		b, _ := rowByLabel(atIdeal, label)
		a, _ := rowByLabel(farOff, label)
		if a.value != b.value {
			t.Errorf("%s changed from %q to %q with the ambient pH; it does not depend on it", label, b.value, a.value)
		}
	}
}

func TestTraitsHoldStillAcrossALifetime(t *testing.T) {
	loadKeyGlobals(t)
	traits := statTestTraits()

	rows := traitRows(traits)
	if len(rows) == 0 {
		t.Fatal("no trait rows")
	}
	for _, row := range rows {
		if row.value == "" && len(row.swatches) == 0 {
			t.Errorf("trait row %q has neither a value nor swatches", row.label)
		}
	}
	if _, ok := rowByLabel(rows, "COLORS:"); !ok {
		t.Error("the colours are a trait with no text form; they need their swatch row")
	}
}

func TestStatValuesComeFromEffects(t *testing.T) {
	loadKeyGlobals(t)
	g := config.GetCurrentGlobals()
	info, traits := statTestInfo(), statTestTraits()
	scores := traits.Abilities

	rows := allStatRows(statColumns(g, info, traits, 7.25))

	cases := []struct {
		label string
		want  string
	}{
		{"ATK DMG:", fmt.Sprintf("%.2f", effects.AttackDamage(g, scores[physiology.AbilityAttack], info.Size))},
		{"DEF DMG:", fmt.Sprintf("%.2f", effects.ThornsDamage(g, scores[physiology.AbilityDefense], info.Size))},
		{"FOOD/EAT:", fmt.Sprintf("%.1f", effects.MaxFoodPerEat(g, scores[physiology.AbilityEating], info.Size))},
		{"MOVE HP:", fmt.Sprintf("%+.2f", effects.MoveCost(g, scores[physiology.AbilityMovement], info.Size))},
		{"DIG HP:", fmt.Sprintf("%+.2f", effects.DigCost(g, scores[physiology.AbilityDigging], info.Size))},
		{"WALL THRU:", fmt.Sprintf("%d", min(manager.MaxWallStrength, effects.MaxBreakableWall(g, scores[physiology.AbilityDigging], info.Size)))},
		{"WALL MADE:", fmt.Sprintf("%d", effects.DigWallCreated(g, scores[physiology.AbilityDigging], info.Size))},
		{"DIG FOOD:", fmt.Sprintf("%d", effects.DigFood(g, scores[physiology.AbilityDigging], info.Size))},
		{"PH BAND:", fmt.Sprintf("%.2f", effects.PhToleranceWidth(g, scores[physiology.AbilityTolerance]))},
	}
	for _, tc := range cases {
		row, ok := rowByLabel(rows, tc.label)
		if !ok {
			t.Errorf("no %s row at all", tc.label)
			continue
		}
		if row.value != tc.want {
			t.Errorf("%s shows %q, effects says %q", tc.label, row.value, tc.want)
		}
	}
}

func TestStatRowsFitTheirColumn(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	face := r.FontSourceCodePro12
	const gap = 4

	check := func(rows []statRow, width int, where string) {
		for _, row := range rows {
			lw := textAdvance(face, row.label)
			vw := textAdvance(face, row.value)
			if n := len(row.swatches); n > 0 {
				vw = n*swatchSize + (n-1)*swatchGap
			}
			if lw+gap+vw > width {
				t.Errorf("%s: %q + %q is %dpx, column holds %d",
					where, row.label, row.value, lw+gap+vw, width)
			}
		}
	}

	// The widest numbers a long-lived organism reaches.
	info := statTestInfo()
	info.Age = 9999999
	info.TraveledDist = 9999999
	info.Children = 99999
	info.AttackHits, info.AttackTotal = 9999999, 9999999
	// The largest organism the settings allow, which is what bounds every size-scaled figure.
	info.Size = config.MaximumMaxSize()

	cols := statColumns(config.GetCurrentGlobals(), info, statTestTraits(), 9.9)
	check(cols[0], statsTabColWidth, "stats col 1")
	check(cols[1], statsTabColWidth, "stats col 2")
	check(traitRows(statTestTraits()), statsColWidth, "traits col")
}

func TestInfoBlockHeightCoversBothTabs(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	lineH := r.FontSourceCodePro12.Metrics().Height.Round()
	got := infoBlockHeight()

	for _, want := range []int{
		traitRowCount * lineH,
		abilitiesHeight(),
		statsTabRowCount * lineH,
	} {
		if got < want {
			t.Errorf("info block reserves %dpx, a tab needs %dpx", got, want)
		}
	}
}

// TestNoNegativeZeroes: a figure whose magnitude rounds away has no sign left to report.
func TestNoNegativeZeroes(t *testing.T) {
	loadKeyGlobals(t)

	if got := signedValue("%+.3f", -0.00004); got != "+0.000" {
		t.Errorf("a magnitude that rounds away formatted as %q", got)
	}
	if got := signedValue("%+.2f", -1.5); got != "-1.50" {
		t.Errorf("a real negative formatted as %q", got)
	}

	info, traits := statTestInfo(), statTestTraits()
	rows := allStatRows(statColumns(config.GetCurrentGlobals(), info, traits, traits.IdealPh+0.001))
	for _, row := range rows {
		if strings.HasPrefix(row.value, "-0.0") && strings.Trim(row.value, "-0.") == "" {
			t.Errorf("%s shows a negative zero: %q", row.label, row.value)
		}
	}
}
