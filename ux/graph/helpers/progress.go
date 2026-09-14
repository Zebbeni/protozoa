package helpers

import (
	"image/color"
	"sync/atomic"

	c "github.com/Zebbeni/protozoa/config"
)

// Progress counts work units for one background graph render, so the UI
// can show how far along a slow render is. Renderers declare the units
// they are about to do with AddWork and tick them off with Step; the main
// goroutine reads Fraction. Both sides are lock-free.
//
// All methods are safe on a nil *Progress, so a renderer can report
// unconditionally and a caller that doesn't care passes nil.
type Progress struct {
	done, total atomic.Int64
}

// AddWork declares n more units of work.
func (p *Progress) AddWork(n int) {
	if p != nil && n > 0 {
		p.total.Add(int64(n))
	}
}

// Step marks one unit of declared work as done.
func (p *Progress) Step() {
	if p != nil {
		p.done.Add(1)
	}
}

// Fraction returns completed work as a share of declared work, clamped to
// [0, 1]. ok is false until some work has been declared, since a render
// that has reported nothing has no meaningful progress to show.
func (p *Progress) Fraction() (fraction float64, ok bool) {
	if p == nil {
		return 0, false
	}
	total := p.total.Load()
	if total <= 0 {
		return 0, false
	}
	return min(1, max(0, float64(p.done.Load())/float64(total))), true
}

// GraphBackground is the fill behind graph series: just off the panel
// fill, so the plot area is visible without competing with the data.
func GraphBackground() color.RGBA {
	if c.IsLightTheme() {
		return color.RGBA{R: 235, G: 235, B: 240, A: 255}
	}
	return color.RGBA{R: 30, G: 30, B: 35, A: 255}
}
