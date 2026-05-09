package ux

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

const (
	splashFadeIn = 900 * time.Millisecond
	splashHold   = 700 * time.Millisecond
	splashTotal  = splashFadeIn + splashHold
	splashTitle  = "protozoa"
	// FontInversionz40 is already a chunky display face — a small
	// FilterNearest upscale gets the deliberately-blocky look without
	// blowing the title out of the centre column.
	splashPixelize = 2
)

// Splash is the title-card screen shown at first launch when no CLI
// flags steered us elsewhere. The title is rendered to a small offscreen
// image and scaled up with nearest-neighbour filtering, so the type
// reads as deliberately blocky rather than just a big font.
type Splash struct {
	start    time.Time
	titleImg *ebiten.Image
	done     bool
}

func NewSplash() *Splash {
	s := &Splash{start: time.Now()}
	s.bakeTitle()
	return s
}

// bakeTitle renders the title text once into a tight offscreen image so
// every Draw can DrawImage it (scaled + tinted) instead of re-rasterising
// the glyphs each frame. Same Inversionz face the replay panel uses,
// then upscaled with FilterNearest for a pixel-art feel.
func (s *Splash) bakeTitle() {
	face := r.FontInversionz40
	bounds := boundString(face, splashTitle)
	pad := 4
	w, h := bounds.Dx()+pad*2, bounds.Dy()+pad*2
	img := ebiten.NewImage(w, h)
	text.Draw(img, splashTitle, face, pad-bounds.Min.X, pad-bounds.Min.Y, color.White)
	s.titleImg = img
}

// Update advances the splash and reports whether it's finished. Returns
// true after the full fade-in + hold elapses, or as soon as the user
// clicks/taps a key — splash is skippable so impatient launches don't
// pay the animation cost.
func (s *Splash) Update() bool {
	if s.done {
		return true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		s.done = true
		return true
	}
	if time.Since(s.start) >= splashTotal {
		s.done = true
		return true
	}
	return false
}

func (s *Splash) Draw(screen *ebiten.Image) {
	fillThemeBackground(screen)
	if s.titleImg == nil {
		return
	}

	// Fade in linearly during splashFadeIn; full opacity afterwards.
	alpha := float32(1)
	if elapsed := time.Since(s.start); elapsed < splashFadeIn {
		alpha = float32(elapsed) / float32(splashFadeIn)
	}

	b := s.titleImg.Bounds()
	drawW := float64(b.Dx() * splashPixelize)
	drawH := float64(b.Dy() * splashPixelize)
	sw, sh := float64(c.ScreenWidth()), float64(c.ScreenHeight())

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(splashPixelize), float64(splashPixelize))
	op.GeoM.Translate(sw/2-drawW/2, sh/2-drawH/2)
	op.Filter = ebiten.FilterNearest

	fr, fg, fb := foregroundFloats()
	op.ColorScale.Scale(fr*alpha, fg*alpha, fb*alpha, alpha)
	screen.DrawImage(s.titleImg, op)
}

// foregroundFloats returns the active theme's foreground colour as
// 0..1 floats for use with ColorScale.Scale (which expects pre-
// multiplied scale factors when the alpha channel is also scaled).
func foregroundFloats() (float32, float32, float32) {
	r, g, b, _ := themedForeground().RGBA()
	return float32(r) / 0xffff, float32(g) / 0xffff, float32(b) / 0xffff
}
