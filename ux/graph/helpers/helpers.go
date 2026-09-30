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
	RealGraphWidth = 1000.0
	// RealGraphHeight is the height graph images render at.
	RealGraphHeight = 512.0
	// MaxGraphImageWidth caps graph image width well inside GPU texture limits (~16384px) and keeps each image to a few megabytes.
	MaxGraphImageWidth = 4096
	PhMaxHue           = 100.0
)

// PhValueColor maps a pH value to RGBA floats using the grid's pH color spectrum.
func PhValueColor(ph float64) (float32, float32, float32, float32) {
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

// GreenRedColor maps t in [0, 1] onto the green→red HSLuv spectrum: 1 = green, 0 = red, 0.5 = yellow.
func GreenRedColor(t float64) colorful.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return colorful.HSLuv(120.0*t, 0.9, 0.5)
}

const (
	grayGreenLowLightness  = 0.28
	grayGreenHighLightness = 0.85
)

// GrayGreenColor maps t in [0, 1] from a dark neutral gray (0) to a bright green (1).
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

const AbilityFullGreenScore = 7.5

// AbilityScoreColor tints a score gray→green on the same scale as the ABILITY views.
func AbilityScoreColor(score float64) colorful.Color {
	return GrayGreenColor(score / AbilityFullGreenScore)
}

// AbilityColor maps an organism's score in one ability onto the gray→green ramp.
func AbilityColor(scores physiology.Scores, a physiology.Ability) colorful.Color {
	return AbilityScoreColor(float64(scores[a]))
}

// Ceiling is the y-axis top to plot a series against, given its peak.
func Ceiling(peak int) int {
	headroom := peak / 8
	if headroom < 2 {
		headroom = 2
	}
	return peak + headroom
}

// PeakFraction is how much of a graph's height the data up to some point occupies, when the graph was drawn against the whole run's peak.
func PeakFraction(peakSoFar, peakOverall int) float64 {
	if peakOverall <= 0 {
		return 1
	}
	return min(1, max(0, float64(Ceiling(peakSoFar))/float64(Ceiling(peakOverall))))
}

// GraphImageWidth is the pixel width to render a graph of the given number of bars at.
func GraphImageWidth(bars int) int {
	return min(MaxGraphImageWidth, max(int(RealGraphWidth), bars))
}
