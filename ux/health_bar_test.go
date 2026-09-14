package ux

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestHealthBarExample pins the worked example: size 35 with health 21 is
// divided into 3.5 segments across the full width, with 60% filled — the
// first two segments full and 10% of the third.
func TestHealthBarExample(t *testing.T) {
	const width = 35.0 // one pixel per point keeps the numbers readable
	fill, dividers := healthBarGeometry(21, 35, width)

	if !approx(fill, 21) {
		t.Errorf("fill %.2f px of %.0f, want 21 (60%%)", fill, width)
	}
	want := []float64{10, 20, 30}
	if len(dividers) != len(want) {
		t.Fatalf("dividers %v, want %v", dividers, want)
	}
	for i := range want {
		if !approx(dividers[i], want[i]) {
			t.Errorf("divider %d at %.2f, want %.2f", i, dividers[i], want[i])
		}
	}
	if third := (fill - 20) / 10; !approx(third, 0.1) {
		t.Errorf("third segment %.0f%% filled, want 10%%", third*100)
	}
}

// TestHealthBarSpansFullWidth: size changes the number of segments, never
// the bar's width, and a healthy organism fills all of it.
func TestHealthBarSpansFullWidth(t *testing.T) {
	for _, size := range []float64{5, 35, 100} {
		fill, dividers := healthBarGeometry(size, size, 96)
		if !approx(fill, 96) {
			t.Errorf("size %.0f at full health: fill %.2f, want the full 96", size, fill)
		}
		wantDividers := int(math.Ceil(size/healthBarSegment)) - 1
		if len(dividers) != wantDividers {
			t.Errorf("size %.0f: %d dividers, want %d", size, len(dividers), wantDividers)
		}
		for _, d := range dividers {
			if d <= 0 || d >= 96 {
				t.Errorf("size %.0f: divider at %.2f lies outside the bar", size, d)
			}
		}
	}
}

func TestHealthBarClamps(t *testing.T) {
	if f, _ := healthBarGeometry(-5, 20, 96); f != 0 {
		t.Errorf("negative health should fill nothing, got %.2f", f)
	}
	if f, _ := healthBarGeometry(50, 20, 96); !approx(f, 96) {
		t.Errorf("health above size should fill exactly the full width, got %.2f", f)
	}
	if f, d := healthBarGeometry(5, 0, 96); f != 0 || d != nil {
		t.Error("zero size should draw nothing")
	}
}
