package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"

	r "github.com/Zebbeni/protozoa/resources"
)

// rulesAppearanceGrid shows the four appearance classes two across and two
// down, each with its own heading over the sprites it can wear.
//
// Stacked, the four were a heading and a strip apiece and ran to most of a
// screen; a class is three or four small sprites and a line of text, which
// fits a half-width column with room to spare.
type rulesAppearanceGrid struct {
	cells []appearanceDimension
}

// appearanceDimension is one class: what decides it, and every look it has.
type appearanceDimension struct {
	title string
	cells []rulesSpriteCellSpec
}

const (
	rulesAppCols      = 2
	rulesAppColW      = rulesPanelW / rulesAppCols
	rulesAppScale     = 3
	rulesAppCellW     = rulesSpriteCell * rulesAppScale
	rulesAppSpriteGap = 12
	rulesAppTitleH    = 18
	rulesAppCaptionH  = 14
	rulesAppRowGap    = 14
)

// appearanceTitles names each class for this page. appearanceRows is shared
// with the animation previewer, where the column is narrow enough that
// "SENS" has to do.
var appearanceTitles = []string{"BODY", "MOTOR", "SENSORS", "MOUTH"}

// appearanceSources says what each class is read from, which is the whole
// point of showing them: none of it is inherited.
var appearanceSources = []string{
	"from Tolerance and Defense",
	"from Movement",
	"from decision tree conditions",
	"from Eating, Attack or Digging",
}

func newRulesAppearanceGrid() rulesAppearanceGrid {
	dims := make([]appearanceDimension, 0, len(appearanceRows))
	for row := range appearanceRows {
		title := appearanceRows[row].label
		if row < len(appearanceTitles) {
			title = appearanceTitles[row]
		}
		if row < len(appearanceSources) {
			title += ", " + appearanceSources[row]
		}
		dims = append(dims, appearanceDimension{title: title, cells: appearanceCells(row)})
	}
	return rulesAppearanceGrid{cells: dims}
}

func (b rulesAppearanceGrid) rows() int {
	return (len(b.cells) + rulesAppCols - 1) / rulesAppCols
}

func (b rulesAppearanceGrid) rowHeight() int {
	return rulesAppTitleH + rulesAppCellW + rulesAppCaptionH + rulesAppRowGap
}

func (b rulesAppearanceGrid) height() int {
	return rulesParaGap + b.rows()*b.rowHeight()
}

func (b rulesAppearanceGrid) draw(screen *ebiten.Image, x, y int, t rulesTick) {
	restore := withHighResSprites()
	defer restore()

	y += rulesParaGap
	for i, dim := range b.cells {
		colX := x + (i%rulesAppCols)*rulesAppColW
		top := y + (i/rulesAppCols)*b.rowHeight()

		text.Draw(screen, dim.title, r.FontSourceCodePro10, colX, top+rulesAppTitleH-4, themedSectionTitle())
		spriteTop := top + rulesAppTitleH
		for j, cell := range dim.cells {
			cx := colX + j*(rulesAppCellW+rulesAppSpriteGap)
			drawRulesOrganismAt(screen, float64(cx), float64(spriteTop), cell.look, cell.anim, t.frame, rulesAppScale)
			lw := boundString(r.FontSourceCodePro8, cell.label).Dx()
			text.Draw(screen, cell.label, r.FontSourceCodePro8,
				cx+(rulesAppCellW-lw)/2, spriteTop+rulesAppCellW+rulesAppCaptionH-3, themedForeground())
		}
	}
}

// appearanceGridWidth is how far one class's sprites reach, for the test
// that keeps them inside their column.
func appearanceGridWidth(cells int) int {
	if cells < 1 {
		return 0
	}
	return cells*(rulesAppCellW+rulesAppSpriteGap) - rulesAppSpriteGap
}
