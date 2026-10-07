package ux

import (
	"math"
	"testing"

	"golang.org/x/image/font/basicfont"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/manager"
)

// wallGraphGlobals loads the shipped defaults and hands back a copy to edit.
func wallGraphGlobals(t *testing.T) *config.Globals {
	t.Helper()
	loadKeyGlobals(t)
	g := *config.GetCurrentGlobals()
	return &g
}

// plotWallCurve samples the graph's line the way drawCurveGraph does.
func plotWallCurve(g *config.Globals) []float64 {
	graph := wallPhDiffusionGraph()
	axis := graph.xAxis()
	n := axis.samples()
	out := make([]float64, n+1)
	for j := 0; j <= n; j++ {
		out[j] = axis.read(graph.series[0], g, j, n)
	}
	return out
}

func TestWallPhGraphPlotsTheSimulationsCurve(t *testing.T) {
	g := wallGraphGlobals(t)
	g.WallPhBlockAtMax = 0.8
	g.WallPhBlockCurve = 1.7

	graph := wallPhDiffusionGraph()
	axis := graph.xAxis()
	n := axis.samples()

	for j := 0; j <= n; j++ {
		strength := int(wallAxisStrength(j, n) + 0.5)
		got := axis.read(graph.series[0], g, j, n)
		want := manager.WallPermeability(g, strength)
		if got != want {
			t.Errorf("sample %d (strength %d): graph plots %v, the simulation uses %v", j, strength, got, want)
		}
	}
}

func TestWallAxisSpansEveryWallStrength(t *testing.T) {
	n := wallAxis{}.samples()

	if got := wallAxisStrength(0, n); got != float64(manager.MinWallStrength) {
		t.Errorf("axis starts at %v, the weakest wall is %d", got, manager.MinWallStrength)
	}
	if got := wallAxisStrength(n, n); got != float64(manager.MaxWallStrength) {
		t.Errorf("axis ends at %v, the strongest wall is %d", got, manager.MaxWallStrength)
	}
	for j := 0; j <= n; j++ {
		s := wallAxisStrength(j, n)
		if math.Abs(s-math.Round(s)) > 1e-9 {
			t.Errorf("sample %d is strength %v, which no wall can have", j, s)
		}
	}

	// 0 is open water, not a weak wall.
	if manager.MinWallStrength < 1 {
		t.Errorf("MinWallStrength is %d; the axis assumes a wall is at least 1", manager.MinWallStrength)
	}

	marks := wallAxis{}.marks(n)
	want := []string{"1", "50", "100"}
	for i, m := range marks {
		if m.label != want[i] {
			t.Errorf("mark %d reads %q, want %q", i, m.label, want[i])
		}
	}
}

func TestWallPhGraphFollowsItsSettings(t *testing.T) {
	base := wallGraphGlobals(t)
	baseline := plotWallCurve(base)

	shallower := *base
	shallower.WallPhBlockAtMax = 0.4
	if sameCurve(baseline, plotWallCurve(&shallower)) {
		t.Error("halving wall_ph_block_at_max left the curve unchanged")
	}

	steeper := *base
	steeper.WallPhBlockCurve = 3
	if sameCurve(baseline, plotWallCurve(&steeper)) {
		t.Error("raising wall_ph_block_curve left the curve unchanged")
	}

	mid := len(baseline) / 2
	if !(plotWallCurve(&steeper)[mid] > baseline[mid]) {
		t.Errorf("at curve 3 a half-strength wall passes %v, no more than the linear ramp's %v",
			plotWallCurve(&steeper)[mid], baseline[mid])
	}
}

func TestWallPhGraphIsNotAConstantLine(t *testing.T) {
	g := wallGraphGlobals(t)
	values := plotWallCurve(g)

	lo, hi := values[0], values[0]
	for _, v := range values {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	if hi-lo < 1e-9 {
		t.Errorf("the curve is flat at %v under the shipped defaults; it needs no graph", lo)
	}
}

func TestWallPhGraphStaysInItsBounds(t *testing.T) {
	base := wallGraphGlobals(t)

	for _, atMax := range []float64{0, 0.5, 1} {
		for _, curve := range []float64{0, 0.5, 1, 2, 4} {
			g := *base
			g.WallPhBlockAtMax, g.WallPhBlockCurve = atMax, curve
			for j, v := range plotWallCurve(&g) {
				if v < 0 || v > 1 {
					t.Errorf("at %v/%v sample %d is %v, outside 0..1", atMax, curve, j, v)
				}
			}
		}
	}
}

func TestWallPhGraphTitleFitsItsPlot(t *testing.T) {
	plotW := wallPhGraphWidth - curveGraphLeft - 8
	// The real face isn't loaded in tests.
	face, fit := basicfont.Face7x13, plotW*92/100

	graph := wallPhDiffusionGraph()
	lines := wrapGraphTitle(graph.title, face, plotW)
	if len(lines) > 2 {
		t.Errorf("title %q wraps to %d lines, the band holds 2", graph.title, len(lines))
	}
	for _, line := range lines {
		if w := boundString(face, line).Dx(); w > fit {
			t.Errorf("title line %q is %dpx, plot holds %d", line, w, fit)
		}
	}
}

func sameCurve(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
