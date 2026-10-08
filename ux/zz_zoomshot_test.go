package ux

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
)

// ZOOM_SHOT=1 ZOOM_SHOT_OUT=zoom8.png go test ./ux/ -run TestZoom8SpriteSetShot
//
// Renders the same world at Zoom8 with the 8x8 sprite set and with the
// 16x16 set at half scale, side by side, so the two can be compared.
// ZOOM_SHOT_CYCLES warms the world up, since a fresh one is a handful of
// founders on an empty grid.
func TestZoom8SpriteSetShot(t *testing.T) {
	if os.Getenv("ZOOM_SHOT") == "" {
		t.Skip("set ZOOM_SHOT=1")
	}
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	cycles := 3000
	if v := os.Getenv("ZOOM_SHOT_CYCLES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		cycles = n
	}
	_ = cycles
	sim := simulation.NewSimulation(&config.Options{Seed: 1})

	out := os.Getenv("ZOOM_SHOT_OUT")
	if out == "" {
		out = "zoom8.png"
	}
	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err := ebiten.RunGame(&zoomShot{sim: sim, out: out, t: t})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
		t.Logf("window teardown: %v (%s was written)", err, out)
	}
}

type zoomShot struct {
	sim   *simulation.Simulation
	infos map[int]*organism.Info
	out   string
	t     *testing.T
	frame int
	saved bool
}

func (s *zoomShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func (s *zoomShot) Layout(w, h int) (int, int) { return config.ScreenWidth(), config.ScreenHeight() }

// renderAt draws the organism, wall and food layers at Zoom8 using the given
// sprite set. A fresh Grid per side, because the layer images are cached
// against the sprite size and reusing one would hand the second pass the
// first one's.
func (s *zoomShot) renderAt(spriteSet int) *ebiten.Image {
	was := zoomSpriteSet[Zoom8]
	zoomSpriteSet[Zoom8] = spriteSet
	defer func() { zoomSpriteSet[Zoom8] = was }()

	grid := NewGrid(s.sim)
	grid.Camera.Zoom = Zoom8
	r.SelectZoom(grid.Camera.SpriteSet())

	unit := grid.Camera.GridUnitSize()
	img := ebiten.NewImage(unit*config.GridUnitsWide(), unit*config.GridUnitsHigh())
	grid.renderOrganisms(img, true, s.infos)
	return img
}

// sceneInfos is a controlled row of organisms, one per size tier, each
// wearing every overlay class the high-res art carries.
//
// A warmed simulation is the wrong subject for this: under the shipped
// settings the biggest organism measured 2.3 against a cap of 100, so every
// sprite in it is the TINY tier and the comparison shows nothing about the
// art. Size tiers bucket on quarters of maximum_max_size.
func sceneInfos() map[int]*organism.Info {
	maxSize := config.MaximumMaxSize()
	sizes := []float64{maxSize * 0.1, maxSize * 0.3, maxSize * 0.6, maxSize * 0.9}
	look := physiology.Appearance{
		Body:   physiology.BodySpikes,
		Motor:  physiology.MotorFlagella,
		Mouth:  physiology.MouthTusks,
		Sensor: physiology.SensorTasters,
	}
	body, _ := colorful.Hex("#6fd3a0")
	overlay, _ := colorful.Hex("#d9c27f")
	infos := map[int]*organism.Info{}
	for i, size := range sizes {
		id := i + 1
		infos[id] = &organism.Info{
			ID:             id,
			Location:       utils.Point{X: 3 + i*4, Y: 4},
			Direction:      utils.Point{X: 0, Y: -1},
			Size:           size,
			Color:          body,
			SecondaryColor: overlay,
			Status:         organism.StatusMoveSuccess,
			Appearance:     look,
		}
	}
	return infos
}

func (s *zoomShot) Draw(screen *ebiten.Image) {
	s.frame++
	if s.frame < 3 {
		return
	}
	s.infos = sceneInfos()
	// The two sprite SETS. Which filter to downscale the 16x16 art with is
	// settled (linear, a constant now), so there is no third panel.
	panels := []*ebiten.Image{s.renderAt(1), s.renderAt(2)}

	cells := envInt("ZOOM_SHOT_CELLS", 17)
	unit := 8
	cw := cells * unit
	crop := image.Rect(0, 0, cw, 8*unit)

	zoom := float64(envInt("ZOOM_SHOT_ZOOM", 10))
	gap := 12
	canvas := ebiten.NewImage(int(float64(cw)*zoom),
		int(float64(crop.Dy()*len(panels)+gap*(len(panels)-1)))*int(zoom))
	canvas.Fill(color.RGBA{A: 255})
	for i, img := range panels {
		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterNearest
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(0, float64(i)*float64(crop.Dy()+gap)*zoom)
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
	s.t.Logf("wrote %s (top: 8x8 set, bottom: 16x16 set at half scale)", s.out)
	s.saved = true
}
