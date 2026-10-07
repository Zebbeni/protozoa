package helpers

import "testing"

func TestProgressFraction(t *testing.T) {
	var p Progress
	if _, ok := p.Fraction(); ok {
		t.Error("progress with no declared work should not report a fraction")
	}
	p.AddWork(4)
	p.Step()
	if f, ok := p.Fraction(); !ok || f != 0.25 {
		t.Errorf("fraction = %v, %v; want 0.25, true", f, ok)
	}
	for i := 0; i < 10; i++ {
		p.Step()
	}
	if f, _ := p.Fraction(); f != 1 {
		t.Errorf("overshooting steps should clamp to 1, got %v", f)
	}
}

func TestNilProgressIsSafe(t *testing.T) {
	var p *Progress
	p.AddWork(3)
	p.Step()
	if _, ok := p.Fraction(); ok {
		t.Error("nil progress should report nothing")
	}
}
