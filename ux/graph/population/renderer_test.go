package population

import "testing"

// TestBaseImageWidthIsBounded is the regression test for the long-replay
// crash: the base image grew two pixels per bar, and at 8833 bars asked
// ebiten for a 17666px texture, which panics. Whatever the bar count, the
// columns the renderer allocates must stay within maxBaseWidth.
func TestBaseImageWidthIsBounded(t *testing.T) {
	for _, numCols := range []int{1, 2000, maxBaseWidth, maxBaseWidth + 1, 8833, 100_000, 5_000_000} {
		stride := strideFor(numCols)
		cols := columnsFor(numCols, stride)
		if cols > maxBaseWidth {
			t.Errorf("%d bars: %d columns at stride %d exceeds maxBaseWidth %d", numCols, cols, stride, maxBaseWidth)
		}
		// Every bar must land in an allocated column.
		if last := (numCols - 1) / stride; last >= cols {
			t.Errorf("%d bars: last bar maps to column %d of %d", numCols, last, cols)
		}
	}
}

// TestStrideOnlyDoubles: a stride change forces a full rebuild, so it must
// change rarely as a run grows, and never below maxBaseWidth bars.
func TestStrideOnlyDoubles(t *testing.T) {
	if s := strideFor(maxBaseWidth); s != 1 {
		t.Errorf("stride at maxBaseWidth bars = %d, want 1", s)
	}
	prev, changes := 1, 0
	for n := 1; n <= 64*maxBaseWidth; n += 97 {
		s := strideFor(n)
		if s != prev {
			if s != prev*2 {
				t.Fatalf("stride jumped %d -> %d at %d bars", prev, s, n)
			}
			changes++
			prev = s
		}
	}
	if changes > 6 {
		t.Errorf("stride changed %d times over 64x maxBaseWidth bars", changes)
	}
}
