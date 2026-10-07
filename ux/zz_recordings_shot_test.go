package ux

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/checkpoint"
	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

func TestRecordingsShot(t *testing.T) {
	if os.Getenv("REC_SHOT") == "" {
		t.Skip("set REC_SHOT=1 to render the recordings browser")
	}
	loadKeyGlobals(t)
	if theme := os.Getenv("SHOT_THEME"); theme != "" {
		g := *config.GetCurrentGlobals()
		g.Theme = theme
		config.SetGlobals(&g)
	}
	r.UseDirAssets("..")
	r.Init()

	out := os.Getenv("REC_SHOT_OUT")
	if out == "" {
		out = "recordings_shot.png"
	}
	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err := ebiten.RunGame(&recShot{out: out, t: t})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
		t.Logf("window teardown: %v (%s was written)", err, out)
	}
}

type recShot struct {
	out   string
	t     *testing.T
	frame int
	saved bool
}

func (s *recShot) Layout(int, int) (int, int) {
	return config.ScreenWidth(), config.ScreenHeight()
}

func (s *recShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func fakeRecordings() []checkpoint.SavedRecording {
	now := time.Now()
	rows := []struct {
		name string
		size int64
		ago  time.Duration
	}{
		{"seed-1789794294595329300", 248 << 20, 30 * time.Second},
		{"two-big-ph-swings", 112 << 20, 22 * time.Minute},
		{"the-one-that-oscillated", 64 << 20, 5 * time.Hour},
		{"digging-specialists", 9 << 20, 3 * 24 * time.Hour},
		{"extinct-at-600", 320 << 10, 40 * 24 * time.Hour},
	}
	out := make([]checkpoint.SavedRecording, 0, len(rows))
	for _, row := range rows {
		out = append(out, checkpoint.SavedRecording{
			Name: row.name,
			Path: filepath.Join(checkpoint.RecordingsDir, row.name+".pzr"),
			Size: row.size,
			Mod:  now.Add(-row.ago),
		})
	}
	return out
}

func (s *recShot) Draw(screen *ebiten.Image) {
	flat := ebiten.NewImage(2*config.ScreenWidth(), config.ScreenHeight())
	flat.Fill(color.RGBA{A: 255})

	full := &RecordingsScreen{confirmDelete: 2, items: fakeRecordings()}
	full.setMessage("click again to delete "+full.items[2].Name, false)
	empty := &RecordingsScreen{confirmDelete: -1}

	for i, sc := range []*RecordingsScreen{full, empty} {
		tile := ebiten.NewImage(config.ScreenWidth(), config.ScreenHeight())
		sc.Draw(tile)
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
