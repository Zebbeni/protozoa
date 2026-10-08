package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"

	d "github.com/Zebbeni/protozoa/decision"
	r "github.com/Zebbeni/protozoa/resources"
)

// rulesTreeBlock prints a decision tree the way the panel prints the selected
// organism's, through the same PrintLines and the same inks, so what the
// rules show is what the player will see.
type rulesTreeBlock struct {
	lines []d.PrintLine
}

const rulesTreeIndent = 40

// exampleTree is a four-node tree with the branch it took last cycle marked,
// which is what gives the printed lines their three tones.
func exampleTree() *d.Tree {
	root := d.NodeFromCondition(d.IsFoodHere)
	root.YesNode = d.NodeFromAction(d.ActEat)
	root.NoNode = d.NodeFromCondition(d.CanMove)
	root.NoNode.YesNode = d.NodeFromAction(d.ActMove)
	root.NoNode.NoNode = d.NodeFromAction(d.ActTurnLeft)

	// No food here, and the way ahead was clear, so it moved.
	root.UsedLastCycle = true
	root.NoNode.UsedLastCycle = true
	root.NoNode.YesNode.UsedLastCycle = true
	// The turn has been taken before; nothing has ever eaten.
	root.NoNode.NoNode.WasTravelled = true

	return d.TreeFromNode(root)
}

func newRulesTreeBlock() rulesTreeBlock {
	return rulesTreeBlock{lines: exampleTree().PrintLines()}
}

func (b rulesTreeBlock) lineHeight() int {
	return r.FontSourceCodePro10.Metrics().Height.Round()
}

func (b rulesTreeBlock) height() int {
	return len(b.lines)*b.lineHeight() + rulesParaGap*2
}

func (b rulesTreeBlock) draw(screen *ebiten.Image, x, y int, _ rulesTick) {
	lineHeight := b.lineHeight()
	y += rulesParaGap + lineHeight
	for _, line := range b.lines {
		text.Draw(screen, line.Text, r.FontSourceCodePro10, x+rulesTreeIndent, y, decisionLineInk(line, false))
		y += lineHeight
	}
}
