package helpers

import (
	"image/color"
	"sync/atomic"

	c "github.com/Zebbeni/protozoa/config"
)

// Progress counts work units for one background graph render, so the UI can show how far along a slow render is.
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

// Fraction returns completed work as a share of declared work, clamped to [0, 1].
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

func GraphBackground() color.RGBA {
	if c.IsLightTheme() {
		return color.RGBA{R: 235, G: 235, B: 240, A: 255}
	}
	return color.RGBA{R: 30, G: 30, B: 35, A: 255}
}
