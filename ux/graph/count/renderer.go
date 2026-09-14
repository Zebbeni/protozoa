package count

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/instrument"
	"github.com/Zebbeni/protozoa/manager"
	s "github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

// FoodColor tints the food series. Same hue (35) as the grid's food
// sprites, but lighter and more saturated so the series reads clearly
// against the graph background under either theme.
var FoodColor = colorful.HSLuv(35, 0.65, 0.55)

// WallColor tints the wall series: the grid's neutral wall grey, lifted
// slightly so it stays visible against the dark theme's background.
var WallColor = colorful.HSLuv(0, 0, 0.6)

// Renderer renders a filled area chart of one count recorded per history
// interval (total food items, total wall cells), read from a history map
// that stores the count under the single fixed key 0.
//
// Unlike the population renderer it keeps no incremental base image: the
// series is a single value per cycle, so a full redraw each frame is
// cheap — and the y-axis ceiling has to rescale as the peak grows anyway.
type Renderer struct {
	history manager.HistoryType
	fill    colorful.Color
}

// NewRenderer returns a renderer plotting the given history series in
// the given fill colour.
func NewRenderer(history manager.HistoryType, fill colorful.Color) *Renderer {
	return &Renderer{history: history, fill: fill}
}

func (r *Renderer) Reset() {}

// Render ignores progress: one value per bar is fast enough to finish
// well before a progress bar would be worth showing.
func (r *Renderer) Render(sim *s.Simulation, oldBarCount, newBarCount int, _ *gh.Progress) *ebiten.Image {
	width := float64(gh.GraphImageWidth(newBarCount))
	img := instrument.NewImage(int(width), int(gh.RealGraphHeight))
	// Faint background just off the panel fill, matching the population
	// graph's treatment so the plot area is visible without competing
	// with the filled series.
	img.Fill(gh.GraphBackground())
	if newBarCount < 1 {
		return img
	}

	sim.LockHistoryForReading()
	hist := sim.GetHistory(r.history)

	// countAt reads the total recorded for a bar's cycle, stored under
	// the single fixed key 0.
	countAt := func(barIdx int) int {
		cycle := barIdx * c.PopulationUpdateInterval()
		if dist, ok := hist[cycle]; ok {
			return int(dist[0])
		}
		return 0
	}

	// y-axis ceiling: peak count + 12.5% headroom (min 2) so the
	// highest point doesn't touch the top edge of the graph.
	peak := 0
	for barIdx := 0; barIdx < newBarCount; barIdx++ {
		if cnt := countAt(barIdx); cnt > peak {
			peak = cnt
		}
	}
	headroom := peak / 8
	if headroom < 2 {
		headroom = 2
	}
	ceiling := float64(peak + headroom)

	barWidth := width / float64(newBarCount)
	bottom := float32(gh.RealGraphHeight)
	cr := float32(r.fill.R)
	cg := float32(r.fill.G)
	cb := float32(r.fill.B)

	yFor := func(count int) float32 {
		return float32(gh.RealGraphHeight * (1.0 - float64(count)/ceiling))
	}

	var vertices []ebiten.Vertex
	var indices []uint16
	src := gh.WhiteSrc()

	for barIdx := 0; barIdx < newBarCount; barIdx++ {
		newCount := countAt(barIdx)
		// Bar 0 has no recorded predecessor; treating prev as 0 would
		// ramp the very first column up from the bottom, hiding the
		// initial count (seeded food or walls) behind a triangle that
		// reads as "created from scratch". Anchoring prev to newCount
		// instead draws bar 0 as a flat rectangle at the true initial
		// level, so the seeded amount shows up immediately.
		prevCount := newCount
		if barIdx > 0 {
			prevCount = countAt(barIdx - 1)
		}
		xLeft := float32(barIdx) * float32(barWidth)
		xRight := float32(barIdx+1) * float32(barWidth)
		gh.FlushAndAppendQuad(&vertices, &indices, img, src,
			xLeft, yFor(prevCount), bottom, xRight, yFor(newCount), bottom,
			cr, cg, cb, 1)
	}

	sim.UnlockHistoryForReading()

	if len(vertices) > 0 {
		img.DrawTriangles(vertices, indices, src, nil)
	}

	return img
}
