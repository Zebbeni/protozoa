package ux

// graphView is the visible window of the panel's graph, as fractions of
// the graph's full time range: [start, end] within [0, 1]. The zero value
// is treated as the full range.
//
// Kept as fractions rather than cycles so the same zoom applies across
// graph modes, which all share the time axis.
type graphView struct {
	start, end float64
}

// minGraphViewSpan is the narrowest window zoom allows: 1/256 of the run.
// Past a pixel per bar in the rendered image there is no more detail, and
// a runaway zoom would otherwise leave the graph showing a single smear.
const minGraphViewSpan = 1.0 / 256

// graphZoomStep is how much one wheel notch zooms.
const graphZoomStep = 1.25

// bounds returns the window, treating the zero value as the full range.
func (v graphView) bounds() (start, end float64) {
	if v.end <= v.start {
		return 0, 1
	}
	return v.start, v.end
}

func (v graphView) span() float64 {
	s, e := v.bounds()
	return e - s
}

// zoomed reports whether the window shows less than the full range.
func (v graphView) zoomed() bool {
	return v.span() < 1-1e-9
}

// zoomFactor is how many times magnified the window is.
func (v graphView) zoomFactor() float64 {
	return 1 / v.span()
}

// toFull maps a position across the visible graph (0 = left edge, 1 =
// right edge) to a fraction of the full range.
func (v graphView) toFull(visible float64) float64 {
	s, _ := v.bounds()
	return s + visible*v.span()
}

// zoomAt magnifies the window by factor (>1 zooms in, <1 out), keeping the
// point under anchor — a position across the visible graph — fixed on
// screen.
func (v *graphView) zoomAt(anchor, factor float64) {
	if factor <= 0 {
		return
	}
	pivot := v.toFull(anchor)
	newSpan := min(1, max(minGraphViewSpan, v.span()/factor))
	v.start = pivot - anchor*newSpan
	v.end = v.start + newSpan
	v.clamp()
}

// panBy shifts the window by delta visible widths; positive moves later
// in time.
func (v *graphView) panBy(delta float64) {
	span := v.span()
	s, _ := v.bounds()
	v.start = s + delta*span
	v.end = v.start + span
	v.clamp()
}

// reset returns to the full range.
func (v *graphView) reset() {
	v.start, v.end = 0, 1
}

// clamp keeps the window inside [0, 1] without changing its span.
func (v *graphView) clamp() {
	span := min(1, max(minGraphViewSpan, v.end-v.start))
	v.start = min(1-span, max(0, v.start))
	v.end = v.start + span
}
