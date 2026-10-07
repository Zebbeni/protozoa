package ux

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// The LOAD SETTINGS browser: a modal list of the .json files in the
// settings directory, opened from the New Simulation screen.
//
// A modal rather than its own app state, because the screen it loads into
// runs both standalone and embedded in the replay menu's popup, and a state
// switch would have to know which to come back to.

const (
	loadBoxW    = 460
	loadRowH    = 26
	loadMaxRows = 9
)

// openLoad reads the settings directory fresh every time, so a file saved
// since the screen opened shows up.
func (cs *ConfigScreen) openLoad() {
	cs.loadFiles = c.SavedSettings(c.SettingsDir)
	cs.loadOpen = true
	cs.loadScroll = 0
	cs.loadErr = ""
}

func (cs *ConfigScreen) closeLoad() {
	cs.loadOpen = false
	cs.loadErr = ""
}

// LoadOpen reports whether the browser is up, so the caller can hold back
// input that would otherwise reach the screen underneath.
func (cs *ConfigScreen) LoadOpen() bool { return cs.loadOpen }

func loadBoxRect() popupRectT {
	rows := loadMaxRows
	h := 54 + rows*loadRowH + 44
	return newRect((c.ScreenWidth()-loadBoxW)/2, (c.ScreenHeight()-h)/2, loadBoxW, h)
}

func loadRowRect(i int) popupRectT {
	box := loadBoxRect()
	return newRect(box.Min.X+replayMenuPad, box.Min.Y+50+i*loadRowH,
		loadBoxW-2*replayMenuPad, loadRowH-2)
}

func loadCancelRect() popupRectT {
	box := loadBoxRect()
	return newRect(box.Max.X-replayMenuPad-popupCancelW, box.Max.Y-replayMenuPad-replayMenuButtonH,
		popupCancelW, replayMenuButtonH)
}

// visibleLoadRows is the window of the list on screen, and where it starts.
func (cs *ConfigScreen) visibleLoadRows() ([]c.SettingsFile, int) {
	start := cs.loadScroll
	if start > len(cs.loadFiles)-loadMaxRows {
		start = len(cs.loadFiles) - loadMaxRows
	}
	if start < 0 {
		start = 0
	}
	end := start + loadMaxRows
	if end > len(cs.loadFiles) {
		end = len(cs.loadFiles)
	}
	return cs.loadFiles[start:end], start
}

func (cs *ConfigScreen) updateLoad() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		cs.closeLoad()
		return
	}
	if _, dy := ebiten.Wheel(); dy != 0 {
		cs.loadScroll -= int(dy)
		if cs.loadScroll < 0 {
			cs.loadScroll = 0
		}
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()
	if hitRect(mx, my, loadCancelRect()) {
		cs.closeLoad()
		return
	}
	rows, start := cs.visibleLoadRows()
	for i := range rows {
		if hitRect(mx, my, loadRowRect(i)) {
			cs.loadSettingsFile(cs.loadFiles[start+i])
			return
		}
	}
}

// loadSettingsFile replaces the settings being edited with the file's.
//
// The viewer's own window size, theme and pH palette are kept, the same
// three a replay's recorded settings are loaded around: they describe the
// machine looking at the simulation rather than the simulation, and taking
// them from a file would resize the window out from under the user.
func (cs *ConfigScreen) loadSettingsFile(f c.SettingsFile) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		cs.loadErr = "Couldn't read " + f.Name + ": " + err.Error()
		return
	}
	var loaded c.Globals
	if err := json.Unmarshal(data, &loaded); err != nil {
		cs.loadErr = f.Name + " isn't a settings file: " + err.Error()
		return
	}
	loaded.ScreenWidth = cs.globals.ScreenWidth
	loaded.ScreenHeight = cs.globals.ScreenHeight
	loaded.Theme = cs.globals.Theme
	loaded.PhColorScheme = cs.globals.PhColorScheme
	// The same repairs a run would apply, so the screen shows the values the
	// simulation would use rather than the zeros absent keys decode to.
	loaded.Repair()
	loaded.InitialAbilityScores = append([]int(nil), loaded.InitialAbilityScores...)
	*cs.globals = loaded
	cs.closeLoad()
	cs.selectedRow = -1
	cs.editingValue = ""
}

func (cs *ConfigScreen) drawLoad(screen *ebiten.Image) {
	drawScreenDim(screen)
	box := loadBoxRect()
	fillRect(screen, box, themeBackgroundColor())
	drawModalBorder(screen, box)

	title := "LOAD SETTINGS"
	tb := boundString(r.FontSourceCodePro12, title)
	text.Draw(screen, title, r.FontSourceCodePro12, box.Min.X+(loadBoxW-tb.Dx())/2,
		box.Min.Y+30, themedForeground())

	if len(cs.loadFiles) == 0 {
		// Says where they come from rather than only reporting none.
		msg := "No settings files in " + c.SettingsDir + "/"
		text.Draw(screen, msg, r.FontSourceCodePro10, box.Min.X+replayMenuPad, box.Min.Y+64, themedMuted())
		text.Draw(screen, "Save one from a replay: MENU > View Settings > Save As...",
			r.FontSourceCodePro10, box.Min.X+replayMenuPad, box.Min.Y+80, themedMuted())
	}

	mx, my := ebiten.CursorPosition()
	rows, _ := cs.visibleLoadRows()
	for i, f := range rows {
		rect := loadRowRect(i)
		if hitRect(mx, my, rect) {
			fillRect(screen, rect, themedSelectedRow())
		}
		ink := themedForeground()
		if hitRect(mx, my, rect) {
			ink = themedSelectedInk()
		}
		text.Draw(screen, f.Name, r.FontSourceCodePro10, rect.Min.X+6, rect.Min.Y+17, ink)
		meta := fmt.Sprintf("%s  %s", formatRecSize(f.Size), humanAge(f.ModTime))
		mb := boundString(r.FontSourceCodePro10, meta)
		text.Draw(screen, meta, r.FontSourceCodePro10, rect.Max.X-6-mb.Dx(), rect.Min.Y+17, themedMuted())
	}

	if cs.loadErr != "" {
		text.Draw(screen, cs.loadErr, r.FontSourceCodePro10, box.Min.X+replayMenuPad,
			box.Max.Y-replayMenuPad-replayMenuButtonH-8, themedBad())
	}

	cancel := loadCancelRect()
	hovered := hitRect(mx, my, cancel)
	drawMenuButton(screen, cancel.Min.X, cancel.Min.Y, cancel.Dx(), cancel.Dy(), "Cancel",
		hovered, hovered && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), false)
}
