package ux

import (
	"math"
	"testing"

	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

func TestGraphViewZeroValueIsFullRange(t *testing.T) {
	var v graphView
	if s, e := v.bounds(); s != 0 || e != 1 || v.zoomed() {
		t.Errorf("zero view = [%g, %g], zoomed %v; want the full range", s, e, v.zoomed())
	}
}

// TestZoomKeepsAnchorInPlace: the point under the cursor stays under the
// cursor as the view zooms in and out.
func TestZoomKeepsAnchorInPlace(t *testing.T) {
	var v graphView
	const anchor = 0.3
	before := v.toFull(anchor)
	v.zoomAt(anchor, 4)
	if !approx(v.span(), 0.25) {
		t.Errorf("span after 4x zoom %.3f, want 0.25", v.span())
	}
	if after := v.toFull(anchor); !approx(after, before) {
		t.Errorf("anchor moved from %.3f to %.3f", before, after)
	}
	v.zoomAt(anchor, 0.5)
	if after := v.toFull(anchor); !approx(after, before) {
		t.Errorf("anchor moved zooming out: %.3f to %.3f", before, after)
	}
}

// TestZoomAndPanStayInRange: the window never leaves [0, 1] or zooms past
// its limits.
func TestZoomAndPanStayInRange(t *testing.T) {
	var v graphView
	v.zoomAt(1, 1e9)
	if !approx(v.span(), minGraphViewSpan) {
		t.Errorf("zoom-in limit: span %.5f, want %.5f", v.span(), minGraphViewSpan)
	}
	if s, e := v.bounds(); s < 0 || e > 1+1e-12 {
		t.Errorf("zoomed at the right edge left the range: [%g, %g]", s, e)
	}
	v.panBy(1e6)
	if _, e := v.bounds(); !approx(e, 1) {
		t.Errorf("panning past the end should stop at 1, got %g", e)
	}
	v.panBy(-1e6)
	if s, _ := v.bounds(); !approx(s, 0) {
		t.Errorf("panning past the start should stop at 0, got %g", s)
	}
	v.zoomAt(0.5, 1e-9)
	if v.zoomed() {
		t.Errorf("zooming out should stop at the full range, span %.3f", v.span())
	}
}

func TestPanMovesByVisibleWidths(t *testing.T) {
	var v graphView
	v.zoomAt(0, 10) // [0, 0.1]
	v.panBy(0.5)
	if s, e := v.bounds(); !approx(s, 0.05) || !approx(e, 0.15) {
		t.Errorf("half-width pan: [%g, %g], want [0.05, 0.15]", s, e)
	}
	v.reset()
	if v.zoomed() {
		t.Error("reset should return to the full range")
	}
}

func TestGraphImageWidth(t *testing.T) {
	for _, tc := range []struct{ bars, want int }{
		{1, int(gh.RealGraphWidth)},
		{2500, 2500},
		{100000, gh.MaxGraphImageWidth},
	} {
		if got := gh.GraphImageWidth(tc.bars); got != tc.want {
			t.Errorf("GraphImageWidth(%d) = %d, want %d", tc.bars, got, tc.want)
		}
	}
}

// TestDetachedWindowHoldsItsCycles: a zoomed window that doesn't reach the
// latest cycle keeps showing the same cycles as the run grows.
func TestDetachedWindowHoldsItsCycles(t *testing.T) {
	var v graphView
	v.syncRange(0, 1000)
	v.zoomAt(0, 4) // cycles 0-250
	v.panBy(1)     // cycles 250-500
	v.syncRange(0, 2000)
	if s, e := v.bounds(); !approx(s, 0.125) || !approx(e, 0.25) {
		t.Errorf("after the run doubled, window = [%.3f, %.3f], want cycles 250-500 = [0.125, 0.25]", s, e)
	}
}

// TestWindowAtLatestCycleKeepsFollowing: a window touching the right edge
// still shows the latest cycle as the run grows.
func TestWindowAtLatestCycleKeepsFollowing(t *testing.T) {
	var v graphView
	v.syncRange(0, 1000)
	v.zoomAt(1, 4)
	v.syncRange(0, 2000)
	if _, e := v.bounds(); !approx(e, 1) {
		t.Errorf("window end %.3f after the run grew, want 1 (following)", e)
	}
	// Panning back to the right edge from a detached window resumes following.
	v.panBy(-2)
	v.syncRange(0, 3000)
	if _, e := v.bounds(); approx(e, 1) {
		t.Fatal("a detached window shouldn't follow")
	}
	v.panBy(10)
	v.syncRange(0, 4000)
	if _, e := v.bounds(); !approx(e, 1) {
		t.Errorf("window end %.3f after panning back to the edge, want 1", e)
	}
}

// TestDetachedWindowClampsOnBackwardSeek: when a seek shrinks the range
// under a detached window, the window stays inside the graph.
func TestDetachedWindowClampsOnBackwardSeek(t *testing.T) {
	var v graphView
	v.syncRange(0, 1000)
	v.zoomAt(0.5, 4) // cycles 375-625
	v.syncRange(0, 500)
	if s, e := v.bounds(); s < 0 || e > 1+1e-9 || !approx(e-s, 0.5) {
		t.Errorf("window [%.3f, %.3f] after seeking back, want inside [0, 1] covering 250 cycles", s, e)
	}
}

// TestToVisibleInvertsToFull: the playhead is placed by mapping a
// fraction of the run back onto the zoomed window, so the two mappings
// have to agree — a playhead drawn through toVisible must land under the
// bar toFull would report at that position.
func TestToVisibleInvertsToFull(t *testing.T) {
	for _, v := range []graphView{{}, {start: 0.25, end: 0.5}, {start: 0.9, end: 1}} {
		for _, pos := range []float64{0, 0.25, 0.5, 1} {
			if got := v.toVisible(v.toFull(pos)); math.Abs(got-pos) > 1e-9 {
				t.Errorf("view %v: toVisible(toFull(%g)) = %g", v, pos, got)
			}
		}
	}

	// A point outside the window reports outside [0, 1], which is what
	// tells the panel not to draw the playhead at all.
	v := graphView{start: 0.5, end: 0.75}
	if got := v.toVisible(0.1); got >= 0 {
		t.Errorf("a point left of the window mapped to %g, want negative", got)
	}
	if got := v.toVisible(0.9); got <= 1 {
		t.Errorf("a point right of the window mapped to %g, want past 1", got)
	}
}

// TestPlayheadFractionClamps: a seek past either end of the graph's
// range still puts the marker on the graph, and an empty range has
// nowhere to put it.
func TestPlayheadFractionClamps(t *testing.T) {
	for _, tc := range []struct {
		start, current, end int
		want                float64
		wantOK              bool
	}{
		{0, 0, 1000, 0, true},
		{0, 250, 1000, 0.25, true},
		{0, 1000, 1000, 1, true},
		{0, 5000, 1000, 1, true},
		{400, 500, 900, 0.2, true},
		{400, 100, 900, 0, true},
		{0, 0, 0, 0, false},
	} {
		got, ok := playheadFraction(tc.start, tc.current, tc.end)
		if ok != tc.wantOK || math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("playheadFraction(%d, %d, %d) = %g, %v; want %g, %v",
				tc.start, tc.current, tc.end, got, ok, tc.want, tc.wantOK)
		}
	}
}
