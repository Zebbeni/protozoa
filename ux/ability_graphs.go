package ux

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

// curveGraphSeries is one line on a curve graph: a label for the legend and
// the effect's value at a score, read through the effects package so the
// graph always matches what the simulation does.
type curveGraphSeries struct {
	label string
	// value reads the effect at an ability score, for graphs plotted
	// against score; phValue reads it at a pH offset from the organism's
	// ideal, for graphs plotted against pH. A series fills in whichever
	// its graph uses.
	value   func(g *c.Globals, score int) float64
	phValue func(g *c.Globals, offset float64) float64
	// color overrides the graph's palette. A series set by one of the
	// builders below picks a colour from the family that says what it
	// varies — ability score, or organism size — so two graphs a row
	// apart aren't drawn in the same three colours for different things.
	color color.Color
}

// curveGraph plots one action or effect, against ability score or —
// when phSpan is set — against how far the water sits from an organism's
// ideal pH, ideal in the middle. The pH form is what makes a curve like
// chemosynthesis legible: a hump that widens and flattens as its knobs
// change, rather than a line against a score.
type curveGraph struct {
	title  string
	series []curveGraphSeries
	phSpan float64
	// wholeUnits marks an effect that only ever takes whole values —
	// wall strength and food are ints — so its axis is labelled in whole
	// numbers and its range rounded out to them. A "4.24" on a graph of
	// something that can only be 4 or 5 invites reading a precision the
	// effect doesn't have.
	wholeUnits bool
}

// Layout of a graph block on the config screen. The graphs are drawn
// with the panel's normal type rather than the smallest face — a curve
// nobody can read the numbers on isn't worth the space — so the plot is
// taller and narrower than the text around it would suggest.
const (
	// Titles say what the curve is and in what units, which is more than
	// fits across a plot this narrow, so the band holds two lines and
	// long titles wrap into it.
	curveGraphTitleLineH = 15
	curveGraphTitleH     = 2*curveGraphTitleLineH + 2
	curveGraphPlotH      = 86
	curveGraphXLabelH    = 14
	curveGraphGap        = 10
	curveGraphLeft       = 62 // room for y-axis labels
	curveGraphPadTop     = 4
	graphToggleW         = 16
)

// curveGraphTitleFont names the graph; curveGraphLabelFont is everything
// inside and around the plot (axis marks, point values, legend).
//
// Functions, not variables: resources.initFonts runs at startup, well
// after package-level variables are initialised, so a var here would
// capture a nil face and every draw would panic inside BoundString.
func curveGraphTitleFont() font.Face { return r.FontSourceCodePro12 }
func curveGraphLabelFont() font.Face { return r.FontSourceCodePro10 }

// curveGraphHeight is the pixel height of one graph, title to x labels.
const curveGraphHeight = curveGraphTitleH + curveGraphPlotH + curveGraphXLabelH + curveGraphGap

// A curve's controls sit in a column to the right of its graphs: the
// shape buttons stacked, then the slider for whichever coefficient the
// chosen shape uses. Choosing a shape while looking at the curve it
// draws is the whole point of putting them side by side.
const (
	curveCtrlW        = 210
	curveCtrlGap      = 12
	curveShapeBtnH    = 18
	curveShapeBtnGap  = 3
	curveCtrlSliderH  = 12
	curveCtrlSliderW  = 140
	curveCtrlValueGap = 6
)

// curveCtrlFont is the type in the controls column. The same face the
// form's own rows use: these are the numbers being tuned, and they were
// unreadable at the smallest size. A function for the same reason as
// curveGraphTitleFont.
func curveCtrlFont() font.Face { return r.FontSourceCodePro10 }

// curveShapeBtnCols lays the shape buttons out in a grid rather than a
// stack. Four buttons in one column cost more height than the graph
// beside them, which is what the bigger type needed back.
const curveShapeBtnCols = 2

// curveShapeBtnRows is how many rows that grid takes.
const curveShapeBtnRows = (len(shapeButtonOrder) + curveShapeBtnCols - 1) / curveShapeBtnCols

// curveShapeGridH is the height of the shape-button grid.
const curveShapeGridH = curveShapeBtnRows * (curveShapeBtnH + curveShapeBtnGap)

// curveCtrlSlot is the vertical step between the sliders under the shape
// buttons: a label line and the slider itself.
const curveCtrlSlot = curveCtrlLabelH + curveCtrlSliderH + 6

// curveCtrlLabelH is the line above each slider naming it and its value.
// Tall enough to read, and to hit: clicking it is how an exact value
// gets typed.
const curveCtrlLabelH = 14

// curveCtrlHeight is the height of a curve's controls column: the shape
// buttons stacked, then one slider per knob — the shape's coefficient
// first, then the settings that curve scales.
func curveCtrlHeight(id physiology.CurveID) int {
	return curveShapeGridH + 6 + (1+len(curveSettingTags[id]))*curveCtrlSlot
}

// curveRowHeight is one graph and the controls beside it.
const curveRowHeight = curveGraphHeight

// shapeButtonOrder is a fixed-size view of physiology.AllShapeKinds, so
// the controls column's height is a constant.
var shapeButtonOrder = [4]physiology.ShapeKind{}

func init() {
	copy(shapeButtonOrder[:], physiology.AllShapeKinds)
}

// graphSeriesColors is the fallback palette, and its first entry is
// what a single-line graph is drawn in.
var graphSeriesColors = []color.RGBA{
	{R: 120, G: 200, B: 255, A: 255},
	{R: 255, G: 190, B: 90, A: 255},
	{R: 150, G: 230, B: 130, A: 255},
}

// sizeClassColors draw the small / medium / large comparisons: a warm
// family, kept clear of the cool one the score comparisons use. The two
// kinds of graph sit a row apart inside one ability's block, and in a
// shared palette a "50" line and a "medium" line were the same colour.
var sizeClassColors = [3]color.RGBA{
	{R: 235, G: 150, B: 70, A: 255},
	{R: 245, G: 195, B: 90, A: 255},
	{R: 250, G: 235, B: 150, A: 255},
}

// scoreSeriesColor is the cool end of that split: one colour per ability
// score, dim blue at nothing and bright cyan at the cap, so the lines
// read as an ordered family instead of three unrelated hues. The square
// root spreads the low scores out, which is where a saturating curve
// puts most of its difference.
func scoreSeriesColor(score int) color.Color {
	p := min(1.0, max(0.0, float64(score)/physiology.MaxAbilityScore))
	lo := colorful.Color{R: 0.31, G: 0.42, B: 0.74}
	hi := colorful.Color{R: 0.58, G: 0.95, B: 1}
	return lo.BlendLuv(hi, math.Sqrt(p)).Clamped()
}

// seriesColor is the colour the i-th series of a graph is drawn in.
func seriesColor(s curveGraphSeries, i int) color.Color {
	if s.color != nil {
		return s.color
	}
	return graphSeriesColors[i%len(graphSeriesColors)]
}

// sizeClassSizes returns a representative organism size in the middle of
// each size class (small, medium, large) under g, for effects that depend
// on size class.
func sizeClassSizes(g *c.Globals) [3]float64 {
	m := g.MaximumMaxSize
	return [3]float64{m / 6, m / 2, m * 5 / 6}
}

// perSize returns a series for an effect that scales with organism size,
// evaluated for a size-1 organism, i.e. per unit of size.
func perSize(label string, f func(g *c.Globals, score int, size float64) float64) curveGraphSeries {
	return curveGraphSeries{label: label, value: func(g *c.Globals, score int) float64 { return f(g, score, 1) }}
}

// atScore returns a pH-axis series showing what an organism with this
// ability score gets at each offset from its ideal pH. Three of them on
// one graph is what shows the hump widening with the score.
func atScore(score int, f func(g *c.Globals, score int, offset float64) float64) curveGraphSeries {
	return curveGraphSeries{
		label:   fmt.Sprintf("%d", score),
		phValue: func(g *c.Globals, offset float64) float64 { return f(g, score, math.Abs(offset)) },
		color:   scoreSeriesColor(score),
	}
}

// magnitude wraps an effect the simulation expresses as a negative health
// change — damage dealt, the cost of an action — and plots how much it
// takes off. Plotting the raw value instead made these graphs read
// upside-down: damage appeared to shrink as Attack rose, and a cost
// appeared to grow as Digging did, when both were just moving away from
// or toward zero.
func magnitude(f func(g *c.Globals, score int) float64) func(g *c.Globals, score int) float64 {
	return func(g *c.Globals, score int) float64 { return math.Abs(f(g, score)) }
}

// perSizeMagnitude is magnitude for an effect that scales with size,
// evaluated per unit of size.
func perSizeMagnitude(label string, f func(g *c.Globals, score int, size float64) float64) curveGraphSeries {
	return curveGraphSeries{label: label, value: magnitude(func(g *c.Globals, s int) float64 { return f(g, s, 1) })}
}

// bySizeClass returns one series per size class for an effect whose value
// depends on the organism's size class.
func bySizeClass(f func(g *c.Globals, score int, size float64) float64) []curveGraphSeries {
	labels := []string{"small", "medium", "large"}
	out := make([]curveGraphSeries, 3)
	for i := range out {
		i := i
		out[i] = curveGraphSeries{label: labels[i], color: sizeClassColors[i], value: func(g *c.Globals, score int) float64 {
			return f(g, score, sizeClassSizes(g)[i])
		}}
	}
	return out
}

// curveGraphsFor lists the graphs shown for a curve: every action or effect
// it changes.
func curveGraphsFor(id physiology.CurveID) []curveGraph {
	switch id {
	case physiology.CurveChemosynthesis:
		chemo := func(g *c.Globals, score int, offset float64) float64 {
			return effects.ChemosynthesisGain(g, score, 1, offset)
		}
		return []curveGraph{
			{title: "Health per chemosynthesis by pH, at Chemosynthesis 1 / 3 / 5 / 10 (per unit of size)", phSpan: 2, series: []curveGraphSeries{
				atScore(1, chemo),
				atScore(3, chemo),
				atScore(physiology.MaxAbilityScore/2, chemo),
				atScore(physiology.MaxAbilityScore, chemo),
			}},
			{title: "pH offset chemosynthesis still pays off at (C)", series: []curveGraphSeries{
				{label: "C", value: effects.ChemoWidth},
			}},
		}
	case physiology.CurveEating:
		return []curveGraph{
			{title: "Max food removed per eating attempt", series: bySizeClass(effects.MaxFoodPerEat)},
			// Flat by design — the Eating score buys capacity, not
			// nourishment — and drawn across the score axis anyway so
			// the two halves of an eat sit side by side.
			{title: "Health gained per food unit eaten", series: []curveGraphSeries{
				{label: "health", value: func(g *c.Globals, score int) float64 {
					return effects.HealthFromFood(g, 1)
				}},
			}},
		}
	case physiology.CurveMovementCost:
		return []curveGraph{
			{title: "Health cost per move (per unit of size)", series: []curveGraphSeries{perSizeMagnitude("move", effects.MoveCost)}},
			{title: "Health cost per turn (per unit of size)", series: []curveGraphSeries{perSizeMagnitude("turn", effects.TurnCost)}},
		}
	case physiology.CurveDiggingCost:
		return []curveGraph{
			{title: "Health cost per dig (per unit of size)", series: []curveGraphSeries{perSizeMagnitude("dig", effects.DigCost)}},
		}
	case physiology.CurveDiggingStrength:
		return []curveGraph{
			{title: "Wall strength cleared ahead per dig", wholeUnits: true,
				series: bySizeClass(func(g *c.Globals, s int, size float64) float64 {
					return float64(effects.DigWallRemoved(g, s, size))
				})},
			{title: "Food rooted up per dig", wholeUnits: true,
				series: bySizeClass(func(g *c.Globals, s int, size float64) float64 {
					return float64(effects.DigFood(g, s, size))
				})},
		}
	case physiology.CurveDiggingCreate:
		return []curveGraph{
			{title: "Wall strength raised per dig (each side)", wholeUnits: true,
				series: bySizeClass(func(g *c.Globals, s int, size float64) float64 {
					return float64(effects.DigWallCreated(g, s, size))
				})},
		}
	case physiology.CurveAttack:
		return []curveGraph{
			{title: "Damage dealt per attack, before the target's Defense",
				series: bySizeClass(effects.AttackDamage)},
		}
	case physiology.CurveDamageTaken:
		return []curveGraph{
			{title: "Attack damage taken (multiple of the incoming damage)", series: []curveGraphSeries{
				{label: "taken", value: effects.DamageTakenMultiplier},
			}},
		}
	case physiology.CurveThorns:
		return []curveGraph{
			{title: "Thorns damage dealt back per hit", series: bySizeClass(effects.ThornsDamage)},
		}
	case physiology.CurvePhTolerance:
		phDamage := func(g *c.Globals, score int, offset float64) float64 {
			return effects.PhDamage(g, score, 1, offset)
		}
		return []curveGraph{
			{title: "Health per cycle by pH, at Tolerance 0 / 1 / 5 / 10 (per unit of size)", phSpan: 6, series: []curveGraphSeries{
				atScore(0, phDamage),
				atScore(1, phDamage),
				atScore(physiology.MaxAbilityScore/2, phDamage),
				atScore(physiology.MaxAbilityScore, phDamage),
			}},
			{title: "pH offset borne for one unit of damage (T)", series: []curveGraphSeries{
				{label: "T", value: effects.PhToleranceWidth},
			}},
		}
	}
	return nil
}

// curveHeadingH is the line naming a curve inside an ability's block.
const curveHeadingH = 18

// curveSectionHeight is one curve's part of a block: its heading, its
// graphs stacked on the left, and its controls column on the right.
func curveSectionHeight(id physiology.CurveID) int {
	graphs := len(curveGraphsFor(id)) * curveRowHeight
	return curveHeadingH + max(graphs, curveCtrlHeight(id))
}

// abilityBlockHeight is the height of an expanded ability: one section
// per curve it drives.
func abilityBlockHeight(a physiology.Ability) int {
	h := curveGraphPadTop
	for _, id := range physiology.CurvesFor(a) {
		h += curveSectionHeight(id)
	}
	return h
}

// curveSectionAt returns which of an ability's curves covers my in a
// block at y, and where that curve's section starts.
func curveSectionAt(a physiology.Ability, y, my int) (physiology.CurveID, int, bool) {
	top := y + curveGraphPadTop
	for _, id := range physiology.CurvesFor(a) {
		h := curveSectionHeight(id)
		if my >= top && my < top+h {
			return id, top, true
		}
		top += h
	}
	return 0, 0, false
}

// curveRowTop is the top of the i-th graph in a block at y.
func curveRowTop(y, i int) int {
	return y + curveGraphPadTop + i*curveRowHeight
}

// curveCtrlX is the left edge of the controls column in a block at x, w
// wide, and the width left for the graphs beside it.
func curveCtrlX(x, w int) (ctrlX, graphW int) {
	return x + w - curveCtrlW, w - curveCtrlW - curveCtrlGap
}

// curveShapeButtonRect is the rect of the i-th shape button in the
// controls column whose top-left is (ctrlX, rowTop), laid out across
// curveShapeBtnCols columns.
func curveShapeButtonRect(ctrlX, rowTop, i int) (bx, by, bw, bh int) {
	bw = (curveCtrlW - (curveShapeBtnCols-1)*curveShapeBtnGap) / curveShapeBtnCols
	col, row := i%curveShapeBtnCols, i/curveShapeBtnCols
	return ctrlX + col*(bw+curveShapeBtnGap), rowTop + row*(curveShapeBtnH+curveShapeBtnGap), bw, curveShapeBtnH
}

// curveSliderRect is the rect of the i-th slider under the shape buttons:
// 0 is the shape's coefficient, then one per setting the curve scales.
func curveSliderRect(ctrlX, top, i int) (sx, sy, sw, sh int) {
	y := top + curveShapeGridH + 6 + i*curveCtrlSlot + curveCtrlLabelH
	return ctrlX, y, curveCtrlSliderW, curveCtrlSliderH
}

// curveSliderLabelPos is where the i-th slider's name and value go, on
// the line above it — click the line to type an exact number.
func curveSliderLabelRect(ctrlX, top, i int) (lx, ly, lw, lh int) {
	_, sy, _, _ := curveSliderRect(ctrlX, top, i)
	// Short of the column's full width: the value is right-aligned in it
	// and the column ends at the block's edge, where it would clip.
	return ctrlX, sy - curveCtrlLabelH, curveCtrlW - curveCtrlValueGap, curveCtrlLabelH
}

// curveSliderAt returns which slider (or its label) covers (mx, my).
func curveSliderAt(ctrlX, top, count, mx, my int) (i int, onLabel, ok bool) {
	for i := 0; i < count; i++ {
		sx, sy, sw, sh := curveSliderRect(ctrlX, top, i)
		if mx >= sx && mx < sx+sw && my >= sy && my < sy+sh {
			return i, false, true
		}
		lx, ly, lw, lh := curveSliderLabelRect(ctrlX, top, i)
		if mx >= lx && mx < lx+lw && my >= ly && my < ly+lh {
			return i, true, true
		}
	}
	return 0, false, false
}

// curveShapeButtonAt returns the shape whose button covers (mx, my) in a
// controls column at (ctrlX, rowTop), if any.
func curveShapeButtonAt(ctrlX, rowTop, mx, my int) (physiology.ShapeKind, bool) {
	for i, kind := range physiology.AllShapeKinds {
		bx, by, bw, bh := curveShapeButtonRect(ctrlX, rowTop, i)
		if mx >= bx && mx < bx+bw && my >= by && my < by+bh {
			return kind, true
		}
	}
	return "", false
}

// curveSlider is one knob in a curve's controls column: its name, where
// its value sits in its range, the value as text, and what the user is
// currently typing into it (empty when they aren't).
type curveSlider struct {
	label string
	ratio float64
	value string
	// editing is what the user has typed so far, and selected is whether
	// this is the slider they clicked to type into. They are separate
	// because a click opens the value before a single character has been
	// typed — with nothing drawn differently in that state, clicking the
	// line looked like it had done nothing at all.
	editing  string
	selected bool
}

// drawCurveControls paints a curve's controls column: the shape buttons
// with the active one highlighted, then a slider per knob — the shape's
// coefficient first, then the settings that curve scales, so an ability's
// numbers sit beside the graphs of what they do.
func drawCurveControls(dst *ebiten.Image, ctrlX, top int, id physiology.CurveID, g *c.Globals, sliders []curveSlider) {
	active := physiology.ShapeFor(g, id)
	for i, kind := range physiology.AllShapeKinds {
		bx, by, bw, bh := curveShapeButtonRect(ctrlX, top, i)
		bg := color.RGBA{R: 45, G: 45, B: 55, A: 255}
		fg := color.RGBA{R: 170, G: 170, B: 180, A: 255}
		if kind == active {
			bg = color.RGBA{R: 70, G: 70, B: 110, A: 255}
			fg = color.RGBA{R: 235, G: 235, B: 245, A: 255}
		}
		vector.DrawFilledRect(dst, float32(bx), float32(by), float32(bw), float32(bh), bg, false)
		tb := boundString(curveCtrlFont(), string(kind))
		text.Draw(dst, string(kind), curveCtrlFont(), bx+(bw-tb.Dx())/2, by+(bh+tb.Dy())/2, fg)
	}

	for i, s := range sliders {
		lx, ly, lw, lh := curveSliderLabelRect(ctrlX, top, i)
		value := s.value
		labelCol := color.Color(color.RGBA{R: 175, G: 175, B: 190, A: 255})
		valueCol := color.Color(color.RGBA{R: 225, G: 225, B: 235, A: 255})
		if s.selected {
			// The line is lit and carries a caret for as long as it's
			// open for typing, whether or not anything has been typed.
			vector.DrawFilledRect(dst, float32(lx-2), float32(ly), float32(lw+4), float32(lh),
				color.RGBA{R: 50, G: 50, B: 85, A: 255}, false)
			labelCol = color.RGBA{R: 200, G: 200, B: 215, A: 255}
			value, valueCol = s.editing+"_", color.RGBA{R: 120, G: 255, B: 120, A: 255}
		}
		text.Draw(dst, s.label, curveCtrlFont(), lx, ly+lh-4, labelCol)
		vb := boundString(curveCtrlFont(), value)
		text.Draw(dst, value, curveCtrlFont(), lx+lw-vb.Dx(), ly+lh-4, valueCol)

		sx, sy, sw, sh := curveSliderRect(ctrlX, top, i)
		vector.DrawFilledRect(dst, float32(sx), float32(sy), float32(sw), float32(sh), color.RGBA{R: 50, G: 50, B: 60, A: 255}, false)
		fill := float32(float64(sw) * min(1, max(0, s.ratio)))
		vector.DrawFilledRect(dst, float32(sx), float32(sy), fill, float32(sh), color.RGBA{R: 80, G: 80, B: 120, A: 255}, false)
		vector.DrawFilledRect(dst, float32(sx)+max(0, fill-2), float32(sy), 4, float32(sh), color.RGBA{R: 150, G: 150, B: 200, A: 255}, false)
	}
}

// drawCurveGraphs draws every graph for curve id into dst, top-left at
// (x, y) and w wide, using the configuration being edited.
func drawAbilityBlock(dst *ebiten.Image, x, y, w int, a physiology.Ability, g *c.Globals, sliders map[physiology.CurveID][]curveSlider) {
	ctrlX, graphW := curveCtrlX(x, w)
	top := y + curveGraphPadTop
	for _, id := range physiology.CurvesFor(a) {
		// Name the curve when the ability drives more than one, so
		// Digging's cost and strength sections don't read as one graph
		// pile.
		if len(physiology.CurvesFor(a)) > 1 {
			text.Draw(dst, id.Name(), curveGraphTitleFont(), x, top+13, color.RGBA{R: 200, G: 200, B: 215, A: 255})
		}
		drawCurveControls(dst, ctrlX, top+curveHeadingH, id, g, sliders[id])
		for i, graph := range curveGraphsFor(id) {
			drawCurveGraph(dst, x, top+curveHeadingH+i*curveRowHeight, graphW, graph, g)
		}
		top += curveSectionHeight(id)
	}
}

// phGraphSamples is how finely a pH-axis graph is sampled. Unrelated to
// the score scale: pH is continuous, so this is just plot resolution.
const phGraphSamples = 120

// curveGraphMarks are the scores whose values each graph labels.
var curveGraphMarks = []int{0, physiology.MaxAbilityScore / 2, physiology.MaxAbilityScore}

// drawCurveGraph draws one graph: its title, the effect over scores 0-100,
// and the values at scores 0, 50 and 100.
func drawCurveGraph(dst *ebiten.Image, x, y, w int, graph curveGraph, g *c.Globals) {
	face := curveGraphTitleFont()
	small := curveGraphLabelFont()
	labelCol := color.RGBA{R: 185, G: 185, B: 198, A: 255}
	dimCol := color.RGBA{R: 90, G: 90, B: 105, A: 255}

	plotX := x + curveGraphLeft
	plotY := y + curveGraphTitleH
	plotW := w - curveGraphLeft - 8

	// Title centred over the plot it describes, wrapped to fit it and
	// centred in the two-line band whether it needs one line or two.
	lines := wrapGraphTitle(graph.title, face, plotW)
	baseline := y + (curveGraphTitleH-len(lines)*curveGraphTitleLineH)/2 + curveGraphTitleLineH - 3
	for _, line := range lines {
		tb := boundString(face, line)
		text.Draw(dst, line, face, plotX+(plotW-tb.Dx())/2, baseline, labelCol)
		baseline += curveGraphTitleLineH
	}
	plotH := curveGraphPlotH
	if plotW < 40 {
		return
	}

	// Sample the axis for every series. A score axis gets one point per
	// whole score — that is every value the effect can take, since scores
	// are integers — while a pH axis is continuous and needs a fine
	// sampling to draw a smooth hump.
	samples := physiology.MaxAbilityScore
	if graph.phSpan > 0 {
		samples = phGraphSamples
	}
	values := make([][]float64, len(graph.series))
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, s := range graph.series {
		values[i] = make([]float64, samples+1)
		for j := 0; j <= samples; j++ {
			var v float64
			if graph.phSpan > 0 {
				v = s.phValue(g, (float64(j)/float64(samples)*2-1)*graph.phSpan)
			} else {
				v = s.value(g, j)
			}
			if math.IsInf(v, 0) || math.IsNaN(v) {
				v = 0
			}
			values[i][j] = v
			lo, hi = min(lo, v), max(hi, v)
		}
	}
	lo, hi = paddedGraphRange(lo, hi)
	if graph.wholeUnits {
		lo, hi = wholeUnitRange(lo, hi)
	}
	label := graph.formatValue
	toX := func(sample float64) float32 {
		return float32(plotX) + float32(sample/float64(samples))*float32(plotW)
	}
	toY := func(v float64) float32 {
		return float32(plotY) + float32((hi-v)/(hi-lo))*float32(plotH)
	}

	// Frame, zero line, and a guide at the midpoint.
	vector.DrawFilledRect(dst, float32(plotX), float32(plotY), float32(plotW), float32(plotH), color.RGBA{R: 22, G: 22, B: 30, A: 255}, false)
	vector.StrokeRect(dst, float32(plotX), float32(plotY), float32(plotW), float32(plotH), 1, dimCol, false)
	if lo < 0 && hi > 0 {
		zy := toY(0)
		vector.StrokeLine(dst, float32(plotX), zy, float32(plotX+plotW), zy, 1, dimCol, false)
	}
	marks := []struct {
		sample float64
		label  string
	}{}
	if graph.phSpan > 0 {
		for _, f := range []float64{0, 0.5, 1} {
			offset := (f*2 - 1) * graph.phSpan
			marks = append(marks, struct {
				sample float64
				label  string
			}{f * float64(samples), fmt.Sprintf("%+.0f", offset)})
		}
		marks[1].label = "ideal"
	} else {
		for _, m := range curveGraphMarks {
			marks = append(marks, struct {
				sample float64
				label  string
			}{float64(m), fmt.Sprintf("%d", m)})
		}
	}
	for _, m := range marks {
		mx := toX(m.sample)
		if m.sample > 0 && m.sample < float64(samples) {
			vector.StrokeLine(dst, mx, float32(plotY), mx, float32(plotY+plotH), 1, dimCol, false)
		}
		lb := boundString(small, m.label)
		lx := int(mx) - lb.Dx()/2
		lx = min(max(lx, plotX), plotX+plotW-lb.Dx())
		text.Draw(dst, m.label, small, lx, plotY+plotH+12, dimCol)
	}

	// Y-axis range labels.
	hiLabel, loLabel := label(hi), label(lo)
	text.Draw(dst, hiLabel, small, plotX-4-boundString(small, hiLabel).Dx(), plotY+10, labelCol)
	text.Draw(dst, loLabel, small, plotX-4-boundString(small, loLabel).Dx(), plotY+plotH-2, labelCol)

	// Series lines.
	for i := range graph.series {
		col := seriesColor(graph.series[i], i)
		for j := 1; j <= samples; j++ {
			vector.StrokeLine(dst, toX(float64(j-1)), toY(values[i][j-1]), toX(float64(j)), toY(values[i][j]), 1.5, col, true)
		}
	}

	// Marked points. Values are labelled on single-line graphs;
	// multi-line graphs get a legend instead.
	for i := range graph.series {
		col := seriesColor(graph.series[i], i)
		for _, m := range marks {
			px, py := toX(m.sample), toY(values[i][int(m.sample)])
			vector.DrawFilledCircle(dst, px, py, 2.5, col, true)
			if len(graph.series) == 1 {
				label := label(values[i][int(m.sample)])
				lb := boundString(small, label)
				lx := int(px) + 4
				if lx+lb.Dx() > plotX+plotW {
					lx = int(px) - 4 - lb.Dx()
				}
				ly := int(py) - 3
				if ly-lb.Dy() < plotY {
					ly = int(py) + lb.Dy() + 3
				}
				text.Draw(dst, label, small, lx, ly, color.White)
			}
		}
	}
	if len(graph.series) > 1 {
		lx := plotX + 4
		for i, s := range graph.series {
			col := seriesColor(s, i)
			vector.DrawFilledRect(dst, float32(lx), float32(plotY+7), 8, 3, col, false)
			text.Draw(dst, s.label, small, lx+11, plotY+12, labelCol)
			lx += 11 + boundString(small, s.label).Dx() + 10
		}
	}
}

// wrapGraphTitle splits a title that doesn't fit the plot into two
// lines, choosing the word break that leaves the two as even as it can
// — a greedy wrap would hang a word or two under a full line, which
// reads badly under a centred title. Titles that fit come back whole,
// and one that can't fit even split is left to overhang rather than
// truncated: the words are what the graph means.
func wrapGraphTitle(title string, face font.Face, w int) []string {
	if boundString(face, title).Dx() <= w {
		return []string{title}
	}
	words := strings.Fields(title)
	if len(words) < 2 {
		return []string{title}
	}
	best, bestWidth := 1, math.MaxInt
	for split := 1; split < len(words); split++ {
		left := boundString(face, strings.Join(words[:split], " ")).Dx()
		right := boundString(face, strings.Join(words[split:], " ")).Dx()
		if wide := max(left, right); wide < bestWidth {
			best, bestWidth = split, wide
		}
	}
	return []string{strings.Join(words[:best], " "), strings.Join(words[best:], " ")}
}

// paddedGraphRange gives the plot a little headroom past the values it
// draws, so a line doesn't sit on the frame — but never past zero. An
// effect that is only ever damage or only ever a gain has zero as a real
// edge of its range, and an axis that runs to -6.67 under it reads as a
// value the effect can produce. Padding still crosses zero for an effect
// that genuinely does, like the chemosynthesis hump.
func paddedGraphRange(lo, hi float64) (float64, float64) {
	pad := (hi - lo) * 0.08
	if hi-lo < 1e-9 {
		// A flat line has no range to take a fraction of.
		pad = max(math.Abs(hi)*0.1, 0.5)
	}
	outLo, outHi := lo-pad, hi+pad
	if lo >= 0 {
		outLo = max(0, outLo)
	}
	if hi <= 0 {
		outHi = min(0, outHi)
	}
	if outHi-outLo < 1e-9 {
		// Every value is exactly zero, and both clamps pinned the same
		// edge. Keep a positive axis rather than a zero-height one, which
		// would divide by zero when plotting.
		outHi = outLo + max(pad, 0.5)
	}
	return outLo, outHi
}

// wholeUnitRange rounds a padded range out to the whole numbers around
// it, so an axis for an integer effect is labelled in the units the
// effect actually takes. Widened by one when rounding collapses it,
// which happens when every value is the same whole number.
func wholeUnitRange(lo, hi float64) (float64, float64) {
	lo, hi = math.Floor(lo), math.Ceil(hi)
	if hi-lo < 1 {
		hi = lo + 1
	}
	return lo, hi
}

// formatValue renders one of this graph's values for an axis or point
// label: whole numbers for a whole-unit effect, compact otherwise.
func (g curveGraph) formatValue(v float64) string {
	if g.wholeUnits {
		return strconv.Itoa(int(math.Round(v)))
	}
	return formatGraphValue(v)
}

// formatGraphValue formats an axis or point value compactly.
func formatGraphValue(v float64) string {
	if math.Abs(v) < 1e-9 {
		return "0"
	}
	return fmt.Sprintf("%.3g", v)
}

// curveKTags maps each per-shape K setting to its curve and the shape
// that uses it. A curve shows only the K row of its current shape, so
// each shape keeps its own tuning when the user switches between them.
var curveKTags = map[string]struct {
	curve physiology.CurveID
	shape physiology.ShapeKind
}{
	"chemosynthesis_cosine_k":       {physiology.CurveChemosynthesis, physiology.ShapeCosine},
	"chemosynthesis_saturating_k":   {physiology.CurveChemosynthesis, physiology.ShapeSaturating},
	"eating_cosine_k":               {physiology.CurveEating, physiology.ShapeCosine},
	"eating_saturating_k":           {physiology.CurveEating, physiology.ShapeSaturating},
	"movement_cost_cosine_k":        {physiology.CurveMovementCost, physiology.ShapeCosine},
	"movement_cost_saturating_k":    {physiology.CurveMovementCost, physiology.ShapeSaturating},
	"digging_cost_cosine_k":         {physiology.CurveDiggingCost, physiology.ShapeCosine},
	"digging_cost_saturating_k":     {physiology.CurveDiggingCost, physiology.ShapeSaturating},
	"digging_strength_cosine_k":     {physiology.CurveDiggingStrength, physiology.ShapeCosine},
	"digging_creation_cosine_k":     {physiology.CurveDiggingCreate, physiology.ShapeCosine},
	"digging_strength_saturating_k": {physiology.CurveDiggingStrength, physiology.ShapeSaturating},
	"digging_creation_saturating_k": {physiology.CurveDiggingCreate, physiology.ShapeSaturating},
	"attack_cosine_k":               {physiology.CurveAttack, physiology.ShapeCosine},
	"attack_saturating_k":           {physiology.CurveAttack, physiology.ShapeSaturating},
	"damage_taken_cosine_k":         {physiology.CurveDamageTaken, physiology.ShapeCosine},
	"damage_taken_saturating_k":     {physiology.CurveDamageTaken, physiology.ShapeSaturating},
	"thorns_cosine_k":               {physiology.CurveThorns, physiology.ShapeCosine},
	"thorns_saturating_k":           {physiology.CurveThorns, physiology.ShapeSaturating},
	"ph_tolerance_cosine_k":         {physiology.CurvePhTolerance, physiology.ShapeCosine},
	"ph_tolerance_saturating_k":     {physiology.CurvePhTolerance, physiology.ShapeSaturating},
}

// withCurveGraphs gives each curve's K row a graph toggle, a shape picker
// above it, and the curve's (collapsible) graph row directly after it.
func withCurveGraphs(fields []configField) []configField {
	out := make([]configField, 0, len(fields)*3)
	seen := map[physiology.Ability]bool{}
	for _, f := range fields {
		tag, ok := curveKTags[f.jsonTag]
		if !ok {
			out = append(out, f)
			continue
		}
		// One row per ability, carrying the toggle for its block — an
		// ability with two curves (Digging, Defense) opens onto both.
		// The per-shape K fields stay in the section (hidden) so resets
		// and "restore defaults" still see them, and the block's sliders
		// edit them through the same fields.
		ability := tag.curve.Ability()
		if !seen[ability] {
			seen[ability] = true
			shapeTag := curveShapeTags[tag.curve]
			idx, kind := globalsField(shapeTag)
			out = append(out, configField{
				label:        ability.Name(),
				jsonTag:      shapeTag,
				kind:         kind,
				fieldIdx:     idx,
				row:          rowCurveHeader,
				curve:        tag.curve,
				curveAbility: ability,
				graphToggle:  true,
			})
		}
		f.curve = tag.curve
		f.shape = tag.shape
		f.hidden = true
		out = append(out, f)
		if f.jsonTag == curveFieldTags[tag.curve].last {
			// The settings this curve scales, hidden like the K fields:
			// the block draws them, but they stay in the section so
			// resets and "restore defaults" still see them.
			for _, tag := range curveSettingTags[tag.curve] {
				idx, kind := globalsField(tag)
				out = append(out, configField{
					label:    curveSettingLabels[tag],
					jsonTag:  tag,
					kind:     kind,
					fieldIdx: idx,
					hidden:   true,
				})
			}
			if last := physiology.CurvesFor(ability); last[len(last)-1] == tag.curve {
				out = append(out, configField{
					label:        ability.Name() + " graphs",
					row:          rowCurveGraph,
					curve:        tag.curve,
					curveAbility: ability,
				})
			}
		}
	}
	return out
}

// setCurveShape writes a curve's shape into g.
func setCurveShape(g *c.Globals, id physiology.CurveID, kind physiology.ShapeKind) {
	reflect.ValueOf(g).Elem().FieldByName(curveFieldNames[id].shape).SetString(string(kind))
}

// curveKValue reads the K the curve's current shape uses.
func curveKValue(g *c.Globals, id physiology.CurveID) float64 {
	return reflect.ValueOf(g).Elem().FieldByName(curveKFieldName(g, id)).Float()
}

// setCurveK writes the K the curve's current shape uses.
func setCurveK(g *c.Globals, id physiology.CurveID, k float64) {
	reflect.ValueOf(g).Elem().FieldByName(curveKFieldName(g, id)).SetFloat(k)
}

// curveKFieldName is the Globals field holding the K of the curve's
// current shape, or the cosine's for shapes that read none (nothing
// reads it in that case).
func curveKFieldName(g *c.Globals, id physiology.CurveID) string {
	if physiology.ShapeFor(g, id) == physiology.ShapeSaturating {
		return curveFieldNames[id].saturatingK
	}
	return curveFieldNames[id].cosineK
}

// curveKTag is the json tag of the K setting the curve's current shape
// uses, so the block's slider can drive the same config field the form
// would.
func curveKTag(g *c.Globals, id physiology.CurveID) string {
	shape := physiology.ShapeFor(g, id)
	for tag, entry := range curveKTags {
		if entry.curve == id && entry.shape == shape {
			return tag
		}
	}
	return ""
}

// curveFieldNames maps each curve to the Globals fields holding its
// per-shape K values and its shape, so the config screen can read and
// write them by curve rather than repeating a nine-way switch per
// operation.
// globalsField is the index and kind of the Globals field carrying this
// json tag, so a row built outside buildSections still resets and
// compares against the defaults like any other.
func globalsField(jsonTag string) (int, reflect.Kind) {
	t := reflect.TypeOf(c.Globals{})
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("json") == jsonTag {
			return i, t.Field(i).Type.Kind()
		}
	}
	panic("config screen: no Globals field with json tag " + jsonTag)
}

// curveSettingTags lists the settings each curve scales, shown as sliders
// in the curve's block under its shape buttons. This is what lets an
// ability be tuned in one place: the curve, its shape, and the numbers it
// multiplies, side by side with the graphs of the result.
var curveSettingTags = map[physiology.CurveID][]string{
	physiology.CurveChemosynthesis: {"max_chemosynthesis_gain", "chemo_ph_effect"},
	physiology.CurveEating: {"max_bite_at_full_eating", "health_per_food_unit", "health_change_from_eating_attempt",
		"eating_growth_factor", "eating_ph_effect"},
	physiology.CurveMovementCost: {"health_change_from_moving", "health_change_from_moving_at_max",
		"health_change_from_turning", "health_change_from_turning_at_max"},
	physiology.CurveDiggingCost: {"health_change_from_digging", "health_change_from_digging_at_max"},
	physiology.CurveDiggingStrength: {"wall_strength_delta_small", "wall_strength_delta_medium", "wall_strength_delta_large",
		"wall_strength_delta_at_zero", "food_from_digging_small", "food_from_digging_medium", "food_from_digging_large",
		"food_from_digging_at_zero"},
	physiology.CurveDiggingCreate: {"wall_created_small", "wall_created_medium", "wall_created_large",
		"wall_created_at_zero"},
	physiology.CurveAttack:      {"health_change_inflicted_by_attack", "health_change_from_attacking"},
	physiology.CurveThorns:      {"health_change_inflicted_by_thorns"},
	physiology.CurvePhTolerance: {"unhealthy_ph_damage", "max_ph_tolerance_width"},
}

// curveSettingLabels are the short names the curve blocks give the
// settings they hold — short because they sit in a narrow column beside
// the graphs.
var curveSettingLabels = map[string]string{
	"max_chemosynthesis_gain":           "max gain",
	"chemo_ph_effect":                   "pH effect",
	"max_bite_at_full_eating":           "max food",
	"health_per_food_unit":              "health/food",
	"health_change_from_eating_attempt": "attempt",
	"eating_growth_factor":              "growth",
	"eating_ph_effect":                  "pH effect",
	"health_change_from_moving":         "move @0",
	"health_change_from_moving_at_max":  "move @max",
	"health_change_from_turning":        "turn @0",
	"health_change_from_turning_at_max": "turn @max",
	"health_change_from_digging":        "dig @0",
	"health_change_from_digging_at_max": "dig @max",
	"food_from_digging_small":           "food S",
	"food_from_digging_medium":          "food M",
	"food_from_digging_large":           "food L",
	"food_from_digging_at_zero":         "food @0",
	"wall_strength_delta_at_zero":       "clear @0",
	"wall_strength_delta_small":         "clear S",
	"wall_strength_delta_medium":        "clear M",
	"wall_strength_delta_large":         "clear L",
	"wall_created_small":                "raise S",
	"wall_created_medium":               "raise M",
	"wall_created_large":                "raise L",
	"wall_created_at_zero":              "raise @0",
	"health_change_inflicted_by_attack": "damage",
	"health_change_from_attacking":      "cost",
	"health_change_inflicted_by_thorns": "thorns",
	"unhealthy_ph_damage":               "pH damage",
	"max_ph_tolerance_width":            "max width",
}

// curveShapeTags maps each curve to its shape setting's json tag, which
// the curve's header row carries so the row explains itself.
var curveShapeTags = map[physiology.CurveID]string{
	physiology.CurveChemosynthesis:  "chemosynthesis_curve_shape",
	physiology.CurveEating:          "eating_curve_shape",
	physiology.CurveMovementCost:    "movement_cost_curve_shape",
	physiology.CurveDiggingCost:     "digging_cost_curve_shape",
	physiology.CurveDiggingStrength: "digging_strength_curve_shape",
	physiology.CurveDiggingCreate:   "digging_creation_curve_shape",
	physiology.CurveAttack:          "attack_curve_shape",
	physiology.CurveDamageTaken:     "damage_taken_curve_shape",
	physiology.CurveThorns:          "thorns_curve_shape",
	physiology.CurvePhTolerance:     "ph_tolerance_curve_shape",
}

var curveFieldTags = map[physiology.CurveID]struct{ last string }{
	physiology.CurveChemosynthesis:  {"chemosynthesis_saturating_k"},
	physiology.CurveEating:          {"eating_saturating_k"},
	physiology.CurveMovementCost:    {"movement_cost_saturating_k"},
	physiology.CurveDiggingCost:     {"digging_cost_saturating_k"},
	physiology.CurveDiggingStrength: {"digging_strength_saturating_k"},
	physiology.CurveDiggingCreate:   {"digging_creation_saturating_k"},
	physiology.CurveAttack:          {"attack_saturating_k"},
	physiology.CurveDamageTaken:     {"damage_taken_saturating_k"},
	physiology.CurveThorns:          {"thorns_saturating_k"},
	physiology.CurvePhTolerance:     {"ph_tolerance_saturating_k"},
}

var curveFieldNames = map[physiology.CurveID]struct{ cosineK, saturatingK, shape string }{
	physiology.CurveChemosynthesis:  {"ChemosynthesisCosineK", "ChemosynthesisSaturatingK", "ChemosynthesisCurveShape"},
	physiology.CurveEating:          {"EatingCosineK", "EatingSaturatingK", "EatingCurveShape"},
	physiology.CurveMovementCost:    {"MovementCostCosineK", "MovementCostSaturatingK", "MovementCostCurveShape"},
	physiology.CurveDiggingCost:     {"DiggingCostCosineK", "DiggingCostSaturatingK", "DiggingCostCurveShape"},
	physiology.CurveDiggingStrength: {"DiggingStrengthCosineK", "DiggingStrengthSaturatingK", "DiggingStrengthCurveShape"},
	physiology.CurveDiggingCreate:   {"DiggingCreationCosineK", "DiggingCreationSaturatingK", "DiggingCreationCurveShape"},
	physiology.CurveAttack:          {"AttackCosineK", "AttackSaturatingK", "AttackCurveShape"},
	physiology.CurveDamageTaken:     {"DamageTakenCosineK", "DamageTakenSaturatingK", "DamageTakenCurveShape"},
	physiology.CurveThorns:          {"ThornsCosineK", "ThornsSaturatingK", "ThornsCurveShape"},
	physiology.CurvePhTolerance:     {"PhToleranceCosineK", "PhToleranceSaturatingK", "PhToleranceCurveShape"},
}

// graphCanvasFor returns a reusable offscreen image at least w × h, so a
// graph block can be drawn whole and then clipped to the visible viewport.
func (cs *ConfigScreen) graphCanvasFor(w, h int) *ebiten.Image {
	if cs.graphCanvas != nil {
		b := cs.graphCanvas.Bounds()
		if b.Dx() >= w && b.Dy() >= h {
			return cs.graphCanvas
		}
		cs.graphCanvas.Deallocate()
	}
	cs.graphCanvas = ebiten.NewImage(w, h)
	return cs.graphCanvas
}

// drawGraphRow draws an expanded curve's graphs at row y, clipped to the
// config screen's visible band.
func (cs *ConfigScreen) drawGraphRow(screen *ebiten.Image, px, y int, field configField) {
	h := cs.rowHeight(field)
	if h == 0 {
		return
	}
	clipTop, clipBottom := cs.clipRange()
	visTop, visBottom := max(y, clipTop), min(y+h, clipBottom)
	if visTop >= visBottom {
		return
	}
	w := cs.panelWidth()
	canvas := cs.graphCanvasFor(w, h)
	canvas.Clear()
	sliders := map[physiology.CurveID][]curveSlider{}
	for _, id := range physiology.CurvesFor(field.curveAbility) {
		sliders[id] = cs.curveSliders(id)
	}
	drawAbilityBlock(canvas, 0, 0, w, field.curveAbility, cs.globals, sliders)
	part := canvas.SubImage(image.Rect(0, visTop-y, w, visBottom-y)).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(px), float64(visTop))
	screen.DrawImage(part, op)
}

// drawGraphToggle draws the ▶ / ▼ button that shows or hides a curve's
// graphs, at the left of its setting row.
func (cs *ConfigScreen) drawGraphToggle(screen *ebiten.Image, px, py int, a physiology.Ability) {
	marker := "▶"
	if cs.graphExpanded[a] {
		marker = "▼"
	}
	vector.DrawFilledRect(screen, float32(px), float32(py), graphToggleW-2, cfgRowHeight-4, color.RGBA{R: 55, G: 55, B: 80, A: 255}, false)
	mb := boundString(r.FontSourceCodePro8, marker)
	text.Draw(screen, marker, r.FontSourceCodePro8, px+(graphToggleW-2-mb.Dx())/2, py+10, color.RGBA{R: 220, G: 220, B: 240, A: 255})
}
