package helpers

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
)

const (
	RealGraphWidth  = 1000.0
	RealGraphHeight = 1000.0
	PhMaxHue        = 100.0
)

// PhValueColor maps a pH value to RGBA floats using the grid's pH color
// spectrum. Matches the env-layer colour logic in ux/grid.go: extremes
// get high-contrast colours, neutral pH is blended towards the active
// theme's background so it visually disappears into the window fill
// (black under dark, white under light, light-blue under light_blue).
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
