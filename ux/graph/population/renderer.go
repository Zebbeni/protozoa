package population

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	s "github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

const baseHeight = 4096

// NodeColorFunc selects which color to use from a DescendantNode
type NodeColorFunc func(node *organism.DescendantNode) color.Color

func TraitColor(node *organism.DescendantNode) color.Color        { return node.Color }
func PhEffectNodeColor(node *organism.DescendantNode) color.Color { return node.PhEffectColor }

// Renderer renders a population bar graph using a descendant tree walk.
type Renderer struct {
	colorFn     NodeColorFunc
	startBar    int
	subTreeRoot *organism.DescendantNode

	baseImage *ebiten.Image
	baseWidth int
	maxAlive  int
}

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
	r.maxAlive = 0
}

func (r *Renderer) Render(sim *s.Simulation, oldBarCount, newBarCount int) *ebiten.Image {
	trees, ids := r.getTrees(sim)
	return r.renderPopGraph(oldBarCount, newBarCount, trees, ids)
}

func (r *Renderer) getTrees(sim *s.Simulation) (map[int]*organism.DescendantNode, []int) {
	if r.subTreeRoot != nil {
		return map[int]*organism.DescendantNode{0: r.subTreeRoot}, []int{0}
	}
	return sim.GetDescendantTrees(), sim.GetAncestorsSorted()
}

func (r *Renderer) renderPopGraph(oldBarCount, newBarCount int,
	trees map[int]*organism.DescendantNode, ancestorIDs []int) *ebiten.Image {

	numCols := newBarCount - r.startBar
	if numCols < 1 {
		return ebiten.NewImage(int(gh.RealGraphWidth), int(gh.RealGraphHeight))
	}

	if r.baseImage == nil {
		max := 0
		for barIdx := r.startBar; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			count := countAliveInTrees(trees, ancestorIDs, cycle)
			if count > max {
				max = count
			}
		}
		if max < 1 {
			max = 1
		}
		r.maxAlive = max
		r.baseWidth = numCols * 2
		if r.baseWidth < 4 {
			r.baseWidth = 4
		}
		r.baseImage = ebiten.NewImage(r.baseWidth, baseHeight)

		for barIdx := r.startBar; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			r.drawColumn(trees, ancestorIDs, cycle, barIdx-r.startBar)
		}
	} else {
		needsRefresh := false
		for barIdx := oldBarCount; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			count := countAliveInTrees(trees, ancestorIDs, cycle)
			if count > r.maxAlive {
				needsRefresh = true
				break
			}
		}
		if needsRefresh {
			r.baseImage = nil
			return r.renderPopGraph(oldBarCount, newBarCount, trees, ancestorIDs)
		}

		if numCols > r.baseWidth {
			newWidth := numCols * 2
			newBase := ebiten.NewImage(newWidth, baseHeight)
			newBase.DrawImage(r.baseImage, nil)
			r.baseImage = newBase
			r.baseWidth = newWidth
		}

		for barIdx := oldBarCount; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			r.drawColumn(trees, ancestorIDs, cycle, barIdx-r.startBar)
		}
	}

	if r.baseImage == nil || numCols == 0 {
		return ebiten.NewImage(int(gh.RealGraphWidth), int(gh.RealGraphHeight))
	}
	img := ebiten.NewImage(int(gh.RealGraphWidth), int(gh.RealGraphHeight))
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Scale(gh.RealGraphWidth/float64(numCols), gh.RealGraphHeight/float64(baseHeight))
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

	col := ebiten.NewImage(1, baseHeight)
	col.WritePixels(colBuf)

	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(barIdx), 0)
	r.baseImage.DrawImage(col, opts)
}

func collectAlive(node *organism.DescendantNode, cycle int, result *[]*organism.DescendantNode) {
	if node.StartCycle <= cycle && (node.EndCycle == 0 || node.EndCycle > cycle) {
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
	count := 0
	if node.StartCycle <= cycle && (node.EndCycle == 0 || node.EndCycle > cycle) {
		count = 1
	}
	node.ForEachChild(func(child *organism.DescendantNode) {
		count += countAlive(child, cycle)
	})
	return count
}
