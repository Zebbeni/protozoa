package organism

import (
	"math"
	"testing"
)

// TestLineageEnds: a line with a survivor has no end all the way up; a line
// that died out records its latest death.
func TestLineageEnds(t *testing.T) {
	// root(0, died 30) ─┬─ a(10, died 40) ── a1(30, died 60)
	//                   └─ b(20, died 50) ── b1(40, alive)
	// other(0, died 15)
	root := &DescendantNode{ID: 1, StartCycle: 0, EndCycle: 30}
	a := &DescendantNode{ID: 2, StartCycle: 10, EndCycle: 40, Parent: root}
	a1 := &DescendantNode{ID: 3, StartCycle: 30, EndCycle: 60, Parent: a}
	b := &DescendantNode{ID: 4, StartCycle: 20, EndCycle: 50, Parent: root}
	b1 := &DescendantNode{ID: 5, StartCycle: 40, Parent: b}
	other := &DescendantNode{ID: 6, StartCycle: 0, EndCycle: 15}
	root.AddChild(a)
	root.AddChild(b)
	a.AddChild(a1)
	b.AddChild(b1)

	if end := ComputeLineageEnds(map[int]*DescendantNode{1: root, 6: other}); end != 60 {
		t.Errorf("run end %d, want 60 (the latest birth or death)", end)
	}
	for _, tc := range []struct {
		name string
		node *DescendantNode
		want int
	}{
		{"survivor", b1, 0},
		{"parent of a survivor", b, 0},
		{"founder of a surviving line", root, 0},
		{"line ending at 60", a, 60},
		{"its last member", a1, 60},
		{"line that died at 15", other, 15},
	} {
		if tc.node.LineageEndCycle != tc.want {
			t.Errorf("%s: lineage end %d, want %d", tc.name, tc.node.LineageEndCycle, tc.want)
		}
	}
}

// TestLineageSuccessUsesTimeLeft pins the ratio: cycles already elapsed
// come off both the lineage end and the run end.
func TestLineageSuccessUsesTimeLeft(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		lineageEnd, now, runEnd int
		want                    float64
	}{
		{"from the start, dies halfway", 50, 0, 100, 0.5},
		{"viewed halfway, dies at 75", 75, 50, 100, 0.5},
		{"viewed halfway, dies at 60", 60, 50, 100, 0.2},
		{"dies just as the run ends", 100, 50, 100, 1},
		{"a descendant survives", 0, 50, 100, 1},
		{"live run, end unknown", 0, 50, 0, 1},
		{"viewed at the end", 90, 100, 100, 1},
	} {
		if got := LineageSuccess(tc.lineageEnd, tc.now, tc.runEnd); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: success %.3f, want %.3f", tc.name, got, tc.want)
		}
	}
}
