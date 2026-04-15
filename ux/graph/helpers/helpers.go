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

// PhValueColor maps a pH value to RGBA floats using the grid's pH color spectrum.
func PhValueColor(ph float64) (float32, float32, float32, float32) {
	hue := PhMaxHue - (PhMaxHue * ph / c.MaxPh())
	sat := math.Abs(ph-((c.MaxPh()+c.MinPh())/2.0)) / (c.MaxPh() - c.MinPh())
	light := 0.5 + (0.5 * math.Sin(math.Pi*(sat-0.5)))
	col := colorful.HSLuv(hue, sat, light)
	return ColorToFloat(col)
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
