package ux

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// MenuChoice is the action the user picked from the main menu. None
// means no choice yet (Update keeps polling).
type MenuChoice int

const (
	MenuChoiceNone MenuChoice = iota
	MenuChoiceNewSimulation
	MenuChoiceLoadPrevious
	MenuChoiceRules
	MenuChoiceExit
)

const (
	mainMenuTitle = "protozoa"

	mainMenuButtonW  = 280
	mainMenuButtonH  = 40
	mainMenuGap      = 14
	mainMenuTitleGap = 50 // pixels between title and first button
)

type menuButton struct {
	label  string
	choice MenuChoice
}

var mainMenuButtons = []menuButton{
	{"New Simulation", MenuChoiceNewSimulation},
	{"Load Previous", MenuChoiceLoadPrevious},
	{"Rules", MenuChoiceRules},
	{"Exit", MenuChoiceExit},
}

// MainMenu renders the top-level menu shown after the splash. Update
// returns the user's choice once a button is clicked, or MenuChoiceNone
// while the menu is idle. The runner reads the choice and transitions.
type MainMenu struct {
	// disabled marks specific choices as click-through-but-no-op. We
	// disable Load Previous when no .pzr file exists — the button still
	// renders so the layout stays stable, but reads as muted.
	disabled map[MenuChoice]bool
}

func NewMainMenu() *MainMenu {
	return &MainMenu{disabled: map[MenuChoice]bool{}}
}

// SetDisabled toggles whether a given choice should appear muted and
// reject clicks. Called by the runner after probing the filesystem for
// a saved replay.
func (m *MainMenu) SetDisabled(choice MenuChoice, off bool) {
	if off {
		m.disabled[choice] = true
	} else {
		delete(m.disabled, choice)
	}
}

func (m *MainMenu) Update() MenuChoice {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return MenuChoiceNone
	}
	mx, my := ebiten.CursorPosition()
	for i, btn := range mainMenuButtons {
		x, y := m.buttonRect(i)
		if mx >= x && mx < x+mainMenuButtonW && my >= y && my < y+mainMenuButtonH {
			if m.disabled[btn.choice] {
				return MenuChoiceNone
			}
			return btn.choice
		}
	}
	return MenuChoiceNone
}

// Draw paints the title and four buttons centred on screen. The hover
// state is computed at draw time so we don't need to track it across
// frames.
func (m *MainMenu) Draw(screen *ebiten.Image) {
	fillThemeBackground(screen)
	m.drawTitle(screen)

	mx, my := ebiten.CursorPosition()
	for i, btn := range mainMenuButtons {
		x, y := m.buttonRect(i)
		hovered := mx >= x && mx < x+mainMenuButtonW && my >= y && my < y+mainMenuButtonH
		pressed := hovered && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		dim := m.disabled[btn.choice]
		drawMenuButton(screen, x, y, mainMenuButtonW, mainMenuButtonH, btn.label, hovered && !dim, pressed && !dim, dim)
	}
}

// drawTitle paints the menu title using the same Inversionz face the
// replay panel uses for its top-left "protozoa" label, drawn at native
// size for an unambiguous "this is the same UI" cue. Centred above
// the button column.
func (m *MainMenu) drawTitle(screen *ebiten.Image) {
	face := r.FontInversionz40
	bounds := boundString(face, mainMenuTitle)
	tx := (c.ScreenWidth() - bounds.Dx()) / 2
	ty := m.titleY()
	text.Draw(screen, mainMenuTitle, face, tx, ty, themedForeground())
}

// titleY returns the baseline-y coordinate of the title so the title +
// gap + button stack renders vertically centred on screen.
func (m *MainMenu) titleY() int {
	titleH := boundString(r.FontInversionz40, mainMenuTitle).Dy()
	totalH := titleH + mainMenuTitleGap +
		len(mainMenuButtons)*mainMenuButtonH + (len(mainMenuButtons)-1)*mainMenuGap
	top := (c.ScreenHeight() - totalH) / 2
	return top + titleH
}

func (m *MainMenu) buttonRect(i int) (int, int) {
	x := (c.ScreenWidth() - mainMenuButtonW) / 2
	y := m.titleY() + mainMenuTitleGap + i*(mainMenuButtonH+mainMenuGap)
	return x, y
}

// drawMenuButton paints a generic themed button. Same chrome the popup
// reuses for Cancel/Start/View/Retry/Edit so the menu and popup share
// look-and-feel without each rolling its own button drawer.
func drawMenuButton(screen *ebiten.Image, x, y, w, h int, label string, hovered, pressed, dim bool) {
	base := chrome(
		color.RGBA{R: 50, G: 50, B: 70, A: 255},   // dark theme
		color.RGBA{R: 220, G: 220, B: 230, A: 255}, // light theme
	)
	fill := base
	switch {
	case dim:
		fill = shiftRGB(base, -20)
	case pressed:
		fill = shiftRGB(base, -15)
	case hovered:
		fill = shiftRGB(base, 25)
	}
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), fill)

	// Border so the button reads as a discrete control rather than a
	// flat patch of background.
	border := chrome(
		color.RGBA{R: 100, G: 100, B: 130, A: 255},
		color.RGBA{R: 150, G: 150, B: 165, A: 255},
	)
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, border)
	ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, border)
	ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), border)
	ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), border)

	bounds := boundString(r.FontSourceCodePro12, label)
	tx := x + (w-bounds.Dx())/2
	ty := y + (h+bounds.Dy())/2
	if pressed {
		ty++
	}
	fg := themedForeground()
	if dim {
		fg = themedForegroundDim()
	}
	text.Draw(screen, label, r.FontSourceCodePro12, tx, ty, fg)
}
