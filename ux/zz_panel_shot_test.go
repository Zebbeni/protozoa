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

// TestPanelShot renders each of the selected organism's four tabs to a PNG.
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
	// PANEL_SHOT_NETS=1 renders the same four tabs for a world running on
	// neural nets, where the third one is the BRAIN diagram rather than a
	// printed decision tree.
	if os.Getenv("PANEL_SHOT_NETS") == "1" {
		g := *config.GetCurrentGlobals()
		g.UseNeuralNets = true
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
	// PANEL_SHOT_SEED picks the world. Worth having now that the panel can be
	// shot in two: a seed that survives thousands of cycles on decision trees
	// need not survive them on neural nets, and an empty world draws nothing.
	seed := 1
	if v := os.Getenv("PANEL_SHOT_SEED"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		seed = n
	}
	// No checkpoints: this is a screenshot, not a recording, and the warm-up
	// becomes I/O bound without it — a neural organism's snapshot record
	// carries its whole net, which is kilobytes apiece.
	sim := simulation.NewSimulation(&config.Options{Seed: seed, CheckpointInterval: 1 << 30})
	for i := 0; i < cycles && sim.OrganismCount() > 0; i++ {
		sim.Update()
	}
	all := sim.GetAllOrganismInfo()
	if len(all) == 0 {
		t.Fatal("no organisms survived the warm-up; nothing to draw")
	}
	// The largest one alive, so the size-scaled figures aren't all the same
	// small number. PANEL_SHOT_PICK=brain picks the biggest NET instead,
	// which is what the BRAIN tab's layout wants: the largest organism often
	// carries a two-node brain, and a two-node brain says nothing about how
	// a twenty-row diagram lays out.
	byBrain := os.Getenv("PANEL_SHOT_PICK") == "brain"
	best, bestScore := -1, -1.0
	for id, info := range all {
		score := info.Size
		if byBrain {
			score = 0
			if net := sim.GetOrganismNeuralNetByID(id); net != nil {
				score = float64(net.Nodes())
			}
		}
		if score > bestScore {
			best, bestScore = id, score
		}
	}
	bestSize := all[best].Size
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

func (s *panelShot) Draw(screen *ebiten.Image) {
	// A screen tall, because the BRAIN tab spreads its rows into what is left
	// of the screen below it and this tile is what it is measuring against.
	panelShotHeight := config.ScreenHeight()
	// The dark theme clears to transparent rather than painting black.
	flat := ebiten.NewImage(len(orgTabLabels())*panelWidth, panelShotHeight)
	flat.Fill(color.RGBA{A: 255})

	for tab := range orgTabLabels() {
		s.panel.selectedTab = tab
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
