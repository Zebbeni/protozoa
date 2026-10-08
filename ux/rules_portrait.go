package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

// rulesPortrait is the panel's portrait column — the organism, its health
// against its size, and this cycle's gains and losses — drawn from the same
// functions the panel draws it with, beside a note naming the three parts.
type rulesPortrait struct {
	health, size float64
	ledger       organism.HealthLedger
	notes        []string
	img          *ebiten.Image
}

const (
	rulesPortraitLeft  = 60
	rulesPortraitNoteX = rulesPortraitLeft + portraitSize + 28
	rulesPortraitNoteY = 12
)

// rulesSampleLedger is a plausible cycle for the organism in the picture: it
// chemosynthesised, paid for the attempt, and lost a little to a pH away from
// its ideal.
func rulesSampleLedger() organism.HealthLedger {
	var l organism.HealthLedger
	l.Recorded = true
	l.Amounts[organism.HealthFromChemo] = 4.61
	l.Amounts[organism.HealthFromAction] = -0.11
	l.Amounts[organism.HealthFromPh] = -0.12
	return l
}

func newRulesPortrait() rulesPortrait {
	return rulesPortrait{
		health: 18.4,
		size:   24,
		ledger: rulesSampleLedger(),
		notes: []string{
			"The portrait shows the organism as the grid draws it,",
			"on the pH of the cell it is standing in.",
			"",
			"Under it, health against size: the bar fills as an",
			"organism grows into its own body.",
			"",
			"Under that, what this cycle gained and lost it, by",
			"source, with the net at the bottom.",
		},
	}
}

func (b rulesPortrait) height() int {
	return portraitColumnHeight() + rulesParaGap*2
}

func (b *rulesPortrait) portrait(frame int) *ebiten.Image {
	if b.img == nil {
		b.img = ebiten.NewImage(portraitSize, portraitSize)
	}
	body, _ := colorful.Hex("#6fd3a0")
	overlay, _ := colorful.Hex("#d9c27f")
	b.img.Fill(portraitPhBackground(rulesPhBlank()[0][0]))
	stampPortrait(b.img, physiology.Appearance{Motor: physiology.MotorPili, Sensor: physiology.SensorAntennae},
		r.RoleOrganismMedium, animation.AnimChemo, frame, utils.Point{X: 0, Y: -1}, body, overlay)
	return b.img
}

func (b rulesPortrait) draw(screen *ebiten.Image, x, y int, t rulesTick) {
	restore := withHighResSprites()
	defer restore()

	px, py := x+rulesPortraitLeft, y+rulesParaGap
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(px), float64(py))
	screen.DrawImage(b.portrait(t.frame), op)

	border := themedForegroundDim()
	fx, fy, fw, fh := float64(px), float64(py), float64(portraitSize), float64(portraitSize)
	ebitenutil.DrawRect(screen, fx, fy, fw, 1, border)
	ebitenutil.DrawRect(screen, fx, fy+fh-1, fw, 1, border)
	ebitenutil.DrawRect(screen, fx, fy, 1, fh, border)
	ebitenutil.DrawRect(screen, fx+fw-1, fy, 1, fh, border)

	drawPortraitHealth(screen, b.health, b.size, false, px, py)
	barY := py + portraitSize + healthBarGap
	drawHealthBar(screen, b.health, b.size, false, px, barY)
	drawHealthLedger(screen, b.ledger, false, px, barY+healthBarH, portraitSize)

	noteY := py + rulesPortraitNoteY
	for _, line := range b.notes {
		drawRulesNoteLine(screen, x+rulesPortraitNoteX, noteY, line)
		noteY += rulesLineHeightBody
	}
}
