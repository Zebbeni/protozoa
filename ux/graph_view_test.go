package ux

import (
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
