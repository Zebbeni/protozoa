package population

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/instrument"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	s "github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

const baseHeight = 4096

// maxBaseWidth caps the base image's width. The base image used to grow
// two pixels per bar without limit, and a long run (8833 bars) asked
// ebiten for a 17666px texture, which panics: GPU textures top out around
// 16384px. The graph is drawn into RealGraphWidth (1000px) anyway, so past
// this width several bars share one column with no visible loss.
const maxBaseWidth = 4096

// strideFor returns how many bars share one base-image column so that
// numCols bars fit in maxBaseWidth. It only ever doubles, so a growing run
// rebuilds the base image O(log n) times rather than on every new bar.
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

// popGraphCeiling returns the y-axis ceiling to render bars against,
// given the current peak population. Adds a small relative headroom
// (12.5%, with an absolute floor of 2) so the highest bars don't
// touch the top edge of the graph but still fill most of the
// vertical space — a previous 50% headroom left a noticeable empty
// band above the peak.
func popGraphCeiling(peak int) int {
	headroom := peak / 8
	if headroom < 2 {
		headroom = 2
	}
	return peak + headroom
}

// NodeColorFunc selects which color to use from a DescendantNode
type NodeColorFunc func(node *organism.DescendantNode) color.Color

func TraitColor(node *organism.DescendantNode) color.Color { return node.Color }

// AbilityColor returns a NodeColorFunc painting each organism gray→green
// by its score in one ability, on the same scale as the grid's ABILITY
// colour mode.
func AbilityColor(a physiology.Ability) NodeColorFunc {
	return func(node *organism.DescendantNode) color.Color {
		return gh.AbilityColor(node.Abilities, a)
	}
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
}

// MaxAlive returns the renderer's current y-axis ceiling (peak alive
// count seen, with 50% headroom). Exposed for diagnostic logging.
func (r *Renderer) MaxAlive() int { return r.maxAlive }

// IsSubTree reports whether this renderer is configured for a
// selection sub-tree (true) or the overall sim (false).
func (r *Renderer) IsSubTree() bool { return r.subTreeRoot != nil }

func NewRenderer(colorFn NodeColorFunc, subTreeRoot *organism.DescendantNode, startBar int) *Renderer {
	return &Renderer{
		colorFn:     colorFn,
		subTreeRoot: subTreeRoot,
		startBar:    startBar,
	}
}

func (r *Renderer) Reset() {
	r.baseImage = nil
	r.baseWidth = 0
	r.stride = 0
	r.maxAlive = 0
}

// Render reports progress as it walks the descendant trees: one unit per
// bar counted and one per column drawn. The walks are what make a full
// rebuild over a long run slow.
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

	// A full rebuild is needed on first render, when the bars no longer
	// fit at the current stride, or when a new bar exceeds the y-axis
	// ceiling. The rebuild is done in place, without nil-ing r.baseImage
	// first — an earlier version did and recursed, which left a brief
	// window where the renderer's image was empty if the goroutine got
	// interleaved with the main thread's frame composition.
	needsRebuild := r.baseImage == nil || stride != r.stride
	if !needsRebuild {
		for barIdx := max(oldBarCount, r.startBar); barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			if countAliveInTrees(trees, ancestorIDs, cycle) > r.maxAlive {
				needsRebuild = true
				break
			}
		}
	}

	if needsRebuild {
		// Two passes: count every bar for the y-axis peak, then draw
		// every column.
		progress.AddWork(numCols + cols)
		peak := 0
		for barIdx := r.startBar; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			peak = max(peak, countAliveInTrees(trees, ancestorIDs, cycle))
			progress.Step()
		}
		r.maxAlive = popGraphCeiling(max(peak, 1))
		r.stride = stride
		r.baseWidth = max(4, min(cols*2, maxBaseWidth))
		r.baseImage = ebiten.NewImage(r.baseWidth, baseHeight)
		// Each column shows the last bar in its group, matching what the
		// incremental path leaves behind as it overwrites a column.
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
		for barIdx := first; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			r.drawColumn(trees, ancestorIDs, cycle, (barIdx-r.startBar)/stride)
			progress.Step()
		}
	}

	width := float64(gh.GraphImageWidth(cols))
	img := instrument.NewImage(int(width), int(gh.RealGraphHeight))
	// Faint background just barely off the panel fill, so the graph
	// area is visible without competing with the bars. Light theme uses
	// a near-white shade; dark theme stays at near-black.
	img.Fill(gh.GraphBackground())
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Scale(width/float64(cols), gh.RealGraphHeight/float64(baseHeight))
	img.DrawImage(r.baseImage, opts)
	return img
}

func (r *Renderer) drawColumn(trees map[int]*organism.DescendantNode, ancestorIDs []int, cycle, barIdx int) {
	var alive []*organism.DescendantNode
	for _, id := range ancestorIDs {
		if root := trees[id]; root != nil {
			collectAlive(root, cycle, &alive)
		}
	}
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
	// Skip entire sub-tree if all branches died before this cycle
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
	// Skip entire sub-tree if all branches died before this cycle
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
