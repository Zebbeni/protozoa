package ux

import (
	"image/color"
	"image/png"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// RULES_SHOT=1 RULES_SHOT_OUT=rules.png go test ./ux/ -run TestRulesShot
//
// RULES_SHOT_SCROLL picks how far down the document to render, since it is
// several screens tall and the sprite strips are most of the point.
func TestRulesShot(t *testing.T) {
	if os.Getenv("RULES_SHOT") == "" {
		t.Skip("set RULES_SHOT=1")
	}
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	out := os.Getenv("RULES_SHOT_OUT")
	if out == "" {
		out = "rules.png"
	}
	scroll := 0
	if v := os.Getenv("RULES_SHOT_SCROLL"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		scroll = n
	}
	// RULES_SHOT_TIME winds the screen's clock forward, so the animated
	// blocks can be shot part-way through rather than only at step 0.
	var elapsed time.Duration
	if v := os.Getenv("RULES_SHOT_TIME"); v != "" {
		secs, err := strconv.ParseFloat(v, 64)
		if err != nil {
			t.Fatal(err)
		}
		elapsed = time.Duration(secs * float64(time.Second))
	}
	// RULES_SHOT_TAB picks which tab to render.
	tab := 0
	if v := os.Getenv("RULES_SHOT_TAB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		tab = n
	}
	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err := ebiten.RunGame(&rulesShot{out: out, scroll: scroll, tab: tab, elapsed: elapsed, t: t})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
		t.Logf("window teardown: %v (%s was written)", err, out)
	}
}

type rulesShot struct {
	out     string
	scroll  int
	tab     int
	elapsed time.Duration
	t       *testing.T
	frame   int
	saved   bool
}

func (s *rulesShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func (s *rulesShot) Layout(w, h int) (int, int) {
	return config.ScreenWidth(), config.ScreenHeight()
}

func (s *rulesShot) Draw(screen *ebiten.Image) {
	s.frame++
	if s.frame < 3 {
		return
	}
	rs := NewRulesScreen()
	rs.started = rs.started.Add(-s.elapsed)
	rs.selectTab(s.tab)
	rs.setScrollY(float64(s.scroll))
	img := ebiten.NewImage(config.ScreenWidth(), config.ScreenHeight())
	rs.Draw(img)

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
	s.t.Logf("wrote %s: tab %q at scroll %d, %dpx of content under a %dpx header",
		s.out, rs.tabs[rs.tab].label, s.scroll, rs.contentHeight(), rs.headerHeight())
	s.saved = true
}
