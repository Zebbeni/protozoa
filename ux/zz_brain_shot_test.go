package ux

import (
	"image/color"
	"image/png"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/replay"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
)

// TestBrainPopupShot renders the expanded brain view over the panel it was
// opened from, so the two sizes of the same net can be compared in one image.
// Same gated-screenshot pattern as the panel and colour-key shots.
func TestBrainPopupShot(t *testing.T) {
	if os.Getenv("BRAIN_SHOT") == "" {
		t.Skip("set BRAIN_SHOT=1 to render the expanded brain view")
	}
	loadKeyGlobals(t)
	g := *config.GetCurrentGlobals()
	g.UseNeuralNets = true
	if theme := os.Getenv("SHOT_THEME"); theme != "" {
		g.Theme = theme
	}
	config.SetGlobals(&g)
	r.UseDirAssets("..")
	r.Init()

	cycles, seed := 1500, 6
	if v := os.Getenv("BRAIN_SHOT_CYCLES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		cycles = n
	}
	if v := os.Getenv("BRAIN_SHOT_SEED"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		seed = n
	}

	// BRAIN_SHOT_REPLAY draws an organism out of a recording instead of a
	// fresh world, which is how a net a user is asking about gets looked at.
	var sim *simulation.Simulation
	if path := os.Getenv("BRAIN_SHOT_REPLAY"); path != "" {
		ctrl, err := replay.NewController(path, &config.Options{IsHeadless: true, ReplayFile: path})
		if err != nil {
			t.Fatal(err)
		}
		defer ctrl.Close()
		if err := ctrl.SeekToCycle(cycles); err != nil {
			t.Fatal(err)
		}
		sim = ctrl.Simulation()
		for sim.Cycle() < cycles {
			ctrl.StepForward()
		}
	} else {
		sim = simulation.NewSimulation(&config.Options{Seed: seed, CheckpointInterval: 1 << 30})
		for i := 0; i < cycles && sim.OrganismCount() > 0; i++ {
			sim.Update()
		}
	}
	all := sim.GetAllOrganismInfo()
	if len(all) == 0 {
		t.Fatal("no organisms survived the warm-up; nothing to draw")
	}
	// BRAIN_SHOT_ID names the organism; without one it is the biggest net,
	// since a two-node brain says nothing about how the diagram lays out.
	best, bestNodes := -1, -1
	if v := os.Getenv("BRAIN_SHOT_ID"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		best = n
		if net := sim.GetOrganismNeuralNetByID(best); net != nil {
			bestNodes = net.Nodes()
		}
	}
	if best < 0 {
		for id := range all {
			nodes := 0
			if net := sim.GetOrganismNeuralNetByID(id); net != nil {
				nodes = net.Nodes()
			}
			if nodes > bestNodes {
				best, bestNodes = id, nodes
			}
		}
	}
	sim.Select(best)
	t.Logf("selected organism %d, %d brain nodes", best, bestNodes)

	out := os.Getenv("BRAIN_SHOT_OUT")
	if out == "" {
		out = "brain_shot.png"
	}

	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	panel := NewPanel(sim, NewGrid(sim))
	// BRAIN_SHOT_FOCUS=col,row picks out a node, the way a click does.
	var focus *brainNode
	if v := os.Getenv("BRAIN_SHOT_FOCUS"); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) != 2 {
			t.Fatalf("BRAIN_SHOT_FOCUS=%q, want col,row", v)
		}
		col, err := strconv.Atoi(parts[0])
		if err != nil {
			t.Fatal(err)
		}
		row, err := strconv.Atoi(parts[1])
		if err != nil {
			t.Fatal(err)
		}
		focus = &brainNode{col: col, row: row}
	}
	if os.Getenv("BRAIN_SHOT_NONUMBERS") != "" {
		panel.brainNumbersHidden = true
	}
	if os.Getenv("BRAIN_SHOT_VIEW") == "weights" {
		panel.brainView = brainViewWeights
	}
	popup := NewBrainPopup(panel)
	err := ebiten.RunGame(&brainShot{panel: panel, popup: popup, focus: focus, out: out, t: t})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
		t.Logf("window teardown: %v (%s was written)", err, out)
	}
}

type brainShot struct {
	panel *Panel
	popup *BrainPopup
	focus *brainNode
	out   string
	t     *testing.T
	frame int
	saved bool
}

func (s *brainShot) Layout(outsideWidth, outsideHeight int) (int, int) {
	return config.ScreenWidth(), config.ScreenHeight()
}

func (s *brainShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func (s *brainShot) Draw(screen *ebiten.Image) {
	// The dark theme clears to transparent rather than painting black.
	flat := ebiten.NewImage(config.ScreenWidth(), config.ScreenHeight())
	flat.Fill(color.RGBA{A: 255})

	// The panel first, since drawing the BRAIN tab is what hands the popup
	// the net to draw.
	s.panel.selectedTab = orgTabDecision
	flat.DrawImage(s.panel.Render(), nil)

	s.popup.Open()
	s.popup.focus = s.focus
	s.popup.Draw(flat)
	screen.DrawImage(flat, nil)

	s.frame++
	if s.frame < 4 {
		return
	}
	if _, _, _, ok := s.panel.ExpandedBrain(); !ok {
		s.t.Error("the panel drew no brain, so the popup had nothing to expand")
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
