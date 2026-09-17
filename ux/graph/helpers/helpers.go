package helpers

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

const (
	// RealGraphWidth is the minimum width graph images render at.
	// Long runs render wider — see GraphImageWidth — so the panel's
	// zoom has real detail to show rather than stretched pixels.
	RealGraphWidth = 1000.0
	// RealGraphHeight is the height graph images render at. The panel
	// draws them 120px tall, so this only needs headroom for smooth
	// downscaling; keeping it modest bounds the memory of wide images.
	RealGraphHeight = 512.0
	// MaxGraphImageWidth caps graph image width well inside GPU texture
	// limits (~16384px) and keeps each image to a few megabytes.
	MaxGraphImageWidth = 4096
	PhMaxHue           = 100.0
)

// PhValueColor maps a pH value to RGBA floats using the grid's pH color
// spectrum. Matches the env-layer colour logic in ux/grid.go: extremes
// get high-contrast colours, neutral pH is blended towards the active
// theme's background so it visually disappears into the window fill
// (black under dark, white under light).
func PhValueColor(ph float64) (float32, float32, float32, float32) {
	// Same colour scheme as the env-layer renderer in ux/grid.go: blend
	// between theme background (at neutral) and the acid (#A9C218) or
	// base (#E74766) extreme colour, weighted linearly by distance from
	// neutral pH.
	neutral := (c.MaxPh() + c.MinPh()) / 2.0
	halfRange := (c.MaxPh() - c.MinPh()) / 2.0
	weight := 0.0
	if halfRange > 0 {
		weight = math.Abs(ph-neutral) / halfRange
		if weight > 1 {
			weight = 1
		}
	}
	bgR, bgG, bgB := c.ThemeBackgroundRGB()
	tgtR, tgtG, tgtB := c.PhTargetColorRGB(ph)
	bg := colorful.Color{R: bgR, G: bgG, B: bgB}
	target := colorful.Color{R: tgtR, G: tgtG, B: tgtB}
	blended := bg.BlendRgb(target, weight).Clamped()
	return float32(blended.R), float32(blended.G), float32(blended.B), 1
}

func WhiteSrc() *ebiten.Image {
	whiteImg := ebiten.NewImage(1, 1)
	whiteImg.Fill(color.White)
	return whiteImg.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image)
}

func ColorToFloat(clr color.Color) (float32, float32, float32, float32) {
	r, g, b, a := clr.RGBA()
	return float32(r) / 65535, float32(g) / 65535, float32(b) / 65535, float32(a) / 65535
}

func FlushAndAppendQuad(vertices *[]ebiten.Vertex, indices *[]uint16,
	img *ebiten.Image, src *ebiten.Image,
	xLeft, prevY1, prevY2, xRight, newY1, newY2 float32,
	cr, cg, cb, ca float32) {

	if len(*vertices) >= 65532 {
		img.DrawTriangles(*vertices, *indices, src, nil)
		*vertices = (*vertices)[:0]
		*indices = (*indices)[:0]
	}

	base := uint16(len(*vertices))
	v := func(x, y float32) ebiten.Vertex {
		return ebiten.Vertex{DstX: x, DstY: y, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca}
	}
	*vertices = append(*vertices, v(xLeft, prevY1), v(xLeft, prevY2), v(xRight, newY1), v(xRight, newY2))
	*indices = append(*indices, base, base+1, base+2, base+2, base+1, base+3)
}

// GreenRedColor maps t in [0, 1] onto the green→red HSLuv spectrum:
// 1 = green, 0 = red, 0.5 = yellow. Hue 0° = red, 120° = green; HSLuv
// keeps the transitions perceptually uniform. Lives here rather than in
// ux because the graph renderers need it too, and they can't import ux.
func GreenRedColor(t float64) colorful.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return colorful.HSLuv(120.0*t, 0.9, 0.5)
}

// Gray→green ramp endpoints, as HSLuv lightness. Lightness does most of
// the work of telling scores apart: an earlier version held it fixed and
// changed only saturation, and neighbouring scores were close to
// indistinguishable. The low end is a dark gray so an uninvested ability
// recedes; the high end is a bright green so a specialist stands out.
const (
	grayGreenLowLightness  = 0.28
	grayGreenHighLightness = 0.85
)

// GrayGreenColor maps t in [0, 1] from a dark neutral gray (0) to a bright
// green (1). Saturation and lightness rise together at a fixed hue, so the
// ramp reads purely as "how much" rather than as a warning — a low score
// looks absent, not alarming — while the lightness change keeps adjacent
// values easy to tell apart. HSLuv keeps every step in gamut and the
// brightness change perceptually even.
func GrayGreenColor(t float64) colorful.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	lightness := grayGreenLowLightness + (grayGreenHighLightness-grayGreenLowLightness)*t
	return colorful.HSLuv(120.0, t, lightness)
}

// AbilityScoreColor tints a score gray→green on the same scale as the
// ABILITY views: gray at nothing, full green at SpecialistScore.
func AbilityScoreColor(score float64) colorful.Color {
	return GrayGreenColor(score / float64(physiology.SpecialistScore))
}

// AbilityColor maps an organism's score in one ability onto the
// gray→green ramp: gray at zero, green at that ability's specialist
// score, clamped above.
//
// Gray rather than red at the low end because a low score isn't a
// problem, just an ability the organism hasn't invested in — red read as
// a warning. Health keeps its red→green scale, where low really is bad.
//
// Anchored on the specialist score rather than on the 100-point budget
// because real scores cluster well below 100 — dividing by the budget
// would leave nearly every organism near the gray end and hide the very
// specialists the view exists to find. Anchoring per ability also means
// green says the same thing for every ability ("fully specialised"),
// even though chemosynthesis starts at a far higher allocation than the
// rest. Shared by the grid's ABILITY colour mode and the population
// graph so the two always agree on what a colour means.
func AbilityColor(scores physiology.Scores, a physiology.Ability) colorful.Color {
	return GrayGreenColor(float64(scores[a]) / float64(physiology.SpecialistScore))
}

// Ceiling is the y-axis top to plot a series against, given its peak:
// the peak plus a small relative headroom (12.5%, with an absolute floor
// of 2) so the highest points don't touch the top edge but still fill
// most of the height.
func Ceiling(peak int) int {
	headroom := peak / 8
	if headroom < 2 {
		headroom = 2
	}
	return peak + headroom
}

// PeakFraction is how much of a graph's height the data up to some point
// occupies, when the graph was drawn against the whole run's peak. The
// viewer stretches that band, so early cycles of a run that ends far
// larger aren't a flat line along the bottom.
func PeakFraction(peakSoFar, peakOverall int) float64 {
	if peakOverall <= 0 {
		return 1
	}
	return min(1, max(0, float64(Ceiling(peakSoFar))/float64(Ceiling(peakOverall))))
}

// GraphImageWidth is the pixel width to render a graph of the given number
// of bars at: one pixel per bar, at least RealGraphWidth and at most
// MaxGraphImageWidth. Zooming into the panel's graph crops this image, so
// the wider it is, the more detail a zoomed view shows.
func GraphImageWidth(bars int) int {
	return min(MaxGraphImageWidth, max(int(RealGraphWidth), bars))
}
