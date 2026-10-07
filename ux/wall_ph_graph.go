package ux

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/manager"
)

type wallAxis struct{}

const wallGraphSamples = manager.MaxWallStrength - manager.MinWallStrength

func (wallAxis) samples() int { return wallGraphSamples }

func (wallAxis) read(s curveGraphSeries, g *c.Globals, j, n int) float64 {
	return s.at(g, wallAxisStrength(j, n))
}

func wallAxisStrength(j, n int) float64 {
	lo := float64(manager.MinWallStrength)
	return lo + float64(j)/float64(n)*(float64(manager.MaxWallStrength)-lo)
}

func (wallAxis) marks(n int) []curveAxisMark {
	out := make([]curveAxisMark, 0, 3)
	for _, f := range []float64{0, 0.5, 1} {
		sample := f * float64(n)
		out = append(out, curveAxisMark{
			sample: sample,
			label:  fmt.Sprintf("%.0f", wallAxisStrength(int(sample), n)),
		})
	}
	return out
}

func wallPhDiffusionGraph() curveGraph {
	return curveGraph{
		title: "Share of pH diffusion passing, by wall strength",
		axis:  wallAxis{},
		// A share of the diffusion, so the axis stops at 1 rather than padding past "all of it".
		share:  true,
		series: []curveGraphSeries{{at: wallPermeabilityAt}},
	}
}

// wallPermeabilityAt reads the permeability at a wall strength, rounding to the whole strength a wall would really have.
func wallPermeabilityAt(g *c.Globals, strength float64) float64 {
	return manager.WallPermeability(g, int(strength+0.5))
}

const wallPhGraphHeight = curveGraphHeight

// wallPhGraphWidth sizes the graph so its PLOT is exactly as wide as the slider column.
const wallPhGraphWidth = cfgSliderWidth + curveGraphLeft + 8

func (cs *ConfigScreen) drawWallPhGraphRow(screen *ebiten.Image, px, py int) {
	drawCurveGraph(screen, px+cfgLabelWidth-curveGraphLeft, py, wallPhGraphWidth, wallPhDiffusionGraph(), cs.globals)
}
