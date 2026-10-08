package ux

import (
	"testing"
	"time"
)

func testStepper() (*wheelStepper, *time.Time) {
	now := time.Now()
	w := newWheelStepper()
	w.now = func() time.Time { return now }
	return w, &now
}

// TestOneNotchIsOneStep: a desktop notch reads 1 for a frame and 0 the next,
// so each notch steps once.
func TestOneNotchIsOneStep(t *testing.T) {
	w, _ := testStepper()
	for i := 0; i < 5; i++ {
		if got := w.step(1); got != 1 {
			t.Fatalf("notch %d stepped %d, want 1", i, got)
		}
		if got := w.step(0); got != 0 {
			t.Fatalf("the gap after notch %d stepped %d, want 0", i, got)
		}
	}
}

// TestABrowserBurstIsOneStep is the reported bug: one physical notch fires a
// burst of smooth-scroll events across many frames, and stepping once per
// non-zero frame ran the zoom to maximum.
func TestABrowserBurstIsOneStep(t *testing.T) {
	w, _ := testStepper()
	steps := 0
	// 30 frames of a browser's deltaY, which is about 100 a notch and never
	// returns to zero during the gesture.
	for i := 0; i < 30; i++ {
		steps += w.step(100)
	}
	if steps != 1 {
		t.Errorf("a 30-frame burst stepped the zoom %d times, want 1", steps)
	}
}

// TestOnlyTheSignIsUsed: the magnitude is not comparable between platforms,
// so it must not reach the zoom.
func TestOnlyTheSignIsUsed(t *testing.T) {
	for _, dy := range []float64{0.5, 1, 100, 4000} {
		w, _ := testStepper()
		if got := w.step(dy); got != 1 {
			t.Errorf("dy %v stepped %d, want 1", dy, got)
		}
	}
	for _, dy := range []float64{-0.5, -1, -100, -4000} {
		w, _ := testStepper()
		if got := w.step(dy); got != -1 {
			t.Errorf("dy %v stepped %d, want -1", dy, got)
		}
	}
}

// TestASustainedScrollKeepsZoomingSlowly: a trackpad swipe may never read
// zero, and should still zoom rather than stop dead after one step.
func TestASustainedScrollKeepsZoomingSlowly(t *testing.T) {
	w, now := testStepper()
	if got := w.step(100); got != 1 {
		t.Fatalf("the first frame stepped %d, want 1", got)
	}
	if got := w.step(100); got != 0 {
		t.Errorf("the next frame stepped %d; the repeat delay has not elapsed", got)
	}
	*now = now.Add(zoomRepeatDelay)
	if got := w.step(100); got != 1 {
		t.Errorf("after the repeat delay a sustained scroll stepped %d, want 1", got)
	}
}

// TestADirectionChangeIsNotDelayed: reversing mid-gesture is a new gesture
// only after the wheel reads zero, but a reversal that follows a gap must
// act at once.
func TestADirectionChangeIsNotDelayed(t *testing.T) {
	w, _ := testStepper()
	if got := w.step(100); got != 1 {
		t.Fatalf("stepped %d in, want 1", got)
	}
	w.step(0)
	if got := w.step(-100); got != -1 {
		t.Errorf("stepped %d out after a gap, want -1", got)
	}
}

// TestScrollTakesTheSignNotTheMagnitude: ebitengine hands the browser's raw
// deltaY through, about 100 a notch against 1 on desktop, so a scroll
// handler multiplying by the reading moved 2000px a notch in a browser.
func TestScrollTakesTheSignNotTheMagnitude(t *testing.T) {
	for _, dy := range []float64{1, 3, 100, 4000} {
		if got := wheelScrollSteps(dy); got != 1 {
			t.Errorf("dy %v scrolls %v, want 1", dy, got)
		}
	}
	for _, dy := range []float64{-1, -3, -100, -4000} {
		if got := wheelScrollSteps(dy); got != -1 {
			t.Errorf("dy %v scrolls %v, want -1", dy, got)
		}
	}
	if got := wheelScrollSteps(0); got != 0 {
		t.Errorf("an idle wheel scrolls %v, want 0", got)
	}
}

// TestScrollIsNotDebounced: unlike zooming, one step a FRAME is right here.
// A desktop notch is one non-zero frame; a browser burst is many, which
// scrolls smoothly for as long as the gesture lasts.
func TestScrollIsNotDebounced(t *testing.T) {
	total := 0.0
	for i := 0; i < 30; i++ {
		total += wheelScrollSteps(100)
	}
	if total != 30 {
		t.Errorf("a 30-frame burst scrolled %v steps, want 30", total)
	}
}
