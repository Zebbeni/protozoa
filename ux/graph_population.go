package ux

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
)

const popBaseHeight = 4096 // fixed base image height; organisms are scaled to fit

// nodeColorFunc selects which color to use from a DescendantNode
type nodeColorFunc func(node *organism.DescendantNode) color.Color

func traitColor(node *organism.DescendantNode) color.Color       { return node.Color }
func phEffectNodeColor(node *organism.DescendantNode) color.Color { return node.PhEffectColor }

// renderPopulation draws the full population graph colored by organism trait color.
func (g *Graph) renderPopulation(oldBarCount, newBarCount int) *ebiten.Image {
	trees := g.simulation.GetDescendantTrees()
	ids := g.simulation.GetAncestorsSorted()
	return renderPopGraph(
		&g.popBaseImage, &g.popBaseWidth, &g.popMaxAlive,
		0, oldBarCount, newBarCount, traitColor, trees, ids,
	)
}

// renderPopulationPhEffect draws the full population graph colored by phEffect.
func (g *Graph) renderPopulationPhEffect(oldBarCount, newBarCount int) *ebiten.Image {
	trees := g.simulation.GetDescendantTrees()
	ids := g.simulation.GetAncestorsSorted()
	return renderPopGraph(
		&g.popPhEffectBaseImage, &g.popPhEffectBaseWidth, &g.popPhEffectMaxAlive,
		0, oldBarCount, newBarCount, phEffectNodeColor, trees, ids,
	)
}

// renderSelectedPopulation draws the sub-tree population graph colored by trait color.
func (g *Graph) renderSelectedPopulation(oldBarCount, newBarCount int) *ebiten.Image {
	if g.selectedSubTreeRoot == nil {
		return nil
	}
	startBar := g.selStartCycle / c.PopulationUpdateInterval()
	trees := map[int]*organism.DescendantNode{0: g.selectedSubTreeRoot}
	ids := []int{0}
	return renderPopGraph(
		&g.selPopBaseImage, &g.selPopBaseWidth, &g.selPopMaxAlive,
		startBar, oldBarCount, newBarCount, traitColor, trees, ids,
	)
}

// renderSelectedPopulationPhEffect draws the sub-tree population graph colored by phEffect.
func (g *Graph) renderSelectedPopulationPhEffect(oldBarCount, newBarCount int) *ebiten.Image {
	if g.selectedSubTreeRoot == nil {
		return nil
	}
	startBar := g.selStartCycle / c.PopulationUpdateInterval()
	trees := map[int]*organism.DescendantNode{0: g.selectedSubTreeRoot}
	ids := []int{0}
	return renderPopGraph(
		&g.selPopPhEffBaseImage, &g.selPopPhEffBaseWidth, &g.selPopPhEffMaxAlive,
		startBar, oldBarCount, newBarCount, phEffectNodeColor, trees, ids,
	)
}

// renderPopGraph is the shared implementation for population-style bar graphs.
// startBar is the first bar index to include (0 for full graphs, later for sub-trees).
// oldBarCount/newBarCount are absolute bar indices.
func renderPopGraph(
	baseImage **ebiten.Image, baseWidth, maxAlive *int,
	startBar, oldBarCount, newBarCount int, colorFn nodeColorFunc,
	trees map[int]*organism.DescendantNode, ancestorIDs []int,
) *ebiten.Image {
	// Number of columns in the base image
	numCols := newBarCount - startBar
	if numCols < 1 {
		return ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	}

	if *baseImage == nil {
		// Full refresh
		max := 0
		for barIdx := startBar; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			count := countAliveInTrees(trees, ancestorIDs, cycle)
			if count > max {
				max = count
			}
		}
		if max < 1 {
			max = 1
		}
		*maxAlive = max
		*baseWidth = numCols * 2
		if *baseWidth < 4 {
			*baseWidth = 4
		}
		img := ebiten.NewImage(*baseWidth, popBaseHeight)

		for barIdx := startBar; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			drawPopColumn(img, *maxAlive, trees, ancestorIDs, cycle, barIdx-startBar, colorFn)
		}

		*baseImage = img
	} else {
		// Check if any new bar's alive count exceeds our current max
		needsRefresh := false
		for barIdx := oldBarCount; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			count := countAliveInTrees(trees, ancestorIDs, cycle)
			if count > *maxAlive {
				fmt.Printf("\nrenderPopGraph full refresh (maxAlive %d -> %d)", *maxAlive, count)
				needsRefresh = true
				break
			}
		}
		if needsRefresh {
			*baseImage = nil
			return renderPopGraph(baseImage, baseWidth, maxAlive, startBar, oldBarCount, newBarCount, colorFn, trees, ancestorIDs)
		}

		// Extend width if needed
		if numCols > *baseWidth {
			newWidth := numCols * 2
			newBase := ebiten.NewImage(newWidth, popBaseHeight)
			newBase.DrawImage(*baseImage, nil)
			*baseImage = newBase
			*baseWidth = newWidth
		}

		// Draw new bars
		for barIdx := oldBarCount; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			drawPopColumn(*baseImage, *maxAlive, trees, ancestorIDs, cycle, barIdx-startBar, colorFn)
		}
	}

	// Scale to display
	if *baseImage == nil || numCols == 0 {
		return ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	}
	img := ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Scale(
		realGraphWidth/float64(numCols),
		realGraphHeight/float64(popBaseHeight),
	)
	img.DrawImage(*baseImage, opts)
	return img
}

func drawPopColumn(
	baseImage *ebiten.Image, maxAlive int,
	trees map[int]*organism.DescendantNode, ancestorIDs []int,
	cycle, barIdx int, colorFn nodeColorFunc,
) {
	var alive []*organism.DescendantNode
	for _, id := range ancestorIDs {
		if root := trees[id]; root != nil {
			collectAlive(root, cycle, &alive)
		}
	}
	if len(alive) == 0 {
		return
	}

	colBuf := make([]byte, 4*popBaseHeight)

	heightPerOrg := float64(popBaseHeight) / float64(maxAlive)
	y := float64(popBaseHeight)
	for _, node := range alive {
		yTop := y - heightPerOrg
		pyStart := int(yTop)
		pyEnd := int(y)
		if pyStart < 0 {
			pyStart = 0
		}
		if pyEnd > popBaseHeight {
			pyEnd = popBaseHeight
		}

		clr := colorFn(node)
		r, gv, b, _ := clr.RGBA()
		cr, cg, cb := byte(r>>8), byte(gv>>8), byte(b>>8)

		for py := pyStart; py < pyEnd; py++ {
			idx := py * 4
			colBuf[idx] = cr
			colBuf[idx+1] = cg
			colBuf[idx+2] = cb
			colBuf[idx+3] = 255
		}
		y = yTop
	}

	col := ebiten.NewImage(1, popBaseHeight)
	col.WritePixels(colBuf)

	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(barIdx), 0)
	baseImage.DrawImage(col, opts)
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
