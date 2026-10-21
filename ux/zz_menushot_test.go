package ux

import (
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// MENU_SHOT=1 SHOT_THEME=light MENU_SHOT_OUT=menu.png go test ./ux/ -run TestMainMenuShot
//
// The title's display size is a judgement about how it sits over the button
// stack, which no measurement settles: TestTheMenuStackFitsTheWindow says it
// fits the window and nothing says how it looks there.
func TestMainMenuShot(t *testing.T) {
	if os.Getenv("MENU_SHOT") == "" {
		t.Skip("set MENU_SHOT=1")
	}
	loadKeyGlobals(t)
	if theme := os.Getenv("SHOT_THEME"); theme != "" {
		g := *config.GetCurrentGlobals()
		g.Theme = theme
		config.SetGlobals(&g)
	}
	r.UseDirAssets("..")
	r.Init()

	out := os.Getenv("MENU_SHOT_OUT")
	if out == "" {
		out = "menu.png"
	}
	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err := ebiten.RunGame(&menuShot{out: out, t: t})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
		t.Logf("window teardown: %v (%s was written)", err, out)
	}
}

type menuShot struct {
	out   string
	t     *testing.T
	frame int
	saved bool
}

func (s *menuShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func (s *menuShot) Layout(w, h int) (int, int) {
	return config.ScreenWidth(), config.ScreenHeight()
}

func (s *menuShot) Draw(screen *ebiten.Image) {
	s.frame++
	if s.frame < 3 {
		return
	}
	img := ebiten.NewImage(config.ScreenWidth(), config.ScreenHeight())
	NewMainMenu().Draw(img)

	// Composited onto opaque black: the dark theme CLEARS rather than
	// painting, so the file would otherwise carry partly-transparent glyphs
	// over nothing and read differently in every viewer.
	canvas := ebiten.NewImage(config.ScreenWidth(), config.ScreenHeight())
	canvas.Fill(color.RGBA{A: 255})
	canvas.DrawImage(img, &ebiten.DrawImageOptions{})

	f, err := os.Create(s.out)
	if err != nil {
		s.t.Error(err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, canvas); err != nil {
		s.t.Error(err)
	}
	title := boundString(mainMenuTitleFace(), mainMenuTitle)
	s.t.Logf("wrote %s: title %dx%dpx over a %dpx stack of %d buttons in a %dx%d window",
		s.out, title.Dx(), title.Dy(), mainMenuButtonW, len(mainMenuButtons),
		config.ScreenWidth(), config.ScreenHeight())
	s.saved = true
}
