package ux

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

// rulesWalker is one organism on a strip of world ten cells wide, doing what
// an organism on the grid does: one action a cycle, holding its cell for the
// whole of that cycle.
//
// It runs a fixed script — nine moves right, two left turns, nine moves back,
// two more left turns — which returns it to where and how it started, so it
// loops without a jump.
//
// The four frames of a move play against one fixed anchor, and the anchor is
// the cell the organism is leaving: the art crouches, stretches across the
// boundary and lands in the cell ahead, so by the last frame the body is
// already where the next cycle's anchor puts it.
type rulesWalker struct {
	look  physiology.Appearance
	steps []walkerStep
}

const (
	rulesWalkerScale = 4
	rulesWalkerCell  = rulesSpriteCell * rulesWalkerScale
	rulesWalkerCols  = 10
	rulesWalkerRun   = rulesWalkerCols - 1
)

// walkerStep is one cycle: where the organism is, which way it faces and
// what it is doing.
type walkerStep struct {
	cell int
	dir  utils.Point
	anim animation.Animation
}

var (
	walkerRight = utils.Point{X: 1}
	walkerUp    = utils.Point{Y: -1}
	walkerLeft  = utils.Point{X: -1}
	walkerDown  = utils.Point{Y: 1}
)

// walkerScript is the loop, a cycle an entry. The two turns at each end are
// what point it back the other way: a left turn from facing right leaves it
// facing up, and a second leaves it facing left.
func walkerScript() []walkerStep {
	steps := make([]walkerStep, 0, 2*rulesWalkerRun+4)
	for i := 0; i < rulesWalkerRun; i++ {
		steps = append(steps, walkerStep{cell: i, dir: walkerRight, anim: animation.AnimMove})
	}
	steps = append(steps,
		walkerStep{cell: rulesWalkerRun, dir: walkerRight, anim: animation.AnimTurnLeft},
		walkerStep{cell: rulesWalkerRun, dir: walkerUp, anim: animation.AnimTurnLeft})
	for i := 0; i < rulesWalkerRun; i++ {
		steps = append(steps, walkerStep{cell: rulesWalkerRun - i, dir: walkerLeft, anim: animation.AnimMove})
	}
	steps = append(steps,
		walkerStep{cell: 0, dir: walkerLeft, anim: animation.AnimTurnLeft},
		walkerStep{cell: 0, dir: walkerDown, anim: animation.AnimTurnLeft})
	return steps
}

func newRulesWalker() rulesWalker {
	return rulesWalker{
		look:  physiology.Appearance{Motor: physiology.MotorFlagella},
		steps: walkerScript(),
	}
}

func (b rulesWalker) height() int { return rulesWalkerCell + rulesParaGap*2 }

// walkerFrames is how many sprite frames one cycle holds, which is also how
// long the organism stays in its cell.
func walkerFrames() int {
	if frames := zoomSpriteFrameCounts[r.ZoomHighRes]; frames > 0 {
		return frames
	}
	return 1
}

// stepAt is the cycle of the script being played at a given elapsed time.
func (b rulesWalker) stepAt(elapsed time.Duration) walkerStep {
	cycle := int(elapsed/rulesFrameHold) / walkerFrames()
	return b.steps[cycle%len(b.steps)]
}

// walkerLeft is the x of the strip's first cell, measured from the block's
// left edge, so the ten cells sit centred on the panel.
func walkerStripLeft() int {
	return (rulesPanelW - rulesWalkerCols*rulesWalkerCell) / 2
}

func (b rulesWalker) draw(screen *ebiten.Image, x, y int, t rulesTick) {
	restore := withHighResSprites()
	defer restore()

	step := b.stepAt(t.elapsed)
	cellX := x + walkerStripLeft() + step.cell*rulesWalkerCell
	drawRulesOrganismFacing(screen, float64(cellX), float64(y+rulesParaGap),
		b.look, step.anim, t.frame, rulesWalkerScale, step.dir)
}
