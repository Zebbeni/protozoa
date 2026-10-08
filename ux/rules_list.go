package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"

	r "github.com/Zebbeni/protozoa/resources"
)

// rulesActionList is the animated organisms, one per action, in a grid with
// each name under its own sprite.
//
// Four columns and the name underneath, rather than two columns with the
// name beside the sprite: at four across there is no room for a name as long
// as "Chemosynthesis Failure" next to a 48px cell, and under it the column
// only has to be as wide as the longer of the two.
type rulesActionList struct {
	cells []rulesSpriteCellSpec
}

const (
	rulesListScale   = 3
	rulesListCellW   = rulesSpriteCell * rulesListScale
	rulesListCols    = 4
	rulesListColW    = rulesPanelW / rulesListCols
	rulesListCaption = 16
	rulesListGap     = 10
)

func (b rulesActionList) rows() int {
	return (len(b.cells) + rulesListCols - 1) / rulesListCols
}

// cellHeight is one sprite cell, or two where a row holds an _xl animation
// that reaches into the cell ahead of it.
func (b rulesActionList) cellHeight(row int) int {
	for col := 0; col < rulesListCols; col++ {
		i := row*rulesListCols + col
		if i < len(b.cells) && isMultiCellAnim(b.cells[i].anim) {
			return rulesListCellW * 2
		}
	}
	return rulesListCellW
}

func (b rulesActionList) rowHeight(row int) int {
	return b.cellHeight(row) + rulesListCaption + rulesListGap
}

func (b rulesActionList) height() int {
	h := rulesParaGap
	for row := 0; row < b.rows(); row++ {
		h += b.rowHeight(row)
	}
	return h
}

func (b rulesActionList) draw(screen *ebiten.Image, x, y int, t rulesTick) {
	restore := withHighResSprites()
	defer restore()

	for row := 0; row < b.rows(); row++ {
		cellH := b.cellHeight(row)
		// The base cell sits at the bottom of the row, so a two-cell
		// animation reaches up into the space above it.
		base := float64(y + cellH - rulesListCellW)
		for col := 0; col < rulesListCols; col++ {
			i := row*rulesListCols + col
			if i >= len(b.cells) {
				break
			}
			colX := x + col*rulesListColW
			drawRulesOrganismAt(screen, float64(colX+(rulesListColW-rulesListCellW)/2), base,
				b.cells[i].look, b.cells[i].anim, t.frame, rulesListScale)

			label := b.cells[i].label
			lw := boundString(r.FontSourceCodePro10, label).Dx()
			text.Draw(screen, label, r.FontSourceCodePro10,
				colX+(rulesListColW-lw)/2, y+cellH+rulesListCaption, themedForeground())
		}
		y += b.rowHeight(row)
	}
}
