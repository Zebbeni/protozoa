package ux

import (
	"image/color"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// DrawLoadingReplayOverlay paints a full-screen dim layer with an
// animated "Loading replay..." message centred on it. Used by the
// runner's stateLoadingReplay when entering from the main menu (no
// popup overlay is in the way) so the user gets immediate visual
// feedback while replay.NewController loads in the background. The
// dot animation is keyed off `since` so the caller controls when the
// animation starts.
func DrawLoadingReplayOverlay(screen *ebiten.Image, since time.Time) {
	sw, sh := c.ScreenWidth(), c.ScreenHeight()

	dim := chrome(
		color.RGBA{R: 0, G: 0, B: 0, A: 200},
		color.RGBA{R: 235, G: 235, B: 240, A: 220},
	)
	ebitenutil.DrawRect(screen, 0, 0, float64(sw), float64(sh), dim)

	nDots := int(time.Since(since).Milliseconds()/300) % 4
	msg := "Loading replay" + strings.Repeat(".", nDots)
	bounds := boundString(r.FontSourceCodePro12, msg)
	tx := (sw - bounds.Dx()) / 2
	ty := sh/2 + bounds.Dy()/2
	text.Draw(screen, msg, r.FontSourceCodePro12, tx, ty, themedForeground())
}
