package ux

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// ReplayMenuChoice is a replay menu action the runner has to carry out,
// since it leaves the replay viewer.
type ReplayMenuChoice int

const (
	ReplayMenuNone ReplayMenuChoice = iota
	// ReplayMenuRunAgain starts a new simulation with the replay's
	// settings and a new seed.
	ReplayMenuRunAgain
	// ReplayMenuEditSettings opens the New Simulation form filled with
	// the replay's settings.
	ReplayMenuEditSettings
	// ReplayMenuMainMenu returns to the main menu.
	ReplayMenuMainMenu
)

// replayMenuAction is what a replay menu button does: close the menu,
// open the settings viewer, or hand a choice to the runner.
type replayMenuAction int

const (
	replayActionClose replayMenuAction = iota
	replayActionViewSettings
	replayActionChoice
)

type replayMenuButton struct {
	label  string
	action replayMenuAction
	choice ReplayMenuChoice
}

var replayMenuButtons = []replayMenuButton{
	{"Run Again (New Seed)", replayActionChoice, ReplayMenuRunAgain},
	{"Edit Settings", replayActionChoice, ReplayMenuEditSettings},
	{"View Settings", replayActionViewSettings, ReplayMenuNone},
	{"Main Menu", replayActionChoice, ReplayMenuMainMenu},
	{"Close", replayActionClose, ReplayMenuNone},
}

const (
	replayMenuW       = 300
	replayMenuPad     = 20
	replayMenuButtonH = 34
	replayMenuGap     = 10
	replayMenuTitleH  = 40
)

// ReplayMenu is the menu the replay viewer opens from its MENU button or
// Escape. It can also show the replay's settings read-only in a popup.
// While it's open it takes all input; the runner polls Take for choices
// that leave the replay.
type ReplayMenu struct {
	open bool
	// globals are the settings the replayed simulation ran with.
	globals c.Globals
	// settings is the read-only settings viewer, non-nil while showing.
	settings *ConfigScreen
	pending  ReplayMenuChoice
	// notice reports what the last footer action did (a copied seed, an
	// exported file); it shows until noticeUntil.
	notice      string
	noticeErr   bool
	noticeUntil time.Time
}

// noticeFor is how long a footer action's notice stays up.
const noticeFor = 4 * time.Second

// settingsButton is one button in the settings viewer's footer. Left
// buttons are laid out from the left edge, the rest from the right.
type settingsButton struct {
	label  string
	width  int
	left   bool
	accent bool
	action func(m *ReplayMenu)
}

var settingsButtons = []settingsButton{
	{"Copy Seed", settingsSmallBtnW, true, false, (*ReplayMenu).copySeed},
	{"Export", settingsSmallBtnW, true, false, (*ReplayMenu).exportSettings},
	{"Close", popupCancelW, false, false, (*ReplayMenu).Close},
	{"Edit Settings", popupBtnW, false, true, func(m *ReplayMenu) {
		m.Close()
		m.pending = ReplayMenuEditSettings
	}},
}

// settingsSmallBtnW is the width of the left-hand footer buttons, narrow
// enough that both groups fit the popup in a small window.
const settingsSmallBtnW = 100

// NewReplayMenu builds a closed menu for a replay recorded with globals.
func NewReplayMenu(globals c.Globals) *ReplayMenu {
	return &ReplayMenu{globals: globals}
}

// IsOpen reports whether the menu or its settings viewer is showing.
func (m *ReplayMenu) IsOpen() bool { return m.open || m.settings != nil }

// Open shows the menu.
func (m *ReplayMenu) Open() { m.open = true }

// Close hides the menu and the settings viewer.
func (m *ReplayMenu) Close() {
	m.open = false
	m.settings = nil
}

// Take returns and clears the pending choice.
func (m *ReplayMenu) Take() ReplayMenuChoice {
	choice := m.pending
	m.pending = ReplayMenuNone
	return choice
}

// Update handles input for whichever of the menu or settings viewer is
// showing.
func (m *ReplayMenu) Update() {
	if m.settings != nil {
		m.updateSettings()
		return
	}
	if !m.open {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		m.Close()
		return
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()
	if !hitRect(mx, my, replayMenuRect()) {
		m.Close()
		return
	}
	for i, b := range replayMenuButtons {
		if hitRect(mx, my, replayMenuButtonRect(i)) {
			m.activate(b)
			return
		}
	}
}

// activate carries out a button's action.
func (m *ReplayMenu) activate(b replayMenuButton) {
	switch b.action {
	case replayActionClose:
		m.Close()
	case replayActionViewSettings:
		m.openSettings()
	case replayActionChoice:
		m.Close()
		m.pending = b.choice
	}
}

// openSettings shows the read-only settings viewer in place of the menu.
func (m *ReplayMenu) openSettings() {
	g := m.globals
	g.InitialAbilityScores = append([]int(nil), g.InitialAbilityScores...)
	m.settings = NewConfigScreen(&g)
	m.settings.SetReadOnly()
	m.open = false
	m.bindSettingsViewport()
}

func (m *ReplayMenu) bindSettingsViewport() {
	rect := modalRect()
	top, bottom := modalBodyBounds(rect)
	m.settings.SetEmbedded(rect.Min.X+popupPad, top, rect.Max.X-popupPad, bottom)
}

func (m *ReplayMenu) updateSettings() {
	m.bindSettingsViewport()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		m.Close()
		return
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		rects := settingsFooterRects()
		for i, b := range settingsButtons {
			if hitRect(mx, my, rects[i]) {
				b.action(m)
				return
			}
		}
		if my >= modalRect().Max.Y-popupFooterH {
			return
		}
	}
	m.settings.Update()
}

// copySeed puts the replay's seed on the clipboard.
func (m *ReplayMenu) copySeed() {
	if err := copyToClipboard(strconv.Itoa(m.globals.Seed)); err != nil {
		m.setNotice("Couldn't copy the seed: "+err.Error(), true)
		return
	}
	m.setNotice(fmt.Sprintf("Copied seed %d", m.globals.Seed), false)
}

// exportSettings saves the replay's settings as a JSON config file, the
// same format -config loads.
func (m *ReplayMenu) exportSettings() {
	var buf bytes.Buffer
	c.DumpGlobals(&m.globals, &buf)
	path, err := saveExport(fmt.Sprintf("settings_seed_%d.json", m.globals.Seed), buf.Bytes())
	if err != nil {
		m.setNotice("Export failed: "+err.Error(), true)
		return
	}
	m.setNotice("Exported to "+path, false)
}

func (m *ReplayMenu) setNotice(msg string, isErr bool) {
	if isErr {
		log.Print(msg)
	}
	m.notice, m.noticeErr, m.noticeUntil = msg, isErr, time.Now().Add(noticeFor)
}

// Draw paints the menu or settings viewer over the replay.
func (m *ReplayMenu) Draw(screen *ebiten.Image) {
	if m.settings != nil {
		m.drawSettings(screen)
		return
	}
	if !m.open {
		return
	}
	drawScreenDim(screen)
	rect := replayMenuRect()
	fillRect(screen, rect, themeBackgroundColor())
	drawModalBorder(screen, rect)

	title := "MENU"
	tb := boundString(r.FontSourceCodePro12, title)
	text.Draw(screen, title, r.FontSourceCodePro12, rect.Min.X+(rect.Dx()-tb.Dx())/2,
		rect.Min.Y+(replayMenuTitleH+tb.Dy())/2, themedForeground())

	mx, my := ebiten.CursorPosition()
	pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	for i, b := range replayMenuButtons {
		br := replayMenuButtonRect(i)
		hovered := hitRect(mx, my, br)
		drawMenuButton(screen, br.Min.X, br.Min.Y, br.Dx(), br.Dy(), b.label, hovered, hovered && pressed, false)
	}
}

func (m *ReplayMenu) drawSettings(screen *ebiten.Image) {
	drawModalChrome(screen, modalRect(), fmt.Sprintf("SIMULATION SETTINGS — SEED %d", m.globals.Seed))
	m.settings.Draw(screen)

	mx, my := ebiten.CursorPosition()
	pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	rects := settingsFooterRects()
	for i, b := range settingsButtons {
		br := rects[i]
		hovered := hitRect(mx, my, br)
		if b.accent {
			drawAccentButton(screen, br.Min.X, br.Min.Y, br.Dx(), br.Dy(), b.label, hovered, hovered && pressed, color.RGBA{R: 80, G: 80, B: 90, A: 255})
		} else {
			drawMenuButton(screen, br.Min.X, br.Min.Y, br.Dx(), br.Dy(), b.label, hovered, hovered && pressed, false)
		}
	}
	if m.notice != "" && time.Now().Before(m.noticeUntil) {
		col := color.Color(themedForegroundDim())
		if m.noticeErr {
			col = themedBad()
		}
		rect := modalRect()
		text.Draw(screen, m.notice, r.FontSourceCodePro10, rect.Min.X+popupPad, rect.Max.Y-popupFooterH+12, col)
	}
	m.settings.DrawTooltip(screen)
}

// replayMenuRect is the menu box, centred on screen and sized to fit
// its buttons.
func replayMenuRect() popupRectT {
	n := len(replayMenuButtons)
	h := replayMenuTitleH + n*replayMenuButtonH + (n-1)*replayMenuGap + replayMenuPad
	return newRect((c.ScreenWidth()-replayMenuW)/2, (c.ScreenHeight()-h)/2, replayMenuW, h)
}

func replayMenuButtonRect(i int) popupRectT {
	menu := replayMenuRect()
	y := menu.Min.Y + replayMenuTitleH + i*(replayMenuButtonH+replayMenuGap)
	return newRect(menu.Min.X+replayMenuPad, y, replayMenuW-2*replayMenuPad, replayMenuButtonH)
}

// settingsFooterRects lays out settingsButtons along the footer, in
// order: left buttons from the left edge, right buttons ending at the
// right edge. The rects line up with settingsButtons by index.
func settingsFooterRects() []popupRectT {
	rect := modalRect()
	// Sit a little below centre, leaving room for the notice line above.
	y := rect.Max.Y - popupFooterH + (popupFooterH-popupBtnH)/2 + 6
	rightW := 0
	for _, b := range settingsButtons {
		if !b.left {
			rightW += b.width + popupBtnSpacing
		}
	}
	leftX, rightX := rect.Min.X+popupPad, rect.Max.X-popupPad-rightW+popupBtnSpacing
	rects := make([]popupRectT, len(settingsButtons))
	for i, b := range settingsButtons {
		if b.left {
			rects[i] = newRect(leftX, y, b.width, popupBtnH-6)
			leftX += b.width + popupBtnSpacing
		} else {
			rects[i] = newRect(rightX, y, b.width, popupBtnH-6)
			rightX += b.width + popupBtnSpacing
		}
	}
	return rects
}

func fillRect(screen *ebiten.Image, rect popupRectT, col color.Color) {
	ebitenutil.DrawRect(screen, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Dx()), float64(rect.Dy()), col)
}
