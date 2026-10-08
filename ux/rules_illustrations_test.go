package ux

import (
	"math"
	"testing"
	"time"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/manager"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

func phSpread(cells [][]float64) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, col := range cells {
		for _, v := range col {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	return lo, hi
}

func phMean(cells [][]float64) float64 {
	total, n := 0.0, 0.0
	for _, col := range cells {
		for _, v := range col {
			total += v
			n++
		}
	}
	return total / n
}

// TestThePhIllustrationStartsUneven: the point of the picture is readings that
// differ, so a starting pattern that is already flat illustrates nothing.
func TestThePhIllustrationStartsUneven(t *testing.T) {
	loadKeyGlobals(t)
	start := rulesPhStart()
	lo, hi := phSpread(start)
	mid := (config.MaxPh() + config.MinPh()) / 2
	if lo >= mid || hi <= mid {
		t.Errorf("the opening pattern runs %.2f to %.2f, which is not either side of %.2f", lo, hi, mid)
	}
	if lo < config.MinPh() || hi > config.MaxPh() {
		t.Errorf("the opening pattern runs %.2f to %.2f, outside the %.0f-%.0f scale",
			lo, hi, config.MinPh(), config.MaxPh())
	}
}

// TestThePhIllustrationEvensOut is the claim the prose beside it makes: the
// readings converge, and they converge toward the average rather than
// draining away, since diffusion moves pH about without creating any.
func TestThePhIllustrationEvensOut(t *testing.T) {
	loadKeyGlobals(t)
	b := newRulesPhGrid()
	if len(b.steps) < 2 {
		t.Fatalf("the illustration holds %d steps", len(b.steps))
	}
	startMean := phMean(b.steps[0])
	lo, hi := phSpread(b.steps[0])
	opening := hi - lo
	prevSpread := opening
	for i, cells := range b.steps[1:] {
		if m := phMean(cells); math.Abs(m-startMean) > 1e-9 {
			t.Fatalf("step %d has mean %.12f against the opening %.12f", i+1, m, startMean)
		}
		lo, hi = phSpread(cells)
		if spread := hi - lo; spread > prevSpread {
			t.Fatalf("step %d spread %.4f against %.4f the step before", i+1, spread, prevSpread)
		} else {
			prevSpread = spread
		}
	}
	if prevSpread > opening*0.1 {
		t.Errorf("the last step still spans %.3f pH against the opening %.3f", prevSpread, opening)
	}
}

// TestThePhIllustrationHoldsThenLoops: without the hold the settled picture is
// on screen for one step and the restart reads as part of the diffusion.
func TestThePhIllustrationHoldsThenLoops(t *testing.T) {
	loadKeyGlobals(t)
	b := newRulesPhGrid()
	last := len(b.steps) - 1
	if got := b.stepAt(0); got != 0 {
		t.Errorf("the loop opens on step %d", got)
	}
	if got := b.stepAt(time.Duration(last) * rulesPhStepTime); got != last {
		t.Errorf("step %d is shown at the end of the play, want %d", got, last)
	}
	held := time.Duration(last+rulesPhHold-1) * rulesPhStepTime
	if got := b.stepAt(held); got != last {
		t.Errorf("the settled picture is not held: step %d at the end of the hold", got)
	}
	if got := b.stepAt(time.Duration(last+rulesPhHold+1) * rulesPhStepTime); got != 0 {
		t.Errorf("the loop restarts on step %d", got)
	}
}

// TestTheExampleTreeShowsAllThreeTones: the paragraph beside it explains three
// kinds of line, so the tree has to actually contain one of each.
func TestTheExampleTreeShowsAllThreeTones(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	lines := newRulesTreeBlock().lines
	var used, travelled, never int
	for _, line := range lines {
		switch {
		case line.UsedLastCycle:
			used++
		case line.WasTravelled:
			travelled++
		default:
			never++
		}
	}
	if used == 0 || travelled == 0 || never == 0 {
		t.Errorf("the example tree has %d used, %d travelled and %d untouched lines", used, travelled, never)
	}
	inks := map[string]bool{}
	for _, line := range lines {
		r1, g1, b1, _ := decisionLineInk(line, false).RGBA()
		inks[string(rune(r1))+string(rune(g1))+string(rune(b1))] = true
	}
	if len(inks) != 3 {
		t.Errorf("the example tree draws %d distinct inks, want 3", len(inks))
	}
}

// TestTheExampleTreeIsOneMutationCouldBuild: a tree bigger than the setting
// allows would be illustrating something the simulation forbids.
func TestTheExampleTreeIsOneMutationCouldBuild(t *testing.T) {
	loadKeyGlobals(t)
	tree := exampleTree()
	if tree == nil {
		t.Fatal("the example tree is nil")
	}
	if got, limit := tree.Size(), config.MaxDecisionTreeSize(); got > limit {
		t.Errorf("the example tree has %d nodes against a limit of %d", got, limit)
	}
	if round := d.DeserializeTree(tree.Serialize()); round == nil || round.Serialize() != tree.Serialize() {
		t.Error("the example tree does not survive a serialize round trip")
	}
}

// TestPhCrossesFasterWhereTheWallsAreThin is the claim the wall picture
// makes, measured by sealing the thin cells and running the same world again.
//
// The drawn lines are strength 75, which is a brake rather than a seal: over
// the 4,500 steps of a loop they pass a lot on their own, 209 pH of total
// movement against 369 with the thin cells open. So the picture shows pH
// crossing FASTER where a wall is thin, not crossing only there.
//
// Measuring it as "the cell past the gap moved and its neighbours did not"
// does not work. The pool behind the barrier mixes vertically within a few
// dozen steps, so by the first frame on screen the pH that came through the
// gap has spread along the whole column: the gap row leads the others by
// 0.46 against 0.36, which is not a claim worth asserting.
func TestPhCrossesFasterWhereTheWallsAreThin(t *testing.T) {
	loadKeyGlobals(t)
	open := newRulesWallPhGrid()

	start, walls := rulesWallPhStart()
	for _, line := range rulesWallPhWalls {
		if line.thinY >= 0 {
			walls[line.columnAt(line.thinY)][line.thinY] = manager.MaxWallStrength
		}
	}
	sealed := buildRulesPhGrid(start, walls, rulesWallPhStride, "sealed")

	// How far the pools have moved from where they started, at the end.
	moved := func(b rulesPhGrid) float64 {
		total := 0.0
		for x := 0; x < rulesPhCols; x++ {
			for y := 0; y < rulesPhRows; y++ {
				if b.walls[x][y] > 0 {
					continue
				}
				total += math.Abs(b.steps[len(b.steps)-1][x][y] - b.steps[0][x][y])
			}
		}
		return total
	}
	withGaps, without := moved(open), moved(sealed)
	if withGaps < without*1.5 {
		t.Errorf("the pools moved %.1f pH with the thin cells open and %.1f with them sealed", withGaps, without)
	}
}

// TestTheWalledPoolsEvenOutInTheEnd: the loop is only worth watching to the
// end if the pools have visibly come together by then, which takes far more
// diffusion steps through a thin cell than an open patch needs.
func TestTheWalledPoolsEvenOutInTheEnd(t *testing.T) {
	loadKeyGlobals(t)
	b := newRulesWallPhGrid()
	lo, hi := phSpread(b.steps[0])
	opening := hi - lo
	lo, hi = phSpread(b.steps[len(b.steps)-1])
	if closing := hi - lo; closing > opening*0.25 {
		t.Errorf("the last frame still spans %.3f pH against the opening %.3f", closing, opening)
	}
}

// TestTheWallsTakeOnThePhAroundThem: a wall is drawn tinted by the pH at its
// own cell, so a sealed line would keep its opening colour all the way
// through while the cells beside it changed.
func TestTheWallsTakeOnThePhAroundThem(t *testing.T) {
	loadKeyGlobals(t)
	b := newRulesWallPhGrid()
	for _, line := range rulesWallPhWalls {
		if line.hidden {
			// Sealed on purpose, and not drawn, so it has no colour to take on.
			continue
		}
		moved := 0.0
		for y := 0; y < rulesPhRows; y++ {
			if y == line.thinY {
				continue
			}
			x := line.columnAt(y)
			for _, cells := range b.steps {
				moved = math.Max(moved, math.Abs(cells[x][y]-b.steps[0][x][y]))
			}
		}
		if moved < 0.2 {
			t.Errorf("the x=%d line never moves more than %.3f pH from where it started", line.x, moved)
		}
	}
}

// TestTheWallPictureSpreadNeverGrows: diffusion averages, so no cell can end
// outside the range it started in however the walls are arranged.
func TestTheWallPictureSpreadNeverGrows(t *testing.T) {
	loadKeyGlobals(t)
	b := newRulesWallPhGrid()
	lo, hi := phSpread(b.steps[0])
	prev := hi - lo
	for i, cells := range b.steps[1:] {
		lo, hi = phSpread(cells)
		if spread := hi - lo; spread > prev+1e-12 {
			t.Fatalf("step %d spread %.4f against %.4f the step before", i+1, spread, prev)
		} else {
			prev = spread
		}
	}
}

// TestTheTwoPhPicturesAreDifferentWorlds: the second is only worth the space
// if the walls change what happens, and it draws them only if they are there.
func TestTheTwoPhPicturesAreDifferentWorlds(t *testing.T) {
	loadKeyGlobals(t)
	if newRulesPhGrid().walls != nil {
		t.Error("the wall-free picture carries walls")
	}
	walled := newRulesWallPhGrid()
	if walled.walls == nil {
		t.Fatal("the wall picture carries no walls")
	}
	var cells int
	for x := 0; x < rulesPhCols; x++ {
		for y := 0; y < rulesPhRows; y++ {
			if walled.walls[x][y] > 0 {
				cells++
			}
		}
	}
	// Every line is one cell a row, and the open gap is the one cell with no
	// wall in it at all.
	want := len(rulesWallPhWalls)*rulesPhRows - 1
	if cells != want {
		t.Errorf("the picture holds %d wall cells, want %d", cells, want)
	}
}

// TestTheWallLinesStayJoined: a line is offset row by row so it does not read
// as ruled, and the offsets may only step by one, including from the last row
// back to the first. Two cells that share only a corner still block, because
// pH moves between cells that share an edge; a bigger jump leaves a hole.
func TestTheWallLinesStayJoined(t *testing.T) {
	loadKeyGlobals(t)
	for _, line := range rulesWallPhWalls {
		for y := 0; y < rulesPhRows; y++ {
			next := line.offsets[(y+1)%rulesPhRows]
			if step := next - line.offsets[y]; step < -1 || step > 1 {
				t.Errorf("the x=%d line steps %d between rows %d and %d", line.x, step, y, (y+1)%rulesPhRows)
			}
		}
	}
}

// TestTheDrawnWallLinesAreNotRuled: the point of the offsets is that the two
// lines on show look dug rather than drawn.
func TestTheDrawnWallLinesAreNotRuled(t *testing.T) {
	loadKeyGlobals(t)
	for _, line := range rulesWallPhWalls {
		if line.hidden {
			continue
		}
		columns := map[int]bool{}
		for y := 0; y < rulesPhRows; y++ {
			columns[line.columnAt(y)] = true
		}
		if len(columns) < 2 {
			t.Errorf("the x=%d line sits in one column", line.x)
		}
	}
}

// TestTheHiddenSealIsSealedAndOffThePage: it is there to stop pH going round
// the wrap, and drawing it would put a wall at the edge of the picture with
// nothing behind it.
func TestTheHiddenSealIsSealedAndOffThePage(t *testing.T) {
	loadKeyGlobals(t)
	b := newRulesWallPhGrid()
	var hidden int
	for _, line := range rulesWallPhWalls {
		if !line.hidden {
			continue
		}
		hidden++
		for y := 0; y < rulesPhRows; y++ {
			x := line.columnAt(y)
			if x >= b.firstDrawnCol() {
				t.Errorf("the hidden line reaches column %d, which is drawn", x)
			}
			if got := b.walls[x][y]; got != manager.MaxWallStrength {
				t.Errorf("the hidden line is strength %d at row %d, want %d", got, y, manager.MaxWallStrength)
			}
		}
	}
	if hidden != 1 {
		t.Errorf("%d lines are hidden, want 1", hidden)
	}
	if got, want := b.drawnCols(), rulesPhCols-rulesPhHiddenCols; got != want {
		t.Errorf("the picture draws %d columns, want %d", got, want)
	}
	if w := b.drawnCols() * rulesPhCell; w > rulesPanelW {
		t.Errorf("the drawn patch is %dpx against a %dpx panel", w, rulesPanelW)
	}
}

// TestTheWallPictureUsesTheGridsSpriteTiers: the thin cells are only legible
// as thin if they land in a different sprite tier from the lines they sit in.
func TestTheWallPictureUsesTheGridsSpriteTiers(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	full := wallRoleForStrength(rulesWallPhStrength)
	for _, line := range rulesWallPhWalls {
		if line.hidden || line.thinY < 0 || line.thinStrength == 0 {
			continue
		}
		if thin := wallRoleForStrength(line.thinStrength); thin == full {
			t.Errorf("the thin cell of the x=%d line (strength %d) draws the same tier as the wall around it",
				line.x, line.thinStrength)
		}
	}
	if r.SpriteLayer(full, r.LayerWallBase, animation.AnimIdle, 0) == nil {
		t.Error("the wall base sprite is missing at the illustration's sprite set")
	}
}

// TestTheHealthIllustrationShowsBothDirections: the picture is there to show
// what a cycle costs and pays, so the sample has to contain some of each and
// a net that is their sum.
func TestTheHealthIllustrationShowsBothDirections(t *testing.T) {
	loadKeyGlobals(t)
	l := rulesSampleLedger()
	if !l.Recorded {
		t.Error("the sample ledger reads as not yet measured, which draws (no cycle yet)")
	}
	var gains, losses int
	sum := 0.0
	for _, amount := range l.Amounts {
		sum += amount
		switch {
		case amount > 0:
			gains++
		case amount < 0:
			losses++
		}
	}
	if gains == 0 || losses == 0 {
		t.Errorf("the sample has %d gains and %d losses", gains, losses)
	}
	if math.Abs(l.Total()-sum) > 1e-9 {
		t.Errorf("the net reads %.4f against a sum of %.4f", l.Total(), sum)
	}
	// Every row is drawn, so a source with no label would print "( )".
	for _, row := range healthLedgerRows(l) {
		if row.label == "" {
			t.Errorf("a sample row of %.2f has no label", row.amount)
		}
	}
}

// TestTheHealthIllustrationFitsItsBlock: the portrait column and the note
// beside it are hand-placed, so either can run past the panel or past the
// height the scroller advances by.
func TestTheHealthIllustrationFitsItsBlock(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	b := newRulesPortrait()
	if drawn := rulesParaGap + portraitColumnHeight(); drawn > b.height() {
		t.Errorf("the block paints %dpx and reports %dpx", drawn, b.height())
	}
	if rulesPortraitLeft+portraitSize > rulesPortraitNoteX {
		t.Errorf("the note starts at %d, inside the portrait ending at %d",
			rulesPortraitNoteX, rulesPortraitLeft+portraitSize)
	}
	for _, line := range b.notes {
		if w := boundString(r.FontSourceCodePro10, line).Dx(); rulesPortraitNoteX+w > rulesPanelW {
			t.Errorf("a note line needs %dpx against a %dpx panel: %q", rulesPortraitNoteX+w, rulesPanelW, line)
		}
	}
	if got, want := len(b.notes)*rulesLineHeightBody+rulesPortraitNoteY, portraitColumnHeight(); got > want {
		t.Errorf("the note is %dpx tall against a %dpx column", got, want)
	}
}

// TestTheWalkerScriptReturnsWhereItStarted: the strip loops, so the last
// cycle has to leave the organism in the cell and facing the direction the
// first cycle expects, or the loop jumps.
func TestTheWalkerScriptReturnsWhereItStarted(t *testing.T) {
	loadKeyGlobals(t)
	steps := walkerScript()
	if got, want := len(steps), 2*rulesWalkerRun+4; got != want {
		t.Fatalf("the script is %d cycles, want %d", got, want)
	}
	if steps[0].cell != 0 || steps[0].dir != walkerRight {
		t.Errorf("it starts at cell %d facing %v", steps[0].cell, steps[0].dir)
	}

	// Nine moves out, two turns, nine moves back, two turns.
	for i := 0; i < rulesWalkerRun; i++ {
		if steps[i].anim != animation.AnimMove || steps[i].cell != i || steps[i].dir != walkerRight {
			t.Errorf("cycle %d is %+v, want a move right at cell %d", i, steps[i], i)
		}
	}
	for _, i := range []int{rulesWalkerRun, rulesWalkerRun + 1} {
		if steps[i].anim != animation.AnimTurnLeft || steps[i].cell != rulesWalkerRun {
			t.Errorf("cycle %d is %+v, want a left turn at cell %d", i, steps[i], rulesWalkerRun)
		}
	}
	for i := 0; i < rulesWalkerRun; i++ {
		s := steps[rulesWalkerRun+2+i]
		if s.anim != animation.AnimMove || s.cell != rulesWalkerRun-i || s.dir != walkerLeft {
			t.Errorf("cycle %d is %+v, want a move left at cell %d", rulesWalkerRun+2+i, s, rulesWalkerRun-i)
		}
	}
	for _, i := range []int{2*rulesWalkerRun + 2, 2*rulesWalkerRun + 3} {
		if steps[i].anim != animation.AnimTurnLeft || steps[i].cell != 0 {
			t.Errorf("cycle %d is %+v, want a left turn at cell 0", i, steps[i])
		}
	}

	// Each turn leaves it facing where the next cycle says it is facing, and
	// the last leaves it ready for the first.
	turns := map[utils.Point]utils.Point{
		walkerRight: walkerUp, walkerUp: walkerLeft, walkerLeft: walkerDown, walkerDown: walkerRight,
	}
	for i, s := range steps {
		next := steps[(i+1)%len(steps)]
		if s.anim != animation.AnimTurnLeft {
			continue
		}
		if turns[s.dir] != next.dir {
			t.Errorf("a left turn facing %v is followed by %v, want %v", s.dir, next.dir, turns[s.dir])
		}
	}
}

// TestTheWalkerHoldsItsCellForAWholeCycle: one action a cycle, like the grid.
// Moving it between frames is what makes it look like it is skating.
func TestTheWalkerHoldsItsCellForAWholeCycle(t *testing.T) {
	loadKeyGlobals(t)
	b := newRulesWalker()
	cycle := rulesFrameHold * time.Duration(walkerFrames())
	for i := range b.steps {
		at := cycle * time.Duration(i)
		want := b.stepAt(at)
		for frame := 0; frame < walkerFrames(); frame++ {
			got := b.stepAt(at + rulesFrameHold*time.Duration(frame) + rulesFrameHold/2)
			if got != want {
				t.Fatalf("cycle %d frame %d shows %+v, want %+v", i, frame, got, want)
			}
		}
		if next := b.stepAt(at + cycle + rulesFrameHold/2); next == want && len(b.steps) > 1 {
			t.Fatalf("cycle %d does not advance: still %+v", i, next)
		}
	}
	// A full loop comes back to the opening cycle.
	if got, want := b.stepAt(cycle*time.Duration(len(b.steps))), b.stepAt(0); got != want {
		t.Errorf("the loop restarts on %+v, want %+v", got, want)
	}
}

// TestTheWalkerStripFitsThePanel: the organism is drawn facing left for half
// the loop, and a two-cell move animation reaches out of the base cell on
// the side it faces.
func TestTheWalkerStripFitsThePanel(t *testing.T) {
	loadKeyGlobals(t)
	if w := rulesWalkerCols * rulesWalkerCell; w > rulesPanelW {
		t.Errorf("the strip is %dpx against a %dpx panel", w, rulesPanelW)
	}
	if walkerStripLeft() < 0 {
		t.Errorf("the strip starts at %d", walkerStripLeft())
	}
}

// foodTotals is the surface and buried units across the whole patch.
func foodTotals(f foodFrame) (surface, buried int) {
	for x := 0; x < rulesFoodCols; x++ {
		for y := 0; y < rulesFoodRows; y++ {
			surface += f.surface[x][y]
			buried += f.buried[x][y]
		}
	}
	return surface, buried
}

// TestFoodSettlesWithoutBeingCreatedOrDestroyed: burial moves food between
// a cell's two layers, so the cell's total never changes. That is the
// property FoodManager.BuryFood is built around, and the picture claims it.
func TestFoodSettlesWithoutBeingCreatedOrDestroyed(t *testing.T) {
	loadKeyGlobals(t)
	b := newRulesFoodGrid()
	first, last := b.steps[0], b.steps[len(b.steps)-1]
	for x := 0; x < rulesFoodCols; x++ {
		for y := 0; y < rulesFoodRows; y++ {
			was := first.surface[x][y] + first.buried[x][y]
			now := last.surface[x][y] + last.buried[x][y]
			if was != now {
				t.Fatalf("cell %d,%d went from %d units to %d", x, y, was, now)
			}
		}
	}
	openSurface, openBuried := foodTotals(first)
	endSurface, endBuried := foodTotals(last)
	if openSurface == 0 {
		t.Fatal("the picture opens with nothing on the surface")
	}
	if endSurface >= openSurface/2 {
		t.Errorf("the surface only went from %d units to %d over the loop", openSurface, endSurface)
	}
	if endBuried <= openBuried {
		t.Errorf("the buried store went from %d units to %d", openBuried, endBuried)
	}
	// Some is still reachable at the end, so the picture is about settling
	// rather than about the surface emptying.
	if endSurface == 0 {
		t.Error("every pile is under by the end of the loop")
	}
}

// TestTheFoodPictureHoldsThenLoops: the same hold the pH pictures have, so
// the restart does not read as part of the settling.
func TestTheFoodPictureHoldsThenLoops(t *testing.T) {
	loadKeyGlobals(t)
	b := newRulesFoodGrid()
	last := len(b.steps) - 1
	if got := b.stepAt(0); got != 0 {
		t.Errorf("the loop opens on step %d", got)
	}
	if got := b.stepAt(time.Duration(last) * rulesFoodStepTime); got != last {
		t.Errorf("step %d is shown at the end of the play, want %d", got, last)
	}
	held := time.Duration(last+rulesFoodHold-1) * rulesFoodStepTime
	if got := b.stepAt(held); got != last {
		t.Errorf("the settled picture is not held: step %d at the end of the hold", got)
	}
	if got := b.stepAt(time.Duration(last+rulesFoodHold+1) * rulesFoodStepTime); got != 0 {
		t.Errorf("the loop restarts on step %d", got)
	}
}

// TestTheFoodPictureFitsThePanel: it is drawn at the same size as the pH
// pictures, and its caption is the longest of the three.
func TestTheFoodPictureFitsThePanel(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	b := newRulesFoodGrid()
	if w := rulesFoodCols * rulesFoodCell; w > rulesPanelW {
		t.Errorf("the patch is %dpx against a %dpx panel", w, rulesPanelW)
	}
	if w := boundString(r.FontSourceCodePro8, b.caption).Dx(); w > rulesPanelW {
		t.Errorf("the caption is %dpx against a %dpx panel: %q", w, rulesPanelW, b.caption)
	}
	for _, pile := range rulesFoodStart {
		if pile.x < 0 || pile.x >= rulesFoodCols || pile.y < 0 || pile.y >= rulesFoodRows {
			t.Errorf("a pile at %d,%d is off a %dx%d patch", pile.x, pile.y, rulesFoodCols, rulesFoodRows)
		}
		if pile.surface > config.MaxFoodValue() {
			t.Errorf("a pile of %d is past the %d a cell can hold", pile.surface, config.MaxFoodValue())
		}
	}
}
