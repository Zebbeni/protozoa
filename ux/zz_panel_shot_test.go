package ux

import (
	"image/color"
	"image/png"
	"os"
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
)

// TestPanelShot renders the selected organism's TRAITS and STATS tabs to a PNG.
func TestPanelShot(t *testing.T) {
	if os.Getenv("PANEL_SHOT") == "" {
		t.Skip("set PANEL_SHOT=1 to render the organism panel")
	}
	loadKeyGlobals(t)
	if theme := os.Getenv("SHOT_THEME"); theme != "" {
		g := *config.GetCurrentGlobals()
		g.Theme = theme
		config.SetGlobals(&g)
	}
	r.UseDirAssets("..")
	r.Init()

	// A real simulation rather than a hand-built organism.
	cycles := 4000
	if v := os.Getenv("PANEL_SHOT_CYCLES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		cycles = n
	}
	sim := simulation.NewSimulation(&config.Options{Seed: 1})
	for i := 0; i < cycles && sim.OrganismCount() > 0; i++ {
		sim.Update()
	}
	all := sim.GetAllOrganismInfo()
	if len(all) == 0 {
		t.Fatal("no organisms survived the warm-up; nothing to draw")
	}
	// The largest one alive, so the size-scaled figures aren't all the same small number.
	best, bestSize := -1, -1.0
	for id, info := range all {
		if info.Size > bestSize {
			best, bestSize = id, info.Size
		}
	}
	sim.Select(best)
	t.Logf("selected organism %d, size %.1f", best, bestSize)

	out := os.Getenv("PANEL_SHOT_OUT")
	if out == "" {
		out = "panel_shot.png"
	}

	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err := ebiten.RunGame(&panelShot{
		panel: NewPanel(sim, NewGrid(sim)),
		out:   out,
		t:     t,
	})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
		t.Logf("window teardown: %v (%s was written)", err, out)
	}
}

type panelShot struct {
	panel *Panel
	out   string
	t     *testing.T
	frame int
	saved bool
}

func (s *panelShot) Layout(outsideWidth, outsideHeight int) (int, int) {
	return config.ScreenWidth(), config.ScreenHeight()
}

func (s *panelShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

const panelShotHeight = 460

func (s *panelShot) Draw(screen *ebiten.Image) {
	// The dark theme clears to transparent rather than painting black.
	flat := ebiten.NewImage(2*panelWidth, panelShotHeight)
	flat.Fill(color.RGBA{A: 255})

	for tab := 0; tab < 2; tab++ {
		s.panel.infoTab = tab
		tile := ebiten.NewImage(panelWidth, panelShotHeight)
		tile.Fill(themeBackgroundColor())
		// -selectedYOffset lifts the section to the top of the tile.
		s.panel.renderSelected(tile, -selectedYOffset+10)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(tab*panelWidth), 0)
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
