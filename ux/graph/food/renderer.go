package food

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/instrument"
	"github.com/Zebbeni/protozoa/manager"
	s "github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

// foodAreaColor tints the filled area. Same hue (35) as the grid's
// food sprites, but lighter and more saturated so the series reads
// clearly against the graph background under either theme.
var foodAreaColor = colorful.HSLuv(35, 0.65, 0.55)

// Renderer renders a filled area chart of the total food-item count
// over time. Unlike the population renderer it keeps no incremental
// base image: the series is a single value per cycle, so a full
// redraw each frame is cheap — and the y-axis ceiling has to rescale
// as the peak grows anyway.
type Renderer struct{}

func NewRenderer() *Renderer { return &Renderer{} }

func (r *Renderer) Reset() {}

func (r *Renderer) Render(sim *s.Simulation, oldBarCount, newBarCount int) *ebiten.Image {
	img := instrument.NewImage(int(gh.RealGraphWidth), int(gh.RealGraphHeight))
	// Faint background just off the panel fill, matching the population
	// graph's treatment so the plot area is visible without competing
	// with the filled series.
	bg := color.RGBA{R: 30, G: 30, B: 35, A: 255}
	if c.IsLightTheme() {
		bg = color.RGBA{R: 235, G: 235, B: 240, A: 255}
	}
	img.Fill(bg)
	if newBarCount < 1 {
		return img
	}

	sim.LockHistoryForReading()
	foodHist := sim.GetHistory(manager.HistoryFood)

	// countAt reads the food total recorded for a bar's cycle. Each
	// HistoryFood entry stores the count under the single fixed key 0.
	countAt := func(barIdx int) int {
		cycle := barIdx * c.PopulationUpdateInterval()
		if dist, ok := foodHist[cycle]; ok {
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

	barWidth := gh.RealGraphWidth / float64(newBarCount)
	bottom := float32(gh.RealGraphHeight)
	cr := float32(foodAreaColor.R)
	cg := float32(foodAreaColor.G)
	cb := float32(foodAreaColor.B)

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
		// initial-food count behind a triangle that reads as "food
		// being created from scratch". Anchoring prev to newCount
		// instead draws bar 0 as a flat rectangle at the true initial
		// level, so the seeded food shows up immediately.
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
