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

// TestColorSectionShot renders the ORGANISM COLOR section — the colour rows,
// the bracketed ability and action groups — so the layout can be looked at.
//
//	COLSEC_SHOT=1 COLSEC_SHOT_OUT=sec.png go test ./ux/ -run TestColorSectionShot
func TestColorSectionShot(t *testing.T) {
	if os.Getenv("COLSEC_SHOT") == "" {
		t.Skip("set COLSEC_SHOT=1 to render the organism colour section")
	}
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	out := os.Getenv("COLSEC_SHOT_OUT")
	if out == "" {
		out = "color_section.png"
	}
	ebiten.SetWindowSize(config.ScreenWidth(), config.ScreenHeight())
	err := ebiten.RunGame(&colSecShot{out: out, t: t})
	if err != nil && err != ebiten.Termination {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatal(err)
		}
	}
}

type colSecShot struct {
	out   string
	t     *testing.T
	frame int
	saved bool
}

func (s *colSecShot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	return nil
}

func (s *colSecShot) Layout(w, h int) (int, int) { return config.ScreenWidth(), config.ScreenHeight() }

func (s *colSecShot) Draw(screen *ebiten.Image) {
	s.frame++
	if s.frame < 3 {
		return
	}
	grid := &Grid{orgColor: orgColorAbility}
	p := &Panel{grid: grid}

	const zoom = 3
	h := (orgColorRows + abilityRows + actionRows) * sectionRowPitch
	panelImg := ebiten.NewImage(panelWidth, orgColorYOffset+h+8)
	panelImg.Fill(color.RGBA{A: 255})
	p.renderOrgColor(panelImg, 0)

	canvas := ebiten.NewImage(panelWidth*zoom, (orgColorYOffset+h+8)*zoom)
	canvas.Fill(color.RGBA{A: 255})
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(zoom, zoom)
	canvas.DrawImage(panelImg, op)

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
