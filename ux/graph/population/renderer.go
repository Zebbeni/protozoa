package population

import (
	"image/color"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/instrument"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	s "github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

const baseHeight = 4096

const maxBaseWidth = 4096

// strideFor returns how many bars share one base-image column so that numCols bars fit in maxBaseWidth.
func strideFor(numCols int) int {
	stride := 1
	for numCols > maxBaseWidth*stride {
		stride *= 2
	}
	return stride
}

// columnsFor is how many base-image columns numCols bars occupy at stride.
func columnsFor(numCols, stride int) int {
	return (numCols + stride - 1) / stride
}

// popGraphCeiling returns the y-axis ceiling to render bars against, given the current peak population.
func popGraphCeiling(peak int) int {
	return gh.Ceiling(peak)
}

type NodeColorFunc func(node *organism.DescendantNode) color.Color

func TraitColor(node *organism.DescendantNode) color.Color { return node.Color }

// AbilityColor returns a NodeColorFunc painting each organism gray→green by its score in one ability, on the same scale as the grid's ABILITY colour mode.
func AbilityColor(a physiology.Ability) NodeColorFunc {
	return func(node *organism.DescendantNode) color.Color {
		return gh.AbilityColor(node.Abilities, a)
	}
}

func (r *Renderer) aliveAt(trees map[int]*organism.DescendantNode, ancestorIDs []int, cycle int) []*organism.DescendantNode {
	if cache := r.aliveCache(); cache != nil {
		return cache.AliveAt(trees, ancestorIDs, cycle)
	}
	r.aliveBuf = collectAliveInTrees(trees, ancestorIDs, cycle, r.aliveBuf[:0])
	return r.aliveBuf
}

func (r *Renderer) aliveCache() *AliveCache {
	if r.subTreeRoot != nil {
		return nil
	}
	return r.alive
}

// Renderer renders a population bar graph using a descendant tree walk.
type Renderer struct {
	colorFn     NodeColorFunc
	startBar    int
	subTreeRoot *organism.DescendantNode

	baseImage *ebiten.Image
	baseWidth int
	stride    int
	maxAlive  int

	// peaks is the highest population seen up to each bar, published for the viewer so it can scale the visible stretch of the graph to the peak reached by then.
	peaks atomic.Pointer[[]int]

	// alive caches which organisms were alive at each cycle, shared with the renderers of the other colourings so the tree walk happens once per cycle.
	alive *AliveCache
	// aliveBuf is scratch for cache misses, reused across columns.
	aliveBuf []*organism.DescendantNode
}

// MaxAlive returns the renderer's current y-axis ceiling (peak alive count seen, with 50% headroom).
func (r *Renderer) MaxAlive() int { return r.maxAlive }

// IsSubTree reports whether this renderer is configured for a selection sub-tree (true) or the overall sim (false).
func (r *Renderer) IsSubTree() bool { return r.subTreeRoot != nil }

func NewRenderer(colorFn NodeColorFunc, subTreeRoot *organism.DescendantNode, startBar int, alive *AliveCache) *Renderer {
	return &Renderer{
		colorFn:     colorFn,
		subTreeRoot: subTreeRoot,
		startBar:    startBar,
		alive:       alive,
	}
}

func (r *Renderer) Reset() {
	r.baseImage = nil
	r.baseWidth = 0
	r.stride = 0
	r.maxAlive = 0
	r.peaks.Store(nil)
}

// HeightFraction is how much of the rendered image's height the bars up to throughBar occupy.
func (r *Renderer) HeightFraction(throughBar int) float64 {
	peaks := r.peaks.Load()
	if peaks == nil || len(*peaks) == 0 {
		return 1
	}
	idx := min(max(throughBar-r.startBar, 0), len(*peaks)-1)
	return gh.PeakFraction((*peaks)[idx], (*peaks)[len(*peaks)-1])
}

// recordPeaks publishes the running peak per bar, extending what an earlier render published when this one only added bars.
func (r *Renderer) recordPeaks(from int, counts []int) {
	peaks := make([]int, 0, from+len(counts))
	if prev := r.peaks.Load(); prev != nil && from > 0 {
		peaks = append(peaks, (*prev)[:min(from, len(*prev))]...)
	}
	running := 0
	if len(peaks) > 0 {
		running = peaks[len(peaks)-1]
	}
	for _, n := range counts {
		running = max(running, n)
		peaks = append(peaks, running)
	}
	r.peaks.Store(&peaks)
}

// Render reports progress as it walks the descendant trees: one unit per bar counted and one per column drawn.
func (r *Renderer) Render(sim *s.Simulation, oldBarCount, newBarCount int, progress *gh.Progress) *ebiten.Image {
	trees, ids := r.getTrees(sim)
	return r.renderPopGraph(oldBarCount, newBarCount, trees, ids, progress)
}

func (r *Renderer) getTrees(sim *s.Simulation) (map[int]*organism.DescendantNode, []int) {
	if r.subTreeRoot != nil {
		return map[int]*organism.DescendantNode{0: r.subTreeRoot}, []int{0}
	}
	return sim.GetDescendantTrees(), sim.GetAncestorsSorted()
}

func (r *Renderer) renderPopGraph(oldBarCount, newBarCount int,
	trees map[int]*organism.DescendantNode, ancestorIDs []int, progress *gh.Progress) *ebiten.Image {

	numCols := newBarCount - r.startBar
	if numCols < 1 {
		return instrument.NewImage(int(gh.RealGraphWidth), int(gh.RealGraphHeight))
	}
	stride := strideFor(numCols)
	cols := columnsFor(numCols, stride)

	needsRebuild := r.baseImage == nil || stride != r.stride
	if !needsRebuild {
		for barIdx := max(oldBarCount, r.startBar); barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			if r.aliveCache().CountAt(trees, ancestorIDs, cycle) > r.maxAlive {
				needsRebuild = true
				break
			}
		}
	}

	if needsRebuild {
		// Two passes: count every bar for the y-axis peak, then draw every column.
		progress.AddWork(numCols + cols)
		peak := 0
		counts := make([]int, 0, numCols)
		for barIdx := r.startBar; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			n := r.aliveCache().CountAt(trees, ancestorIDs, cycle)
			counts = append(counts, n)
			peak = max(peak, n)
			progress.Step()
		}
		r.recordPeaks(0, counts)
		r.maxAlive = popGraphCeiling(max(peak, 1))
		r.stride = stride
		r.baseWidth = max(4, min(cols*2, maxBaseWidth))
		r.baseImage = ebiten.NewImage(r.baseWidth, baseHeight)
		// Each column shows the last bar in its group, matching what the incremental path leaves behind as it overwrites a column.
		for col := 0; col < cols; col++ {
			barIdx := min(r.startBar+(col+1)*stride, newBarCount) - 1
			r.drawColumn(trees, ancestorIDs, barIdx*c.PopulationUpdateInterval(), col)
			progress.Step()
		}
	} else {
		if cols > r.baseWidth {
			newWidth := min(cols*2, maxBaseWidth)
			newBase := ebiten.NewImage(newWidth, baseHeight)
			newBase.DrawImage(r.baseImage, nil)
			r.baseImage = newBase
			r.baseWidth = newWidth
		}
		first := max(oldBarCount, r.startBar)
		progress.AddWork(newBarCount - first)
		counts := make([]int, 0, max(newBarCount-first, 0))
		for barIdx := first; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			counts = append(counts, r.aliveCache().CountAt(trees, ancestorIDs, cycle))
			r.drawColumn(trees, ancestorIDs, cycle, (barIdx-r.startBar)/stride)
			progress.Step()
		}
		r.recordPeaks(first-r.startBar, counts)
	}

	width := float64(gh.GraphImageWidth(cols))
	img := instrument.NewImage(int(width), int(gh.RealGraphHeight))
	// Faint background just barely off the panel fill, so the graph area is visible without competing with the bars.
	img.Fill(gh.GraphBackground())
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Scale(width/float64(cols), gh.RealGraphHeight/float64(baseHeight))
	img.DrawImage(r.baseImage, opts)
	return img
}

func (r *Renderer) drawColumn(trees map[int]*organism.DescendantNode, ancestorIDs []int, cycle, barIdx int) {
	alive := r.aliveAt(trees, ancestorIDs, cycle)
	if len(alive) == 0 {
		return
	}

	colBuf := make([]byte, 4*baseHeight)
	heightPerOrg := float64(baseHeight) / float64(r.maxAlive)
	y := float64(baseHeight)
	for _, node := range alive {
		yTop := y - heightPerOrg
		pyStart := int(yTop)
		pyEnd := int(y)
		if pyStart < 0 {
			pyStart = 0
		}
		if pyEnd > baseHeight {
			pyEnd = baseHeight
		}

		clr := r.colorFn(node)
		rv, gv, bv, _ := clr.RGBA()
		cr, cg, cb := byte(rv>>8), byte(gv>>8), byte(bv>>8)

		for py := pyStart; py < pyEnd; py++ {
			idx := py * 4
			colBuf[idx] = cr
			colBuf[idx+1] = cg
			colBuf[idx+2] = cb
			colBuf[idx+3] = 255
		}
		y = yTop
	}

	col := instrument.NewImage(1, baseHeight)
	col.WritePixels(colBuf)

	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(barIdx), 0)
	r.baseImage.DrawImage(col, opts)
}

func collectAlive(node *organism.DescendantNode, cycle int, result *[]*organism.DescendantNode) {
	// Skip entire sub-tree if it hasn't been born yet
	if node.StartCycle > cycle {
		return
	}
	if node.AllBranchesDeadCycle != 0 && cycle >= node.AllBranchesDeadCycle {
		return
	}
	if node.EndCycle == 0 || node.EndCycle > cycle {
		*result = append(*result, node)
	}
	node.ForEachChild(func(child *organism.DescendantNode) {
		collectAlive(child, cycle, result)
	})
}

func countAliveInTrees(trees map[int]*organism.DescendantNode, ancestorIDs []int, cycle int) int {
	count := 0
	for _, id := range ancestorIDs {
		if root := trees[id]; root != nil {
			count += countAlive(root, cycle)
		}
	}
	return count
}

func countAlive(node *organism.DescendantNode, cycle int) int {
	// Skip entire sub-tree if it hasn't been born yet
	if node.StartCycle > cycle {
		return 0
	}
	if node.AllBranchesDeadCycle != 0 && cycle >= node.AllBranchesDeadCycle {
		return 0
	}
	count := 0
	if node.EndCycle == 0 || node.EndCycle > cycle {
		count = 1
	}
	node.ForEachChild(func(child *organism.DescendantNode) {
		count += countAlive(child, cycle)
	})
	return count
}
