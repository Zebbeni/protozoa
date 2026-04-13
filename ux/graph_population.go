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

// renderPopulation draws the population graph colored by organism trait color.
func (g *Graph) renderPopulation(oldBarCount, newBarCount int) *ebiten.Image {
	return g.renderPopGraph(
		&g.popBaseImage, &g.popBaseWidth, &g.popMaxAlive,
		oldBarCount, newBarCount, traitColor,
	)
}

// renderPopulationPhEffect draws the population graph colored by phEffect.
func (g *Graph) renderPopulationPhEffect(oldBarCount, newBarCount int) *ebiten.Image {
	return g.renderPopGraph(
		&g.popPhEffectBaseImage, &g.popPhEffectBaseWidth, &g.popPhEffectMaxAlive,
		oldBarCount, newBarCount, phEffectNodeColor,
	)
}

// renderPopGraph is the shared implementation for population-style bar graphs.
func (g *Graph) renderPopGraph(
	baseImage **ebiten.Image, baseWidth, maxAlive *int,
	oldBarCount, newBarCount int, colorFn nodeColorFunc,
) *ebiten.Image {
	trees := g.simulation.GetDescendantTrees()
	sortedAncestorIDs := g.simulation.GetAncestorsSorted()

	if *baseImage == nil {
		// Full refresh
		max := 0
		for barIdx := 0; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			count := countAliveInTrees(trees, sortedAncestorIDs, cycle)
			if count > max {
				max = count
			}
		}
		if max < 1 {
			max = 1
		}
		*maxAlive = max
		*baseWidth = newBarCount * 2
		if *baseWidth < 4 {
			*baseWidth = 4
		}
		img := ebiten.NewImage(*baseWidth, popBaseHeight)
		for barIdx := 0; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			drawPopColumn(img, *maxAlive, trees, sortedAncestorIDs, cycle, barIdx, colorFn)
		}
		*baseImage = img
	} else {
		// Check if any new bar's alive count exceeds our current max
		needsRefresh := false
		for barIdx := oldBarCount; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			count := countAliveInTrees(trees, sortedAncestorIDs, cycle)
			if count > *maxAlive {
				fmt.Printf("\nrenderPopGraph full refresh (maxAlive %d -> %d)", *maxAlive, count)
				needsRefresh = true
				break
			}
		}
		if needsRefresh {
			// Redo as full refresh (recurse with nil base)
			*baseImage = nil
			return g.renderPopGraph(baseImage, baseWidth, maxAlive, oldBarCount, newBarCount, colorFn)
		}

		// Extend width if needed
		if newBarCount > *baseWidth {
			newWidth := newBarCount * 2
			newBase := ebiten.NewImage(newWidth, popBaseHeight)
			newBase.DrawImage(*baseImage, nil)
			*baseImage = newBase
			*baseWidth = newWidth
		}

		// Draw new bars
		for barIdx := oldBarCount; barIdx < newBarCount; barIdx++ {
			cycle := barIdx * c.PopulationUpdateInterval()
			drawPopColumn(*baseImage, *maxAlive, trees, sortedAncestorIDs, cycle, barIdx, colorFn)
		}
	}

	// Scale to display
	if *baseImage == nil || newBarCount == 0 {
		return ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	}
	img := ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Scale(
		realGraphWidth/float64(newBarCount),
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
