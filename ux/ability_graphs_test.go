package ux

import (
	"image/color"
	"math"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"

	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

// TestEveryCurveHasGraphs: each curve setting row gets a toggle and a
// graph block, and each block has at least one graph.
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
	// One row and one block per ability — Digging and Defense each drive
	// two curves and share a row.
	for _, id := range physiology.AllCurves {
		a := id.Ability()
		if !toggles[a] || !blocks[a] {
			t.Errorf("%s (%s): toggle %v, graph block %v", id.Name(), a.Name(), toggles[a], blocks[a])
		}
		if len(curveGraphsFor(id)) == 0 {
			t.Errorf("%s has no graphs", id.Name())
		}
	}
	// The abilities that drive more than one curve show them all in one
	// block rather than one row per curve.
	for a, want := range map[physiology.Ability]int{
		physiology.AbilityDigging: 3, physiology.AbilityDefense: 2,
	} {
		if got := len(physiology.CurvesFor(a)); got != want {
			t.Errorf("%s drives %d curves, want %d sharing one row", a.Name(), got, want)
		}
	}
}

// TestGraphsExpandAndCollapse: a graph block takes no space until its
// toggle is expanded, and the scrollable height grows by its block.
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

// TestGraphsReadTheFormValues: graphs use the configuration being edited,
// through the effects package, so editing a curve's end changes the graph.
func TestGraphsReadTheFormValues(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	g := cs.globals
	move := curveGraphsFor(physiology.CurveMovementCost)[0].series[0]

	// Costs are plotted as what they take off, so the graph is the
	// magnitude of the health change.
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

// TestDamageGraphsRiseWithScore: graphs titled as damage plot how hard a
// hit lands, so they run from nothing at score 0 up to the full value at
// 100. The simulation states damage as a negative health change, and
// plotting that raw made these graphs read upside-down.
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

// TestCostGraphsFallWithScore: graphs titled as a cost plot what an
// action takes off, so they run from the full cost at score 0 down as the
// ability improves.
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

// TestEveryCurveHasAShapeRow: each curve is pickable through the form,
// and cycling moves through every shape and back round.
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

// TestEachShapeKeepsItsOwnK: the cosine and saturating shapes read K on
// different scales, so each curve stores one K per shape. Switching shape
// must leave both untouched, and show only the K row in play.
func TestEachShapeKeepsItsOwnK(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	// The form edits its own copy of the globals, which is what the
	// shape picker writes to.
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

	// The block's coefficient slider edits the K of the shape in use, and
	// its range is that shape's.
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

// TestShapeButtonsSitBesideTheGraphs: every shape has a button in the
// controls column beside the curve's graphs, and hit-testing one picks
// that shape.
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

// TestCurveKAcceptsTypedValues: clicking a knob's label opens it for
// typing, and what's typed lands on the field that knob edits.
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

// TestCurveBlockHoldsItsSettings: an ability's numbers live with its
// curve — the shape's coefficient first, then every setting that curve
// scales — and each has a slider in the block.
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

// TestMultiCurveAbilitiesShareOneBlock: Digging drives cost, removal and
// creation curves, Defense a damage-taken and a thorns curve, and each
// ability shows them in one block — a section per curve, each with its
// own controls.
func TestMultiCurveAbilitiesShareOneBlock(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	for _, a := range []physiology.Ability{physiology.AbilityDigging, physiology.AbilityDefense} {
		curves := physiology.CurvesFor(a)
		if len(curves) < 2 {
			t.Fatalf("%s drives %d curves, want more than one", a.Name(), len(curves))
		}

		// The block is tall enough for both sections, and a point inside
		// each resolves to that curve.
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

		// Each curve keeps its own shape.
		cs.setCurveShape(curves[0], physiology.ShapeLinear)
		cs.setCurveShape(curves[1], physiology.ShapeQuadratic)
		if physiology.ShapeFor(cs.globals, curves[0]) == physiology.ShapeFor(cs.globals, curves[1]) {
			t.Errorf("%s: both curves took the same shape", a.Name())
		}
	}
}

// curveSectionMidpoint is a y inside the given curve's section of an
// ability's block at y=0.
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

// TestClickingASliderLineOpensItForTyping: the line above each slider is
// the click-to-edit target, and the block has to show that it's open —
// the value is typed into a hidden row, so without the highlight and
// caret a click on it looks like it did nothing.
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

	// And the bar under the line still drags rather than opening an edit.
	sx, sy, sw, sh := curveSliderRect(ctrlX, top, 0)
	cs.selectedRow, cs.editingValue = -1, ""
	cs.handleCurveBlockClick(0, 0, sx+sw/2, sy+sh/2, a)
	if !cs.draggingSlider {
		t.Error("clicking the bar should start a drag")
	}
}

// TestCurveControlsStayInsideTheBlock: the shape buttons sit in a grid
// and the sliders under them, all within the controls column — the
// column's contents are laid out by hand, so an overrun would only show
// up as text clipped at the panel edge.
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

// TestBlockFontsAreReadAtDrawTime: resources.initFonts runs at startup,
// after package-level variables are initialised, so a var holding one of
// these faces captures nil and every block draw panics inside
// BoundString. They have to be read through a call each time.
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

// TestGraphTitlesFitTheirPlots: every title has to land inside the plot
// it sits over, in at most the two lines the title band holds. A title
// that can't is a title to shorten — wrapGraphTitle lets it overhang
// rather than cutting words out of it, so nothing else would catch it.
func TestGraphTitlesFitTheirPlots(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	_, graphW := curveCtrlX(0, cs.panelWidth())
	plotW := graphW - curveGraphLeft - 8
	// The real face isn't loaded in tests; this one is close enough in
	// advance width (7px vs Source Code Pro 12's ~7.2) to catch a title
	// that needs a third line. The margin covers the difference, so a
	// title that only just passes here doesn't overhang on screen.
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

	// And a title that fits stays on one line, so short titles don't sit
	// oddly split above their plots.
	if got := wrapGraphTitle("Wall strength moved per dig", face, plotW); len(got) != 1 {
		t.Errorf("a title that fits wrapped anyway: %q", got)
	}
}

// TestSeriesColoursKeepScoreAndSizeApart: the lines comparing ability
// scores and the lines comparing organism sizes sit a row apart in the
// same block, so they're drawn from two families — cool for scores,
// warm for sizes — and no graph repeats a colour within itself.
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

// TestGraphRangeNeverPadsPastZero: an effect that is only ever damage,
// or only ever a gain, has zero as a real edge of its range. Padding the
// axis through it put a -6.67 label under a damage graph, which reads as
// an amount the effect can produce.
func TestGraphRangeNeverPadsPastZero(t *testing.T) {
	for _, tc := range []struct {
		name           string
		lo, hi         float64
		wantLo, wantHi float64
	}{
		{"damage from zero", 0, 83.33, 0, 90},
		{"cost to zero", -83.33, 0, -90, 0},
		{"a flat positive line", 0.05, 0.05, 0, 0.55},
		{"a flat zero line", 0, 0, 0, 0.5},
	} {
		lo, hi := paddedGraphRange(tc.lo, tc.hi)
		if math.Abs(lo-tc.wantLo) > 0.01 || math.Abs(hi-tc.wantHi) > 0.01 {
			t.Errorf("%s: [%g, %g] padded to [%g, %g], want [%g, %g]",
				tc.name, tc.lo, tc.hi, lo, hi, tc.wantLo, tc.wantHi)
		}
	}

	// An effect that really does cross zero still gets headroom both
	// ways — the chemosynthesis hump pays above its width and costs
	// below it.
	if lo, hi := paddedGraphRange(-1, 2); !(lo < -1 && hi > 2) {
		t.Errorf("a range crossing zero should pad both ends, got [%g, %g]", lo, hi)
	}

	// And every real graph's axis stays on the side its values are on.
	cs, _ := abilityConfigScreen(t)
	for _, id := range physiology.AllCurves {
		for _, graph := range curveGraphsFor(id) {
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, s := range graph.series {
				for j := 0; j <= physiology.MaxAbilityScore; j++ {
					v := 0.0
					if graph.phSpan > 0 {
						v = s.phValue(cs.globals, (float64(j)/physiology.MaxAbilityScore*2-1)*graph.phSpan)
					} else {
						v = s.value(cs.globals, j)
					}
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

// TestWholeUnitGraphsLabelWholeNumbers: wall strength and food are ints,
// so their graphs have to be labelled in the units the effect takes. An
// axis reading "0.76" to "4.24" under a line that can only sit on 1, 2 or
// 3 invites reading a precision the effect doesn't have.
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

	for _, want := range []string{"Wall strength cleared ahead per dig", "Wall strength raised per dig (each side)", "Food rooted up per dig"} {
		if !whole[want] {
			t.Errorf("%q should be marked as a whole-unit graph", want)
		}
	}

	// A flat whole-unit graph still gets an axis to draw on: every value
	// the same integer would otherwise round to a zero-height range.
	if lo, hi := wholeUnitRange(paddedGraphRange(2, 2)); hi-lo < 1 {
		t.Errorf("a flat graph got axis [%v, %v], want at least one unit of height", lo, hi)
	}

	// Everything else keeps the compact format — a multiplier of 0.667
	// must not be rounded to 1.
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
