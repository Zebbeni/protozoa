package ux

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

type SimPopupMode int

const (
	SimPopupConfig   SimPopupMode = iota // user editing settings; Cancel / Start
	SimPopupRunning                      // sim running headless; Stop
	SimPopupComplete                     // sim finished; View / Retry / Edit
)

// PopupRequest is the signal the runner reads each tick to advance the outer state machine.
type PopupRequest int

const (
	PopupReqNone   PopupRequest = iota
	PopupReqCancel              // close popup, back to main menu
	PopupReqStart               // begin headless sim
	PopupReqRetry               // begin a fresh sim (re-randomized seed)
	PopupReqView                // hand off to replay viewer
)

// Popup geometry — sized as a rect within the screen so the menu behind reads as a clearly-dimmed background.
const (
	popupMaxW       = 1000
	popupMaxH       = 720
	popupMargin     = 60
	popupHeaderH    = 36
	popupFooterH    = 60
	popupPad        = 16
	popupBtnH       = 30
	popupBtnSpacing = 12
	popupBtnW       = 150
	popupCancelW    = 110
)

type SimPopup struct {
	mode    SimPopupMode
	globals *c.Globals

	config *ConfigScreen

	// Running mode log buffer; written from the sim-step path on the main goroutine but locked anyway in case future versions move stepping back to a goroutine.
	logs    []LogLine
	mu      sync.Mutex
	scrollY float64

	// Captured at Start and displayed at the top of the running and complete bodies.
	displaySeed int

	// stopRequested is set when the user clicks Stop; the runner reads it via StopRequested() and ends the sim loop.
	stopRequested bool
	// replayBytes is the projected size of the replay file, pushed in by the runner each step.
	replayBytes int64
	// endCondition is which end condition has fired, EndNone while the run is going.
	endCondition simulation.EndCondition

	// Captured by SetSimComplete and shown in the complete body.
	finalCycle int
	finalOrgs  int
	finalFood  int
	elapsed    time.Duration
	replayPath string
	replaySize string

	// pending is the next request the runner should pick up.
	pending PopupRequest

	// loadingReplay, when true, paints a "Loading replay..." overlay across the popup body + footer and ignores all input.
	loadingReplay bool
	loadingStart  time.Time
}

// NewSimPopup builds the popup with a config screen ready in the initial popupConfig mode.
func NewSimPopup(globals *c.Globals) *SimPopup {
	p := &SimPopup{
		mode:    SimPopupConfig,
		globals: globals,
		config:  NewConfigScreen(globals),
	}
	p.applyConfigViewport()
	return p
}

// applyConfigViewport binds the embedded ConfigScreen to the popup's body region (both axes).
func (p *SimPopup) applyConfigViewport() {
	rect := p.popupRect()
	bodyTop, bodyBottom := p.bodyBounds()
	bodyLeft := rect.Min.X + popupPad
	bodyRight := rect.Max.X - popupPad
	p.config.SetEmbedded(bodyLeft, bodyTop, bodyRight, bodyBottom)
}

// Globals exposes the editable globals so the runner can promote them to c.SetGlobals when starting a sim.
func (p *SimPopup) Globals() *c.Globals { return p.globals }

func (p *SimPopup) Mode() SimPopupMode { return p.mode }

// DisplaySeed is the seed value the popup is currently advertising in the body header.
func (p *SimPopup) DisplaySeed() int { return p.displaySeed }

func (p *SimPopup) SetDisplaySeed(seed int) { p.displaySeed = seed }

// AddLog appends a line to the running-mode log buffer.
func (p *SimPopup) AddLog(line LogLine) {
	p.mu.Lock()
	p.logs = append(p.logs, line)
	p.mu.Unlock()
}

func (p *SimPopup) SetEndCondition(end simulation.EndCondition) {
	p.mu.Lock()
	p.endCondition = end
	p.mu.Unlock()
}

func (p *SimPopup) SetReplayBytes(b int64) {
	p.mu.Lock()
	p.replayBytes = b
	p.mu.Unlock()
}

// StopRequested reports whether the user clicked Stop while running.
func (p *SimPopup) StopRequested() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopRequested
}

// SetSimComplete moves the popup into popupComplete mode, recording the final stats for display.
func (p *SimPopup) SetSimComplete(finalCycle, finalOrgs, finalFood int, elapsed time.Duration, replayPath, replaySize string) {
	p.mu.Lock()
	p.finalCycle = finalCycle
	p.finalOrgs = finalOrgs
	p.finalFood = finalFood
	p.elapsed = elapsed
	p.replayPath = replayPath
	p.replaySize = replaySize
	p.mode = SimPopupComplete
	p.stopRequested = false
	p.mu.Unlock()
}

func (p *SimPopup) SetRunning() {
	p.mu.Lock()
	p.logs = p.logs[:0]
	p.mode = SimPopupRunning
	p.scrollY = 0
	p.stopRequested = false
	p.mu.Unlock()
}

// Take returns and clears the pending request, so each user click produces exactly one runner action.
func (p *SimPopup) Take() PopupRequest {
	req := p.pending
	p.pending = PopupReqNone
	return req
}

func (p *SimPopup) SetLoadingReplay(loading bool) {
	p.loadingReplay = loading
	if loading {
		p.loadingStart = time.Now()
	}
}

// Update routes input to the active sub-mode and returns immediately.
func (p *SimPopup) Update() {
	// While the runner is loading a replay we ignore all input.
	if p.loadingReplay {
		return
	}

	p.applyConfigViewport()

	switch p.mode {
	case SimPopupConfig:
		// The form does its own scroll/click/keyboard work.
		if p.handleConfigFooterClicks() {
			return
		}
		p.config.Update()

	case SimPopupRunning:
		p.handleRunningInput()

	case SimPopupComplete:
		p.handleCompleteInput()
	}
}

// handleConfigFooterClicks tests Cancel / Start before forwarding to the form.
func (p *SimPopup) handleConfigFooterClicks() bool {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return false
	}
	mx, my := ebiten.CursorPosition()
	cancelRect, startRect := p.configFooterRects()
	if hitRect(mx, my, cancelRect) {
		p.pending = PopupReqCancel
		return true
	}
	if hitRect(mx, my, startRect) {
		// Settings that can't start (e.g. initial abilities not adding up to 100) swallow the click.
		if p.config.StartBlockedReason() == "" {
			p.pending = PopupReqStart
		}
		return true
	}
	// Click anywhere else in the footer band still gets eaten (so it doesn't reach the form).
	footerY := p.popupRect().Max.Y - popupFooterH
	if my >= footerY {
		return true
	}
	return false
}

func (p *SimPopup) handleRunningInput() {
	_, wy := ebiten.Wheel()
	p.scrollY -= wy * 20
	if p.scrollY < 0 {
		p.scrollY = 0
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()
	stopRect := p.runningStopRect()
	if hitRect(mx, my, stopRect) {
		p.mu.Lock()
		p.stopRequested = true
		p.mu.Unlock()
	}
}

func (p *SimPopup) handleCompleteInput() {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()
	view, retry, edit := p.completeButtonRects()
	switch {
	case hitRect(mx, my, view):
		p.pending = PopupReqView
	case hitRect(mx, my, retry):
		p.pending = PopupReqRetry
	case hitRect(mx, my, edit):
		p.mode = SimPopupConfig
		p.scrollY = 0
		p.applyConfigViewport()
	}
}

// Draw paints the dimmed background, popup chrome, and per-mode body.
func (p *SimPopup) Draw(screen *ebiten.Image) {
	rect := p.popupRect()
	drawModalChrome(screen, rect, p.title())

	switch p.mode {
	case SimPopupConfig:
		p.config.Draw(screen)
		p.drawConfigFooter(screen)
		p.config.DrawTooltip(screen)
	case SimPopupRunning:
		p.drawRunningBody(screen)
		p.drawRunningFooter(screen)
	case SimPopupComplete:
		p.drawCompleteBody(screen)
		p.drawCompleteFooter(screen)
	}

	if p.loadingReplay {
		p.drawLoadingOverlay(screen)
	}
}

// drawLoadingOverlay paints a translucent panel over the popup's body + footer with an animated "Loading replay..." message.
func (p *SimPopup) drawLoadingOverlay(screen *ebiten.Image) {
	rect := p.popupRect()
	bodyTop, _ := p.bodyBounds()

	overlayFill := chrome(
		color.RGBA{R: 0, G: 0, B: 0, A: 210},
		color.RGBA{R: 240, G: 240, B: 245, A: 230},
	)
	ebitenutil.DrawRect(screen,
		float64(rect.Min.X+1), float64(bodyTop),
		float64(rect.Dx()-2), float64(rect.Max.Y-bodyTop-1),
		overlayFill)

	// Animated trailing dots — three frames at 300ms each so the motion is gentle and obvious without strobing.
	nDots := int(time.Since(p.loadingStart).Milliseconds()/300) % 4
	msg := "Loading replay" + strings.Repeat(".", nDots)
	bounds := boundString(r.FontSourceCodePro12, msg)
	tx := rect.Min.X + (rect.Dx()-bounds.Dx())/2
	ty := bodyTop + (rect.Max.Y-bodyTop-popupFooterH)/2 + bounds.Dy()/2
	text.Draw(screen, msg, r.FontSourceCodePro12, tx, ty, themedForeground())
}

// drawModalChrome dims the whole screen, then paints a modal window at rect.
func drawModalChrome(screen *ebiten.Image, rect popupRectT, title string) {
	drawScreenDim(screen)
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y),
		float64(rect.Dx()), float64(rect.Dy()), themeBackgroundColor())
	drawModalBorder(screen, rect)
	border := themedForegroundDim()
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y+popupHeaderH),
		float64(rect.Dx()), 1, border)
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Max.Y-popupFooterH),
		float64(rect.Dx()), 1, border)
	tb := boundString(r.FontSourceCodePro12, title)
	tx := rect.Min.X + (rect.Dx()-tb.Dx())/2
	ty := rect.Min.Y + (popupHeaderH+tb.Dy())/2
	text.Draw(screen, title, r.FontSourceCodePro12, tx, ty, themedForeground())
}

func drawScreenDim(screen *ebiten.Image) {
	dim := chrome(
		color.RGBA{R: 0, G: 0, B: 0, A: 160},
		color.RGBA{R: 235, G: 235, B: 240, A: 200},
	)
	ebitenutil.DrawRect(screen, 0, 0, float64(c.ScreenWidth()), float64(c.ScreenHeight()), dim)
}

func drawModalBorder(screen *ebiten.Image, rect popupRectT) {
	border := themedForegroundDim()
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Dx()), 1, border)
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Max.Y-1), float64(rect.Dx()), 1, border)
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y), 1, float64(rect.Dy()), border)
	ebitenutil.DrawRect(screen, float64(rect.Max.X-1), float64(rect.Min.Y), 1, float64(rect.Dy()), border)
}

// title is the header text for the popup's current mode.
func (p *SimPopup) title() string {
	switch p.mode {
	case SimPopupRunning:
		return fmt.Sprintf("RUNNING — SEED %d", p.displaySeed)
	case SimPopupComplete:
		return fmt.Sprintf("COMPLETE — SEED %d", p.displaySeed)
	}
	return "NEW SIMULATION"
}

func (p *SimPopup) drawConfigFooter(screen *ebiten.Image) {
	mx, my := ebiten.CursorPosition()
	cancelRect, startRect := p.configFooterRects()
	hC := hitRect(mx, my, cancelRect)
	hS := hitRect(mx, my, startRect)
	pressedC := hC && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	pressedS := hS && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	drawMenuButton(screen, cancelRect.Min.X, cancelRect.Min.Y, cancelRect.Dx(), cancelRect.Dy(),
		"Cancel", hC, pressedC, false)
	if reason := p.config.StartBlockedReason(); reason != "" {
		// Greyed-out Start with the reason beside the buttons.
		drawAccentButton(screen, startRect.Min.X, startRect.Min.Y, startRect.Dx(), startRect.Dy(),
			"Start", false, false, color.RGBA{R: 70, G: 70, B: 70, A: 255})
		rb := boundString(r.FontSourceCodePro10, reason)
		tx := cancelRect.Min.X - popupBtnSpacing - rb.Dx()
		ty := startRect.Min.Y + (startRect.Dy()+rb.Dy())/2
		text.Draw(screen, reason, r.FontSourceCodePro10, tx, ty, themedBad())
		return
	}
	drawAccentButton(screen, startRect.Min.X, startRect.Min.Y, startRect.Dx(), startRect.Dy(),
		"Start", hS, pressedS,
		color.RGBA{R: 40, G: 100, B: 40, A: 255})
}

func (p *SimPopup) drawRunningBody(screen *ebiten.Image) {
	bodyTop, bodyBottom := p.bodyBounds()
	rect := p.popupRect()

	p.mu.Lock()
	logsCopy := make([]LogLine, len(p.logs))
	copy(logsCopy, p.logs)
	p.mu.Unlock()

	lineHeight := r.FontSourceCodePro10.Metrics().Height.Round()

	// Auto-scroll to bottom while content keeps growing.
	totalH := len(logsCopy) * lineHeight
	logHeight := bodyBottom - bodyTop - 8
	if totalH > logHeight {
		p.scrollY = float64(totalH - logHeight)
	}

	// Clipped to the body, so the line scrolling in at the top is cut off at the header divider instead of being drawn over the title banner (and likewise at the footer).
	body := screen.SubImage(image.Rect(rect.Min.X+1, bodyTop, rect.Max.X-1, bodyBottom)).(*ebiten.Image)

	x := rect.Min.X + popupPad
	y := bodyTop + 4 - int(p.scrollY)
	for _, line := range logsCopy {
		if y+lineHeight > bodyTop && y < bodyBottom {
			drawLogLine(body, line, x, y+lineHeight)
		}
		y += lineHeight
	}
}

func (p *SimPopup) drawRunningFooter(screen *ebiten.Image) {
	mx, my := ebiten.CursorPosition()
	rect := p.runningStopRect()

	// What will stop this run, to the left of the button that stops it by hand.
	p.mu.Lock()
	fired := p.endCondition
	replayBytes := p.replayBytes
	p.mu.Unlock()
	lines := endConditionLines(p.globals, fired, replayBytes)
	drawEndConditions(screen, lines, p.popupRect().Min.X+popupPad, rect.Max.Y)

	hovered := hitRect(mx, my, rect)
	pressed := hovered && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	drawAccentButton(screen, rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy(),
		"Stop", hovered, pressed,
		color.RGBA{R: 150, G: 40, B: 40, A: 255})
}

func (p *SimPopup) drawCompleteBody(screen *ebiten.Image) {
	bodyTop, _ := p.bodyBounds()
	rect := p.popupRect()
	x := rect.Min.X + popupPad
	y := bodyTop + 24

	lh := r.FontSourceCodePro12.Metrics().Height.Round() + 4
	lines := []string{
		fmt.Sprintf("Seed:        %d", p.displaySeed),
		fmt.Sprintf("Final cycle: %d", p.finalCycle),
		fmt.Sprintf("Organisms:   %d", p.finalOrgs),
		fmt.Sprintf("Food items:  %d", p.finalFood),
		fmt.Sprintf("Wall time:   %s", p.elapsed.Round(time.Millisecond)),
	}
	if p.replayPath != "" {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Replay: %s", p.replayPath))
		if p.replaySize != "" {
			lines = append(lines, fmt.Sprintf("Size:   %s", p.replaySize))
		}
	}
	for _, line := range lines {
		text.Draw(screen, line, r.FontSourceCodePro12, x, y, themedForeground())
		y += lh
	}
}

func (p *SimPopup) drawCompleteFooter(screen *ebiten.Image) {
	mx, my := ebiten.CursorPosition()
	view, retry, edit := p.completeButtonRects()
	for _, b := range []struct {
		rect  popupRectT
		label string
		fill  color.RGBA
	}{
		{view, "View", color.RGBA{R: 40, G: 100, B: 40, A: 255}},
		{retry, "Retry (New Seed)", color.RGBA{R: 60, G: 60, B: 130, A: 255}},
		{edit, "Edit Settings", color.RGBA{R: 80, G: 80, B: 90, A: 255}},
	} {
		hovered := hitRect(mx, my, b.rect)
		pressed := hovered && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		drawAccentButton(screen, b.rect.Min.X, b.rect.Min.Y, b.rect.Dx(), b.rect.Dy(),
			b.label, hovered, pressed, b.fill)
	}
}

// popupRectT is a tiny axis-aligned rect used for hit-testing without pulling in image.Rectangle.
type popupRectT struct{ Min, Max struct{ X, Y int } }

func newRect(x, y, w, h int) popupRectT {
	r := popupRectT{}
	r.Min.X, r.Min.Y = x, y
	r.Max.X, r.Max.Y = x+w, y+h
	return r
}
func (r popupRectT) Dx() int { return r.Max.X - r.Min.X }
func (r popupRectT) Dy() int { return r.Max.Y - r.Min.Y }

func hitRect(mx, my int, r popupRectT) bool {
	return mx >= r.Min.X && mx < r.Max.X && my >= r.Min.Y && my < r.Max.Y
}

// popupRect returns the popup window's screen bounds, clamped to a max size so the popup stays a sensible-sized modal even on very large screens.
func (p *SimPopup) popupRect() popupRectT { return modalRect() }

// modalRect is the screen rect of a full-size modal window, centred and capped at popupMaxW x popupMaxH.
func modalRect() popupRectT {
	sw, sh := c.ScreenWidth(), c.ScreenHeight()
	w := sw - 2*popupMargin
	h := sh - 2*popupMargin
	if w > popupMaxW {
		w = popupMaxW
	}
	if h > popupMaxH {
		h = popupMaxH
	}
	x := (sw - w) / 2
	y := (sh - h) / 2
	return newRect(x, y, w, h)
}

// bodyBounds returns the inner top/bottom y-coords of the popup's content area (between header and footer dividers), used by the embedded config screen and the running/complete bodies.
func (p *SimPopup) bodyBounds() (int, int) { return modalBodyBounds(modalRect()) }

// modalBodyBounds is the top and bottom of rect's content area, between the header and footer dividers.
func modalBodyBounds(r popupRectT) (int, int) {
	return r.Min.Y + popupHeaderH + 4, r.Max.Y - popupFooterH - 4
}

func (p *SimPopup) configFooterRects() (popupRectT, popupRectT) {
	r := p.popupRect()
	footerY := r.Max.Y - popupFooterH + (popupFooterH-popupBtnH)/2
	rightEdge := r.Max.X - popupPad
	startX := rightEdge - popupBtnW
	cancelX := startX - popupBtnSpacing - popupCancelW
	return newRect(cancelX, footerY, popupCancelW, popupBtnH),
		newRect(startX, footerY, popupBtnW, popupBtnH)
}

func (p *SimPopup) runningStopRect() popupRectT {
	r := p.popupRect()
	footerY := r.Max.Y - popupFooterH + (popupFooterH-popupBtnH)/2
	x := r.Max.X - popupPad - popupBtnW
	return newRect(x, footerY, popupBtnW, popupBtnH)
}

// completeButtonRects lays out View | Retry | Edit Settings right-to- left along the footer.
func (p *SimPopup) completeButtonRects() (popupRectT, popupRectT, popupRectT) {
	r := p.popupRect()
	footerY := r.Max.Y - popupFooterH + (popupFooterH-popupBtnH)/2
	editW := 150
	retryW := 170
	viewW := 120

	rightEdge := r.Max.X - popupPad
	viewX := rightEdge - viewW
	retryX := viewX - popupBtnSpacing - retryW
	editX := retryX - popupBtnSpacing - editW

	return newRect(viewX, footerY, viewW, popupBtnH),
		newRect(retryX, footerY, retryW, popupBtnH),
		newRect(editX, footerY, editW, popupBtnH)
}

type LogLine struct {
	Text   string
	Scores [physiology.AbilityCount]float64
}

// String is the whole line as plain text, for the headless CLI.
func (l LogLine) String() string {
	out := l.Text
	for _, a := range physiology.AllAbilities {
		out += fmt.Sprintf("  %s %s", abilityLogLabels[a], formatAbilityScore(l.Scores[a]))
	}
	return out
}

var abilityLogLabels = map[physiology.Ability]string{
	physiology.AbilityChemosynthesis: "CHM",
	physiology.AbilityEating:         "EAT",
	physiology.AbilityMovement:       "MOV",
	physiology.AbilityDigging:        "DIG",
	physiology.AbilityAttack:         "ATK",
	physiology.AbilityDefense:        "DEF",
	physiology.AbilityTolerance:      "TOL",
}

func formatAbilityScore(v float64) string { return fmt.Sprintf("%4.1f", v) }

// FormatLogLine creates one entry for the running-mode log buffer.
func FormatLogLine(cycle, organisms, food, buried, walls int, avgPh, minPh, maxPh float64, scores [physiology.AbilityCount]float64) LogLine {
	return LogLine{
		// Labels are abbreviated to make room for the buried count without pushing the ability columns off the popup.
		Text: fmt.Sprintf("C: %6d   Orgs: %5d   Food: %6d   Bur: %6d   Walls: %5d   pH: %5.2f (%5.2f-%5.2f)",
			cycle, organisms, food, buried, walls, avgPh, minPh, maxPh),
		Scores: scores,
	}
}

func drawLogLine(dst *ebiten.Image, line LogLine, x, baseline int) {
	face := r.FontSourceCodePro10
	text.Draw(dst, line.Text, face, x, baseline, themedForegroundDim())
	cur := x + textAdvance(face, line.Text)
	for _, a := range physiology.AllAbilities {
		label := "  " + abilityLogLabels[a] + " "
		text.Draw(dst, label, face, cur, baseline, themedForegroundDim())
		cur += textAdvance(face, label)

		value := formatAbilityScore(line.Scores[a])
		text.Draw(dst, value, face, cur, baseline, gh.AbilityScoreColor(line.Scores[a]))
		cur += textAdvance(face, value)
	}
}

// drawAccentButton paints a button whose fill is a specific accent colour (Start green, Stop red, etc.) instead of the neutral menu chrome.
func drawAccentButton(screen *ebiten.Image, x, y, w, h int, label string, hovered, pressed bool, base color.RGBA) {
	fill := base
	switch {
	case pressed:
		fill = shiftRGB(base, -25)
	case hovered:
		fill = shiftRGB(base, 25)
	}
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), fill)
	if hovered && !pressed {
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, shiftRGB(base, 70))
	}
	bounds := boundString(r.FontSourceCodePro12, label)
	tx := x + (w-bounds.Dx())/2
	ty := y + (h+bounds.Dy())/2
	if pressed {
		ty++
	}
	text.Draw(screen, label, r.FontSourceCodePro12, tx, ty, color.White)
}
