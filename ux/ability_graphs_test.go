package ux

import (
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"

	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

func TestEveryCurveHasGraphs(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	toggles, blocks := map[physiology.Ability]bool{}, map[physiology.Ability]bool{}
	for _, section := range cs.sections {
		for _, field := range section.fields {
			if field.graphToggle {
				toggles[field.curveAbility] = true
			}
			if field.row == rowCurveGraph {
				blocks[field.curveAbility] = true
			}
		}
	}
	// One row and one block per ability — Digging and Defense each drive two curves and share a row.
	for _, id := range physiology.AllCurves {
		a := id.Ability()
		if !toggles[a] || !blocks[a] {
			t.Errorf("%s (%s): toggle %v, graph block %v", id.Name(), a.Name(), toggles[a], blocks[a])
		}
		if len(curveGraphsFor(id)) == 0 {
			t.Errorf("%s has no graphs", id.Name())
		}
	}
	// The abilities that drive more than one curve show them all in one block rather than one row per curve.
	for a, want := range map[physiology.Ability]int{
		physiology.AbilityDigging: 3, physiology.AbilityDefense: 2,
	} {
		if got := len(physiology.CurvesFor(a)); got != want {
			t.Errorf("%s drives %d curves, want %d sharing one row", a.Name(), got, want)
		}
	}
}

func TestGraphsExpandAndCollapse(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	for i := range cs.sections {
		cs.sections[i].collapsed = false
	}
	before := cs.contentHeight()
	a := physiology.AbilityDigging
	cs.graphExpanded[a] = true
	if got, want := cs.contentHeight()-before, abilityBlockHeight(a); got != want {
		t.Errorf("expanding the Digging block grew the form by %d, want %d", got, want)
	}
	cs.graphExpanded[a] = false
	if cs.contentHeight() != before {
		t.Error("collapsing the graphs should restore the form's height")
	}
}

func TestGraphsReadTheFormValues(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	g := cs.globals
	move := curveGraphsFor(physiology.CurveMovementCost)[0].series[0]

	// Costs are plotted as what they take off, so the graph is the magnitude of the health change.
	if got, want := move.value(g, 100), math.Abs(effects.MoveCost(g, 100, 1)); got != want {
		t.Errorf("move graph at 100 = %v, want effects.MoveCost %v", got, want)
	}
	if got, want := move.value(g, 0), math.Abs(g.HealthChangeFromMoving); got != want {
		t.Errorf("move cost at score 0 = %v, want the full Moving cost %v", got, want)
	}
	// The cosine's K, since that's the shape this checks.
	g.MovementCostCurveShape = string(physiology.ShapeCosine)
	g.MovementCostCosineK = 0
	before := move.value(g, physiology.MaxAbilityScore/2)
	g.MovementCostCosineK = 1
	if after := move.value(g, physiology.MaxAbilityScore/2); math.Abs(after-before) < 1e-12 {
		t.Error("editing Movement cost K didn't change the move graph")
	}
}

func TestDamageGraphsRiseWithScore(t *testing.T) {
	_, globals := abilityConfigScreen(t)
	checked := 0
	for _, id := range physiology.AllCurves {
		for _, graph := range curveGraphsFor(id) {
			if !strings.Contains(strings.ToLower(graph.title), "damage dealt") {
				continue
			}
			checked++
			for _, series := range graph.series {
				lo, mid, hi := series.value(globals, 0), series.value(globals, 50), series.value(globals, 100)
				if lo < 0 || mid < 0 || hi < 0 {
					t.Errorf("%s / %s: plots negative damage (%v, %v, %v)", graph.title, series.label, lo, mid, hi)
				}
				if !(hi > lo) || mid < lo || mid > hi {
					t.Errorf("%s / %s: %v at 0, %v at 50, %v at 100; want damage rising with the score",
						graph.title, series.label, lo, mid, hi)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no damage graphs found to check")
	}
}

func TestCostGraphsFallWithScore(t *testing.T) {
	_, globals := abilityConfigScreen(t)
	checked := 0
	for _, id := range physiology.AllCurves {
		for _, graph := range curveGraphsFor(id) {
			if !strings.Contains(strings.ToLower(graph.title), "health cost") {
				continue
			}
			checked++
			for _, series := range graph.series {
				lo, mid, hi := series.value(globals, 0), series.value(globals, 50), series.value(globals, 100)
				if lo < 0 || mid < 0 || hi < 0 {
					t.Errorf("%s / %s: plots a negative cost (%v, %v, %v)", graph.title, series.label, lo, mid, hi)
				}
				if !(lo > hi) || mid > lo || mid < hi {
					t.Errorf("%s / %s: %v at 0, %v at 50, %v at 100; want the cost falling as the score rises",
						graph.title, series.label, lo, mid, hi)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no cost graphs found to check")
	}
}

func TestEveryCurveHasAShapeRow(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	rows := map[physiology.Ability]bool{}
	for _, section := range cs.sections {
		for _, field := range section.fields {
			if field.row == rowCurveHeader {
				rows[field.curveAbility] = true
			}
		}
	}
	for _, id := range physiology.AllCurves {
		if !rows[id.Ability()] {
			t.Errorf("curve %s has no row under %s", id.Name(), id.Ability().Name())
		}
	}

	start := physiology.ShapeFor(cs.globals, physiology.CurveAttack)
	seen := map[physiology.ShapeKind]bool{start: true}
	for i := 1; i < len(physiology.AllShapeKinds); i++ {
		cs.cycleCurveShape(physiology.CurveAttack, 1)
		seen[physiology.ShapeFor(cs.globals, physiology.CurveAttack)] = true
	}
	if len(seen) != len(physiology.AllShapeKinds) {
		t.Errorf("cycling reached %d shapes, want all %d", len(seen), len(physiology.AllShapeKinds))
	}
	cs.cycleCurveShape(physiology.CurveAttack, 1)
	if got := physiology.ShapeFor(cs.globals, physiology.CurveAttack); got != start {
		t.Errorf("a full cycle ended on %v, want back at %v", got, start)
	}
	cs.cycleCurveShape(physiology.CurveAttack, -1)
	if got := physiology.ShapeFor(cs.globals, physiology.CurveAttack); got == start {
		t.Error("cycling backwards should move to another shape")
	}
}

func TestEachShapeKeepsItsOwnK(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	// The form edits its own copy of the globals, which is what the shape picker writes to.
	g := cs.globals
	g.AttackCosineK, g.AttackSaturatingK = 0.4, 60

	cs.setCurveShape(physiology.CurveAttack, physiology.ShapeSaturating)
	if g.AttackCosineK != 0.4 || g.AttackSaturatingK != 60 {
		t.Errorf("switching shape changed a K: cosine %v, saturating %v", g.AttackCosineK, g.AttackSaturatingK)
	}
	half := physiology.MaxAbilityScore / 2
	saturating := effects.Multiplier(g, physiology.CurveAttack, half)
	cs.setCurveShape(physiology.CurveAttack, physiology.ShapeCosine)
	if cosine := effects.Multiplier(g, physiology.CurveAttack, half); cosine == saturating {
		t.Error("the curve didn't change when the shape did")
	}

	// The block's coefficient slider edits the K of the shape in use, and its range is that shape's.
	cosineRow := findField(t, cs, "attack_cosine_k")
	saturatingRow := findField(t, cs, "attack_saturating_k")
	if lo, hi := cs.getSliderRange(cosineRow); lo != 0 || hi != 1 {
		t.Errorf("cosine K slider range [%v, %v], want [0, 1]", lo, hi)
	}
	if lo, hi := cs.getSliderRange(saturatingRow); lo != physiology.MinSaturatingK || hi != physiology.MaxSaturatingK {
		t.Errorf("saturating K slider range [%v, %v], want [%v, %v]", lo, hi, physiology.MinSaturatingK, physiology.MaxSaturatingK)
	}
	if field, ok := cs.curveKField(physiology.CurveAttack); !ok || field.jsonTag != "attack_cosine_k" {
		t.Errorf("the block edits %q while the cosine shape is in use, want attack_cosine_k", field.jsonTag)
	}
	cs.setCurveShape(physiology.CurveAttack, physiology.ShapeSaturating)
	if field, ok := cs.curveKField(physiology.CurveAttack); !ok || field.jsonTag != "attack_saturating_k" {
		t.Errorf("the block edits %q while the saturating shape is in use, want attack_saturating_k", field.jsonTag)
	}

	// Shapes that read no coefficient leave both Ks alone.
	cs.setCurveShape(physiology.CurveAttack, physiology.ShapeLinear)
	if g.AttackCosineK != 0.4 || g.AttackSaturatingK != 60 {
		t.Errorf("a shape without a coefficient changed one: cosine %v, saturating %v", g.AttackCosineK, g.AttackSaturatingK)
	}
}

func TestShapeButtonsSitBesideTheGraphs(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	for i, kind := range physiology.AllShapeKinds {
		x, y, w, h := curveShapeButtonRect(0, 0, i)
		got, ok := curveShapeButtonAt(0, 0, x+w/2, y+h/2)
		if !ok || got != kind {
			t.Fatalf("button %d covers %v (%v), want %v", i, got, ok, kind)
		}
		cs.setCurveShape(physiology.CurvePhTolerance, got)
		if now := physiology.ShapeFor(cs.globals, physiology.CurvePhTolerance); now != kind {
			t.Errorf("clicking %v set the shape to %v", kind, now)
		}
	}
	if _, ok := curveShapeButtonAt(0, 0, -20, -20); ok {
		t.Error("a point outside the strip matched a button")
	}
	// The block is at least as tall as its controls column.
	if abilityBlockHeight(physiology.AbilityTolerance) < curveCtrlHeight(physiology.CurvePhTolerance) {
		t.Error("the block should leave room for the controls beside the graphs")
	}
}

func TestCurveKAcceptsTypedValues(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	g := cs.globals
	curve := physiology.CurveAttack
	cs.setCurveShape(curve, physiology.ShapeCosine)

	row := cs.rowIndexOfTag("attack_cosine_k")
	if row < 0 {
		t.Fatal("the coefficient in use has no row to edit")
	}
	if got := cs.curveSliders(curve)[0].editing; got != "" {
		t.Errorf("nothing is being typed yet, got %q", got)
	}

	cs.selectedRow, cs.editingValue = row, "0.8"
	if got := cs.curveSliders(curve)[0].editing; got != "0.8" {
		t.Errorf("the block shows %q as the value being typed, want 0.8", got)
	}
	cs.commitEdit()
	if g.AttackCosineK != 0.8 {
		t.Errorf("typed K landed as %v, want 0.8", g.AttackCosineK)
	}

	// The same path edits the saturating K once that shape is in use.
	cs.setCurveShape(curve, physiology.ShapeSaturating)
	cs.selectedRow, cs.editingValue = cs.rowIndexOfTag("attack_saturating_k"), "40"
	cs.commitEdit()
	if g.AttackSaturatingK != 40 || g.AttackCosineK != 0.8 {
		t.Errorf("saturating %v / cosine %v, want 40 and 0.8 kept apart", g.AttackSaturatingK, g.AttackCosineK)
	}
}

func TestCurveBlockHoldsItsSettings(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	for _, curve := range physiology.AllCurves {
		fields := cs.curveSliderFields(curve)
		want := len(curveSettingTags[curve])
		if physiology.ShapeFor(cs.globals, curve).UsesK() {
			want++
		}
		if len(fields) != want {
			t.Errorf("%s block has %d sliders, want %d", curve.Name(), len(fields), want)
		}
		if len(cs.curveSliders(curve)) != len(fields) {
			t.Errorf("%s draws %d sliders for %d fields", curve.Name(), len(cs.curveSliders(curve)), len(fields))
		}
		// Every slider is hit-testable at its own rect.
		for i := range fields {
			sx, sy, sw, sh := curveSliderRect(0, 0, i)
			got, onLabel, ok := curveSliderAt(0, 0, len(fields), sx+sw/2, sy+sh/2)
			if !ok || onLabel || got != i {
				t.Errorf("%s slider %d hit-tests as %d (label %v, ok %v)", curve.Name(), i, got, onLabel, ok)
			}
			lx, ly, lw, lh := curveSliderLabelRect(0, 0, i)
			got, onLabel, ok = curveSliderAt(0, 0, len(fields), lx+lw/2, ly+lh/2)
			if !ok || !onLabel || got != i {
				t.Errorf("%s label %d hit-tests as %d (label %v, ok %v)", curve.Name(), i, got, onLabel, ok)
			}
		}
		if h := abilityBlockHeight(curve.Ability()); h < curveCtrlHeight(curve) {
			t.Errorf("%s block is %d tall, too short for its controls (%d)", curve.Name(), h, curveCtrlHeight(curve))
		}
	}
}

func TestMultiCurveAbilitiesShareOneBlock(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	for _, a := range []physiology.Ability{physiology.AbilityDigging, physiology.AbilityDefense} {
		curves := physiology.CurvesFor(a)
		if len(curves) < 2 {
			t.Fatalf("%s drives %d curves, want more than one", a.Name(), len(curves))
		}

		// The block is tall enough for both sections, and a point inside each resolves to that curve.
		want := curveGraphPadTop
		for _, id := range curves {
			want += curveSectionHeight(id)
		}
		if got := abilityBlockHeight(a); got != want {
			t.Errorf("%s block is %d tall, want %d for both curves", a.Name(), got, want)
		}
		for _, id := range curves {
			_, sectionTop, ok := curveSectionAt(a, 0, curveSectionMidpoint(a, id))
			if !ok {
				t.Fatalf("%s: no section covers %s", a.Name(), id.Name())
			}
			got, _, _ := curveSectionAt(a, 0, sectionTop+1)
			if got != id {
				t.Errorf("%s: the section at %d is %s, want %s", a.Name(), sectionTop, got.Name(), id.Name())
			}
		}
		if _, _, ok := curveSectionAt(a, 0, abilityBlockHeight(a)+5); ok {
			t.Errorf("%s: a point past the block matched a section", a.Name())
		}

		cs.setCurveShape(curves[0], physiology.ShapeLinear)
		cs.setCurveShape(curves[1], physiology.ShapeQuadratic)
		if physiology.ShapeFor(cs.globals, curves[0]) == physiology.ShapeFor(cs.globals, curves[1]) {
			t.Errorf("%s: both curves took the same shape", a.Name())
		}
	}
}

// curveSectionMidpoint is a y inside the given curve's section of an ability's block at y=0.
func curveSectionMidpoint(a physiology.Ability, want physiology.CurveID) int {
	top := curveGraphPadTop
	for _, id := range physiology.CurvesFor(a) {
		h := curveSectionHeight(id)
		if id == want {
			return top + h/2
		}
		top += h
	}
	return top
}

func TestClickingASliderLineOpensItForTyping(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	a, curve := physiology.AbilityAttack, physiology.CurveAttack
	ctrlX, _ := curveCtrlX(0, cs.panelWidth())
	_, sectionTop, ok := curveSectionAt(a, 0, curveSectionMidpoint(a, curve))
	if !ok {
		t.Fatal("the attack curve has no section in its ability's block")
	}
	top := sectionTop + curveHeadingH

	fields := cs.curveSliderFields(curve)
	for i, f := range fields {
		lx, ly, lw, lh := curveSliderLabelRect(ctrlX, top, i)
		cs.selectedRow, cs.editingValue = -1, ""
		cs.handleCurveBlockClick(0, 0, lx+lw/2, ly+lh/2, a)

		if want := cs.rowIndexOfTag(f.jsonTag); cs.selectedRow != want {
			t.Fatalf("%s: clicking its line selected row %d, want %d", f.jsonTag, cs.selectedRow, want)
		}
		if cs.draggingSlider {
			t.Fatalf("%s: clicking the line started a drag instead of an edit", f.jsonTag)
		}
		if s := cs.curveSliders(curve)[i]; !s.selected {
			t.Fatalf("%s: the block doesn't show the line as open for typing", f.jsonTag)
		}
	}

	sx, sy, sw, sh := curveSliderRect(ctrlX, top, 0)
	cs.selectedRow, cs.editingValue = -1, ""
	cs.handleCurveBlockClick(0, 0, sx+sw/2, sy+sh/2, a)
	if !cs.draggingSlider {
		t.Error("clicking the bar should start a drag")
	}
}

func TestCurveControlsStayInsideTheBlock(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	w := cs.panelWidth()
	ctrlX, graphW := curveCtrlX(0, w)
	for _, a := range physiology.AllAbilities {
		for _, curve := range physiology.CurvesFor(a) {
			_, sectionTop, ok := curveSectionAt(a, 0, curveSectionMidpoint(a, curve))
			if !ok {
				t.Fatalf("%s has no section for %s", a.Name(), curve.Name())
			}
			top := sectionTop + curveHeadingH
			bottom := sectionTop + curveSectionHeight(curve)

			for i := range physiology.AllShapeKinds {
				bx, by, bw, bh := curveShapeButtonRect(ctrlX, top, i)
				if bx < ctrlX || bx+bw > w || by < top || by+bh > bottom {
					t.Errorf("%s shape button %d at (%d,%d,%d,%d) leaves the column",
						curve.Name(), i, bx, by, bw, bh)
				}
				if bx < graphW {
					t.Errorf("%s shape button %d overlaps the graphs", curve.Name(), i)
				}
			}
			for i := range cs.curveSliderFields(curve) {
				lx, ly, lw, _ := curveSliderLabelRect(ctrlX, top, i)
				sx, sy, sw, sh := curveSliderRect(ctrlX, top, i)
				if lx+lw > w || sx+sw > w {
					t.Errorf("%s slider %d runs past the panel edge", curve.Name(), i)
				}
				if ly < top || sy+sh > bottom {
					t.Errorf("%s slider %d leaves its section", curve.Name(), i)
				}
			}
		}
	}
}

func TestBlockFontsAreReadAtDrawTime(t *testing.T) {
	saved10, saved12 := r.FontSourceCodePro10, r.FontSourceCodePro12
	defer func() { r.FontSourceCodePro10, r.FontSourceCodePro12 = saved10, saved12 }()

	r.FontSourceCodePro10 = basicfont.Face7x13
	r.FontSourceCodePro12 = basicfont.Face7x13
	for name, got := range map[string]font.Face{
		"curveCtrlFont":       curveCtrlFont(),
		"curveGraphLabelFont": curveGraphLabelFont(),
		"curveGraphTitleFont": curveGraphTitleFont(),
	} {
		if got != font.Face(basicfont.Face7x13) {
			t.Errorf("%s captured a face instead of reading it when called", name)
		}
	}
}

func TestGraphTitlesFitTheirPlots(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	_, graphW := curveCtrlX(0, cs.panelWidth())
	plotW := graphW - curveGraphLeft - 8
	// The real face isn't loaded in tests.
	face, fit := basicfont.Face7x13, plotW*92/100

	for _, id := range physiology.AllCurves {
		for _, graph := range curveGraphsFor(id) {
			lines := wrapGraphTitle(graph.title, face, plotW)
			if len(lines) > 2 {
				t.Errorf("%s: %q wraps to %d lines, the band holds 2", id.Name(), graph.title, len(lines))
			}
			for _, line := range lines {
				if got := boundString(face, line).Dx(); got > fit {
					t.Errorf("%s: %q measures %dpx against a %dpx plot; shorten it", id.Name(), line, got, plotW)
				}
			}
			if strings.Join(lines, " ") != graph.title {
				t.Errorf("%s: wrapping changed the title to %q", id.Name(), strings.Join(lines, " "))
			}
		}
	}

	// And a title that fits stays on one line, so short titles don't sit oddly split above their plots.
	if got := wrapGraphTitle("Wall strength moved per dig", face, plotW); len(got) != 1 {
		t.Errorf("a title that fits wrapped anyway: %q", got)
	}
}

func TestSeriesColoursKeepScoreAndSizeApart(t *testing.T) {
	rgb := func(c color.Color) [3]uint32 {
		r, g, b, _ := c.RGBA()
		return [3]uint32{r, g, b}
	}

	for _, id := range physiology.AllCurves {
		for _, graph := range curveGraphsFor(id) {
			seen := map[[3]uint32]string{}
			for i, s := range graph.series {
				key := rgb(seriesColor(s, i))
				if other, dup := seen[key]; dup {
					t.Errorf("%s: series %q and %q are the same colour", id.Name(), other, s.label)
				}
				seen[key] = s.label
			}
		}
	}

	for _, score := range []int{0, 1, 3, physiology.MaxAbilityScore / 2, physiology.MaxAbilityScore} {
		c := rgb(scoreSeriesColor(score))
		if c[2] <= c[0] {
			t.Errorf("score %d is drawn warm (%v); the score family is the cool one", score, c)
		}
	}
	for i, c := range sizeClassColors {
		if v := rgb(c); v[0] <= v[2] {
			t.Errorf("size class %d is drawn cool (%v); the size family is the warm one", i, v)
		}
	}

	// A higher score is a brighter line, so the legend reads in order.
	prev := -1.0
	for _, score := range []int{0, 1, 3, physiology.MaxAbilityScore / 2, physiology.MaxAbilityScore} {
		c := rgb(scoreSeriesColor(score))
		lum := 0.2126*float64(c[0]) + 0.7152*float64(c[1]) + 0.0722*float64(c[2])
		if lum <= prev {
			t.Errorf("score %d isn't brighter than the score below it", score)
		}
		prev = lum
	}
}

func TestGraphRangeNeverPadsPastZero(t *testing.T) {
	for _, tc := range []struct {
		name           string
		lo, hi         float64
		wantLo, wantHi float64
	}{
		{"damage from zero", 0, 83.33, 0, 90},
		{"cost to zero", -83.33, 0, -90, 0},
		// A flat line's pad is a tenth of the value, so the line sits in the middle of an axis labelled around it.
		{"a flat positive line", 0.05, 0.05, 0.045, 0.055},
		{"a flat zero line", 0, 0, 0, 0.5},
	} {
		lo, hi := paddedGraphRange(tc.lo, tc.hi)
		if math.Abs(lo-tc.wantLo) > 0.01 || math.Abs(hi-tc.wantHi) > 0.01 {
			t.Errorf("%s: [%g, %g] padded to [%g, %g], want [%g, %g]",
				tc.name, tc.lo, tc.hi, lo, hi, tc.wantLo, tc.wantHi)
		}
	}

	// An effect that really does cross zero still gets headroom both ways.
	for _, v := range []float64{0.00125, 0.05, 3, 900} {
		lo, hi := paddedGraphRange(v, v)
		if !(lo < v && hi > v) {
			t.Errorf("a constant %v got axis %v..%v, which doesn't contain it", v, lo, hi)
		}
		if hi-lo > math.Abs(v) {
			t.Errorf("a constant %v got axis %v..%v, too wide to read the value off", v, lo, hi)
		}
	}
	// Except at exactly zero, where there is no magnitude to scale by and a zero-height axis would divide by zero when plotting.
	if lo, hi := paddedGraphRange(0, 0); hi <= lo {
		t.Errorf("a constant 0 got a zero-height axis %v..%v", lo, hi)
	}

	if lo, hi := paddedGraphRange(-1, 2); !(lo < -1 && hi > 2) {
		t.Errorf("a range crossing zero should pad both ends, got [%g, %g]", lo, hi)
	}

	cs, _ := abilityConfigScreen(t)
	for _, id := range physiology.AllCurves {
		for _, graph := range curveGraphsFor(id) {
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, s := range graph.series {
				// Sampled through the graph's own axis, at the same points the old branch used.
				axis := graph.xAxis()
				for j := 0; j <= physiology.MaxAbilityScore; j++ {
					v := axis.read(s, cs.globals, j, physiology.MaxAbilityScore)
					lo, hi = math.Min(lo, v), math.Max(hi, v)
				}
			}
			axisLo, axisHi := paddedGraphRange(lo, hi)
			if lo >= 0 && axisLo < 0 {
				t.Errorf("%s %q: values start at %g but the axis runs to %g", id.Name(), graph.title, lo, axisLo)
			}
			if hi <= 0 && axisHi > 0 {
				t.Errorf("%s %q: values top out at %g but the axis runs to %g", id.Name(), graph.title, hi, axisHi)
			}
		}
	}
}

func TestWholeUnitGraphsLabelWholeNumbers(t *testing.T) {
	cs, _ := abilityConfigScreen(t)

	whole := map[string]bool{}
	for _, id := range physiology.AllCurves {
		for _, graph := range curveGraphsFor(id) {
			if !graph.wholeUnits {
				continue
			}
			whole[graph.title] = true

			lo, hi := math.Inf(1), math.Inf(-1)
			for _, s := range graph.series {
				for score := 0; score <= physiology.MaxAbilityScore; score++ {
					v := s.value(cs.globals, score)
					if v != math.Trunc(v) {
						t.Errorf("%q: series %q gives %v at score %d, which isn't a whole unit",
							graph.title, s.label, v, score)
					}
					lo, hi = math.Min(lo, v), math.Max(hi, v)
				}
			}

			axisLo, axisHi := wholeUnitRange(paddedGraphRange(lo, hi))
			if axisLo != math.Trunc(axisLo) || axisHi != math.Trunc(axisHi) {
				t.Errorf("%q: axis runs [%v, %v], want whole numbers", graph.title, axisLo, axisHi)
			}
			if axisHi-axisLo < 1 {
				t.Errorf("%q: axis [%v, %v] has no height", graph.title, axisLo, axisHi)
			}
			if axisLo > lo || axisHi < hi {
				t.Errorf("%q: axis [%v, %v] cuts off values [%v, %v]", graph.title, axisLo, axisHi, lo, hi)
			}
			for _, v := range []float64{axisLo, axisHi} {
				if got := graph.formatValue(v); strings.ContainsAny(got, ".e") {
					t.Errorf("%q: axis label %q isn't a whole number", graph.title, got)
				}
			}
		}
	}

	for _, want := range []string{"Wall strength raised per dig (each side)", "Buried food brought back up per dig"} {
		if !whole[want] {
			t.Errorf("%q should be marked as a whole-unit graph", want)
		}
	}

	// A flat whole-unit graph still gets an axis to draw on.
	if lo, hi := wholeUnitRange(paddedGraphRange(2, 2)); hi-lo < 1 {
		t.Errorf("a flat graph got axis [%v, %v], want at least one unit of height", lo, hi)
	}

	// Everything else keeps the compact format — a multiplier of 0.667 must not be rounded to 1.
	for _, id := range physiology.AllCurves {
		for _, graph := range curveGraphsFor(id) {
			if graph.wholeUnits {
				continue
			}
			if got := graph.formatValue(0.667); got != "0.667" {
				t.Errorf("%q formatted 0.667 as %q", graph.title, got)
			}
		}
	}
}

// TestNoCurveGraphIsAConstantLine: a plot whose line can't move is a number drawn the long way round.
func TestNoCurveGraphIsAConstantLine(t *testing.T) {
	_, active := abilityConfigScreen(t)

	for _, id := range physiology.AllCurves {
		g := *active
		setCurveShape(&g, id, physiology.ShapeLinear)
		// Every setting this curve scales is forced non-zero as well.
		for n, tag := range curveSettingTags[id] {
			idx, _ := globalsField(tag)
			f := reflect.ValueOf(&g).Elem().Field(idx)
			switch f.Kind() {
			case reflect.Float64:
				f.SetFloat(float64(n+1) * 10)
			case reflect.Int:
				f.SetInt(int64(n+1) * 10)
			}
		}

		for _, graph := range curveGraphsFor(id) {
			for _, series := range graph.series {
				lo, hi, varies := 0.0, 0.0, false
				axis := graph.xAxis()
				for i := 0; i <= physiology.MaxAbilityScore; i++ {
					v := axis.read(series, &g, i, physiology.MaxAbilityScore)
					if i == 0 {
						lo, hi = v, v
					}
					lo, hi = min(lo, v), max(hi, v)
					if hi-lo > 1e-12 {
						varies = true
						break
					}
				}
				if !varies {
					t.Errorf("%s: graph %q series %q is a constant %v — show it as a value, not a plot",
						id.Name(), graph.title, series.label, lo)
				}
			}
		}
	}
}

// TestEveryCurveSettingHasALabel: a curve block builds its sliders from
// curveSettingTags and names each one with curveSettingLabels[tag], so a
// tag with no entry draws a working slider with no label at all. Five did
// — the three burrow settings and the two damage at-zero endpoints — and
// nothing failed, because a missing map entry is an empty string rather
// than an error.
func TestEveryCurveSettingHasALabel(t *testing.T) {
	seen := map[string]bool{}
	for curve, tags := range curveSettingTags {
		for _, tag := range tags {
			seen[tag] = true
			if curveSettingLabels[tag] == "" {
				t.Errorf("%v holds %q with no label, so its slider draws nameless",
					curve, tag)
			}
		}
	}
	// And no label left behind for a setting no block holds any more, which
	// is the other way the two lists drift apart.
	for tag := range curveSettingLabels {
		if !seen[tag] {
			t.Errorf("curveSettingLabels has %q, which no curve block holds", tag)
		}
	}
}

// TestEveryCurveIsRegisteredEverywhere: a curve has to appear in four maps
// before the screen can draw it, and a missing entry is silent in three of
// them. Adding the attack cost curve missed curveFieldTags and
// curveFieldNames, and the symptoms pointed somewhere else entirely — the
// ATTACK ability lost its graph block, because the block is emitted for an
// ability's LAST curve and the new curve had become the last one without a
// curveFieldTags entry to trigger it.
func TestEveryCurveIsRegisteredEverywhere(t *testing.T) {
	for _, id := range physiology.AllCurves {
		if curveShapeTags[id] == "" {
			t.Errorf("%s has no entry in curveShapeTags", id.Name())
		}
		if curveFieldTags[id].last == "" {
			t.Errorf("%s has no entry in curveFieldTags, so its block emits no "+
				"sliders and its ability may lose its graph row", id.Name())
		}
		names := curveFieldNames[id]
		if names.shape == "" || names.cosineK == "" || names.saturatingK == "" {
			t.Errorf("%s has an incomplete entry in curveFieldNames: %+v", id.Name(), names)
		}
		// Deliberately not asserting that a curve holds settings: Damage
		// taken holds none, being a pure multiplier on damage the
		// ATTACKER's setting defines, so its block is graphs and a shape.
	}
	// And the K tags resolve back to a real curve, so a renamed setting
	// cannot leave an entry pointing at nothing.
	for tag, kt := range curveKTags {
		if curveFieldNames[kt.curve].shape == "" {
			t.Errorf("curveKTags has %q for an unregistered curve", tag)
		}
	}
}
