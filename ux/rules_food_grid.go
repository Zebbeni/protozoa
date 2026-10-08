package ux

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// rulesFoodGrid is a scatter of food settling into the ground: each pile
// loses a little to the buried store under it every burial, until the
// surface is bare and everything that was there is underfoot.
type rulesFoodGrid struct {
	steps   []foodFrame
	caption string
}

// foodFrame is the two food layers at one point in the settling.
type foodFrame struct {
	surface [][]int
	buried  [][]int
}

const (
	// The same patch as the pH pictures, so the three read as one world.
	rulesFoodCols = rulesPhCols
	rulesFoodRows = rulesPhRows
	rulesFoodCell = rulesPhCell
	// rulesFoodFrames is how many burials the loop plays, which is enough to
	// take the biggest pile in the picture under.
	rulesFoodFrames   = 60
	rulesFoodHold     = 20
	rulesFoodStepTime = 110 * time.Millisecond
	// rulesFoodBuried is how much a pile loses to the ground each burial,
	// which is config.BurialAmount() in a running world. One, so the small
	// piles go under early in the loop and the biggest is still on the
	// surface at the end of it.
	rulesFoodBuried = 1
)

// rulesFoodStart is the scatter the loop opens on: piles of several sizes so
// the size tiers are all on show, and a few cells that already hold
// something buried.
var rulesFoodStart = []struct{ x, y, surface, buried int }{
	{1, 1, 90, 0}, {3, 3, 28, 0}, {5, 0, 12, 20}, {7, 2, 55, 0},
	{9, 4, 20, 0}, {10, 1, 75, 0}, {12, 3, 14, 30}, {14, 0, 45, 0},
	{16, 2, 85, 0}, {18, 4, 18, 0}, {20, 1, 62, 25}, {22, 3, 36, 0},
	{2, 4, 24, 0}, {6, 4, 50, 0}, {13, 1, 40, 0}, {19, 0, 70, 0},
}

func newRulesFoodGrid() rulesFoodGrid {
	cur := foodFrame{surface: newFoodLayer(), buried: newFoodLayer()}
	for _, pile := range rulesFoodStart {
		cur.surface[pile.x][pile.y] = pile.surface
		cur.buried[pile.x][pile.y] = pile.buried
	}
	steps := make([]foodFrame, 0, rulesFoodFrames+1)
	steps = append(steps, cur)
	for i := 0; i < rulesFoodFrames; i++ {
		cur = buryStep(cur)
		steps = append(steps, cur)
	}
	return rulesFoodGrid{
		steps:   steps,
		caption: "food settling out of reach: what is left on the surface can be eaten, what is under it has to be dug up",
	}
}

func newFoodLayer() [][]int {
	layer := make([][]int, rulesFoodCols)
	for x := range layer {
		layer[x] = make([]int, rulesFoodRows)
	}
	return layer
}

// buryStep is one burial: every pile gives the ground under it what it can,
// and an emptied pile is gone. Conservative, like FoodManager.BuryFood — the
// cell's total never changes.
func buryStep(prev foodFrame) foodFrame {
	next := foodFrame{surface: newFoodLayer(), buried: newFoodLayer()}
	for x := 0; x < rulesFoodCols; x++ {
		for y := 0; y < rulesFoodRows; y++ {
			surface, buried := prev.surface[x][y], prev.buried[x][y]
			moved := min(surface, rulesFoodBuried)
			next.surface[x][y] = surface - moved
			next.buried[x][y] = buried + moved
		}
	}
	return next
}

func (b rulesFoodGrid) stepAt(elapsed time.Duration) int {
	if len(b.steps) == 0 {
		return 0
	}
	n := int(elapsed/rulesFoodStepTime) % (len(b.steps) + rulesFoodHold)
	return min(n, len(b.steps)-1)
}

func (b rulesFoodGrid) height() int {
	return rulesFoodRows*rulesFoodCell + rulesCaptionH + rulesParaGap*2
}

func (b rulesFoodGrid) draw(screen *ebiten.Image, x, y int, t rulesTick) {
	restore := withHighResSprites()
	defer restore()

	frame := b.steps[b.stepAt(t.elapsed)]
	left := x + (rulesPanelW-rulesFoodCols*rulesFoodCell)/2
	top := y + rulesParaGap

	// Empty cells first, so the patch reads as ground rather than as sprites
	// floating on the page, then the two food layers in the order the grid
	// draws them: buried underneath at a quarter opacity, surface on top.
	for cx := 0; cx < rulesFoodCols; cx++ {
		for cy := 0; cy < rulesFoodRows; cy++ {
			ebitenutil.DrawRect(screen, float64(left+cx*rulesFoodCell), float64(top+cy*rulesFoodCell),
				rulesFoodCell, rulesFoodCell, phCellColor((config.MaxPh()+config.MinPh())/2))
		}
	}
	for cx := 0; cx < rulesFoodCols; cx++ {
		for cy := 0; cy < rulesFoodRows; cy++ {
			px, py := float64(left+cx*rulesFoodCell), float64(top+cy*rulesFoodCell)
			if v := frame.buried[cx][cy]; v > 0 {
				drawRulesFoodSprite(screen, px, py, foodRoleAgainst(v, config.MaxBuriedFoodValue()), buriedFoodAlpha)
			}
			if v := frame.surface[cx][cy]; v > 0 {
				drawRulesFoodSprite(screen, px, py, foodRoleForValue(v), 1)
			}
		}
	}

	cb := boundString(r.FontSourceCodePro8, b.caption)
	text.Draw(screen, b.caption, r.FontSourceCodePro8,
		x+(rulesPanelW-cb.Dx())/2, top+rulesFoodRows*rulesFoodCell+rulesCaptionH,
		themedForegroundDim())
}

// drawRulesFoodSprite is the grid's drawStaticSpriteAlpha at the
// illustration's own scale, which has no camera to read one from.
func drawRulesFoodSprite(img *ebiten.Image, x, y float64, role r.ImageRole, alpha float64) {
	sprite := r.Sprite(role, animation.AnimIdle, 0)
	if sprite == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(rulesPhScale, rulesPhScale)
	op.GeoM.Translate(x, y)
	a := float32(alpha)
	op.ColorScale.Scale(float32(foodColor.R)*a, float32(foodColor.G)*a, float32(foodColor.B)*a, a)
	img.DrawImage(sprite, op)
}
