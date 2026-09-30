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

func TestSaveMenuShot(t *testing.T) {
	if os.Getenv("SAVE_SHOT") == "" {
		t.Skip("set SAVE_SHOT=1 to render the save prompt")
	}
	loadKeyGlobals(t)
	if theme := os.Getenv("SHOT_THEME"); theme != "" {
		g := *config.GetCurrentGlobals()
		g.Theme = theme
		config.SetGlobals(&g)
	}
	r.UseDirAssets("..")
	r.Init()

	out := os.Getenv("SAVE_SHOT_OUT")
	if out == "" {
		out = "save_shot.png"
	}
	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err := ebiten.RunGame(&saveShot{out: out, t: t})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
		t.Logf("window teardown: %v (%s was written)", err, out)
	}
}

type saveShot struct {
	out   string
	t     *testing.T
	frame int
	saved bool
}

func (s *saveShot) Layout(int, int) (int, int) {
	return config.ScreenWidth(), config.ScreenHeight()
}

func (s *saveShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func (s *saveShot) Draw(screen *ebiten.Image) {
	g := *config.GetCurrentGlobals()
	// The menu on the left half, the save prompt on the right, so the two can be compared in one image.
	flat := ebiten.NewImage(2*config.ScreenWidth(), config.ScreenHeight())
	flat.Fill(color.RGBA{A: 255})

	for i, build := range []func(*ReplayMenu){
		func(m *ReplayMenu) { m.Open() },
		func(m *ReplayMenu) { m.Open(); m.openNaming() },
	} {
		m := NewReplayMenu(g, "/tmp/protozoa_last.pzr")
		build(m)
		tile := ebiten.NewImage(config.ScreenWidth(), config.ScreenHeight())
		tile.Fill(themeBackgroundColor())
		m.Draw(tile)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(i*config.ScreenWidth()), 0)
		flat.DrawImage(tile, op)
	}
	screen.DrawImage(flat, nil)

	s.frame++
	if s.frame < 4 {
		return
	}
	f, err := os.Create(s.out)
	if err != nil {
		s.t.Error(err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, flat); err != nil {
		s.t.Error(err)
	}
	s.saved = true
}
