package ux

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/manager"
	r "github.com/Zebbeni/protozoa/resources"
)

// rulesPhGrid is a patch of world with pH readings spread across it, playing
// the diffusion forward so a reader can watch them move.
type rulesPhGrid struct {
	// steps[i][x][y] is the pH at cell x,y after i diffusion steps.
	steps [][][]float64
	// walls[x][y] is the wall strength at a cell, 0 where there is no wall.
	walls   [][]int
	caption string
}

const (
	rulesPhCols = 23
	rulesPhRows = 5
	// One cell is a whole number of sprite pixels, so the wall art is scaled
	// by 2 rather than landing some source pixels on two screen pixels.
	rulesPhSprite = 16
	rulesPhScale  = 2
	rulesPhCell   = rulesPhSprite * rulesPhScale
	// rulesPhFrames is how many pictures a loop plays before it starts over.
	// How many diffusion steps those cover is the stride, which differs per
	// picture: an open patch settles in 180 steps, where pH threading through a
	// thin wall cell takes 4,500 to get as far.
	rulesPhFrames     = 180
	rulesPhOpenStride = 1
	rulesWallPhStride = 25
	// rulesPhHold holds the settled picture at the end of a loop, so the
	// restart does not read as part of the diffusion.
	rulesPhHold     = 25
	rulesPhStepTime = 40 * time.Millisecond
	// rulesPhDiffuse is the rate the illustration runs at. The simulation's
	// own ph_diffuse_factor ships at 1, which takes a cell all the way to its
	// neighbours' mean in one step and leaves a checkerboard pattern flipping
	// between two states forever instead of settling. At 0.5 and below it
	// damps.
	rulesPhDiffuse = 0.5
)

// buildRulesPhGrid plays the diffusion forward once and keeps every step, so
// a draw is a lookup rather than a simulation and the block holds no clock.
func buildRulesPhGrid(start [][]float64, walls [][]int, stride int, caption string) rulesPhGrid {
	perm := make([][]float64, rulesPhCols)
	g := config.GetCurrentGlobals()
	for x := range perm {
		perm[x] = make([]float64, rulesPhRows)
		for y := range perm[x] {
			strength := 0
			if walls != nil {
				strength = walls[x][y]
			}
			perm[x][y] = manager.WallPermeability(g, strength)
		}
	}

	steps := make([][][]float64, 0, rulesPhFrames+1)
	cur := start
	steps = append(steps, cur)
	for i := 0; i < rulesPhFrames*stride; i++ {
		cur = rulesPhDiffuseStep(cur, perm)
		if (i+1)%stride == 0 {
			steps = append(steps, cur)
		}
	}
	return rulesPhGrid{steps: steps, walls: walls, caption: caption}
}

func newRulesPhGrid() rulesPhGrid {
	return buildRulesPhGrid(rulesPhStart(), nil, rulesPhOpenStride,
		"acid and alkaline pools evening out, with nothing alive to keep them apart")
}

func newRulesWallPhGrid() rulesPhGrid {
	start, walls := rulesWallPhStart()
	return buildRulesPhGrid(start, walls, rulesWallPhStride,
		"the same thing with walls in the way: pH crosses quickly where they are thin, and only slowly through the wall itself")
}

// rulesPhBlank is the patch at the pH every cell of a new world starts at.
func rulesPhBlank() [][]float64 {
	mid := (config.MaxPh() + config.MinPh()) / 2
	cells := make([][]float64, rulesPhCols)
	for x := range cells {
		cells[x] = make([]float64, rulesPhRows)
		for y := range cells[x] {
			cells[x][y] = mid
		}
	}
	return cells
}

// rulesPhLow / rulesPhHigh are readings near each end of the scale, short of
// the ends themselves so neither pool reads as a limit of the world.
func rulesPhLow() float64 {
	mid := (config.MaxPh() + config.MinPh()) / 2
	return config.MinPh() + (mid-config.MinPh())*0.15
}

func rulesPhHigh() float64 {
	mid := (config.MaxPh() + config.MinPh()) / 2
	return config.MaxPh() - (config.MaxPh()-mid)*0.15
}

// rulesPhStart is the opening pattern of the wall-free illustration: two
// acid pools and one alkaline one.
func rulesPhStart() [][]float64 {
	cells := rulesPhBlank()
	blob := func(cx, cy, radius int, value float64) {
		for x := cx - radius; x <= cx+radius; x++ {
			for y := cy - radius; y <= cy+radius; y++ {
				dx, dy := x-cx, y-cy
				if dx*dx+dy*dy > radius*radius {
					continue
				}
				cells[(x+rulesPhCols)%rulesPhCols][(y+rulesPhRows)%rulesPhRows] = value
			}
		}
	}
	blob(4, 2, 2, rulesPhLow())
	blob(17, 1, 2, rulesPhHigh())
	blob(11, 4, 1, rulesPhLow())
	return cells
}

// rulesWallPhStrength is what the drawn wall lines are built at.
//
// Not the full 100: a cell at full strength is sealed out of the diffusion
// entirely, so it would hold its opening colour for the whole loop while the
// cells around it moved. At 75 it takes on the pH either side of it and still
// passes only about 1% of what an open cell does.
const rulesWallPhStrength = 75

// rulesPhHiddenCols is how many columns at the left of the patch are
// simulated and not drawn.
//
// The patch wraps, so separating three pools takes three barriers, and the
// third is the one between the right-hand pool and the left. Drawn, it is a
// wall at the edge of the picture with nothing behind it, which reads as
// decoration. It is kept out of the simulated world's way instead: one
// sealed column, off the left of what is shown.
const rulesPhHiddenCols = 1

// rulesWallPhLine is one barrier: a column of wall with a per-row sideways
// offset, so it reads as something dug rather than something ruled.
//
// Successive offsets differ by at most one, including from the last row to
// the first, since the patch wraps. That keeps the wall cells diagonally
// joined, which is what stops pH taking a step around them: diffusion moves
// between cells that share an edge, so a diagonal join is a seal.
type rulesWallPhLine struct {
	x       int
	offsets [rulesPhRows]int
	// thinY is the row with the weak spot, or -1 for a line with none.
	thinY        int
	thinStrength int
	hidden       bool
}

func (l rulesWallPhLine) columnAt(y int) int {
	return (l.x + l.offsets[y] + rulesPhCols) % rulesPhCols
}

func (l rulesWallPhLine) strengthAt(y int) int {
	if y == l.thinY {
		return l.thinStrength
	}
	if l.hidden {
		return manager.MaxWallStrength
	}
	return rulesWallPhStrength
}

var rulesWallPhWalls = []rulesWallPhLine{
	{x: 0, thinY: -1, hidden: true},
	{x: 8, offsets: [rulesPhRows]int{0, 0, 1, 1, 0}, thinY: 1, thinStrength: 0},
	{x: 16, offsets: [rulesPhRows]int{0, -1, -1, 0, 0}, thinY: 3, thinStrength: 1},
}

// rulesWallPhStart is an acid pool, a neutral one and an alkaline one, each
// walled off from the next but for one thin cell.
func rulesWallPhStart() ([][]float64, [][]int) {
	cells := rulesPhBlank()
	for x := 1; x < 8; x++ {
		for y := 0; y < rulesPhRows; y++ {
			cells[x][y] = rulesPhLow()
		}
	}
	for x := 17; x < rulesPhCols; x++ {
		for y := 0; y < rulesPhRows; y++ {
			cells[x][y] = rulesPhHigh()
		}
	}

	walls := make([][]int, rulesPhCols)
	for x := range walls {
		walls[x] = make([]int, rulesPhRows)
	}
	for _, line := range rulesWallPhWalls {
		for y := 0; y < rulesPhRows; y++ {
			x := line.columnAt(y)
			walls[x][y] = line.strengthAt(y)
			// Every wall opens at the middle reading whichever pool it was
			// built into, so the colour it takes on over the loop is the
			// pH reaching it rather than where it started.
			cells[x][y] = (config.MaxPh() + config.MinPh()) / 2
		}
	}
	return cells, walls
}

// rulesPhDiffuseStep is one cycle of manager.diffusePhLevels: every cell moves
// toward the mean of its four neighbours, each weighted by how freely pH gets
// through it, and the patch wraps at its edges like the world does.
func rulesPhDiffuseStep(prev [][]float64, perm [][]float64) [][]float64 {
	next := make([][]float64, rulesPhCols)
	for x := 0; x < rulesPhCols; x++ {
		next[x] = make([]float64, rulesPhRows)
		for y := 0; y < rulesPhRows; y++ {
			self := perm[x][y]
			if self <= 0 {
				next[x][y] = prev[x][y]
				continue
			}
			total, weight := 0.0, 0.0
			for _, n := range [4][2]int{{x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}} {
				nx := (n[0] + rulesPhCols) % rulesPhCols
				ny := (n[1] + rulesPhRows) % rulesPhRows
				w := perm[nx][ny]
				if w <= 0 {
					continue
				}
				total += prev[nx][ny] * w
				weight += w
			}
			if weight == 0 {
				next[x][y] = prev[x][y]
				continue
			}
			next[x][y] = prev[x][y] + (total/weight-prev[x][y])*rulesPhDiffuse*self
		}
	}
	return next
}

// stepAt is the step to show at a given elapsed time, holding the settled
// picture before looping back to the start.
func (b rulesPhGrid) stepAt(elapsed time.Duration) int {
	if len(b.steps) == 0 {
		return 0
	}
	n := int(elapsed/rulesPhStepTime) % (len(b.steps) + rulesPhHold)
	return min(n, len(b.steps)-1)
}

func (b rulesPhGrid) height() int {
	return rulesPhRows*rulesPhCell + rulesCaptionH + rulesParaGap*2
}

// firstDrawnCol is the leftmost column the picture shows. The wall picture
// keeps its wrap-around seal off the page; the open one draws everything.
func (b rulesPhGrid) firstDrawnCol() int {
	if b.walls == nil {
		return 0
	}
	return rulesPhHiddenCols
}

func (b rulesPhGrid) drawnCols() int {
	return rulesPhCols - b.firstDrawnCol()
}

// drawRulesWallAt paints one wall cell the way the grid's wall layer does:
// the strength tier's base sprite, tinted by the pH it sits in, plus a
// connector for each neighbouring cell that also holds a wall.
func (b rulesPhGrid) drawRulesWallAt(screen *ebiten.Image, px, py float64, cx, cy int, ph float64) {
	role := wallRoleForStrength(b.walls[cx][cy])
	tint := wallTintForPh(ph)
	drawRulesStaticSprite(screen, px, py,
		r.SpriteLayer(role, r.LayerWallBase, animation.AnimIdle, 0), tint)

	for _, c := range wallConnectors {
		nx := (cx + c.offset.X + rulesPhCols) % rulesPhCols
		ny := (cy + c.offset.Y + rulesPhRows) % rulesPhRows
		if b.walls[nx][ny] <= 0 || nx < b.firstDrawnCol() {
			continue
		}
		drawRulesStaticSprite(screen, px, py,
			r.SpriteLayer(role, c.layer, animation.AnimIdle, 0), tint)
	}
}

// drawRulesStaticSprite is the grid's drawStaticSprite at the illustration's
// own scale, which has no camera to read one from.
func drawRulesStaticSprite(img *ebiten.Image, x, y float64, sprite *ebiten.Image, col colorful.Color) {
	if sprite == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(rulesPhScale, rulesPhScale)
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(float32(col.R), float32(col.G), float32(col.B), 1)
	img.DrawImage(sprite, op)
}

func (b rulesPhGrid) draw(screen *ebiten.Image, x, y int, t rulesTick) {
	restore := withHighResSprites()
	defer restore()

	cells := b.steps[b.stepAt(t.elapsed)]
	first := b.firstDrawnCol()
	left := x + (rulesPanelW-b.drawnCols()*rulesPhCell)/2
	top := y + rulesParaGap
	colX := func(cx int) float64 { return float64(left + (cx-first)*rulesPhCell) }
	// Both passes in the layer order the grid draws them: the pH wash under
	// everything, walls on top of the cell they stand in.
	for cx := first; cx < rulesPhCols; cx++ {
		for cy := 0; cy < rulesPhRows; cy++ {
			ebitenutil.DrawRect(screen, colX(cx), float64(top+cy*rulesPhCell),
				rulesPhCell, rulesPhCell, phCellColor(cells[cx][cy]))
		}
	}
	if b.walls != nil {
		for cx := first; cx < rulesPhCols; cx++ {
			for cy := 0; cy < rulesPhRows; cy++ {
				if b.walls[cx][cy] <= 0 {
					continue
				}
				b.drawRulesWallAt(screen, colX(cx), float64(top+cy*rulesPhCell),
					cx, cy, cells[cx][cy])
			}
		}
	}
	cb := boundString(r.FontSourceCodePro8, b.caption)
	text.Draw(screen, b.caption, r.FontSourceCodePro8,
		x+(rulesPanelW-cb.Dx())/2, top+rulesPhRows*rulesPhCell+rulesCaptionH,
		themedForegroundDim())
}
