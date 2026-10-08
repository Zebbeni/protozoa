package ux

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

// A rules document is a list of blocks, each owning its own height and its
// own drawing. Text, a strip of animated organisms and a table of abilities
// have nothing in common but where they sit, so they are separate types
// behind one interface rather than a kind field and a switch.
type rulesBlock interface {
	// height is what the scroller advances by, and must match what draw paints.
	height() int
	// draw paints the block with its top-left at (x, y).
	draw(screen *ebiten.Image, x, y int, t rulesTick)
}

// rulesTick is the clock the animated blocks read, so everything on screen
// runs off one time source rather than each block holding its own.
type rulesTick struct {
	// frame is the sprite animation frame to show.
	frame int
	// elapsed is time since the screen opened, for animations longer than a
	// sprite loop.
	elapsed time.Duration
}

const (
	rulesSpriteScale = 3
	rulesSpriteCell  = 16
	// An _xl animation is two cells tall, and the base cell sits at the bottom.
	rulesSpriteH   = rulesSpriteCell * 2 * rulesSpriteScale
	rulesSpriteW   = rulesSpriteCell * rulesSpriteScale
	rulesCaptionH  = 14
	rulesRowGap    = 10
	rulesTableRowH = 16
)

// rulesHeading is a section title.
type rulesHeading struct{ text string }

func (b rulesHeading) height() int { return rulesLineHeightHead + rulesParaGap }
func (b rulesHeading) draw(screen *ebiten.Image, x, y int, _ rulesTick) {
	text.Draw(screen, b.text, r.FontSourceCodePro12, x, y+14, themedSectionTitle())
}

// rulesText is a wrapped paragraph.
type rulesText struct{ text string }

func (b rulesText) height() int {
	return len(wrapParagraph(b.text, rulesPanelW))*rulesLineHeightBody + rulesParaGap
}
func (b rulesText) draw(screen *ebiten.Image, x, y int, _ rulesTick) {
	for _, line := range wrapParagraph(b.text, rulesPanelW) {
		text.Draw(screen, line, r.FontSourceCodePro10, x, y+12, themedForegroundDim())
		y += rulesLineHeightBody
	}
}

// rulesSpriteCellSpec is one animated organism with a label under it.
type rulesSpriteCellSpec struct {
	label string
	anim  animation.Animation
	look  physiology.Appearance
	note  string
}

// rulesTable is a two-column list: a name and what it does.
type rulesTable struct {
	rows [][2]string
	// nameW is the pixel column the second entry starts at.
	nameW int
}

func (b rulesTable) height() int { return len(b.rows)*rulesTableRowH + rulesParaGap }
func (b rulesTable) draw(screen *ebiten.Image, x, y int, _ rulesTick) {
	for _, row := range b.rows {
		text.Draw(screen, row[0], r.FontSourceCodePro10, x, y+12, themedForeground())
		text.Draw(screen, row[1], r.FontSourceCodePro10, x+b.nameW, y+12, themedForegroundDim())
		y += rulesTableRowH
	}
}

// drawRulesOrganismAt paints one sample organism at a given baseline and
// scale, composited from the layers its appearance unlocks exactly as the
// grid does.
func drawRulesOrganismAt(screen *ebiten.Image, x, base float64, app physiology.Appearance, anim animation.Animation, frame int, scale float64) {
	// Facing up, so the two-cell animations reach upward into the space the
	// block already reserves above the base cell.
	drawRulesOrganismFacing(screen, x, base, app, anim, frame, scale, utils.Point{X: 0, Y: -1})
}

// drawRulesOrganismFacing is the same with the direction chosen, which decides
// which way a two-cell animation reaches out of its base cell.
func drawRulesOrganismFacing(screen *ebiten.Image, x, base float64, app physiology.Appearance, anim animation.Animation, frame int, scale float64, dir utils.Point) {
	body, _ := colorful.Hex("#6fd3a0")
	overlay, _ := colorful.Hex("#d9c27f")

	drew := false
	for _, layer := range r.OrganismLayersFor(app) {
		sprite := r.SpriteLayer(r.RoleOrganismMedium, layer, anim, frame)
		if sprite == nil {
			continue
		}
		col := overlay
		if r.UsesPrimaryColor(layer) {
			col = body
		}
		drawAnimatedSprite(screen, x, base, sprite, dir, col, rulesSpriteCell, scale)
		drew = true
	}
	if !drew {
		if sprite := r.Sprite(r.RoleOrganismMedium, anim, frame); sprite != nil {
			drawAnimatedSprite(screen, x, base, sprite, dir, body, rulesSpriteCell, scale)
		}
	}
}

// drawRulesNoteLine is one line of the small print beside an illustration.
func drawRulesNoteLine(screen *ebiten.Image, x, y int, line string) {
	if line == "" {
		return
	}
	text.Draw(screen, line, r.FontSourceCodePro10, x, y, themedForegroundDim())
}
