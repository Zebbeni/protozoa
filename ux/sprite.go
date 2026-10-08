package ux

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/utils"
)

// drawAnimatedSprite draws a sprite rotated to face `direction`, anchored to its base cell.
// spriteDownscaleFilter is the texture filter for a sprite drawn SMALLER
// than its source, which is Zoom8 drawing the 16x16 art at half size and
// nothing else (TestZoom8IsTheOnlyDownscale).
//
// Linear, chosen by looking at the three side by side: the 8x8 set has no
// overlay layers at all, so it draws a plain capsule, and nearest-neighbour
// at half scale drops alternate pixels and breaks the 16x16 art into
// speckle. Linear keeps the silhouette and the overlays legible. An UPSCALE
// stays nearest whatever this says, because linear on pixel art blurs it.
const spriteDownscaleFilter = ebiten.FilterLinear

// spriteFilter picks the filter for a draw at this scale.
func spriteFilter(scale float64) ebiten.Filter {
	if scale < 1 {
		return spriteDownscaleFilter
	}
	return ebiten.FilterNearest
}

func drawAnimatedSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, direction utils.Point, col colorful.Color, cellSize, scale float64) {
	if spriteImg == nil {
		return
	}
	b := spriteImg.Bounds()
	spriteH := float64(b.Dy())

	// Base cell sits at the BOTTOM cellSize x cellSize region for multi- cell (vertical-extending) sprites.
	anchorX := cellSize / 2
	anchorY := spriteH - cellSize/2

	// Compensate the final translate so the anchor (base-cell centre) lands at world (x + cellSize*scale/2, y + cellSize*scale/2) regardless of sprite height.
	compensateY := (cellSize/2 - anchorY) * scale

	op := &ebiten.DrawImageOptions{}
	op.Filter = spriteFilter(scale)
	op.GeoM.Translate(-anchorX, -anchorY)
	op.GeoM.Rotate(directionAngle(direction))
	op.GeoM.Translate(anchorX, anchorY)
	if scale != 1 {
		op.GeoM.Scale(scale, scale)
	}
	op.GeoM.Translate(x, y+compensateY)
	op.ColorScale.Scale(float32(col.R), float32(col.G), float32(col.B), 1)
	img.DrawImage(spriteImg, op)
}

// directionAngle converts a cardinal utils.Point direction into the rotation angle to apply to an up-facing (-Y) sprite.
func directionAngle(d utils.Point) float64 {
	if d.X == 0 && d.Y == 0 {
		return 0
	}
	return math.Atan2(float64(d.Y), float64(d.X)) + math.Pi/2
}
