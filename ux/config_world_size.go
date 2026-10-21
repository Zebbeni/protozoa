package ux

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// The SMALL / MEDIUM / LARGE row at the top of the form. One click sets the
// grid and both food stocks together, which is the combination that has to
// move as a unit: a grid changed on its own leaves the world at the previous
// size's density, and the two rows that would fix it are in a different
// section further down.
//
// It sits above Seed, so it is the first thing on the screen — the question
// "how big is this world" is asked before any of the balance questions under
// it, and on the browser it is the one setting that is not the desktop's.
const worldSizeButtonH = 14

// worldSizeButtonRect is the geometry apart from the drawing, so what a button
// covers is measured without a render target — the same split the brain view's
// buttons use.
func (cs *ConfigScreen) worldSizeButtonRect(px, py, i int) (x, y, w, h int) {
	const gap = 4
	n := len(c.AllWorldSizes)
	total := cs.sliderColWidth()
	w = (total - gap*(n-1)) / n
	x = px + cfgLabelWidth + i*(w+gap)
	return x, py, w, worldSizeButtonH
}

func (cs *ConfigScreen) drawWorldSizeRow(screen *ebiten.Image, px, py int) {
	text.Draw(screen, "World Size", r.FontSourceCodePro10, px, py+10, themedLabel())

	active, known := c.WorldSizeOf(cs.globals)
	for i, size := range c.AllWorldSizes {
		x, y, w, h := cs.worldSizeButtonRect(px, py, i)
		// Lit for the size the configuration is on. With the grid or a food
		// stock hand-edited nothing is lit, rather than the nearest button
		// claiming a world it would not restore.
		cs.drawSmallButton(screen, x, y, w, h, size.Name(), known && size == active)
	}

	label := fmt.Sprintf("%dx%d", cs.globals.GridUnitsWide, cs.globals.GridUnitsHigh)
	if !known {
		label += " custom"
	}
	text.Draw(screen, label, r.FontSourceCodePro10,
		px+cfgLabelWidth+cs.sliderColWidth()+10, py+10, themedValue())
}

func (cs *ConfigScreen) clickWorldSize(px, py, mx, my int) {
	if cs.readOnly {
		return
	}
	for i, size := range c.AllWorldSizes {
		x, y, w, h := cs.worldSizeButtonRect(px, py, i)
		if mx >= x && mx < x+w && my >= y && my < y+h {
			c.ApplyWorldSize(cs.globals, size)
			return
		}
	}
}
