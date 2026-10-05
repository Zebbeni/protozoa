package ux

import (
	"encoding/json"
	d "github.com/Zebbeni/protozoa/decision"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/resources"
)

// TestConfigScreenShot renders the settings screen to a PNG so a theme can be looked at without clicking through the GUI.
func TestConfigScreenShot(t *testing.T) {
	if os.Getenv("CFG_SHOT") != "1" {
		t.Skip("set CFG_SHOT=1 to render a settings screenshot")
	}

	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if theme := os.Getenv("SHOT_THEME"); theme != "" {
		g.Theme = theme
	}
	config.SetGlobals(&g)
	// The embedded bundle lives in main, which a test can't reach, so point the loaders at the repo on disk instead.
	resources.UseDirAssets("..")
	resources.Init()

	// NewConfigScreen reads the defaults out of the embedded bundle.
	defaults := g
	defaults.InitialAbilityScores = append([]int(nil), g.InitialAbilityScores...)
	loadConfigDefaults = func() config.Globals { return defaults }

	form := g
	cs := NewConfigScreen(&form)
	for i := range cs.sections {
		cs.sections[i].collapsed = false
	}
	if a := os.Getenv("SHOT_ABILITY"); a != "" {
		i, err := strconv.Atoi(a)
		if err != nil {
			t.Fatal(err)
		}
		cs.graphExpanded[physiology.Ability(i)] = true
	}
	if s := os.Getenv("SHOT_SCROLL"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatal(err)
		}
		cs.scrollY = v
	}

	out := os.Getenv("SHOT_OUT")
	if out == "" {
		out = "config_shot.png"
	}

	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err = ebiten.RunGame(&configShot{cs: cs, out: out, t: t})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
		t.Logf("window teardown: %v (%s was written)", err, out)
	}
}

type configShot struct {
	cs    *ConfigScreen
	out   string
	t     *testing.T
	frame int
	saved bool
}

// Update closes the window once the frame has been written.
func (s *configShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func (s *configShot) Draw(screen *ebiten.Image) {
	s.cs.Draw(screen)
	// A couple of warm-up frames: the first has nothing composited yet.
	s.frame++
	if s.frame < 3 {
		return
	}
	// The dark theme clears to transparent rather than painting black.
	flat := ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
	flat.Fill(color.RGBA{A: 255})
	flat.DrawImage(screen, nil)

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

func (s *configShot) Layout(w, h int) (int, int) {
	return config.ScreenWidth(), config.ScreenHeight()
}

func TestColorKeyShot(t *testing.T) {
	if os.Getenv("KEY_SHOT") != "1" {
		t.Skip("set KEY_SHOT=1 to render the colour keys")
	}

	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if theme := os.Getenv("SHOT_THEME"); theme != "" {
		g.Theme = theme
	}
	config.SetGlobals(&g)
	resources.UseDirAssets("..")
	resources.Init()

	out := os.Getenv("SHOT_OUT")
	if out == "" {
		out = "color_keys.png"
	}
	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err = ebiten.RunGame(&keyShot{out: out, t: t})
	if err != nil && err != ebiten.Termination {
		t.Fatal(err)
	}
}

type keyShot struct {
	out   string
	t     *testing.T
	frame int
	saved bool
}

func (s *keyShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func (s *keyShot) Draw(screen *ebiten.Image) {
	fillThemeBackground(screen)
	if os.Getenv("KEY_CORNER") == "1" {
		k := colorKeyFor(orgColorTolerance, physiology.AbilityDigging, d.ActEat)
		drawColorKey(screen, k, minimapMaxW, config.ScreenHeight()-minimapPadding-minimapMaxH-minimapPadding)
	} else {
		if os.Getenv("KEY_ABILITIES") == "1" {
			// One key per ability, since each now carries its own explanation of what that ability does.
			bottom := 20
			for _, a := range physiology.AllAbilities {
				k := colorKeyFor(orgColorAbility, a, d.ActEat)
				bottom += k.height(minimapMaxW) + 12
				drawColorKey(screen, k, minimapMaxW, bottom)
			}
			s.frame++
			if s.frame >= 3 {
				s.save(screen)
			}
			return
		}
		bottom := 20
		for _, m := range allOrgColorModes {
			k := colorKeyFor(m, physiology.AbilityDigging, d.ActEat)
			if k.empty() {
				continue
			}
			bottom += k.height(minimapMaxW) + 12
			drawColorKey(screen, k, minimapMaxW, bottom)
		}
	}

	s.frame++
	if s.frame < 3 {
		return
	}
	s.save(screen)
}

func (s *keyShot) save(screen *ebiten.Image) {
	flat := ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
	flat.Fill(color.RGBA{A: 255})
	flat.DrawImage(screen, nil)
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

func (s *keyShot) Layout(w, h int) (int, int) {
	return config.ScreenWidth(), config.ScreenHeight()
}
