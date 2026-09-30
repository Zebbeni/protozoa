package ux

// graphView is the visible window of the panel's graph, as fractions of the graph's full time range.
type graphView struct {
	start, end float64
	// rangeStart / rangeEnd are the cycles the full range covered when the window was last synced.
	rangeStart, rangeEnd int
}

// minGraphViewSpan is the narrowest window zoom allows: 1/256 of the run.
const minGraphViewSpan = 1.0 / 256

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

func (v graphView) zoomFactor() float64 {
	return 1 / v.span()
}

// toFull maps a position across the visible graph (0 = left edge, 1 = right edge) to a fraction of the full range.
func (v graphView) toFull(visible float64) float64 {
	s, _ := v.bounds()
	return s + visible*v.span()
}

// toVisible maps a fraction of the full range to a position across the visible graph, the inverse of toFull.
func (v graphView) toVisible(full float64) float64 {
	s, _ := v.bounds()
	return (full - s) / v.span()
}

// zoomAt magnifies the window by factor (>1 zooms in, <1 out), keeping the point under anchor.
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

// panBy shifts the window by delta visible widths; positive moves later in time.
func (v *graphView) panBy(delta float64) {
	span := v.span()
	s, _ := v.bounds()
	v.start = s + delta*span
	v.end = v.start + span
	v.clamp()
}

// syncRange tells the view the full range now covers cycles [start, end].
func (v *graphView) syncRange(start, end int) {
	oldStart, oldEnd := v.rangeStart, v.rangeEnd
	v.rangeStart, v.rangeEnd = start, end
	if oldEnd <= oldStart || end <= start || (start == oldStart && end == oldEnd) || !v.zoomed() {
		return
	}
	s, e := v.bounds()
	if e >= 1-1e-9 {
		return
	}
	toCycle := func(f float64) float64 { return float64(oldStart) + f*float64(oldEnd-oldStart) }
	toFraction := func(cycle float64) float64 { return (cycle - float64(start)) / float64(end-start) }
	v.start, v.end = toFraction(toCycle(s)), toFraction(toCycle(e))
	v.clamp()
}

func (v *graphView) reset() {
	v.start, v.end = 0, 1
}

// clamp keeps the window inside [0, 1] without changing its span.
func (v *graphView) clamp() {
	span := min(1, max(minGraphViewSpan, v.end-v.start))
	v.start = min(1-span, max(0, v.start))
	v.end = v.start + span
}
