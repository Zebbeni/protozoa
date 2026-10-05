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

	"github.com/Zebbeni/protozoa/checkpoint"
	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// ReplayMenuChoice is a replay menu action the runner has to carry out, since it leaves the replay viewer.
type ReplayMenuChoice int

const (
	ReplayMenuNone ReplayMenuChoice = iota
	// ReplayMenuRunAgain starts a new simulation with the replay's settings and a new seed.
	ReplayMenuRunAgain
	// ReplayMenuEditSettings opens the New Simulation form filled with the replay's settings.
	ReplayMenuEditSettings
	ReplayMenuMainMenu
)

type replayMenuAction int

const (
	replayActionClose replayMenuAction = iota
	replayActionViewSettings
	replayActionChoice
	// replayActionSaveAs opens the name prompt for saving a copy of the recording being viewed.
	replayActionSaveAs
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
	{"Save Recording As...", replayActionSaveAs, ReplayMenuNone},
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

type ReplayMenu struct {
	open bool
	// globals are the settings the replayed simulation ran with.
	globals c.Globals
	// source is the .pzr being viewed, copied out by Save Recording As.
	source string
	// naming is true while the save prompt is up; name is what has been typed into it so far.
	naming bool
	// namingWhat is what the prompt is saving. One prompt serves the
	// recording and the settings: they differ in the title, the pre-filled
	// name, the extension shown under the field and where the bytes go.
	namingWhat namingTarget
	name       string
	// settings is the read-only settings viewer, non-nil while showing.
	settings *ConfigScreen
	pending  ReplayMenuChoice
	// notice reports what the last footer action did (a copied seed, an exported file); it shows until noticeUntil.
	notice      string
	noticeErr   bool
	noticeUntil time.Time
}

// noticeFor is how long a footer action's notice stays up.
const noticeFor = 4 * time.Second

type settingsButton struct {
	label  string
	width  int
	left   bool
	accent bool
	action func(m *ReplayMenu)
}

var settingsButtons = []settingsButton{
	{"Copy Seed", settingsSmallBtnW, true, false, (*ReplayMenu).copySeed},
	{"Save As...", settingsSmallBtnW, true, false, (*ReplayMenu).exportSettings},
	{"Close", popupCancelW, false, false, (*ReplayMenu).Close},
	{"Edit Settings", popupBtnW, false, true, func(m *ReplayMenu) {
		m.Close()
		m.pending = ReplayMenuEditSettings
	}},
}

// settingsSmallBtnW is the width of the left-hand footer buttons, narrow enough that both groups fit the popup in a small window.
const settingsSmallBtnW = 100

// NewReplayMenu builds a closed menu for the replay at source, recorded with globals.
func NewReplayMenu(globals c.Globals, source string) *ReplayMenu {
	return &ReplayMenu{globals: globals, source: source}
}

// IsOpen reports whether the menu, its settings viewer or the save prompt is showing.
func (m *ReplayMenu) IsOpen() bool { return m.open || m.settings != nil || m.naming }

func (m *ReplayMenu) Open() { m.open = true }

func (m *ReplayMenu) Close() {
	m.open = false
	m.settings = nil
	m.naming = false
}

func (m *ReplayMenu) Take() ReplayMenuChoice {
	choice := m.pending
	m.pending = ReplayMenuNone
	return choice
}

// Update handles input for whichever of the menu or settings viewer is showing.
func (m *ReplayMenu) Update() {
	if m.naming {
		m.updateNaming()
		return
	}
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

func (m *ReplayMenu) activate(b replayMenuButton) {
	switch b.action {
	case replayActionClose:
		m.Close()
	case replayActionViewSettings:
		m.openSettings()
	case replayActionChoice:
		m.Close()
		m.pending = b.choice
	case replayActionSaveAs:
		m.openNaming(namingRecording)
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

func (m *ReplayMenu) copySeed() {
	if err := copyToClipboard(strconv.Itoa(m.globals.Seed)); err != nil {
		m.setNotice("Couldn't copy the seed: "+err.Error(), true)
		return
	}
	m.setNotice(fmt.Sprintf("Copied seed %d", m.globals.Seed), false)
}

// exportSettings opens the name prompt for the replay's settings.
func (m *ReplayMenu) exportSettings() {
	m.openNaming(namingSettings)
}

func (m *ReplayMenu) setNotice(msg string, isErr bool) {
	if isErr {
		log.Print(msg)
	}
	m.notice, m.noticeErr, m.noticeUntil = msg, isErr, time.Now().Add(noticeFor)
}

func (m *ReplayMenu) Draw(screen *ebiten.Image) {
	if m.naming {
		m.drawNaming(screen)
		return
	}
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
	rect := modalRect()
	m.drawNotice(screen, rect.Min.X+popupPad, rect.Max.Y-popupFooterH+12)
	m.settings.DrawTooltip(screen)
}

const (
	nameBoxW     = 420
	nameBoxH     = 190
	nameFieldH   = 34
	nameMaxRunes = 40
)

type namingTarget int

const (
	namingRecording namingTarget = iota
	namingSettings
)

func (t namingTarget) title() string {
	if t == namingSettings {
		return "SAVE SETTINGS"
	}
	return "SAVE RECORDING"
}

func (t namingTarget) fileName(name string) string {
	if t == namingSettings {
		return c.SettingsFileName(name)
	}
	return checkpoint.RecordingFileName(name)
}

// openNaming shows the prompt, pre-filled with the replay's seed so a user who just wants it kept can press Enter.
func (m *ReplayMenu) openNaming(what namingTarget) {
	m.open = false
	m.settings = nil
	m.naming = true
	m.namingWhat = what
	m.name = fmt.Sprintf("seed-%d", m.globals.Seed)
}

// updateNaming handles typing and the prompt's two buttons.
func (m *ReplayMenu) updateNaming() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		m.Close()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		m.commitNaming()
		return
	}
	m.typeName()

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()
	if hitRect(mx, my, nameSaveRect()) {
		m.saveRecording()
		return
	}
	if hitRect(mx, my, nameCancelRect()) {
		m.Close()
		return
	}
	// Clicks outside the box cancel, like every other modal here.
	if !hitRect(mx, my, nameBoxRect()) {
		m.Close()
	}
}

// typeName routes keystrokes into the name, the same alphabet the organism designer accepts.
func (m *ReplayMenu) typeName() {
	for _, ch := range ebiten.AppendInputChars(nil) {
		if ch == '\n' || ch == '\r' {
			continue
		}
		if ch >= ' ' && ch != 127 && len([]rune(m.name)) < nameMaxRunes {
			m.name += string(ch)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && m.name != "" {
		runes := []rune(m.name)
		m.name = string(runes[:len(runes)-1])
	}
}

// saveRecording copies the viewed .pzr into the recordings directory.
// commitNaming saves whatever the prompt was opened for.
func (m *ReplayMenu) commitNaming() {
	if m.namingWhat == namingSettings {
		m.saveSettings()
		return
	}
	m.saveRecording()
}

// saveSettings writes the replay's settings as a JSON config file under the
// typed name, in the format -config loads and the New Simulation screen's
// Load Settings lists.
func (m *ReplayMenu) saveSettings() {
	var buf bytes.Buffer
	c.DumpGlobals(&m.globals, &buf)
	path, err := c.SaveSettingsAs(c.SettingsDir, m.name, buf.Bytes())
	if err != nil {
		// The prompt stays up with the name intact, so a collision is
		// corrected rather than retyped.
		m.setNotice("Save failed: "+err.Error(), true)
		return
	}
	m.naming = false
	m.setNotice("Saved to "+path, false)
}

func (m *ReplayMenu) saveRecording() {
	if m.source == "" {
		m.setNotice("No recording file to save", true)
		return
	}
	path, err := checkpoint.SaveRecordingAs(m.source, checkpoint.RecordingsDir, m.name)
	if err != nil {
		m.setNotice("Save failed: "+err.Error(), true)
		return
	}
	m.naming = false
	m.setNotice("Saved to "+path, false)
}

func nameBoxRect() popupRectT {
	return newRect((c.ScreenWidth()-nameBoxW)/2, (c.ScreenHeight()-nameBoxH)/2, nameBoxW, nameBoxH)
}

func nameFieldRect() popupRectT {
	box := nameBoxRect()
	return newRect(box.Min.X+replayMenuPad, box.Min.Y+62, nameBoxW-2*replayMenuPad, nameFieldH)
}

func nameSaveRect() popupRectT {
	box := nameBoxRect()
	return newRect(box.Max.X-replayMenuPad-popupBtnW, box.Max.Y-replayMenuPad-replayMenuButtonH,
		popupBtnW, replayMenuButtonH)
}

func nameCancelRect() popupRectT {
	save := nameSaveRect()
	return newRect(save.Min.X-replayMenuGap-popupCancelW, save.Min.Y, popupCancelW, replayMenuButtonH)
}

func (m *ReplayMenu) drawNaming(screen *ebiten.Image) {
	drawScreenDim(screen)
	box := nameBoxRect()
	fillRect(screen, box, themeBackgroundColor())
	drawModalBorder(screen, box)

	title := m.namingWhat.title()
	tb := boundString(r.FontSourceCodePro12, title)
	text.Draw(screen, title, r.FontSourceCodePro12, box.Min.X+(nameBoxW-tb.Dx())/2,
		box.Min.Y+30, themedForeground())

	hint := "Saved into " + checkpoint.RecordingsDir + "/ — the next run overwrites the original."
	text.Draw(screen, hint, r.FontSourceCodePro8, box.Min.X+replayMenuPad, box.Min.Y+50, themedMuted())

	// The field, with a caret so it reads as something being typed into rather than a label.
	field := nameFieldRect()
	fillRect(screen, field, themedControlFill())
	drawModalBorder(screen, field)
	shown := m.name
	if time.Now().UnixMilli()/500%2 == 0 {
		shown += "_"
	}
	text.Draw(screen, shown, r.FontSourceCodePro12, field.Min.X+8, field.Min.Y+22, themedValue())

	// What it will actually be called, since the name is slugged.
	asFile := m.namingWhat.fileName(m.name)
	text.Draw(screen, "saves as "+asFile, r.FontSourceCodePro8,
		field.Min.X, field.Max.Y+13, themedMuted())

	mx, my := ebiten.CursorPosition()
	pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	for _, b := range []struct {
		rect   popupRectT
		label  string
		accent bool
	}{
		{nameCancelRect(), "Cancel", false},
		{nameSaveRect(), "Save", true},
	} {
		hovered := hitRect(mx, my, b.rect)
		if b.accent {
			drawAccentButton(screen, b.rect.Min.X, b.rect.Min.Y, b.rect.Dx(), b.rect.Dy(),
				b.label, hovered, hovered && pressed, color.RGBA{R: 80, G: 80, B: 90, A: 255})
		} else {
			drawMenuButton(screen, b.rect.Min.X, b.rect.Min.Y, b.rect.Dx(), b.rect.Dy(),
				b.label, hovered, hovered && pressed, false)
		}
	}

	m.drawNotice(screen, box.Min.X+replayMenuPad, nameSaveRect().Min.Y-8)
}

// drawNotice paints the last action's message, if it is still current.
func (m *ReplayMenu) drawNotice(screen *ebiten.Image, x, y int) {
	if m.notice == "" || !time.Now().Before(m.noticeUntil) {
		return
	}
	col := color.Color(themedForegroundDim())
	if m.noticeErr {
		col = themedBad()
	}
	text.Draw(screen, m.notice, r.FontSourceCodePro8, x, y, col)
}

// replayMenuRect is the menu box, centred on screen and sized to fit its buttons.
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

// settingsFooterRects lays out settingsButtons along the footer, in order.
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
