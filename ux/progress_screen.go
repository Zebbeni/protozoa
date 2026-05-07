package ux

import (
	"fmt"
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// ProgressScreen shows simulation progress while running headless,
// with a scrollable log and Stop/Explore buttons.
type ProgressScreen struct {
	logs    []string
	mu      sync.Mutex
	scrollY float64

	stopped  bool // simulation has stopped (done or user clicked Stop)
	explored bool // user clicked Explore

	stopRequested bool // signals the sim goroutine to stop
}

func NewProgressScreen() *ProgressScreen {
	return &ProgressScreen{}
}

// AddLog appends a log line (thread-safe, called from sim goroutine).
func (p *ProgressScreen) AddLog(line string) {
	p.mu.Lock()
	p.logs = append(p.logs, line)
	p.mu.Unlock()
}

// SetStopped marks the simulation as finished (thread-safe).
func (p *ProgressScreen) SetStopped() {
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
}

// IsStopRequested returns true if the user clicked Stop.
func (p *ProgressScreen) IsStopRequested() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopRequested
}

// IsExplored returns true if the user clicked Explore.
func (p *ProgressScreen) IsExplored() bool {
	return p.explored
}

// Update handles input. Returns true when the user clicks Explore.
func (p *ProgressScreen) Update() bool {
	// Scroll
	_, wy := ebiten.Wheel()
	p.scrollY -= wy * 20
	if p.scrollY < 0 {
		p.scrollY = 0
	}

	// Button click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		p.handleClick(mx, my)
	}

	return p.explored
}

// Draw renders the progress screen.
func (p *ProgressScreen) Draw(screen *ebiten.Image) {
	fillThemeBackground(screen)

	sw := config.ScreenWidth()
	sh := config.ScreenHeight()
	panelW := 600
	panelX := (sw - panelW) / 2

	// Title
	title := "SIMULATION RUNNING..."
	p.mu.Lock()
	if p.stopped {
		title = "SIMULATION COMPLETE"
	}
	logsCopy := make([]string, len(p.logs))
	copy(logsCopy, p.logs)
	p.mu.Unlock()

	titleBounds := boundString(r.FontSourceCodePro12, title)
	titleX := panelX + (panelW-titleBounds.Dx())/2
	text.Draw(screen, title, r.FontSourceCodePro12, titleX, 30, themedForeground())

	// Scrollable log area
	lineHeight := r.FontSourceCodePro10.Metrics().Height.Round()
	logTop := 50
	logBottom := sh - 60
	logHeight := logBottom - logTop

	// Auto-scroll to bottom
	totalLogHeight := len(logsCopy) * lineHeight
	if totalLogHeight > logHeight {
		p.scrollY = float64(totalLogHeight - logHeight)
	}

	y := logTop - int(p.scrollY)
	for _, line := range logsCopy {
		if y+lineHeight > logTop && y < logBottom {
			text.Draw(screen, line, r.FontSourceCodePro10, panelX, y+lineHeight, color.RGBA{R: 180, G: 180, B: 180, A: 255})
		}
		y += lineHeight
	}

	// Draw log area border
	ebitenutil.DrawRect(screen, float64(panelX-5), float64(logTop-2), float64(panelW+10), 1, color.RGBA{R: 60, G: 60, B: 60, A: 255})
	ebitenutil.DrawRect(screen, float64(panelX-5), float64(logBottom+2), float64(panelW+10), 1, color.RGBA{R: 60, G: 60, B: 60, A: 255})

// Button
	btnW, btnH := 200, 30
	btnX := panelX + (panelW-btnW)/2
	btnY := sh - 45

	p.mu.Lock()
	stopped := p.stopped
	p.mu.Unlock()

	mx, my := ebiten.CursorPosition()
	hovered := mx >= btnX && mx < btnX+btnW && my >= btnY && my < btnY+btnH
	pressed := hovered && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)

	if !stopped {
		// Stop button
		p.drawButton(screen, btnX, btnY, btnW, btnH, "STOP SIMULATION", color.RGBA{R: 150, G: 40, B: 40, A: 255}, hovered, pressed)
	} else {
		// Explore button
		p.drawButton(screen, btnX, btnY, btnW, btnH, "EXPLORE", color.RGBA{R: 40, G: 100, B: 40, A: 255}, hovered, pressed)
	}
}

func (p *ProgressScreen) handleClick(mx, my int) {
	sw := config.ScreenWidth()
	sh := config.ScreenHeight()
	panelW := 600
	panelX := (sw - panelW) / 2

	btnW, btnH := 200, 30
	btnX := panelX + (panelW-btnW)/2
	btnY := sh - 45

	if mx >= btnX && mx < btnX+btnW && my >= btnY && my < btnY+btnH {
		p.mu.Lock()
		if !p.stopped {
			p.stopRequested = true
		} else {
			p.explored = true
		}
		p.mu.Unlock()
	}
}

func (p *ProgressScreen) drawButton(screen *ebiten.Image, x, y, w, h int, label string, bgColor color.RGBA, hovered, pressed bool) {
	// Tint the base colour: lighten on hover, darken slightly when held
	// down. The shifts are small on purpose — just enough feedback to
	// confirm the cursor is over the button and the click registered.
	fill := bgColor
	switch {
	case pressed:
		fill = shiftRGB(bgColor, -25)
	case hovered:
		fill = shiftRGB(bgColor, 25)
	}
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), fill)

	// Hover highlight: a 1px top border lightened further than the fill,
	// so the button reads as raised when the cursor is over it.
	if hovered && !pressed {
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, shiftRGB(bgColor, 70))
	}

	bounds := boundString(r.FontSourceCodePro12, label)
	tx := x + (w-bounds.Dx())/2
	ty := y + (h+bounds.Dy())/2
	if pressed {
		ty++ // nudge label down by 1px while held to imply press depth
	}
	text.Draw(screen, label, r.FontSourceCodePro12, tx, ty, themedForeground())
}

// shiftRGB returns c with each colour channel shifted by delta and clamped
// to [0, 255]. Positive delta lightens, negative darkens. Alpha is
// preserved.
func shiftRGB(c color.RGBA, delta int) color.RGBA {
	clamp := func(v int) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	return color.RGBA{
		R: clamp(int(c.R) + delta),
		G: clamp(int(c.G) + delta),
		B: clamp(int(c.B) + delta),
		A: c.A,
	}
}

// FormatLogLine creates a standard log line.
func FormatLogLine(cycle, organisms int, avgPh float64) string {
	return fmt.Sprintf("Cycle: %6d   Organisms: %5d   AvgPh: %2.2f", cycle, organisms, avgPh)
}
