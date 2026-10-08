package ux

import "time"

// zoomRepeatDelay is how soon a scroll that never stops may step the zoom
// again. Short enough that holding a trackpad swipe keeps zooming, long
// enough that it is a rate the eye can follow.
const zoomRepeatDelay = 120 * time.Millisecond

// wheelStepper turns a wheel reading into at most one zoom step.
//
// The reading is not comparable between platforms. Ebitengine hands the
// browser's raw WheelEvent.deltaY through (internal/ui/input_js.go, with a
// TODO acknowledging it ignores deltaMode), so one notch is about 100 there
// against 1 on a desktop build — and smooth scrolling fires a burst of
// events spread over many frames. Stepping once per frame with a non-zero
// wheel therefore ran a single notch to maximum zoom in the browser.
//
// Only the SIGN is used, never the magnitude. A step is taken on the first
// frame of a gesture, and then no more often than zoomRepeatDelay for as
// long as the wheel keeps reporting.
type wheelStepper struct {
	now      func() time.Time
	idle     bool
	last     time.Time
	interval time.Duration
}

func newWheelStepper() *wheelStepper {
	return &wheelStepper{now: time.Now, idle: true, interval: zoomRepeatDelay}
}

// step returns -1, 0 or +1 for this frame's wheel reading.
func (w *wheelStepper) step(dy float64) int {
	if dy == 0 {
		w.idle = true
		return 0
	}
	started := w.idle
	w.idle = false
	if !started && w.now().Sub(w.last) < w.interval {
		return 0
	}
	w.last = w.now()
	if dy > 0 {
		return 1
	}
	return -1
}

// wheelScrollSteps is the scroll direction for this frame: -1, 0 or +1.
//
// Scrolling takes the sign for the same reason zooming does, but needs no
// gesture debounce: one step a FRAME is exactly right in both places. A
// desktop notch is one non-zero frame, so it scrolls one step. A browser's
// smooth-scroll burst is many consecutive non-zero frames, so it scrolls
// smoothly for as long as the gesture lasts.
//
// What it must never do is use the magnitude. Ebitengine hands the browser's
// raw deltaY through, about 100 a notch against 1 on a desktop build, so
// `scrollY -= wy * 20` moved 2000px a notch in a browser.
func wheelScrollSteps(dy float64) float64 {
	switch {
	case dy > 0:
		return 1
	case dy < 0:
		return -1
	}
	return 0
}
