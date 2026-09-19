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

// SimPopupMode is the popup's current sub-state. It swaps the body and
// footer content in-place without rebuilding the popup window.
type SimPopupMode int

const (
	SimPopupConfig   SimPopupMode = iota // user editing settings; Cancel / Start
	SimPopupRunning                      // sim running headless; Stop
	SimPopupComplete                     // sim finished; View / Retry / Edit
)

// PopupRequest is the signal the runner reads each tick to advance
// the outer state machine. Most requests are handled internally by
// the popup; the few that affect simulation lifecycle (start a sim,
// retry with a new seed, view replay) bubble up here.
type PopupRequest int

const (
	PopupReqNone   PopupRequest = iota
	PopupReqCancel              // close popup, back to main menu
	PopupReqStart               // begin headless sim
	PopupReqRetry               // begin a fresh sim (re-randomized seed)
	PopupReqView                // hand off to replay viewer
)

// Popup geometry — sized as a rect within the screen so the menu
// behind reads as a clearly-dimmed background. Width and height
// adapt to small windows.
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

// SimPopup is the New Simulation modal. The runner creates one when
// the user clicks "New Simulation" from the main menu and feeds it
// per-cycle log lines while the sim runs. The popup owns mode
// transitions internally; the runner polls Take() for the few
// requests that affect outer state (start a sim, retry, view).
type SimPopup struct {
	mode    SimPopupMode
	globals *c.Globals

	config *ConfigScreen

	// Running mode log buffer; written from the sim-step path on the
	// main goroutine but locked anyway in case future versions move
	// stepping back to a goroutine.
	logs    []LogLine
	mu      sync.Mutex
	scrollY float64

	// Captured at Start and displayed at the top of the running and
	// complete bodies. Persists across Edit Settings so the user
	// returns to the form with the seed they ran already populated.
	displaySeed int

	// stopRequested is set when the user clicks Stop; the runner reads
	// it via StopRequested() and ends the sim loop.
	stopRequested bool
	// replayBytes is the projected size of the replay file, pushed in by
	// the runner each step. Read rather than computed here so the draw
	// path doesn't reach into a running simulation's recorder.
	replayBytes int64
	// endCondition is which end condition has fired, EndNone while the
	// run is going. Kept here rather than read off the simulation so the
	// footer doesn't reach across into a running goroutine's state.
	endCondition simulation.EndCondition

	// Captured by SetSimComplete and shown in the complete body.
	finalCycle int
	finalOrgs  int
	finalFood  int
	elapsed    time.Duration
	replayPath string
	replaySize string

	// pending is the next request the runner should pick up. Cleared
	// on read (Take) so each request fires exactly once.
	pending PopupRequest

	// loadingReplay, when true, paints a "Loading replay..." overlay
	// across the popup body + footer and ignores all input. Set by the
	// runner immediately after a View click so the user sees feedback
	// while replay.NewController loads on a background goroutine.
	loadingReplay bool
	loadingStart  time.Time
}

// NewSimPopup builds the popup with a config screen ready in the
// initial popupConfig mode. globals is the working set the form
// edits in place — the same instance is reused if the user clicks
// Edit Settings later, so the seed they last ran with is preserved.
func NewSimPopup(globals *c.Globals) *SimPopup {
	p := &SimPopup{
		mode:    SimPopupConfig,
		globals: globals,
		config:  NewConfigScreen(globals),
	}
	p.applyConfigViewport()
	return p
}

// applyConfigViewport binds the embedded ConfigScreen to the popup's
// body region (both axes). Re-call after the screen size changes so
// the form scroll bounds and panel-fit math stay aligned with the
// visible body — narrow popups make the form's slider column shrink
// rather than overflow off-canvas.
func (p *SimPopup) applyConfigViewport() {
	rect := p.popupRect()
	bodyTop, bodyBottom := p.bodyBounds()
	bodyLeft := rect.Min.X + popupPad
	bodyRight := rect.Max.X - popupPad
	p.config.SetEmbedded(bodyLeft, bodyTop, bodyRight, bodyBottom)
}

// Globals exposes the editable globals so the runner can promote
// them to c.SetGlobals when starting a sim.
func (p *SimPopup) Globals() *c.Globals { return p.globals }

// Mode reports the popup's internal sub-state — used by the runner
// to know whether the simulation should be running.
func (p *SimPopup) Mode() SimPopupMode { return p.mode }

// DisplaySeed is the seed value the popup is currently advertising
// in the body header. The runner reads this when starting / restarting
// a sim so the resolved random seed is locked in across modes.
func (p *SimPopup) DisplaySeed() int { return p.displaySeed }

// SetDisplaySeed records the seed the runner ended up using. Called
// by the runner after resolving a 0/random seed into a concrete
// integer so the popup body shows the actual value.
func (p *SimPopup) SetDisplaySeed(seed int) { p.displaySeed = seed }

// AddLog appends a line to the running-mode log buffer. Safe to call
// from any goroutine.
func (p *SimPopup) AddLog(line LogLine) {
	p.mu.Lock()
	p.logs = append(p.logs, line)
	p.mu.Unlock()
}

// SetEndCondition records which end condition has fired, so the footer's
// list can light the one that stopped the run. EndNone while it runs.
// Safe to call from any goroutine.
func (p *SimPopup) SetEndCondition(end simulation.EndCondition) {
	p.mu.Lock()
	p.endCondition = end
	p.mu.Unlock()
}

// SetReplayBytes records the projected replay size for the footer.
// Safe to call from any goroutine.
func (p *SimPopup) SetReplayBytes(b int64) {
	p.mu.Lock()
	p.replayBytes = b
	p.mu.Unlock()
}

// StopRequested reports whether the user clicked Stop while running.
// The runner polls this each step and ends the sim loop on true.
func (p *SimPopup) StopRequested() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopRequested
}

// SetSimComplete moves the popup into popupComplete mode, recording
// the final stats for display. The runner calls this after the sim
// loop ends (either naturally or because Stop was clicked).
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

// SetRunning transitions the popup into running mode. Called by the
// runner on Start (popupConfig → popupRunning) and Retry
// (popupComplete → popupRunning) after kicking off a fresh sim. The
// log buffer is cleared so the user sees only the active run.
func (p *SimPopup) SetRunning() {
	p.mu.Lock()
	p.logs = p.logs[:0]
	p.mode = SimPopupRunning
	p.scrollY = 0
	p.stopRequested = false
	p.mu.Unlock()
}

// Take returns and clears the pending request, so each user click
// produces exactly one runner action.
func (p *SimPopup) Take() PopupRequest {
	req := p.pending
	p.pending = PopupReqNone
	return req
}

// SetLoadingReplay flips the loading overlay on or off. While on, the
// popup ignores input and paints "Loading replay..." across its body
// — used during the gap between a View click and the replay controller
// finishing on a goroutine. Calling with true (re)anchors the
// animation start time so the dots restart from zero.
func (p *SimPopup) SetLoadingReplay(loading bool) {
	p.loadingReplay = loading
	if loading {
		p.loadingStart = time.Now()
	}
}

// Update routes input to the active sub-mode and returns immediately.
// Mode transitions that don't need runner help (Edit Settings →
// popupConfig) happen here in place; the rest are queued via pending
// for the runner to read with Take.
func (p *SimPopup) Update() {
	// While the runner is loading a replay we ignore all input — the
	// popup is a frozen "loading" view and any clicks would race the
	// load goroutine.
	if p.loadingReplay {
		return
	}

	// Re-bind viewport every tick — cheap, and keeps things sane if
	// the window was resized or if Edit Settings just rebuilt the
	// form.
	p.applyConfigViewport()

	switch p.mode {
	case SimPopupConfig:
		// The form does its own scroll/click/keyboard work. We layer
		// our footer click handling on top: clicks on the Cancel /
		// Start row need to be routed before the form sees them, or
		// they'll register as a click below the last row.
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

// handleConfigFooterClicks tests Cancel / Start before forwarding to
// the form. Returns true if a footer click consumed input, so the
// form's own handleClick doesn't ALSO see the press.
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
		// Settings that can't start (e.g. initial abilities not adding
		// up to 100) swallow the click; the footer says why.
		if p.config.StartBlockedReason() == "" {
			p.pending = PopupReqStart
		}
		return true
	}
	// Click anywhere else in the footer band still gets eaten (so
	// it doesn't reach the form), since it's outside the popup body
	// anyway.
	footerY := p.popupRect().Max.Y - popupFooterH
	if my >= footerY {
		return true
	}
	return false
}

func (p *SimPopup) handleRunningInput() {
	// Scroll log
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
		// Edit Settings is purely internal — we just swap modes back
		// to the form with the same globals (and seed) intact.
		p.mode = SimPopupConfig
		p.scrollY = 0
		p.applyConfigViewport()
	}
}

// Draw paints the dimmed background, popup chrome, and per-mode body.
// Caller is responsible for drawing the main menu first so the dim
// reads as "menu through frosted glass".
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

// drawLoadingOverlay paints a translucent panel over the popup's body
// + footer with an animated "Loading replay..." message, so the user
// gets feedback while the replay controller loads on a background
// goroutine. Header chrome is left visible so the user still sees
// which run they're loading.
func (p *SimPopup) drawLoadingOverlay(screen *ebiten.Image) {
	rect := p.popupRect()
	bodyTop, _ := p.bodyBounds()

	// Cover everything below the header divider.
	overlayFill := chrome(
		color.RGBA{R: 0, G: 0, B: 0, A: 210},
		color.RGBA{R: 240, G: 240, B: 245, A: 230},
	)
	ebitenutil.DrawRect(screen,
		float64(rect.Min.X+1), float64(bodyTop),
		float64(rect.Dx()-2), float64(rect.Max.Y-bodyTop-1),
		overlayFill)

	// Animated trailing dots — three frames at 300ms each so the
	// motion is gentle and obvious without strobing.
	nDots := int(time.Since(p.loadingStart).Milliseconds()/300) % 4
	msg := "Loading replay" + strings.Repeat(".", nDots)
	bounds := boundString(r.FontSourceCodePro12, msg)
	tx := rect.Min.X + (rect.Dx()-bounds.Dx())/2
	ty := bodyTop + (rect.Max.Y-bodyTop-popupFooterH)/2 + bounds.Dy()/2
	text.Draw(screen, msg, r.FontSourceCodePro12, tx, ty, themedForeground())
}

// drawModalChrome dims the whole screen, then paints a modal window at
// rect: background, border, header and footer dividers, and the title
// centred in the header. Shared by the New Simulation popup and the
// replay settings viewer.
func drawModalChrome(screen *ebiten.Image, rect popupRectT, title string) {
	drawScreenDim(screen)
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y),
		float64(rect.Dx()), float64(rect.Dy()), themeBackgroundColor())
	drawModalBorder(screen, rect)
	border := themedForegroundDim()
	// Header divider
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y+popupHeaderH),
		float64(rect.Dx()), 1, border)
	// Footer divider
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Max.Y-popupFooterH),
		float64(rect.Dx()), 1, border)
	tb := boundString(r.FontSourceCodePro12, title)
	tx := rect.Min.X + (rect.Dx()-tb.Dx())/2
	ty := rect.Min.Y + (popupHeaderH+tb.Dy())/2
	text.Draw(screen, title, r.FontSourceCodePro12, tx, ty, themedForeground())
}

// drawScreenDim covers the whole screen with the modal dim layer: 60%
// black (or near-white in the light theme) so whatever is underneath
// still hints through but doesn't compete.
func drawScreenDim(screen *ebiten.Image) {
	dim := chrome(
		color.RGBA{R: 0, G: 0, B: 0, A: 160},
		color.RGBA{R: 235, G: 235, B: 240, A: 200},
	)
	ebitenutil.DrawRect(screen, 0, 0, float64(c.ScreenWidth()), float64(c.ScreenHeight()), dim)
}

// drawModalBorder outlines rect with a 1px border.
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

	// Auto-scroll to bottom while content keeps growing — same behaviour
	// the old ProgressScreen had: the user can scroll up freely, but if
	// they're at the bottom we keep them pinned there.
	totalH := len(logsCopy) * lineHeight
	logHeight := bodyBottom - bodyTop - 8
	if totalH > logHeight {
		p.scrollY = float64(totalH - logHeight)
	}

	// Clipped to the body, so the line scrolling in at the top is cut off
	// at the header divider instead of being drawn over the title banner
	// (and likewise at the footer).
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

	// What will stop this run, to the left of the button that stops it
	// by hand. Bottom-aligned with the button so the block grows upward
	// as conditions are added rather than pushing the footer about.
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

// popupRectT is a tiny axis-aligned rect used for hit-testing without
// pulling in image.Rectangle. Min is inclusive, Max is exclusive.
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

// popupRect returns the popup window's screen bounds, clamped to a
// max size so the popup stays a sensible-sized modal even on very
// large screens.
func (p *SimPopup) popupRect() popupRectT { return modalRect() }

// modalRect is the screen rect of a full-size modal window, centred and
// capped at popupMaxW x popupMaxH.
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

// bodyBounds returns the inner top/bottom y-coords of the popup's
// content area (between header and footer dividers), used by the
// embedded config screen and the running/complete bodies.
func (p *SimPopup) bodyBounds() (int, int) { return modalBodyBounds(modalRect()) }

// modalBodyBounds is the top and bottom of rect's content area, between
// the header and footer dividers.
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

// completeButtonRects lays out View | Retry | Edit Settings right-to-
// left along the footer. View is the accent action so it sits closest
// to the dominant-hand corner.
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

// LogLine is one entry in the running-mode log: a line of text, and the
// average ability scores drawn after it as their own columns so they can
// be coloured by value.
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

// abilityLogLabels are the three-letter column headings the log uses, one
// per ability, since a full name per column would dwarf the numbers.
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
// Shared with the headless CLI path so log output is consistent.
func FormatLogLine(cycle, organisms, food, walls int, avgPh, minPh, maxPh float64, scores [physiology.AbilityCount]float64) LogLine {
	return LogLine{
		// pH fields are 5 wide so a reading of 10.00 lines up with 4.66
		// instead of pushing the ability columns along.
		Text: fmt.Sprintf("Cycle: %6d   Organisms: %5d   Food: %6d   Walls: %5d   pH: %5.2f (%5.2f-%5.2f)",
			cycle, organisms, food, walls, avgPh, minPh, maxPh),
		Scores: scores,
	}
}

// drawLogLine draws one log entry: the counts as plain text, then a
// column per ability whose number is tinted gray→green by the score, so a
// population's drift into one ability is visible down the column without
// reading every figure.
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

// drawAccentButton paints a button whose fill is a specific accent
// colour (Start green, Stop red, etc.) instead of the neutral menu
// chrome. Hover/press still tints the base; label stays white.
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
