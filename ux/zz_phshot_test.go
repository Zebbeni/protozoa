package ux

import (
	"image"
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

// TestPhFilterShot renders the pH layer unsmoothed and smoothed side by side,
// so the two can be compared without running the simulation.
//
//	PH_SHOT=1 PH_SHOT_OUT=ph.png go test ./ux/ -run TestPhFilterShot
//
// PH_SHOT_CYCLES sets the warm-up, because the pH field starts uniform: a
// shot of a fresh world compares two pictures of one flat colour.
func TestPhFilterShot(t *testing.T) {
	if os.Getenv("PH_SHOT") == "" {
		t.Skip("set PH_SHOT=1 to render the pH layer both ways")
	}
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	cycles := 3000
	if v := os.Getenv("PH_SHOT_CYCLES"); v != "" {
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

	out := os.Getenv("PH_SHOT_OUT")
	if out == "" {
		out = "ph_filter.png"
	}
	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err := ebiten.RunGame(&phShot{sim: sim, out: out, t: t})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
		t.Logf("window teardown: %v (%s was written)", err, out)
	}
}

type phShot struct {
	sim   *simulation.Simulation
	out   string
	t     *testing.T
	frame int
	saved bool
}

func (s *phShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func (s *phShot) Layout(w, h int) (int, int) { return config.ScreenWidth(), config.ScreenHeight() }

// render draws the pH layer once with the given smoothing.
//
// A fresh Grid per side, because the upscale is cached against the sprite
// size and reusing one would hand the second pass the first one's image.
func (s *phShot) render(smooth bool) *ebiten.Image {
	g := *config.GetCurrentGlobals()
	g.PhSmoothing = smooth
	config.SetGlobals(&g)
	grid := NewGrid(s.sim)
	unit := grid.Camera.GridUnitSize()
	img := ebiten.NewImage(unit*config.GridUnitsWide(), unit*config.GridUnitsHigh())
	grid.renderPh(img, true)
	return img
}

func (s *phShot) Draw(screen *ebiten.Image) {
	s.frame++
	if s.frame < 3 {
		return
	}
	flat := s.render(false)
	blended := s.render(true)

	// A CROP, blown up: the whole world at one pixel per cell is far too
	// small to show what this is asking about, which is what one cell's
	// edge looks like. PH_SHOT_CELLS cells across, from PH_SHOT_AT.
	cells := envInt("PH_SHOT_CELLS", 24)
	atX := envInt("PH_SHOT_AT_X", 0)
	atY := envInt("PH_SHOT_AT_Y", 0)
	unit := NewGrid(s.sim).Camera.GridUnitSize()
	cw := cells * unit
	crop := image.Rect(atX*unit, atY*unit, atX*unit+cw, atY*unit+cw)

	zoom := float64(envInt("PH_SHOT_ZOOM", 8))
	gap := 16
	canvas := ebiten.NewImage(int(float64(cw*2+gap)*zoom), int(float64(cw)*zoom))
	canvas.Fill(color.RGBA{A: 255})
	for i, img := range []*ebiten.Image{flat, blended} {
		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterNearest
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(i)*float64(cw+gap)*zoom, 0)
		canvas.DrawImage(img.SubImage(crop).(*ebiten.Image), op)
	}

	f, err := os.Create(s.out)
	if err != nil {
		s.t.Error(err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, canvas); err != nil {
		s.t.Error(err)
	}
	s.saved = true
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
